package cli

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/efs"
	efstypes "github.com/aws/aws-sdk-go-v2/service/efs/types"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	kmstypes "github.com/aws/aws-sdk-go-v2/service/kms/types"
	"github.com/aws/aws-sdk-go-v2/service/sagemaker"
	sagemakertypes "github.com/aws/aws-sdk-go-v2/service/sagemaker/types"
)

type mockMilestone5ECSRunner struct {
	calls []mockMilestone5CommandCall
}

type mockMilestone5CommandCall struct {
	binary string
	args   []string
	stdin  string
}

func (m *mockMilestone5ECSRunner) Run(_ context.Context, binary string, args []string, stdin string) (string, error) {
	m.calls = append(m.calls, mockMilestone5CommandCall{binary: binary, args: append([]string(nil), args...), stdin: stdin})
	if binary == "aws" {
		return "mock-password", nil
	}
	return "", nil
}

func TestMilestone5ECSPublishImageRunsPipeline(t *testing.T) {
	runner := &mockMilestone5ECSRunner{}
	oldRunner := ecsRunner
	ecsRunner = runner
	t.Cleanup(func() {
		ecsRunner = oldRunner
	})

	output, err := executeCommand(
		t,
		"--output", "json",
		"ecs", "publish-image",
		"--ecr-url", "123456789012.dkr.ecr.us-east-1.amazonaws.com/my-app",
		"--dockerfile", "Dockerfile",
		"--tag", "v1",
	)
	if err != nil {
		t.Fatalf("execute ecs publish-image: %v", err)
	}

	if len(runner.calls) != 5 {
		t.Fatalf("expected 5 command executions, got %d", len(runner.calls))
	}

	expected := []string{
		"aws ecr get-login-password",
		"docker login --username AWS --password-stdin 123456789012.dkr.ecr.us-east-1.amazonaws.com",
		"docker build -t awstbx-ecs-publish:v1 -f Dockerfile .",
		"docker tag awstbx-ecs-publish:v1 123456789012.dkr.ecr.us-east-1.amazonaws.com/my-app:v1",
		"docker push 123456789012.dkr.ecr.us-east-1.amazonaws.com/my-app:v1",
	}

	for i, want := range expected {
		got := runner.calls[i].binary + " " + strings.Join(runner.calls[i].args, " ")
		if got != want {
			t.Fatalf("unexpected command %d: got %q want %q", i, got, want)
		}
	}

	if runner.calls[1].stdin == "" {
		t.Fatal("expected docker login stdin to contain password")
	}
	if !strings.Contains(output, "\"step\": \"push\"") || !strings.Contains(output, "\"action\": \"completed\"") {
		t.Fatalf("unexpected output: %s", output)
	}
}

func TestMilestone5SageMakerCleanupSpacesNoConfirm(t *testing.T) {
	deleted := make([]string, 0)
	client := &mockMilestone5SageMakerClient{
		listDomainsFn: func(_ context.Context, _ *sagemaker.ListDomainsInput, _ ...func(*sagemaker.Options)) (*sagemaker.ListDomainsOutput, error) {
			return &sagemaker.ListDomainsOutput{Domains: []sagemakertypes.DomainDetails{{DomainId: ptr("d-123")}}}, nil
		},
		listSpacesFn: func(_ context.Context, in *sagemaker.ListSpacesInput, _ ...func(*sagemaker.Options)) (*sagemaker.ListSpacesOutput, error) {
			if pointerToString(in.DomainIdEquals) != "d-123" {
				t.Fatalf("unexpected domain id: %s", pointerToString(in.DomainIdEquals))
			}
			return &sagemaker.ListSpacesOutput{Spaces: []sagemakertypes.SpaceDetails{
				{SpaceName: ptr("space-a"), Status: sagemakertypes.SpaceStatusInService},
				{SpaceName: ptr("space-b"), Status: sagemakertypes.SpaceStatusPending},
			}}, nil
		},
		deleteSpaceFn: func(_ context.Context, in *sagemaker.DeleteSpaceInput, _ ...func(*sagemaker.Options)) (*sagemaker.DeleteSpaceOutput, error) {
			deleted = append(deleted, pointerToString(in.SpaceName))
			return &sagemaker.DeleteSpaceOutput{}, nil
		},
	}

	withMockMilestone5SageMakerDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) sageMakerAPI { return client },
	)

	output, err := executeCommand(t, "--output", "json", "--no-confirm", "sagemaker", "cleanup-spaces")
	if err != nil {
		t.Fatalf("execute sagemaker cleanup-spaces --no-confirm: %v", err)
	}

	if len(deleted) != 2 {
		t.Fatalf("expected 2 spaces deleted, got %d", len(deleted))
	}
	if !strings.Contains(output, "\"action\": \"deleted\"") {
		t.Fatalf("unexpected output: %s", output)
	}
}

func TestMilestone5SageMakerDeleteUserProfileWaitsForDependencies(t *testing.T) {
	listAppsCalls := 0
	describeSpaceCalls := 0
	deleteAppCalls := 0
	deleteSpaceCalls := 0
	deleteUserProfileCalls := 0

	client := &mockMilestone5SageMakerClient{
		listAppsFn: func(_ context.Context, _ *sagemaker.ListAppsInput, _ ...func(*sagemaker.Options)) (*sagemaker.ListAppsOutput, error) {
			listAppsCalls++
			switch listAppsCalls {
			case 1:
				return &sagemaker.ListAppsOutput{Apps: []sagemakertypes.AppDetails{{AppName: ptr("app-a"), AppType: sagemakertypes.AppTypeJupyterServer, Status: sagemakertypes.AppStatusInService}}}, nil
			case 2:
				return &sagemaker.ListAppsOutput{Apps: []sagemakertypes.AppDetails{{AppName: ptr("app-a"), AppType: sagemakertypes.AppTypeJupyterServer, Status: sagemakertypes.AppStatusDeleting}}}, nil
			default:
				return &sagemaker.ListAppsOutput{Apps: []sagemakertypes.AppDetails{{AppName: ptr("app-a"), AppType: sagemakertypes.AppTypeJupyterServer, Status: sagemakertypes.AppStatusDeleted}}}, nil
			}
		},
		listSpacesFn: func(_ context.Context, _ *sagemaker.ListSpacesInput, _ ...func(*sagemaker.Options)) (*sagemaker.ListSpacesOutput, error) {
			return &sagemaker.ListSpacesOutput{Spaces: []sagemakertypes.SpaceDetails{{SpaceName: ptr("space-a"), Status: sagemakertypes.SpaceStatusInService}}}, nil
		},
		describeSpaceFn: func(_ context.Context, _ *sagemaker.DescribeSpaceInput, _ ...func(*sagemaker.Options)) (*sagemaker.DescribeSpaceOutput, error) {
			describeSpaceCalls++
			status := sagemakertypes.SpaceStatus("Deleted")
			switch describeSpaceCalls {
			case 1:
				status = sagemakertypes.SpaceStatusInService
			case 2:
				status = sagemakertypes.SpaceStatusDeleting
			}
			return &sagemaker.DescribeSpaceOutput{
				Status: status,
				OwnershipSettings: &sagemakertypes.OwnershipSettings{
					OwnerUserProfileName: ptr("alice"),
				},
			}, nil
		},
		deleteAppFn: func(_ context.Context, _ *sagemaker.DeleteAppInput, _ ...func(*sagemaker.Options)) (*sagemaker.DeleteAppOutput, error) {
			deleteAppCalls++
			return &sagemaker.DeleteAppOutput{}, nil
		},
		deleteSpaceFn: func(_ context.Context, _ *sagemaker.DeleteSpaceInput, _ ...func(*sagemaker.Options)) (*sagemaker.DeleteSpaceOutput, error) {
			deleteSpaceCalls++
			return &sagemaker.DeleteSpaceOutput{}, nil
		},
		deleteUserProfileFn: func(_ context.Context, _ *sagemaker.DeleteUserProfileInput, _ ...func(*sagemaker.Options)) (*sagemaker.DeleteUserProfileOutput, error) {
			if listAppsCalls < 3 || describeSpaceCalls < 3 {
				t.Fatalf("expected dependency polling before deleting profile, listApps=%d describeSpace=%d", listAppsCalls, describeSpaceCalls)
			}
			deleteUserProfileCalls++
			return &sagemaker.DeleteUserProfileOutput{}, nil
		},
	}

	withMockMilestone5SageMakerDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) sageMakerAPI { return client },
	)

	oldSleep := sageMakerSleep
	sageMakerSleep = func(_ time.Duration) {}
	t.Cleanup(func() {
		sageMakerSleep = oldSleep
	})

	output, err := executeCommand(t, "--output", "json", "--no-confirm", "sagemaker", "delete-user-profile", "--domain-id", "d-123", "--user-profile", "alice")
	if err != nil {
		t.Fatalf("execute sagemaker delete-user-profile --no-confirm: %v", err)
	}

	if deleteAppCalls != 1 || deleteSpaceCalls != 1 || deleteUserProfileCalls != 1 {
		t.Fatalf("unexpected delete call counts app=%d space=%d profile=%d", deleteAppCalls, deleteSpaceCalls, deleteUserProfileCalls)
	}
	if !strings.Contains(output, "\"step\": \"user-profile\"") || !strings.Contains(output, "\"action\": \"deleted\"") {
		t.Fatalf("unexpected output: %s", output)
	}
}

func TestMilestone5SageMakerDeleteUserProfileRequiredFlags(t *testing.T) {
	if _, err := executeCommand(t, "sagemaker", "delete-user-profile"); err == nil || !strings.Contains(err.Error(), "--domain-id is required") {
		t.Fatalf("expected required flag validation error, got %v", err)
	}
}

func TestMilestone5KMSDeleteKeysDryRun(t *testing.T) {
	scheduled := 0
	client := &mockMilestone5KMSClient{
		listKeysFn: func(_ context.Context, _ *kms.ListKeysInput, _ ...func(*kms.Options)) (*kms.ListKeysOutput, error) {
			return &kms.ListKeysOutput{Keys: []kmstypes.KeyListEntry{{KeyId: ptr("key-1")}}}, nil
		},
		describeKeyFn: func(_ context.Context, in *kms.DescribeKeyInput, _ ...func(*kms.Options)) (*kms.DescribeKeyOutput, error) {
			if pointerToString(in.KeyId) != "key-1" {
				t.Fatalf("unexpected key id: %s", pointerToString(in.KeyId))
			}
			return &kms.DescribeKeyOutput{KeyMetadata: &kmstypes.KeyMetadata{KeyId: ptr("key-1"), KeyManager: kmstypes.KeyManagerTypeCustomer, KeyState: kmstypes.KeyStateDisabled}}, nil
		},
		scheduleKeyDeletionFn: func(_ context.Context, _ *kms.ScheduleKeyDeletionInput, _ ...func(*kms.Options)) (*kms.ScheduleKeyDeletionOutput, error) {
			scheduled++
			return &kms.ScheduleKeyDeletionOutput{}, nil
		},
	}

	withMockMilestone5KMSDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) kmsAPI { return client },
	)

	output, err := executeCommand(t, "--output", "json", "--dry-run", "kms", "delete-keys", "--unused")
	if err != nil {
		t.Fatalf("execute kms delete-keys --unused --dry-run: %v", err)
	}

	if scheduled != 0 {
		t.Fatalf("expected 0 scheduled deletions in dry-run, got %d", scheduled)
	}
	if !strings.Contains(output, "would-schedule-deletion") || !strings.Contains(output, "key-1") {
		t.Fatalf("unexpected output: %s", output)
	}
}

func TestMilestone5ECSPublishImageRequiresECRURL(t *testing.T) {
	if _, err := executeCommand(t, "ecs", "publish-image"); err == nil || !strings.Contains(err.Error(), "--ecr-url is required") {
		t.Fatalf("expected required flag validation error, got %v", err)
	}
}

func TestMilestone5EFSDeleteFilesystemsWaitsForMountTargets(t *testing.T) {
	describeMountTargetsCalls := 0
	deleteMountTargetCalls := 0
	deleteFileSystemCalls := 0

	client := &mockMilestone5EFSClient{
		describeFileSystemsFn: func(_ context.Context, _ *efs.DescribeFileSystemsInput, _ ...func(*efs.Options)) (*efs.DescribeFileSystemsOutput, error) {
			return &efs.DescribeFileSystemsOutput{
				FileSystems: []efstypes.FileSystemDescription{
					{FileSystemId: ptr("fs-123")},
				},
			}, nil
		},
		describeMountTargetsFn: func(_ context.Context, _ *efs.DescribeMountTargetsInput, _ ...func(*efs.Options)) (*efs.DescribeMountTargetsOutput, error) {
			describeMountTargetsCalls++
			if describeMountTargetsCalls < 3 {
				return &efs.DescribeMountTargetsOutput{
					MountTargets: []efstypes.MountTargetDescription{{MountTargetId: ptr("mt-1")}},
				}, nil
			}
			return &efs.DescribeMountTargetsOutput{}, nil
		},
		deleteMountTargetFn: func(_ context.Context, _ *efs.DeleteMountTargetInput, _ ...func(*efs.Options)) (*efs.DeleteMountTargetOutput, error) {
			deleteMountTargetCalls++
			return &efs.DeleteMountTargetOutput{}, nil
		},
		deleteFileSystemFn: func(_ context.Context, _ *efs.DeleteFileSystemInput, _ ...func(*efs.Options)) (*efs.DeleteFileSystemOutput, error) {
			deleteFileSystemCalls++
			return &efs.DeleteFileSystemOutput{}, nil
		},
	}

	withMockMilestone5EFSDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) efsAPI { return client },
	)

	oldSleep := efsSleep
	efsSleep = func(_ time.Duration) {}
	t.Cleanup(func() {
		efsSleep = oldSleep
	})

	output, err := executeCommand(t, "--output", "json", "--no-confirm", "efs", "delete-filesystems")
	if err != nil {
		t.Fatalf("execute efs delete-filesystems --no-confirm: %v", err)
	}

	if deleteMountTargetCalls != 1 || deleteFileSystemCalls != 1 {
		t.Fatalf("unexpected delete call counts mount-target=%d file-system=%d", deleteMountTargetCalls, deleteFileSystemCalls)
	}
	if describeMountTargetsCalls < 3 {
		t.Fatalf("expected mount-target polling before deleting file system, describe calls=%d", describeMountTargetsCalls)
	}
	if !strings.Contains(output, "\"action\": \"deleted\"") {
		t.Fatalf("unexpected output: %s", output)
	}
}

type mockMilestone5SageMakerClient struct {
	deleteAppFn         func(context.Context, *sagemaker.DeleteAppInput, ...func(*sagemaker.Options)) (*sagemaker.DeleteAppOutput, error)
	deleteSpaceFn       func(context.Context, *sagemaker.DeleteSpaceInput, ...func(*sagemaker.Options)) (*sagemaker.DeleteSpaceOutput, error)
	deleteUserProfileFn func(context.Context, *sagemaker.DeleteUserProfileInput, ...func(*sagemaker.Options)) (*sagemaker.DeleteUserProfileOutput, error)
	describeSpaceFn     func(context.Context, *sagemaker.DescribeSpaceInput, ...func(*sagemaker.Options)) (*sagemaker.DescribeSpaceOutput, error)
	listAppsFn          func(context.Context, *sagemaker.ListAppsInput, ...func(*sagemaker.Options)) (*sagemaker.ListAppsOutput, error)
	listDomainsFn       func(context.Context, *sagemaker.ListDomainsInput, ...func(*sagemaker.Options)) (*sagemaker.ListDomainsOutput, error)
	listSpacesFn        func(context.Context, *sagemaker.ListSpacesInput, ...func(*sagemaker.Options)) (*sagemaker.ListSpacesOutput, error)
}

type mockMilestone5EFSClient struct {
	deleteFileSystemFn     func(context.Context, *efs.DeleteFileSystemInput, ...func(*efs.Options)) (*efs.DeleteFileSystemOutput, error)
	deleteMountTargetFn    func(context.Context, *efs.DeleteMountTargetInput, ...func(*efs.Options)) (*efs.DeleteMountTargetOutput, error)
	describeFileSystemsFn  func(context.Context, *efs.DescribeFileSystemsInput, ...func(*efs.Options)) (*efs.DescribeFileSystemsOutput, error)
	describeMountTargetsFn func(context.Context, *efs.DescribeMountTargetsInput, ...func(*efs.Options)) (*efs.DescribeMountTargetsOutput, error)
	listTagsForResourceFn  func(context.Context, *efs.ListTagsForResourceInput, ...func(*efs.Options)) (*efs.ListTagsForResourceOutput, error)
}

func (m *mockMilestone5EFSClient) DeleteFileSystem(ctx context.Context, in *efs.DeleteFileSystemInput, optFns ...func(*efs.Options)) (*efs.DeleteFileSystemOutput, error) {
	if m.deleteFileSystemFn == nil {
		return nil, errors.New("DeleteFileSystem not mocked")
	}
	return m.deleteFileSystemFn(ctx, in, optFns...)
}

func (m *mockMilestone5EFSClient) DeleteMountTarget(ctx context.Context, in *efs.DeleteMountTargetInput, optFns ...func(*efs.Options)) (*efs.DeleteMountTargetOutput, error) {
	if m.deleteMountTargetFn == nil {
		return nil, errors.New("DeleteMountTarget not mocked")
	}
	return m.deleteMountTargetFn(ctx, in, optFns...)
}

func (m *mockMilestone5EFSClient) DescribeFileSystems(ctx context.Context, in *efs.DescribeFileSystemsInput, optFns ...func(*efs.Options)) (*efs.DescribeFileSystemsOutput, error) {
	if m.describeFileSystemsFn == nil {
		return nil, errors.New("DescribeFileSystems not mocked")
	}
	return m.describeFileSystemsFn(ctx, in, optFns...)
}

func (m *mockMilestone5EFSClient) DescribeMountTargets(ctx context.Context, in *efs.DescribeMountTargetsInput, optFns ...func(*efs.Options)) (*efs.DescribeMountTargetsOutput, error) {
	if m.describeMountTargetsFn == nil {
		return nil, errors.New("DescribeMountTargets not mocked")
	}
	return m.describeMountTargetsFn(ctx, in, optFns...)
}

func (m *mockMilestone5EFSClient) ListTagsForResource(ctx context.Context, in *efs.ListTagsForResourceInput, optFns ...func(*efs.Options)) (*efs.ListTagsForResourceOutput, error) {
	if m.listTagsForResourceFn == nil {
		return &efs.ListTagsForResourceOutput{}, nil
	}
	return m.listTagsForResourceFn(ctx, in, optFns...)
}

func (m *mockMilestone5SageMakerClient) DeleteApp(ctx context.Context, in *sagemaker.DeleteAppInput, optFns ...func(*sagemaker.Options)) (*sagemaker.DeleteAppOutput, error) {
	if m.deleteAppFn == nil {
		return nil, errors.New("DeleteApp not mocked")
	}
	return m.deleteAppFn(ctx, in, optFns...)
}

func (m *mockMilestone5SageMakerClient) DeleteSpace(ctx context.Context, in *sagemaker.DeleteSpaceInput, optFns ...func(*sagemaker.Options)) (*sagemaker.DeleteSpaceOutput, error) {
	if m.deleteSpaceFn == nil {
		return nil, errors.New("DeleteSpace not mocked")
	}
	return m.deleteSpaceFn(ctx, in, optFns...)
}

func (m *mockMilestone5SageMakerClient) DeleteUserProfile(ctx context.Context, in *sagemaker.DeleteUserProfileInput, optFns ...func(*sagemaker.Options)) (*sagemaker.DeleteUserProfileOutput, error) {
	if m.deleteUserProfileFn == nil {
		return nil, errors.New("DeleteUserProfile not mocked")
	}
	return m.deleteUserProfileFn(ctx, in, optFns...)
}

func (m *mockMilestone5SageMakerClient) DescribeSpace(ctx context.Context, in *sagemaker.DescribeSpaceInput, optFns ...func(*sagemaker.Options)) (*sagemaker.DescribeSpaceOutput, error) {
	if m.describeSpaceFn == nil {
		return nil, errors.New("DescribeSpace not mocked")
	}
	return m.describeSpaceFn(ctx, in, optFns...)
}

func (m *mockMilestone5SageMakerClient) ListApps(ctx context.Context, in *sagemaker.ListAppsInput, optFns ...func(*sagemaker.Options)) (*sagemaker.ListAppsOutput, error) {
	if m.listAppsFn == nil {
		return nil, errors.New("ListApps not mocked")
	}
	return m.listAppsFn(ctx, in, optFns...)
}

func (m *mockMilestone5SageMakerClient) ListDomains(ctx context.Context, in *sagemaker.ListDomainsInput, optFns ...func(*sagemaker.Options)) (*sagemaker.ListDomainsOutput, error) {
	if m.listDomainsFn == nil {
		return nil, errors.New("ListDomains not mocked")
	}
	return m.listDomainsFn(ctx, in, optFns...)
}

func (m *mockMilestone5SageMakerClient) ListSpaces(ctx context.Context, in *sagemaker.ListSpacesInput, optFns ...func(*sagemaker.Options)) (*sagemaker.ListSpacesOutput, error) {
	if m.listSpacesFn == nil {
		return nil, errors.New("ListSpaces not mocked")
	}
	return m.listSpacesFn(ctx, in, optFns...)
}

func withMockMilestone5SageMakerDeps(t *testing.T, loader func(string, string) (awssdk.Config, error), newClient func(awssdk.Config) sageMakerAPI) {
	t.Helper()

	oldLoader := sageMakerLoadAWSConfig
	oldNewClient := sageMakerNewClient

	sageMakerLoadAWSConfig = loader
	sageMakerNewClient = newClient

	t.Cleanup(func() {
		sageMakerLoadAWSConfig = oldLoader
		sageMakerNewClient = oldNewClient
	})
}

func withMockMilestone5EFSDeps(t *testing.T, loader func(string, string) (awssdk.Config, error), newClient func(awssdk.Config) efsAPI) {
	t.Helper()

	oldLoader := efsLoadAWSConfig
	oldNewClient := efsNewClient

	efsLoadAWSConfig = loader
	efsNewClient = newClient

	t.Cleanup(func() {
		efsLoadAWSConfig = oldLoader
		efsNewClient = oldNewClient
	})
}

type mockMilestone5KMSClient struct {
	describeKeyFn         func(context.Context, *kms.DescribeKeyInput, ...func(*kms.Options)) (*kms.DescribeKeyOutput, error)
	listKeysFn            func(context.Context, *kms.ListKeysInput, ...func(*kms.Options)) (*kms.ListKeysOutput, error)
	listResourceTagsFn    func(context.Context, *kms.ListResourceTagsInput, ...func(*kms.Options)) (*kms.ListResourceTagsOutput, error)
	scheduleKeyDeletionFn func(context.Context, *kms.ScheduleKeyDeletionInput, ...func(*kms.Options)) (*kms.ScheduleKeyDeletionOutput, error)
}

func (m *mockMilestone5KMSClient) DescribeKey(ctx context.Context, in *kms.DescribeKeyInput, optFns ...func(*kms.Options)) (*kms.DescribeKeyOutput, error) {
	if m.describeKeyFn == nil {
		return nil, errors.New("DescribeKey not mocked")
	}
	return m.describeKeyFn(ctx, in, optFns...)
}

func (m *mockMilestone5KMSClient) ListKeys(ctx context.Context, in *kms.ListKeysInput, optFns ...func(*kms.Options)) (*kms.ListKeysOutput, error) {
	if m.listKeysFn == nil {
		return nil, errors.New("ListKeys not mocked")
	}
	return m.listKeysFn(ctx, in, optFns...)
}

func (m *mockMilestone5KMSClient) ListResourceTags(ctx context.Context, in *kms.ListResourceTagsInput, optFns ...func(*kms.Options)) (*kms.ListResourceTagsOutput, error) {
	if m.listResourceTagsFn == nil {
		return nil, errors.New("ListResourceTags not mocked")
	}
	return m.listResourceTagsFn(ctx, in, optFns...)
}

func (m *mockMilestone5KMSClient) ScheduleKeyDeletion(ctx context.Context, in *kms.ScheduleKeyDeletionInput, optFns ...func(*kms.Options)) (*kms.ScheduleKeyDeletionOutput, error) {
	if m.scheduleKeyDeletionFn == nil {
		return nil, errors.New("ScheduleKeyDeletion not mocked")
	}
	return m.scheduleKeyDeletionFn(ctx, in, optFns...)
}

func withMockMilestone5KMSDeps(t *testing.T, loader func(string, string) (awssdk.Config, error), newClient func(awssdk.Config) kmsAPI) {
	t.Helper()

	oldLoader := kmsLoadAWSConfig
	oldNewClient := kmsNewClient

	kmsLoadAWSConfig = loader
	kmsNewClient = newClient

	t.Cleanup(func() {
		kmsLoadAWSConfig = oldLoader
		kmsNewClient = oldNewClient
	})
}
