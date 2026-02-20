package sagemaker

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sagemaker"
	sagemakertypes "github.com/aws/aws-sdk-go-v2/service/sagemaker/types"
	"github.com/towardsthecloud/aws-toolbox/internal/cliutil"
)

type mockClient struct {
	deleteAppFn         func(context.Context, *sagemaker.DeleteAppInput, ...func(*sagemaker.Options)) (*sagemaker.DeleteAppOutput, error)
	deleteSpaceFn       func(context.Context, *sagemaker.DeleteSpaceInput, ...func(*sagemaker.Options)) (*sagemaker.DeleteSpaceOutput, error)
	deleteUserProfileFn func(context.Context, *sagemaker.DeleteUserProfileInput, ...func(*sagemaker.Options)) (*sagemaker.DeleteUserProfileOutput, error)
	describeSpaceFn     func(context.Context, *sagemaker.DescribeSpaceInput, ...func(*sagemaker.Options)) (*sagemaker.DescribeSpaceOutput, error)
	listAppsFn          func(context.Context, *sagemaker.ListAppsInput, ...func(*sagemaker.Options)) (*sagemaker.ListAppsOutput, error)
	listDomainsFn       func(context.Context, *sagemaker.ListDomainsInput, ...func(*sagemaker.Options)) (*sagemaker.ListDomainsOutput, error)
	listSpacesFn        func(context.Context, *sagemaker.ListSpacesInput, ...func(*sagemaker.Options)) (*sagemaker.ListSpacesOutput, error)
}

func (m *mockClient) DeleteApp(ctx context.Context, in *sagemaker.DeleteAppInput, optFns ...func(*sagemaker.Options)) (*sagemaker.DeleteAppOutput, error) {
	if m.deleteAppFn == nil {
		return nil, errors.New("DeleteApp not mocked")
	}
	return m.deleteAppFn(ctx, in, optFns...)
}

func (m *mockClient) DeleteSpace(ctx context.Context, in *sagemaker.DeleteSpaceInput, optFns ...func(*sagemaker.Options)) (*sagemaker.DeleteSpaceOutput, error) {
	if m.deleteSpaceFn == nil {
		return nil, errors.New("DeleteSpace not mocked")
	}
	return m.deleteSpaceFn(ctx, in, optFns...)
}

func (m *mockClient) DeleteUserProfile(ctx context.Context, in *sagemaker.DeleteUserProfileInput, optFns ...func(*sagemaker.Options)) (*sagemaker.DeleteUserProfileOutput, error) {
	if m.deleteUserProfileFn == nil {
		return nil, errors.New("DeleteUserProfile not mocked")
	}
	return m.deleteUserProfileFn(ctx, in, optFns...)
}

func (m *mockClient) DescribeSpace(ctx context.Context, in *sagemaker.DescribeSpaceInput, optFns ...func(*sagemaker.Options)) (*sagemaker.DescribeSpaceOutput, error) {
	if m.describeSpaceFn == nil {
		return nil, errors.New("DescribeSpace not mocked")
	}
	return m.describeSpaceFn(ctx, in, optFns...)
}

func (m *mockClient) ListApps(ctx context.Context, in *sagemaker.ListAppsInput, optFns ...func(*sagemaker.Options)) (*sagemaker.ListAppsOutput, error) {
	if m.listAppsFn == nil {
		return nil, errors.New("ListApps not mocked")
	}
	return m.listAppsFn(ctx, in, optFns...)
}

func (m *mockClient) ListDomains(ctx context.Context, in *sagemaker.ListDomainsInput, optFns ...func(*sagemaker.Options)) (*sagemaker.ListDomainsOutput, error) {
	if m.listDomainsFn == nil {
		return nil, errors.New("ListDomains not mocked")
	}
	return m.listDomainsFn(ctx, in, optFns...)
}

func (m *mockClient) ListSpaces(ctx context.Context, in *sagemaker.ListSpacesInput, optFns ...func(*sagemaker.Options)) (*sagemaker.ListSpacesOutput, error) {
	if m.listSpacesFn == nil {
		return nil, errors.New("ListSpaces not mocked")
	}
	return m.listSpacesFn(ctx, in, optFns...)
}

func withMockDeps(t *testing.T, loader func(string, string) (awssdk.Config, error), nc func(awssdk.Config) API) {
	t.Helper()

	oldLoader := loadAWSConfig
	oldNewClient := newClient

	loadAWSConfig = loader
	newClient = nc

	t.Cleanup(func() {
		loadAWSConfig = oldLoader
		newClient = oldNewClient
	})
}

func executeCommand(t *testing.T, args ...string) (string, error) {
	t.Helper()

	root := cliutil.NewTestRootCommand(NewCommand())
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs(args)

	err := root.Execute()
	return buf.String(), err
}

func TestCleanupSpacesNoConfirm(t *testing.T) {
	deleted := make([]string, 0)
	client := &mockClient{
		listDomainsFn: func(_ context.Context, _ *sagemaker.ListDomainsInput, _ ...func(*sagemaker.Options)) (*sagemaker.ListDomainsOutput, error) {
			return &sagemaker.ListDomainsOutput{Domains: []sagemakertypes.DomainDetails{{DomainId: cliutil.Ptr("d-123")}}}, nil
		},
		listSpacesFn: func(_ context.Context, in *sagemaker.ListSpacesInput, _ ...func(*sagemaker.Options)) (*sagemaker.ListSpacesOutput, error) {
			if cliutil.PointerToString(in.DomainIdEquals) != "d-123" {
				t.Fatalf("unexpected domain id: %s", cliutil.PointerToString(in.DomainIdEquals))
			}
			return &sagemaker.ListSpacesOutput{Spaces: []sagemakertypes.SpaceDetails{
				{SpaceName: cliutil.Ptr("space-a"), Status: sagemakertypes.SpaceStatusInService},
				{SpaceName: cliutil.Ptr("space-b"), Status: sagemakertypes.SpaceStatusPending},
			}}, nil
		},
		deleteSpaceFn: func(_ context.Context, in *sagemaker.DeleteSpaceInput, _ ...func(*sagemaker.Options)) (*sagemaker.DeleteSpaceOutput, error) {
			deleted = append(deleted, cliutil.PointerToString(in.SpaceName))
			return &sagemaker.DeleteSpaceOutput{}, nil
		},
	}

	withMockDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) API { return client },
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

func TestDeleteUserProfileWaitsForDependencies(t *testing.T) {
	listAppsCalls := 0
	describeSpaceCalls := 0
	deleteAppCalls := 0
	deleteSpaceCalls := 0
	deleteUserProfileCalls := 0

	client := &mockClient{
		listAppsFn: func(_ context.Context, _ *sagemaker.ListAppsInput, _ ...func(*sagemaker.Options)) (*sagemaker.ListAppsOutput, error) {
			listAppsCalls++
			switch listAppsCalls {
			case 1:
				return &sagemaker.ListAppsOutput{Apps: []sagemakertypes.AppDetails{{AppName: cliutil.Ptr("app-a"), AppType: sagemakertypes.AppTypeJupyterServer, Status: sagemakertypes.AppStatusInService}}}, nil
			case 2:
				return &sagemaker.ListAppsOutput{Apps: []sagemakertypes.AppDetails{{AppName: cliutil.Ptr("app-a"), AppType: sagemakertypes.AppTypeJupyterServer, Status: sagemakertypes.AppStatusDeleting}}}, nil
			default:
				return &sagemaker.ListAppsOutput{Apps: []sagemakertypes.AppDetails{{AppName: cliutil.Ptr("app-a"), AppType: sagemakertypes.AppTypeJupyterServer, Status: sagemakertypes.AppStatusDeleted}}}, nil
			}
		},
		listSpacesFn: func(_ context.Context, _ *sagemaker.ListSpacesInput, _ ...func(*sagemaker.Options)) (*sagemaker.ListSpacesOutput, error) {
			return &sagemaker.ListSpacesOutput{Spaces: []sagemakertypes.SpaceDetails{{SpaceName: cliutil.Ptr("space-a"), Status: sagemakertypes.SpaceStatusInService}}}, nil
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
					OwnerUserProfileName: cliutil.Ptr("alice"),
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

	withMockDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) API { return client },
	)

	oldSleep := sleep
	sleep = func(_ time.Duration) {}
	t.Cleanup(func() {
		sleep = oldSleep
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

func TestDeleteUserProfileRequiredFlags(t *testing.T) {
	if _, err := executeCommand(t, "sagemaker", "delete-user-profile"); err == nil || !strings.Contains(err.Error(), "--domain-id is required") {
		t.Fatalf("expected required flag validation error, got %v", err)
	}
}

func TestDeleteUserProfileMissingUserProfile(t *testing.T) {
	_, err := executeCommand(t, "sagemaker", "delete-user-profile", "--domain-id", "d-123")
	if err == nil || !strings.Contains(err.Error(), "--user-profile is required") {
		t.Fatalf("expected --user-profile required error, got %v", err)
	}
}

func TestCleanupSpacesSpacesWithoutDomainID(t *testing.T) {
	_, err := executeCommand(t, "sagemaker", "cleanup-spaces", "--spaces", "space-a")
	if err == nil || !strings.Contains(err.Error(), "--domain-id is required when --spaces is set") {
		t.Fatalf("expected domain-id required error, got %v", err)
	}
}

func TestCleanupSpacesDryRun(t *testing.T) {
	client := &mockClient{
		listDomainsFn: func(_ context.Context, _ *sagemaker.ListDomainsInput, _ ...func(*sagemaker.Options)) (*sagemaker.ListDomainsOutput, error) {
			return &sagemaker.ListDomainsOutput{Domains: []sagemakertypes.DomainDetails{{DomainId: cliutil.Ptr("d-123")}}}, nil
		},
		listSpacesFn: func(_ context.Context, _ *sagemaker.ListSpacesInput, _ ...func(*sagemaker.Options)) (*sagemaker.ListSpacesOutput, error) {
			return &sagemaker.ListSpacesOutput{Spaces: []sagemakertypes.SpaceDetails{
				{SpaceName: cliutil.Ptr("space-a"), Status: sagemakertypes.SpaceStatusInService},
			}}, nil
		},
	}

	withMockDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) API { return client },
	)

	output, err := executeCommand(t, "--output", "json", "--dry-run", "sagemaker", "cleanup-spaces")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, "\"action\": \"would-delete\"") {
		t.Fatalf("expected would-delete action in dry-run output, got: %s", output)
	}
}

func TestCleanupSpacesEmptyResult(t *testing.T) {
	client := &mockClient{
		listDomainsFn: func(_ context.Context, _ *sagemaker.ListDomainsInput, _ ...func(*sagemaker.Options)) (*sagemaker.ListDomainsOutput, error) {
			return &sagemaker.ListDomainsOutput{Domains: []sagemakertypes.DomainDetails{{DomainId: cliutil.Ptr("d-123")}}}, nil
		},
		listSpacesFn: func(_ context.Context, _ *sagemaker.ListSpacesInput, _ ...func(*sagemaker.Options)) (*sagemaker.ListSpacesOutput, error) {
			return &sagemaker.ListSpacesOutput{Spaces: []sagemakertypes.SpaceDetails{}}, nil
		},
	}

	withMockDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) API { return client },
	)

	_, err := executeCommand(t, "--output", "json", "--no-confirm", "sagemaker", "cleanup-spaces")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCleanupSpacesWithSpecificDomainAndSpacesFilter(t *testing.T) {
	deleted := make([]string, 0)
	client := &mockClient{
		listSpacesFn: func(_ context.Context, in *sagemaker.ListSpacesInput, _ ...func(*sagemaker.Options)) (*sagemaker.ListSpacesOutput, error) {
			if cliutil.PointerToString(in.DomainIdEquals) != "d-456" {
				t.Fatalf("unexpected domain id: %s", cliutil.PointerToString(in.DomainIdEquals))
			}
			return &sagemaker.ListSpacesOutput{Spaces: []sagemakertypes.SpaceDetails{
				{SpaceName: cliutil.Ptr("space-a"), Status: sagemakertypes.SpaceStatusInService},
				{SpaceName: cliutil.Ptr("space-b"), Status: sagemakertypes.SpaceStatusInService},
				{SpaceName: cliutil.Ptr("space-c"), Status: sagemakertypes.SpaceStatusInService},
			}}, nil
		},
		deleteSpaceFn: func(_ context.Context, in *sagemaker.DeleteSpaceInput, _ ...func(*sagemaker.Options)) (*sagemaker.DeleteSpaceOutput, error) {
			deleted = append(deleted, cliutil.PointerToString(in.SpaceName))
			return &sagemaker.DeleteSpaceOutput{}, nil
		},
	}

	withMockDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) API { return client },
	)

	output, err := executeCommand(t, "--output", "json", "--no-confirm", "sagemaker", "cleanup-spaces", "--domain-id", "d-456", "--spaces", "space-a,space-c")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(deleted) != 2 {
		t.Fatalf("expected 2 spaces deleted, got %d: %v", len(deleted), deleted)
	}
	if !strings.Contains(output, "space-a") || !strings.Contains(output, "space-c") {
		t.Fatalf("expected space-a and space-c in output: %s", output)
	}
	if strings.Contains(output, "space-b") {
		t.Fatalf("space-b should be filtered out: %s", output)
	}
}

func TestCleanupSpacesSkipsDeletingAndDeleteFailed(t *testing.T) {
	client := &mockClient{
		listDomainsFn: func(_ context.Context, _ *sagemaker.ListDomainsInput, _ ...func(*sagemaker.Options)) (*sagemaker.ListDomainsOutput, error) {
			return &sagemaker.ListDomainsOutput{Domains: []sagemakertypes.DomainDetails{{DomainId: cliutil.Ptr("d-123")}}}, nil
		},
		listSpacesFn: func(_ context.Context, _ *sagemaker.ListSpacesInput, _ ...func(*sagemaker.Options)) (*sagemaker.ListSpacesOutput, error) {
			return &sagemaker.ListSpacesOutput{Spaces: []sagemakertypes.SpaceDetails{
				{SpaceName: cliutil.Ptr("space-deleting"), Status: sagemakertypes.SpaceStatusDeleting},
				{SpaceName: cliutil.Ptr("space-failed"), Status: sagemakertypes.SpaceStatus("Delete_Failed")},
				{SpaceName: cliutil.Ptr("space-ok"), Status: sagemakertypes.SpaceStatusInService},
			}}, nil
		},
		deleteSpaceFn: func(_ context.Context, _ *sagemaker.DeleteSpaceInput, _ ...func(*sagemaker.Options)) (*sagemaker.DeleteSpaceOutput, error) {
			return &sagemaker.DeleteSpaceOutput{}, nil
		},
	}

	withMockDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) API { return client },
	)

	output, err := executeCommand(t, "--output", "json", "--no-confirm", "sagemaker", "cleanup-spaces")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(output, "space-deleting") || strings.Contains(output, "space-failed") {
		t.Fatalf("deleting/delete_failed spaces should be skipped: %s", output)
	}
	if !strings.Contains(output, "space-ok") {
		t.Fatalf("expected space-ok in output: %s", output)
	}
}

func TestCleanupSpacesSkipsNilSpaceName(t *testing.T) {
	client := &mockClient{
		listDomainsFn: func(_ context.Context, _ *sagemaker.ListDomainsInput, _ ...func(*sagemaker.Options)) (*sagemaker.ListDomainsOutput, error) {
			return &sagemaker.ListDomainsOutput{Domains: []sagemakertypes.DomainDetails{{DomainId: cliutil.Ptr("d-123")}}}, nil
		},
		listSpacesFn: func(_ context.Context, _ *sagemaker.ListSpacesInput, _ ...func(*sagemaker.Options)) (*sagemaker.ListSpacesOutput, error) {
			return &sagemaker.ListSpacesOutput{Spaces: []sagemakertypes.SpaceDetails{
				{SpaceName: nil, Status: sagemakertypes.SpaceStatusInService},
			}}, nil
		},
	}

	withMockDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) API { return client },
	)

	_, err := executeCommand(t, "--output", "json", "--no-confirm", "sagemaker", "cleanup-spaces")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCleanupSpacesListDomainsError(t *testing.T) {
	client := &mockClient{
		listDomainsFn: func(_ context.Context, _ *sagemaker.ListDomainsInput, _ ...func(*sagemaker.Options)) (*sagemaker.ListDomainsOutput, error) {
			return nil, errors.New("access denied")
		},
	}

	withMockDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) API { return client },
	)

	_, err := executeCommand(t, "--output", "json", "--no-confirm", "sagemaker", "cleanup-spaces")
	if err == nil || !strings.Contains(err.Error(), "list SageMaker domains") {
		t.Fatalf("expected list domains error, got: %v", err)
	}
}

func TestCleanupSpacesListSpacesError(t *testing.T) {
	client := &mockClient{
		listDomainsFn: func(_ context.Context, _ *sagemaker.ListDomainsInput, _ ...func(*sagemaker.Options)) (*sagemaker.ListDomainsOutput, error) {
			return &sagemaker.ListDomainsOutput{Domains: []sagemakertypes.DomainDetails{{DomainId: cliutil.Ptr("d-123")}}}, nil
		},
		listSpacesFn: func(_ context.Context, _ *sagemaker.ListSpacesInput, _ ...func(*sagemaker.Options)) (*sagemaker.ListSpacesOutput, error) {
			return nil, errors.New("throttle")
		},
	}

	withMockDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) API { return client },
	)

	_, err := executeCommand(t, "--output", "json", "--no-confirm", "sagemaker", "cleanup-spaces")
	if err == nil || !strings.Contains(err.Error(), "list spaces for domain") {
		t.Fatalf("expected list spaces error, got: %v", err)
	}
}

func TestCleanupSpacesDeleteError(t *testing.T) {
	client := &mockClient{
		listDomainsFn: func(_ context.Context, _ *sagemaker.ListDomainsInput, _ ...func(*sagemaker.Options)) (*sagemaker.ListDomainsOutput, error) {
			return &sagemaker.ListDomainsOutput{Domains: []sagemakertypes.DomainDetails{{DomainId: cliutil.Ptr("d-123")}}}, nil
		},
		listSpacesFn: func(_ context.Context, _ *sagemaker.ListSpacesInput, _ ...func(*sagemaker.Options)) (*sagemaker.ListSpacesOutput, error) {
			return &sagemaker.ListSpacesOutput{Spaces: []sagemakertypes.SpaceDetails{
				{SpaceName: cliutil.Ptr("space-a"), Status: sagemakertypes.SpaceStatusInService},
			}}, nil
		},
		deleteSpaceFn: func(_ context.Context, _ *sagemaker.DeleteSpaceInput, _ ...func(*sagemaker.Options)) (*sagemaker.DeleteSpaceOutput, error) {
			return nil, errors.New("in use")
		},
	}

	withMockDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) API { return client },
	)

	output, err := executeCommand(t, "--output", "json", "--no-confirm", "sagemaker", "cleanup-spaces")
	if err != nil {
		t.Fatalf("unexpected error (delete failures should not propagate): %v", err)
	}
	if !strings.Contains(output, "failed:") {
		t.Fatalf("expected failed action in output: %s", output)
	}
}

func TestCleanupSpacesLoadAWSConfigError(t *testing.T) {
	withMockDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{}, errors.New("bad creds") },
		func(awssdk.Config) API { return &mockClient{} },
	)

	_, err := executeCommand(t, "--output", "json", "sagemaker", "cleanup-spaces")
	if err == nil || !strings.Contains(err.Error(), "load AWS config") {
		t.Fatalf("expected load config error, got: %v", err)
	}
}

func TestCleanupSpacesDryRunWithSpaces(t *testing.T) {
	client := &mockClient{
		listSpacesFn: func(_ context.Context, _ *sagemaker.ListSpacesInput, _ ...func(*sagemaker.Options)) (*sagemaker.ListSpacesOutput, error) {
			return &sagemaker.ListSpacesOutput{Spaces: []sagemakertypes.SpaceDetails{
				{SpaceName: cliutil.Ptr("space-a"), Status: sagemakertypes.SpaceStatusInService},
				{SpaceName: cliutil.Ptr("space-b"), Status: sagemakertypes.SpaceStatusInService},
			}}, nil
		},
	}

	withMockDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) API { return client },
	)

	output, err := executeCommand(t, "--output", "json", "--dry-run", "sagemaker", "cleanup-spaces", "--domain-id", "d-123", "--spaces", "space-a")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, "\"action\": \"would-delete\"") {
		t.Fatalf("expected would-delete in dry-run output: %s", output)
	}
	if strings.Contains(output, "space-b") {
		t.Fatalf("space-b should be filtered out: %s", output)
	}
}

func TestDeleteUserProfileDryRun(t *testing.T) {
	client := &mockClient{
		listAppsFn: func(_ context.Context, _ *sagemaker.ListAppsInput, _ ...func(*sagemaker.Options)) (*sagemaker.ListAppsOutput, error) {
			return &sagemaker.ListAppsOutput{Apps: []sagemakertypes.AppDetails{
				{AppName: cliutil.Ptr("app-a"), AppType: sagemakertypes.AppTypeJupyterServer, Status: sagemakertypes.AppStatusInService},
			}}, nil
		},
		listSpacesFn: func(_ context.Context, _ *sagemaker.ListSpacesInput, _ ...func(*sagemaker.Options)) (*sagemaker.ListSpacesOutput, error) {
			return &sagemaker.ListSpacesOutput{Spaces: []sagemakertypes.SpaceDetails{}}, nil
		},
	}

	withMockDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) API { return client },
	)

	output, err := executeCommand(t, "--output", "json", "--dry-run", "sagemaker", "delete-user-profile", "--domain-id", "d-123", "--user-profile", "alice")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, "\"action\": \"would-delete\"") {
		t.Fatalf("expected would-delete in dry-run output: %s", output)
	}
}

func TestDeleteUserProfileDependencyFailureSkipsProfile(t *testing.T) {
	client := &mockClient{
		listAppsFn: func(_ context.Context, _ *sagemaker.ListAppsInput, _ ...func(*sagemaker.Options)) (*sagemaker.ListAppsOutput, error) {
			return &sagemaker.ListAppsOutput{Apps: []sagemakertypes.AppDetails{
				{AppName: cliutil.Ptr("app-a"), AppType: sagemakertypes.AppTypeJupyterServer, Status: sagemakertypes.AppStatusInService},
			}}, nil
		},
		listSpacesFn: func(_ context.Context, _ *sagemaker.ListSpacesInput, _ ...func(*sagemaker.Options)) (*sagemaker.ListSpacesOutput, error) {
			return &sagemaker.ListSpacesOutput{Spaces: []sagemakertypes.SpaceDetails{}}, nil
		},
		deleteAppFn: func(_ context.Context, _ *sagemaker.DeleteAppInput, _ ...func(*sagemaker.Options)) (*sagemaker.DeleteAppOutput, error) {
			return nil, errors.New("app delete failed")
		},
	}

	withMockDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) API { return client },
	)

	output, err := executeCommand(t, "--output", "json", "--no-confirm", "sagemaker", "delete-user-profile", "--domain-id", "d-123", "--user-profile", "alice")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, "skipped:dependency cleanup failed") {
		t.Fatalf("expected skipped user profile due to dependency failure: %s", output)
	}
	if !strings.Contains(output, "failed:") {
		t.Fatalf("expected failed app delete in output: %s", output)
	}
}

func TestDeleteUserProfileDeleteProfileError(t *testing.T) {
	client := &mockClient{
		listAppsFn: func(_ context.Context, _ *sagemaker.ListAppsInput, _ ...func(*sagemaker.Options)) (*sagemaker.ListAppsOutput, error) {
			return &sagemaker.ListAppsOutput{Apps: []sagemakertypes.AppDetails{}}, nil
		},
		listSpacesFn: func(_ context.Context, _ *sagemaker.ListSpacesInput, _ ...func(*sagemaker.Options)) (*sagemaker.ListSpacesOutput, error) {
			return &sagemaker.ListSpacesOutput{Spaces: []sagemakertypes.SpaceDetails{}}, nil
		},
		deleteUserProfileFn: func(_ context.Context, _ *sagemaker.DeleteUserProfileInput, _ ...func(*sagemaker.Options)) (*sagemaker.DeleteUserProfileOutput, error) {
			return nil, errors.New("profile in use")
		},
	}

	withMockDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) API { return client },
	)

	oldSleep := sleep
	sleep = func(_ time.Duration) {}
	t.Cleanup(func() { sleep = oldSleep })

	output, err := executeCommand(t, "--output", "json", "--no-confirm", "sagemaker", "delete-user-profile", "--domain-id", "d-123", "--user-profile", "alice")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, "failed:profile in use") {
		t.Fatalf("expected failed action for profile delete: %s", output)
	}
}

func TestDeleteUserProfileWaitTimeout(t *testing.T) {
	callCount := 0
	client := &mockClient{
		listAppsFn: func(_ context.Context, _ *sagemaker.ListAppsInput, _ ...func(*sagemaker.Options)) (*sagemaker.ListAppsOutput, error) {
			callCount++
			if callCount == 1 {
				// First call during discovery (includeDeleting=false): return one active app
				return &sagemaker.ListAppsOutput{Apps: []sagemakertypes.AppDetails{}}, nil
			}
			// During wait polling: always return a deleting app to force timeout
			return &sagemaker.ListAppsOutput{Apps: []sagemakertypes.AppDetails{
				{AppName: cliutil.Ptr("stuck-app"), AppType: sagemakertypes.AppTypeJupyterServer, Status: sagemakertypes.AppStatusDeleting},
			}}, nil
		},
		listSpacesFn: func(_ context.Context, _ *sagemaker.ListSpacesInput, _ ...func(*sagemaker.Options)) (*sagemaker.ListSpacesOutput, error) {
			return &sagemaker.ListSpacesOutput{Spaces: []sagemakertypes.SpaceDetails{}}, nil
		},
		deleteUserProfileFn: func(_ context.Context, _ *sagemaker.DeleteUserProfileInput, _ ...func(*sagemaker.Options)) (*sagemaker.DeleteUserProfileOutput, error) {
			t.Fatal("should not have called DeleteUserProfile after timeout")
			return nil, nil
		},
	}

	withMockDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) API { return client },
	)

	oldSleep := sleep
	sleep = func(_ time.Duration) {}
	t.Cleanup(func() { sleep = oldSleep })

	output, err := executeCommand(t, "--output", "json", "--no-confirm", "sagemaker", "delete-user-profile", "--domain-id", "d-123", "--user-profile", "alice")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, "failed:") || !strings.Contains(output, "timed out") {
		t.Fatalf("expected timed out failure in output: %s", output)
	}
}

func TestDeleteUserProfileListAppsError(t *testing.T) {
	client := &mockClient{
		listAppsFn: func(_ context.Context, _ *sagemaker.ListAppsInput, _ ...func(*sagemaker.Options)) (*sagemaker.ListAppsOutput, error) {
			return nil, errors.New("apps access denied")
		},
	}

	withMockDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) API { return client },
	)

	_, err := executeCommand(t, "--output", "json", "--no-confirm", "sagemaker", "delete-user-profile", "--domain-id", "d-123", "--user-profile", "alice")
	if err == nil || !strings.Contains(err.Error(), "list apps for user profile") {
		t.Fatalf("expected list apps error, got: %v", err)
	}
}

func TestDeleteUserProfileListSpacesError(t *testing.T) {
	client := &mockClient{
		listAppsFn: func(_ context.Context, _ *sagemaker.ListAppsInput, _ ...func(*sagemaker.Options)) (*sagemaker.ListAppsOutput, error) {
			return &sagemaker.ListAppsOutput{Apps: []sagemakertypes.AppDetails{}}, nil
		},
		listSpacesFn: func(_ context.Context, _ *sagemaker.ListSpacesInput, _ ...func(*sagemaker.Options)) (*sagemaker.ListSpacesOutput, error) {
			return nil, errors.New("spaces access denied")
		},
	}

	withMockDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) API { return client },
	)

	_, err := executeCommand(t, "--output", "json", "--no-confirm", "sagemaker", "delete-user-profile", "--domain-id", "d-123", "--user-profile", "alice")
	if err == nil || !strings.Contains(err.Error(), "list spaces for user profile") {
		t.Fatalf("expected list spaces error, got: %v", err)
	}
}

func TestDeleteUserProfileLoadConfigError(t *testing.T) {
	withMockDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{}, errors.New("bad creds") },
		func(awssdk.Config) API { return &mockClient{} },
	)

	_, err := executeCommand(t, "--output", "json", "sagemaker", "delete-user-profile", "--domain-id", "d-123", "--user-profile", "alice")
	if err == nil || !strings.Contains(err.Error(), "load AWS config") {
		t.Fatalf("expected load config error, got: %v", err)
	}
}

func TestDeleteUserProfileNoDependenciesSuccess(t *testing.T) {
	deleteProfileCalled := false
	client := &mockClient{
		listAppsFn: func(_ context.Context, _ *sagemaker.ListAppsInput, _ ...func(*sagemaker.Options)) (*sagemaker.ListAppsOutput, error) {
			return &sagemaker.ListAppsOutput{Apps: []sagemakertypes.AppDetails{}}, nil
		},
		listSpacesFn: func(_ context.Context, _ *sagemaker.ListSpacesInput, _ ...func(*sagemaker.Options)) (*sagemaker.ListSpacesOutput, error) {
			return &sagemaker.ListSpacesOutput{Spaces: []sagemakertypes.SpaceDetails{}}, nil
		},
		deleteUserProfileFn: func(_ context.Context, _ *sagemaker.DeleteUserProfileInput, _ ...func(*sagemaker.Options)) (*sagemaker.DeleteUserProfileOutput, error) {
			deleteProfileCalled = true
			return &sagemaker.DeleteUserProfileOutput{}, nil
		},
	}

	withMockDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) API { return client },
	)

	oldSleep := sleep
	sleep = func(_ time.Duration) {}
	t.Cleanup(func() { sleep = oldSleep })

	output, err := executeCommand(t, "--output", "json", "--no-confirm", "sagemaker", "delete-user-profile", "--domain-id", "d-123", "--user-profile", "alice")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !deleteProfileCalled {
		t.Fatal("expected DeleteUserProfile to be called")
	}
	if !strings.Contains(output, "\"action\": \"deleted\"") {
		t.Fatalf("expected deleted action: %s", output)
	}
}

func TestDeleteUserProfileWithSpaceDependency(t *testing.T) {
	describeSpaceCalls := 0
	deleteSpaceCalled := false
	deleteProfileCalled := false

	client := &mockClient{
		listAppsFn: func(_ context.Context, _ *sagemaker.ListAppsInput, _ ...func(*sagemaker.Options)) (*sagemaker.ListAppsOutput, error) {
			return &sagemaker.ListAppsOutput{Apps: []sagemakertypes.AppDetails{}}, nil
		},
		listSpacesFn: func(_ context.Context, _ *sagemaker.ListSpacesInput, _ ...func(*sagemaker.Options)) (*sagemaker.ListSpacesOutput, error) {
			return &sagemaker.ListSpacesOutput{Spaces: []sagemakertypes.SpaceDetails{
				{SpaceName: cliutil.Ptr("my-space"), Status: sagemakertypes.SpaceStatusInService},
			}}, nil
		},
		describeSpaceFn: func(_ context.Context, _ *sagemaker.DescribeSpaceInput, _ ...func(*sagemaker.Options)) (*sagemaker.DescribeSpaceOutput, error) {
			describeSpaceCalls++
			status := sagemakertypes.SpaceStatusInService
			// After initial listing, return deleted to pass wait
			if describeSpaceCalls > 2 {
				status = sagemakertypes.SpaceStatus("Deleted")
			}
			return &sagemaker.DescribeSpaceOutput{
				Status: status,
				OwnershipSettings: &sagemakertypes.OwnershipSettings{
					OwnerUserProfileName: cliutil.Ptr("bob"),
				},
			}, nil
		},
		deleteSpaceFn: func(_ context.Context, _ *sagemaker.DeleteSpaceInput, _ ...func(*sagemaker.Options)) (*sagemaker.DeleteSpaceOutput, error) {
			deleteSpaceCalled = true
			return &sagemaker.DeleteSpaceOutput{}, nil
		},
		deleteUserProfileFn: func(_ context.Context, _ *sagemaker.DeleteUserProfileInput, _ ...func(*sagemaker.Options)) (*sagemaker.DeleteUserProfileOutput, error) {
			deleteProfileCalled = true
			return &sagemaker.DeleteUserProfileOutput{}, nil
		},
	}

	withMockDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) API { return client },
	)

	oldSleep := sleep
	sleep = func(_ time.Duration) {}
	t.Cleanup(func() { sleep = oldSleep })

	output, err := executeCommand(t, "--output", "json", "--no-confirm", "sagemaker", "delete-user-profile", "--domain-id", "d-123", "--user-profile", "bob")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !deleteSpaceCalled {
		t.Fatal("expected DeleteSpace to be called")
	}
	if !deleteProfileCalled {
		t.Fatal("expected DeleteUserProfile to be called")
	}
	if !strings.Contains(output, "\"step\": \"space\"") {
		t.Fatalf("expected space step in output: %s", output)
	}
}

func TestDeleteUserProfileSpaceDeleteFailureSkipsProfile(t *testing.T) {
	client := &mockClient{
		listAppsFn: func(_ context.Context, _ *sagemaker.ListAppsInput, _ ...func(*sagemaker.Options)) (*sagemaker.ListAppsOutput, error) {
			return &sagemaker.ListAppsOutput{Apps: []sagemakertypes.AppDetails{}}, nil
		},
		listSpacesFn: func(_ context.Context, _ *sagemaker.ListSpacesInput, _ ...func(*sagemaker.Options)) (*sagemaker.ListSpacesOutput, error) {
			return &sagemaker.ListSpacesOutput{Spaces: []sagemakertypes.SpaceDetails{
				{SpaceName: cliutil.Ptr("my-space"), Status: sagemakertypes.SpaceStatusInService},
			}}, nil
		},
		describeSpaceFn: func(_ context.Context, _ *sagemaker.DescribeSpaceInput, _ ...func(*sagemaker.Options)) (*sagemaker.DescribeSpaceOutput, error) {
			return &sagemaker.DescribeSpaceOutput{
				Status: sagemakertypes.SpaceStatusInService,
				OwnershipSettings: &sagemakertypes.OwnershipSettings{
					OwnerUserProfileName: cliutil.Ptr("alice"),
				},
			}, nil
		},
		deleteSpaceFn: func(_ context.Context, _ *sagemaker.DeleteSpaceInput, _ ...func(*sagemaker.Options)) (*sagemaker.DeleteSpaceOutput, error) {
			return nil, errors.New("space in use")
		},
	}

	withMockDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) API { return client },
	)

	output, err := executeCommand(t, "--output", "json", "--no-confirm", "sagemaker", "delete-user-profile", "--domain-id", "d-123", "--user-profile", "alice")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, "skipped:dependency cleanup failed") {
		t.Fatalf("expected skipped user profile due to space dependency failure: %s", output)
	}
}

func TestDeleteUserProfileWaitListAppsError(t *testing.T) {
	listAppsCalls := 0
	client := &mockClient{
		listAppsFn: func(_ context.Context, _ *sagemaker.ListAppsInput, _ ...func(*sagemaker.Options)) (*sagemaker.ListAppsOutput, error) {
			listAppsCalls++
			if listAppsCalls == 1 {
				return &sagemaker.ListAppsOutput{Apps: []sagemakertypes.AppDetails{}}, nil
			}
			return nil, errors.New("wait list apps error")
		},
		listSpacesFn: func(_ context.Context, _ *sagemaker.ListSpacesInput, _ ...func(*sagemaker.Options)) (*sagemaker.ListSpacesOutput, error) {
			return &sagemaker.ListSpacesOutput{Spaces: []sagemakertypes.SpaceDetails{}}, nil
		},
		deleteUserProfileFn: func(_ context.Context, _ *sagemaker.DeleteUserProfileInput, _ ...func(*sagemaker.Options)) (*sagemaker.DeleteUserProfileOutput, error) {
			t.Fatal("should not call DeleteUserProfile after wait error")
			return nil, nil
		},
	}

	withMockDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) API { return client },
	)

	oldSleep := sleep
	sleep = func(_ time.Duration) {}
	t.Cleanup(func() { sleep = oldSleep })

	output, err := executeCommand(t, "--output", "json", "--no-confirm", "sagemaker", "delete-user-profile", "--domain-id", "d-123", "--user-profile", "alice")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, "failed:") {
		t.Fatalf("expected failed action in output: %s", output)
	}
}

func TestDeleteUserProfileWaitListSpacesError(t *testing.T) {
	listSpacesCalls := 0
	client := &mockClient{
		listAppsFn: func(_ context.Context, _ *sagemaker.ListAppsInput, _ ...func(*sagemaker.Options)) (*sagemaker.ListAppsOutput, error) {
			return &sagemaker.ListAppsOutput{Apps: []sagemakertypes.AppDetails{}}, nil
		},
		listSpacesFn: func(_ context.Context, _ *sagemaker.ListSpacesInput, _ ...func(*sagemaker.Options)) (*sagemaker.ListSpacesOutput, error) {
			listSpacesCalls++
			if listSpacesCalls == 1 {
				return &sagemaker.ListSpacesOutput{Spaces: []sagemakertypes.SpaceDetails{}}, nil
			}
			return nil, errors.New("wait list spaces error")
		},
		describeSpaceFn: func(_ context.Context, _ *sagemaker.DescribeSpaceInput, _ ...func(*sagemaker.Options)) (*sagemaker.DescribeSpaceOutput, error) {
			return &sagemaker.DescribeSpaceOutput{
				Status: sagemakertypes.SpaceStatusInService,
				OwnershipSettings: &sagemakertypes.OwnershipSettings{
					OwnerUserProfileName: cliutil.Ptr("alice"),
				},
			}, nil
		},
		deleteUserProfileFn: func(_ context.Context, _ *sagemaker.DeleteUserProfileInput, _ ...func(*sagemaker.Options)) (*sagemaker.DeleteUserProfileOutput, error) {
			t.Fatal("should not call DeleteUserProfile after wait error")
			return nil, nil
		},
	}

	withMockDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) API { return client },
	)

	oldSleep := sleep
	sleep = func(_ time.Duration) {}
	t.Cleanup(func() { sleep = oldSleep })

	output, err := executeCommand(t, "--output", "json", "--no-confirm", "sagemaker", "delete-user-profile", "--domain-id", "d-123", "--user-profile", "alice")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, "failed:") {
		t.Fatalf("expected failed action in output: %s", output)
	}
}

func TestDeleteUserProfileAppsWithDeletedAndDeletingStatus(t *testing.T) {
	deleteProfileCalled := false
	listAppsCalls := 0
	client := &mockClient{
		listAppsFn: func(_ context.Context, _ *sagemaker.ListAppsInput, _ ...func(*sagemaker.Options)) (*sagemaker.ListAppsOutput, error) {
			listAppsCalls++
			if listAppsCalls == 1 {
				// Initial discovery (includeDeleting=false): returns apps with Deleted and Deleting status + nil name
				return &sagemaker.ListAppsOutput{Apps: []sagemakertypes.AppDetails{
					{AppName: cliutil.Ptr("deleted-app"), AppType: sagemakertypes.AppTypeJupyterServer, Status: sagemakertypes.AppStatusDeleted},
					{AppName: cliutil.Ptr("deleting-app"), AppType: sagemakertypes.AppTypeJupyterServer, Status: sagemakertypes.AppStatusDeleting},
					{AppName: nil, AppType: sagemakertypes.AppTypeJupyterServer, Status: sagemakertypes.AppStatusInService},
				}}, nil
			}
			// Wait loop (includeDeleting=true): all apps now deleted
			return &sagemaker.ListAppsOutput{Apps: []sagemakertypes.AppDetails{
				{AppName: cliutil.Ptr("deleted-app"), AppType: sagemakertypes.AppTypeJupyterServer, Status: sagemakertypes.AppStatusDeleted},
				{AppName: cliutil.Ptr("deleting-app"), AppType: sagemakertypes.AppTypeJupyterServer, Status: sagemakertypes.AppStatusDeleted},
			}}, nil
		},
		listSpacesFn: func(_ context.Context, _ *sagemaker.ListSpacesInput, _ ...func(*sagemaker.Options)) (*sagemaker.ListSpacesOutput, error) {
			return &sagemaker.ListSpacesOutput{Spaces: []sagemakertypes.SpaceDetails{}}, nil
		},
		deleteUserProfileFn: func(_ context.Context, _ *sagemaker.DeleteUserProfileInput, _ ...func(*sagemaker.Options)) (*sagemaker.DeleteUserProfileOutput, error) {
			deleteProfileCalled = true
			return &sagemaker.DeleteUserProfileOutput{}, nil
		},
	}

	withMockDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) API { return client },
	)

	oldSleep := sleep
	sleep = func(_ time.Duration) {}
	t.Cleanup(func() { sleep = oldSleep })

	output, err := executeCommand(t, "--output", "json", "--no-confirm", "sagemaker", "delete-user-profile", "--domain-id", "d-123", "--user-profile", "alice")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !deleteProfileCalled {
		t.Fatal("expected DeleteUserProfile to be called")
	}
	// Both deleted/deleting apps and nil-name app should be filtered out during discovery; only user-profile step remains
	if strings.Contains(output, "deleted-app") || strings.Contains(output, "deleting-app") {
		t.Fatalf("expected deleted/deleting apps to be filtered out: %s", output)
	}
	if !strings.Contains(output, "\"action\": \"deleted\"") {
		t.Fatalf("expected deleted action: %s", output)
	}
}

func TestDeleteUserProfileDescribeSpaceError(t *testing.T) {
	client := &mockClient{
		listAppsFn: func(_ context.Context, _ *sagemaker.ListAppsInput, _ ...func(*sagemaker.Options)) (*sagemaker.ListAppsOutput, error) {
			return &sagemaker.ListAppsOutput{Apps: []sagemakertypes.AppDetails{}}, nil
		},
		listSpacesFn: func(_ context.Context, _ *sagemaker.ListSpacesInput, _ ...func(*sagemaker.Options)) (*sagemaker.ListSpacesOutput, error) {
			return &sagemaker.ListSpacesOutput{Spaces: []sagemakertypes.SpaceDetails{
				{SpaceName: cliutil.Ptr("some-space"), Status: sagemakertypes.SpaceStatusInService},
			}}, nil
		},
		describeSpaceFn: func(_ context.Context, _ *sagemaker.DescribeSpaceInput, _ ...func(*sagemaker.Options)) (*sagemaker.DescribeSpaceOutput, error) {
			return nil, errors.New("describe failed")
		},
	}

	withMockDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) API { return client },
	)

	_, err := executeCommand(t, "--output", "json", "--no-confirm", "sagemaker", "delete-user-profile", "--domain-id", "d-123", "--user-profile", "alice")
	if err == nil || !strings.Contains(err.Error(), "list spaces for user profile") {
		t.Fatalf("expected list spaces error, got: %v", err)
	}
}

func TestDeleteUserProfileSpaceNoOwnership(t *testing.T) {
	deleteProfileCalled := false
	client := &mockClient{
		listAppsFn: func(_ context.Context, _ *sagemaker.ListAppsInput, _ ...func(*sagemaker.Options)) (*sagemaker.ListAppsOutput, error) {
			return &sagemaker.ListAppsOutput{Apps: []sagemakertypes.AppDetails{}}, nil
		},
		listSpacesFn: func(_ context.Context, _ *sagemaker.ListSpacesInput, _ ...func(*sagemaker.Options)) (*sagemaker.ListSpacesOutput, error) {
			return &sagemaker.ListSpacesOutput{Spaces: []sagemakertypes.SpaceDetails{
				{SpaceName: cliutil.Ptr("shared-space"), Status: sagemakertypes.SpaceStatusInService},
			}}, nil
		},
		describeSpaceFn: func(_ context.Context, _ *sagemaker.DescribeSpaceInput, _ ...func(*sagemaker.Options)) (*sagemaker.DescribeSpaceOutput, error) {
			return &sagemaker.DescribeSpaceOutput{
				Status:            sagemakertypes.SpaceStatusInService,
				OwnershipSettings: nil, // no ownership settings
			}, nil
		},
		deleteUserProfileFn: func(_ context.Context, _ *sagemaker.DeleteUserProfileInput, _ ...func(*sagemaker.Options)) (*sagemaker.DeleteUserProfileOutput, error) {
			deleteProfileCalled = true
			return &sagemaker.DeleteUserProfileOutput{}, nil
		},
	}

	withMockDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) API { return client },
	)

	oldSleep := sleep
	sleep = func(_ time.Duration) {}
	t.Cleanup(func() { sleep = oldSleep })

	output, err := executeCommand(t, "--output", "json", "--no-confirm", "sagemaker", "delete-user-profile", "--domain-id", "d-123", "--user-profile", "alice")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !deleteProfileCalled {
		t.Fatal("expected DeleteUserProfile to be called (space not owned by user)")
	}
	if strings.Contains(output, "shared-space") {
		t.Fatalf("space with no ownership should not appear as dependency: %s", output)
	}
}

func TestDeleteUserProfileSpaceDifferentOwner(t *testing.T) {
	deleteProfileCalled := false
	client := &mockClient{
		listAppsFn: func(_ context.Context, _ *sagemaker.ListAppsInput, _ ...func(*sagemaker.Options)) (*sagemaker.ListAppsOutput, error) {
			return &sagemaker.ListAppsOutput{Apps: []sagemakertypes.AppDetails{}}, nil
		},
		listSpacesFn: func(_ context.Context, _ *sagemaker.ListSpacesInput, _ ...func(*sagemaker.Options)) (*sagemaker.ListSpacesOutput, error) {
			return &sagemaker.ListSpacesOutput{Spaces: []sagemakertypes.SpaceDetails{
				{SpaceName: cliutil.Ptr("other-space"), Status: sagemakertypes.SpaceStatusInService},
			}}, nil
		},
		describeSpaceFn: func(_ context.Context, _ *sagemaker.DescribeSpaceInput, _ ...func(*sagemaker.Options)) (*sagemaker.DescribeSpaceOutput, error) {
			return &sagemaker.DescribeSpaceOutput{
				Status: sagemakertypes.SpaceStatusInService,
				OwnershipSettings: &sagemakertypes.OwnershipSettings{
					OwnerUserProfileName: cliutil.Ptr("bob"), // different owner
				},
			}, nil
		},
		deleteUserProfileFn: func(_ context.Context, _ *sagemaker.DeleteUserProfileInput, _ ...func(*sagemaker.Options)) (*sagemaker.DeleteUserProfileOutput, error) {
			deleteProfileCalled = true
			return &sagemaker.DeleteUserProfileOutput{}, nil
		},
	}

	withMockDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) API { return client },
	)

	oldSleep := sleep
	sleep = func(_ time.Duration) {}
	t.Cleanup(func() { sleep = oldSleep })

	output, err := executeCommand(t, "--output", "json", "--no-confirm", "sagemaker", "delete-user-profile", "--domain-id", "d-123", "--user-profile", "alice")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !deleteProfileCalled {
		t.Fatal("expected DeleteUserProfile to be called (space owned by different user)")
	}
	if strings.Contains(output, "other-space") {
		t.Fatalf("space owned by different user should not appear as dependency: %s", output)
	}
}

func TestListDomainIDsWithPagination(t *testing.T) {
	callCount := 0
	client := &mockClient{
		listDomainsFn: func(_ context.Context, in *sagemaker.ListDomainsInput, _ ...func(*sagemaker.Options)) (*sagemaker.ListDomainsOutput, error) {
			callCount++
			if callCount == 1 {
				return &sagemaker.ListDomainsOutput{
					Domains:   []sagemakertypes.DomainDetails{{DomainId: cliutil.Ptr("d-001")}},
					NextToken: cliutil.Ptr("token2"),
				}, nil
			}
			return &sagemaker.ListDomainsOutput{
				Domains: []sagemakertypes.DomainDetails{{DomainId: cliutil.Ptr("d-002")}},
			}, nil
		},
	}

	ids, err := listDomainIDs(context.Background(), client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ids) != 2 || ids[0] != "d-001" || ids[1] != "d-002" {
		t.Fatalf("expected [d-001 d-002], got %v", ids)
	}
	if callCount != 2 {
		t.Fatalf("expected 2 list calls for pagination, got %d", callCount)
	}
}

func TestListDomainIDsEmptyDomainID(t *testing.T) {
	client := &mockClient{
		listDomainsFn: func(_ context.Context, _ *sagemaker.ListDomainsInput, _ ...func(*sagemaker.Options)) (*sagemaker.ListDomainsOutput, error) {
			return &sagemaker.ListDomainsOutput{
				Domains: []sagemakertypes.DomainDetails{
					{DomainId: cliutil.Ptr("")},
					{DomainId: nil},
					{DomainId: cliutil.Ptr("d-real")},
				},
			}, nil
		},
	}

	ids, err := listDomainIDs(context.Background(), client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ids) != 1 || ids[0] != "d-real" {
		t.Fatalf("expected [d-real], got %v", ids)
	}
}

func TestListDomainIDsError(t *testing.T) {
	client := &mockClient{
		listDomainsFn: func(_ context.Context, _ *sagemaker.ListDomainsInput, _ ...func(*sagemaker.Options)) (*sagemaker.ListDomainsOutput, error) {
			return nil, errors.New("service error")
		},
	}

	_, err := listDomainIDs(context.Background(), client)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestListSpacesError(t *testing.T) {
	client := &mockClient{
		listSpacesFn: func(_ context.Context, _ *sagemaker.ListSpacesInput, _ ...func(*sagemaker.Options)) (*sagemaker.ListSpacesOutput, error) {
			return nil, errors.New("space error")
		},
	}

	_, err := listSpaces(context.Background(), client, "d-123")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestListSpacesPagination(t *testing.T) {
	callCount := 0
	client := &mockClient{
		listSpacesFn: func(_ context.Context, in *sagemaker.ListSpacesInput, _ ...func(*sagemaker.Options)) (*sagemaker.ListSpacesOutput, error) {
			callCount++
			if callCount == 1 {
				return &sagemaker.ListSpacesOutput{
					Spaces:    []sagemakertypes.SpaceDetails{{SpaceName: cliutil.Ptr("s1")}},
					NextToken: cliutil.Ptr("tok"),
				}, nil
			}
			return &sagemaker.ListSpacesOutput{
				Spaces: []sagemakertypes.SpaceDetails{{SpaceName: cliutil.Ptr("s2")}},
			}, nil
		},
	}

	spaces, err := listSpaces(context.Background(), client, "d-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(spaces) != 2 {
		t.Fatalf("expected 2 spaces, got %d", len(spaces))
	}
}

func TestListUserProfileAppsFiltering(t *testing.T) {
	client := &mockClient{
		listAppsFn: func(_ context.Context, _ *sagemaker.ListAppsInput, _ ...func(*sagemaker.Options)) (*sagemaker.ListAppsOutput, error) {
			return &sagemaker.ListAppsOutput{Apps: []sagemakertypes.AppDetails{
				{AppName: cliutil.Ptr("active"), AppType: sagemakertypes.AppTypeJupyterServer, Status: sagemakertypes.AppStatusInService},
				{AppName: cliutil.Ptr("deleted"), AppType: sagemakertypes.AppTypeJupyterServer, Status: sagemakertypes.AppStatusDeleted},
				{AppName: cliutil.Ptr("deleting"), AppType: sagemakertypes.AppTypeJupyterServer, Status: sagemakertypes.AppStatusDeleting},
			}}, nil
		},
	}

	// includeDeleting = false
	apps, err := listUserProfileApps(context.Background(), client, "d-123", "alice", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(apps) != 1 || cliutil.PointerToString(apps[0].AppName) != "active" {
		t.Fatalf("expected only active app, got %v", apps)
	}

	// includeDeleting = true
	apps, err = listUserProfileApps(context.Background(), client, "d-123", "alice", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(apps) != 2 {
		t.Fatalf("expected active + deleting apps, got %d", len(apps))
	}
}

func TestListUserProfileAppsError(t *testing.T) {
	client := &mockClient{
		listAppsFn: func(_ context.Context, _ *sagemaker.ListAppsInput, _ ...func(*sagemaker.Options)) (*sagemaker.ListAppsOutput, error) {
			return nil, errors.New("app error")
		},
	}

	_, err := listUserProfileApps(context.Background(), client, "d-123", "alice", false)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestListUserProfileSpacesFiltering(t *testing.T) {
	client := &mockClient{
		listSpacesFn: func(_ context.Context, _ *sagemaker.ListSpacesInput, _ ...func(*sagemaker.Options)) (*sagemaker.ListSpacesOutput, error) {
			return &sagemaker.ListSpacesOutput{Spaces: []sagemakertypes.SpaceDetails{
				{SpaceName: cliutil.Ptr("owned-active"), Status: sagemakertypes.SpaceStatusInService},
				{SpaceName: cliutil.Ptr("owned-deleting"), Status: sagemakertypes.SpaceStatusInService},
				{SpaceName: cliutil.Ptr("owned-deleted"), Status: sagemakertypes.SpaceStatusInService},
				{SpaceName: cliutil.Ptr("not-owned"), Status: sagemakertypes.SpaceStatusInService},
				{SpaceName: cliutil.Ptr("no-ownership"), Status: sagemakertypes.SpaceStatusInService},
				{SpaceName: nil, Status: sagemakertypes.SpaceStatusInService},
			}}, nil
		},
		describeSpaceFn: func(_ context.Context, in *sagemaker.DescribeSpaceInput, _ ...func(*sagemaker.Options)) (*sagemaker.DescribeSpaceOutput, error) {
			name := cliutil.PointerToString(in.SpaceName)
			switch name {
			case "owned-active":
				return &sagemaker.DescribeSpaceOutput{
					Status:            sagemakertypes.SpaceStatusInService,
					OwnershipSettings: &sagemakertypes.OwnershipSettings{OwnerUserProfileName: cliutil.Ptr("alice")},
				}, nil
			case "owned-deleting":
				return &sagemaker.DescribeSpaceOutput{
					Status:            sagemakertypes.SpaceStatusDeleting,
					OwnershipSettings: &sagemakertypes.OwnershipSettings{OwnerUserProfileName: cliutil.Ptr("alice")},
				}, nil
			case "owned-deleted":
				return &sagemaker.DescribeSpaceOutput{
					Status:            sagemakertypes.SpaceStatus("Deleted"),
					OwnershipSettings: &sagemakertypes.OwnershipSettings{OwnerUserProfileName: cliutil.Ptr("alice")},
				}, nil
			case "not-owned":
				return &sagemaker.DescribeSpaceOutput{
					Status:            sagemakertypes.SpaceStatusInService,
					OwnershipSettings: &sagemakertypes.OwnershipSettings{OwnerUserProfileName: cliutil.Ptr("bob")},
				}, nil
			case "no-ownership":
				return &sagemaker.DescribeSpaceOutput{
					Status:            sagemakertypes.SpaceStatusInService,
					OwnershipSettings: nil,
				}, nil
			default:
				t.Fatalf("unexpected space name: %s", name)
				return nil, nil
			}
		},
	}

	// includeDeleting = false: only owned-active
	spaces, err := listUserProfileSpaces(context.Background(), client, "d-123", "alice", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(spaces) != 1 || spaces[0] != "owned-active" {
		t.Fatalf("expected [owned-active], got %v", spaces)
	}

	// includeDeleting = true: owned-active + owned-deleting (owned-deleted still excluded)
	spaces, err = listUserProfileSpaces(context.Background(), client, "d-123", "alice", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(spaces) != 2 {
		t.Fatalf("expected 2 spaces, got %d: %v", len(spaces), spaces)
	}
}

func TestListUserProfileSpacesError(t *testing.T) {
	client := &mockClient{
		listSpacesFn: func(_ context.Context, _ *sagemaker.ListSpacesInput, _ ...func(*sagemaker.Options)) (*sagemaker.ListSpacesOutput, error) {
			return nil, errors.New("list spaces error")
		},
	}

	_, err := listUserProfileSpaces(context.Background(), client, "d-123", "alice", false)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestListUserProfileSpacesDescribeError(t *testing.T) {
	client := &mockClient{
		listSpacesFn: func(_ context.Context, _ *sagemaker.ListSpacesInput, _ ...func(*sagemaker.Options)) (*sagemaker.ListSpacesOutput, error) {
			return &sagemaker.ListSpacesOutput{Spaces: []sagemakertypes.SpaceDetails{
				{SpaceName: cliutil.Ptr("space-a"), Status: sagemakertypes.SpaceStatusInService},
			}}, nil
		},
		describeSpaceFn: func(_ context.Context, _ *sagemaker.DescribeSpaceInput, _ ...func(*sagemaker.Options)) (*sagemaker.DescribeSpaceOutput, error) {
			return nil, errors.New("describe error")
		},
	}

	_, err := listUserProfileSpaces(context.Background(), client, "d-123", "alice", false)
	if err == nil || !strings.Contains(err.Error(), "describe error") {
		t.Fatalf("expected describe error, got: %v", err)
	}
}

func TestCleanupSpacesMultipleDomainsSorted(t *testing.T) {
	deleted := make([]string, 0)
	client := &mockClient{
		listDomainsFn: func(_ context.Context, _ *sagemaker.ListDomainsInput, _ ...func(*sagemaker.Options)) (*sagemaker.ListDomainsOutput, error) {
			return &sagemaker.ListDomainsOutput{Domains: []sagemakertypes.DomainDetails{
				{DomainId: cliutil.Ptr("d-bbb")},
				{DomainId: cliutil.Ptr("d-aaa")},
			}}, nil
		},
		listSpacesFn: func(_ context.Context, in *sagemaker.ListSpacesInput, _ ...func(*sagemaker.Options)) (*sagemaker.ListSpacesOutput, error) {
			domainID := cliutil.PointerToString(in.DomainIdEquals)
			switch domainID {
			case "d-aaa":
				return &sagemaker.ListSpacesOutput{Spaces: []sagemakertypes.SpaceDetails{
					{SpaceName: cliutil.Ptr("space-z"), Status: sagemakertypes.SpaceStatusInService},
					{SpaceName: cliutil.Ptr("space-a"), Status: sagemakertypes.SpaceStatusInService},
				}}, nil
			case "d-bbb":
				return &sagemaker.ListSpacesOutput{Spaces: []sagemakertypes.SpaceDetails{
					{SpaceName: cliutil.Ptr("space-m"), Status: sagemakertypes.SpaceStatusInService},
				}}, nil
			default:
				t.Fatalf("unexpected domain: %s", domainID)
				return nil, nil
			}
		},
		deleteSpaceFn: func(_ context.Context, in *sagemaker.DeleteSpaceInput, _ ...func(*sagemaker.Options)) (*sagemaker.DeleteSpaceOutput, error) {
			deleted = append(deleted, cliutil.PointerToString(in.DomainId)+"/"+cliutil.PointerToString(in.SpaceName))
			return &sagemaker.DeleteSpaceOutput{}, nil
		},
	}

	withMockDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) API { return client },
	)

	output, err := executeCommand(t, "--output", "json", "--no-confirm", "sagemaker", "cleanup-spaces")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Targets should be sorted: d-aaa/space-a, d-aaa/space-z, d-bbb/space-m
	if len(deleted) != 3 {
		t.Fatalf("expected 3 deletes, got %d", len(deleted))
	}
	if deleted[0] != "d-aaa/space-a" || deleted[1] != "d-aaa/space-z" || deleted[2] != "d-bbb/space-m" {
		t.Fatalf("expected sorted delete order, got %v", deleted)
	}
	if !strings.Contains(output, "d-aaa") && !strings.Contains(output, "d-bbb") {
		t.Fatalf("expected both domains in output: %s", output)
	}
}

func TestWaitForUserProfileDependenciesContextCancelled(t *testing.T) {
	client := &mockClient{
		listAppsFn: func(_ context.Context, _ *sagemaker.ListAppsInput, _ ...func(*sagemaker.Options)) (*sagemaker.ListAppsOutput, error) {
			// Always return an active app to keep waiting
			return &sagemaker.ListAppsOutput{Apps: []sagemakertypes.AppDetails{
				{AppName: cliutil.Ptr("stuck"), AppType: sagemakertypes.AppTypeJupyterServer, Status: sagemakertypes.AppStatusDeleting},
			}}, nil
		},
		listSpacesFn: func(_ context.Context, _ *sagemaker.ListSpacesInput, _ ...func(*sagemaker.Options)) (*sagemaker.ListSpacesOutput, error) {
			return &sagemaker.ListSpacesOutput{Spaces: []sagemakertypes.SpaceDetails{}}, nil
		},
	}

	oldSleep := sleep
	sleep = func(_ time.Duration) {}
	t.Cleanup(func() { sleep = oldSleep })

	ctx, cancel := context.WithCancel(context.Background())
	// Cancel immediately after first sleep to trigger ctx.Done() branch
	sleepCount := 0
	sleep = func(_ time.Duration) {
		sleepCount++
		if sleepCount >= 1 {
			cancel()
		}
	}

	err := waitForUserProfileDependenciesDeleted(ctx, client, "d-123", "alice")
	if err == nil {
		t.Fatal("expected context cancellation error")
	}
	if !strings.Contains(err.Error(), "context canceled") {
		t.Fatalf("expected context canceled error, got: %v", err)
	}
}
