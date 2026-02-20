package cli

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	cloudformationtypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/aws/aws-sdk-go-v2/service/efs"
	efstypes "github.com/aws/aws-sdk-go-v2/service/efs/types"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	kmstypes "github.com/aws/aws-sdk-go-v2/service/kms/types"
	organizationtypes "github.com/aws/aws-sdk-go-v2/service/organizations/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/aws-sdk-go-v2/service/sagemaker"
	sagemakertypes "github.com/aws/aws-sdk-go-v2/service/sagemaker/types"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/ssoadmin"
	ssoadmintypes "github.com/aws/aws-sdk-go-v2/service/ssoadmin/types"
	"github.com/spf13/cobra"
)

type mockCoverageECSClient struct {
	listTaskDefinitionsFn   func(context.Context, *ecs.ListTaskDefinitionsInput, ...func(*ecs.Options)) (*ecs.ListTaskDefinitionsOutput, error)
	deleteTaskDefinitionsFn func(context.Context, *ecs.DeleteTaskDefinitionsInput, ...func(*ecs.Options)) (*ecs.DeleteTaskDefinitionsOutput, error)
}

func (m *mockCoverageECSClient) DeleteTaskDefinitions(ctx context.Context, in *ecs.DeleteTaskDefinitionsInput, optFns ...func(*ecs.Options)) (*ecs.DeleteTaskDefinitionsOutput, error) {
	if m.deleteTaskDefinitionsFn == nil {
		return nil, errors.New("DeleteTaskDefinitions not mocked")
	}
	return m.deleteTaskDefinitionsFn(ctx, in, optFns...)
}

func (m *mockCoverageECSClient) ListTaskDefinitions(ctx context.Context, in *ecs.ListTaskDefinitionsInput, optFns ...func(*ecs.Options)) (*ecs.ListTaskDefinitionsOutput, error) {
	if m.listTaskDefinitionsFn == nil {
		return nil, errors.New("ListTaskDefinitions not mocked")
	}
	return m.listTaskDefinitionsFn(ctx, in, optFns...)
}

func withMockCoverageECSDeps(t *testing.T, loader func(string, string) (awssdk.Config, error), newClient func(awssdk.Config) ecsAPI) {
	t.Helper()

	oldLoader := ecsLoadAWSConfig
	oldNewClient := ecsNewClient
	ecsLoadAWSConfig = loader
	ecsNewClient = newClient

	t.Cleanup(func() {
		ecsLoadAWSConfig = oldLoader
		ecsNewClient = oldNewClient
	})
}

func TestCoverageBoostECSDeleteTaskDefinitionsDryRunAndExecute(t *testing.T) {
	deleteCalls := 0
	client := &mockCoverageECSClient{
		listTaskDefinitionsFn: func(_ context.Context, in *ecs.ListTaskDefinitionsInput, _ ...func(*ecs.Options)) (*ecs.ListTaskDefinitionsOutput, error) {
			if in.NextToken == nil {
				return &ecs.ListTaskDefinitionsOutput{
					TaskDefinitionArns: []string{"arn:aws:ecs:us-east-1:123:task-definition/app:2"},
					NextToken:          ptr("n2"),
				}, nil
			}
			return &ecs.ListTaskDefinitionsOutput{
				TaskDefinitionArns: []string{"arn:aws:ecs:us-east-1:123:task-definition/app:1"},
			}, nil
		},
		deleteTaskDefinitionsFn: func(_ context.Context, _ *ecs.DeleteTaskDefinitionsInput, _ ...func(*ecs.Options)) (*ecs.DeleteTaskDefinitionsOutput, error) {
			deleteCalls++
			return &ecs.DeleteTaskDefinitionsOutput{}, nil
		},
	}

	withMockCoverageECSDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) ecsAPI { return client },
	)

	dryRunOutput, err := executeCommand(t, "--output", "json", "--dry-run", "ecs", "delete-task-definitions")
	if err != nil {
		t.Fatalf("execute dry-run delete-task-definitions: %v", err)
	}
	if deleteCalls != 0 || !strings.Contains(dryRunOutput, "would-delete") {
		t.Fatalf("unexpected dry-run output: %s", dryRunOutput)
	}

	runOutput, err := executeCommand(t, "--output", "json", "--no-confirm", "ecs", "delete-task-definitions")
	if err != nil {
		t.Fatalf("execute delete-task-definitions: %v", err)
	}
	if deleteCalls != 2 || !strings.Contains(runOutput, "deleted") {
		t.Fatalf("unexpected execution output: %s", runOutput)
	}
}

func TestCoverageBoostS3DownloadListAndDeleteFlows(t *testing.T) {
	now := time.Now().UTC()
	deleteObjectCalls := 0
	deleteBucketCalls := 0
	getObjectCalls := 0

	client := &mockS3Client{
		listBucketsFn: func(_ context.Context, _ *s3.ListBucketsInput, _ ...func(*s3.Options)) (*s3.ListBucketsOutput, error) {
			return &s3.ListBucketsOutput{Buckets: []s3types.Bucket{{Name: ptr("demo-bucket")}}}, nil
		},
		listObjectsV2Fn: func(_ context.Context, in *s3.ListObjectsV2Input, _ ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
			bucket := pointerToString(in.Bucket)
			if bucket != "demo-bucket" {
				return nil, errors.New("unexpected bucket")
			}
			if in.Prefix != nil {
				return &s3.ListObjectsV2Output{
					Contents: []s3types.Object{{Key: ptr("logs/app.log"), LastModified: &now, Size: ptr(int64(9))}},
				}, nil
			}
			return &s3.ListObjectsV2Output{
				Contents: []s3types.Object{{Key: ptr("object-a")}},
			}, nil
		},
		listObjectVersionsFn: func(_ context.Context, _ *s3.ListObjectVersionsInput, _ ...func(*s3.Options)) (*s3.ListObjectVersionsOutput, error) {
			return &s3.ListObjectVersionsOutput{
				Versions: []s3types.ObjectVersion{{Key: ptr("object-a"), VersionId: ptr("v1")}},
			}, nil
		},
		deleteObjectsFn: func(_ context.Context, _ *s3.DeleteObjectsInput, _ ...func(*s3.Options)) (*s3.DeleteObjectsOutput, error) {
			deleteObjectCalls++
			return &s3.DeleteObjectsOutput{}, nil
		},
		deleteBucketFn: func(_ context.Context, _ *s3.DeleteBucketInput, _ ...func(*s3.Options)) (*s3.DeleteBucketOutput, error) {
			deleteBucketCalls++
			return &s3.DeleteBucketOutput{}, nil
		},
		getObjectFn: func(_ context.Context, _ *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
			getObjectCalls++
			return &s3.GetObjectOutput{Body: io.NopCloser(strings.NewReader("log-data"))}, nil
		},
	}

	withMockS3Deps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) s3API { return client },
	)

	downloadDir := t.TempDir()
	downloadOut, err := executeCommand(t, "--output", "json", "s3", "download-bucket", "--bucket-name", "demo-bucket", "--prefix", "logs/", "--output-dir", downloadDir)
	if err != nil {
		t.Fatalf("execute download-bucket: %v", err)
	}
	if !strings.Contains(downloadOut, "downloaded") || getObjectCalls == 0 {
		t.Fatalf("unexpected download output: %s", downloadOut)
	}

	listOut, err := executeCommand(t, "--output", "json", "s3", "list-old-files", "--bucket-name", "demo-bucket", "--prefix", "logs/", "--older-than-days", "0")
	if err != nil {
		t.Fatalf("execute list-old-files: %v", err)
	}
	if !strings.Contains(listOut, "logs/app.log") {
		t.Fatalf("unexpected list-old-files output: %s", listOut)
	}

	deleteOut, err := executeCommand(t, "--output", "json", "--no-confirm", "s3", "delete-buckets", "--filter-name-contains", "demo")
	if err != nil {
		t.Fatalf("execute delete-buckets: %v", err)
	}
	if deleteBucketCalls != 1 || deleteObjectCalls < 2 || !strings.Contains(deleteOut, "deleted") {
		t.Fatalf("unexpected delete output: %s", deleteOut)
	}
}

func TestCoverageBoostSSMDeleteParametersAndReaders(t *testing.T) {
	deleteCalls := 0
	client := &mockSSMClient{
		deleteParameterFn: func(_ context.Context, _ *ssm.DeleteParameterInput, _ ...func(*ssm.Options)) (*ssm.DeleteParameterOutput, error) {
			deleteCalls++
			return &ssm.DeleteParameterOutput{}, nil
		},
	}

	withMockSSMDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) ssmAPI { return client },
	)

	inputPath := filepath.Join(t.TempDir(), "names.json")
	if err := os.WriteFile(inputPath, []byte(`["/app/a","/app/b"]`), 0o600); err != nil {
		t.Fatalf("write names file: %v", err)
	}

	out, err := executeCommand(t, "--output", "json", "--no-confirm", "ssm", "delete-parameters", "--input-file", inputPath)
	if err != nil {
		t.Fatalf("execute delete-parameters: %v", err)
	}
	if deleteCalls != 2 || !strings.Contains(out, "deleted") {
		t.Fatalf("unexpected delete-parameters output: %s", out)
	}

	recordsPath := filepath.Join(t.TempDir(), "records.json")
	if err := os.WriteFile(recordsPath, []byte(`{"parameters":[{"Name":"/x"},{"name":"/y"}]}`), 0o600); err != nil {
		t.Fatalf("write records file: %v", err)
	}
	names, err := readSSMParameterNamesFile(recordsPath)
	if err != nil {
		t.Fatalf("readSSMParameterNamesFile: %v", err)
	}
	if len(names) != 2 {
		t.Fatalf("expected 2 names, got %d", len(names))
	}
}

func TestCoverageBoostUtilityHelpers(t *testing.T) {
	if got := failedAction(nil); !strings.HasPrefix(got, "failed:") {
		t.Fatalf("expected failed action value, got %q", got)
	}
	if got := skippedActionMessage(""); got != "skipped:skipped" {
		t.Fatalf("unexpected skipped action default: %q", got)
	}
	if got := failedActionMessage(""); got != "failed:unknown" {
		t.Fatalf("unexpected failed action default: %q", got)
	}

	rows := [][]string{{"x", "pending"}}
	if err := setActionForRow(rows, 0, 1, actionDeleted); err != nil {
		t.Fatalf("setActionForRow success: %v", err)
	}
	if rows[0][1] != actionDeleted {
		t.Fatalf("unexpected row state: %#v", rows)
	}
	if err := setActionForRow(rows, 3, 1, actionDeleted); err == nil {
		t.Fatal("expected out-of-bounds error")
	}
}

func TestCoverageBoostEFSAndKMSFilterTagPaths(t *testing.T) {
	efsDeletes := 0
	efsClient := &mockMilestone5EFSClient{
		describeFileSystemsFn: func(_ context.Context, _ *efs.DescribeFileSystemsInput, _ ...func(*efs.Options)) (*efs.DescribeFileSystemsOutput, error) {
			return &efs.DescribeFileSystemsOutput{FileSystems: []efstypes.FileSystemDescription{{FileSystemId: ptr("fs-1")}}}, nil
		},
		listTagsForResourceFn: func(_ context.Context, _ *efs.ListTagsForResourceInput, _ ...func(*efs.Options)) (*efs.ListTagsForResourceOutput, error) {
			return &efs.ListTagsForResourceOutput{Tags: []efstypes.Tag{{Key: ptr("env"), Value: ptr("dev")}}}, nil
		},
		describeMountTargetsFn: func(_ context.Context, _ *efs.DescribeMountTargetsInput, _ ...func(*efs.Options)) (*efs.DescribeMountTargetsOutput, error) {
			return &efs.DescribeMountTargetsOutput{}, nil
		},
		deleteFileSystemFn: func(_ context.Context, _ *efs.DeleteFileSystemInput, _ ...func(*efs.Options)) (*efs.DeleteFileSystemOutput, error) {
			efsDeletes++
			return &efs.DeleteFileSystemOutput{}, nil
		},
	}
	withMockMilestone5EFSDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) efsAPI { return efsClient },
	)

	efsOutput, err := executeCommand(t, "--output", "json", "--no-confirm", "efs", "delete-filesystems", "--filter-tag", "env=dev")
	if err != nil {
		t.Fatalf("execute efs delete-filesystems: %v", err)
	}
	if efsDeletes != 1 || !strings.Contains(efsOutput, "deleted") {
		t.Fatalf("unexpected efs output: %s", efsOutput)
	}

	kmsScheduled := 0
	kmsClient := &mockMilestone5KMSClient{
		listKeysFn: func(_ context.Context, _ *kms.ListKeysInput, _ ...func(*kms.Options)) (*kms.ListKeysOutput, error) {
			return &kms.ListKeysOutput{Keys: []kmstypes.KeyListEntry{{KeyId: ptr("key-1")}}}, nil
		},
		describeKeyFn: func(_ context.Context, _ *kms.DescribeKeyInput, _ ...func(*kms.Options)) (*kms.DescribeKeyOutput, error) {
			return &kms.DescribeKeyOutput{
				KeyMetadata: &kmstypes.KeyMetadata{
					KeyId:      ptr("key-1"),
					KeyManager: kmstypes.KeyManagerTypeCustomer,
					KeyState:   kmstypes.KeyStateDisabled,
				},
			}, nil
		},
		listResourceTagsFn: func(_ context.Context, _ *kms.ListResourceTagsInput, _ ...func(*kms.Options)) (*kms.ListResourceTagsOutput, error) {
			return &kms.ListResourceTagsOutput{Tags: []kmstypes.Tag{{TagKey: ptr("env"), TagValue: ptr("dev")}}}, nil
		},
		scheduleKeyDeletionFn: func(_ context.Context, _ *kms.ScheduleKeyDeletionInput, _ ...func(*kms.Options)) (*kms.ScheduleKeyDeletionOutput, error) {
			kmsScheduled++
			return &kms.ScheduleKeyDeletionOutput{}, nil
		},
	}
	withMockMilestone5KMSDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) kmsAPI { return kmsClient },
	)

	kmsOutput, err := executeCommand(t, "--output", "json", "--no-confirm", "kms", "delete-keys", "--filter-tag", "env=dev")
	if err != nil {
		t.Fatalf("execute kms delete-keys: %v", err)
	}
	if kmsScheduled != 1 || !strings.Contains(kmsOutput, "deleted") {
		t.Fatalf("unexpected kms output: %s", kmsOutput)
	}
}

type mockCoverageFailRunner struct{}

func (mockCoverageFailRunner) Run(_ context.Context, _ string, _ []string, _ string) (string, error) {
	return "", errors.New("boom")
}

func TestCoverageBoostECSRunnerAndHelpers(t *testing.T) {
	// Cover markRowsSkipped and ecsFailureReason helper branches.
	rows := [][]string{{"x", "cmd", "pending"}, {"y", "cmd", "completed"}}
	markRowsSkipped(rows, 0)
	if !strings.HasPrefix(rows[0][2], "skipped:") {
		t.Fatalf("unexpected skip marker: %#v", rows)
	}
	if reason := ecsFailureReason(ecstypes.Failure{Reason: ptr("BadRequest"), Detail: ptr("denied")}); !strings.Contains(reason, "BadRequest") {
		t.Fatalf("unexpected ecs failure reason: %s", reason)
	}

	// Cover system runner success and failure paths.
	runner := systemECSCommandRunner{}
	if _, err := runner.Run(context.Background(), "echo", []string{"ok"}, ""); err != nil {
		t.Fatalf("system runner success path: %v", err)
	}
	if _, err := runner.Run(context.Background(), "definitely-not-a-real-binary", nil, ""); err == nil {
		t.Fatal("expected system runner error path")
	}

	// Cover publish-image failure path with skipped rows.
	oldRunner := ecsRunner
	ecsRunner = mockCoverageFailRunner{}
	t.Cleanup(func() { ecsRunner = oldRunner })

	output, err := executeCommand(t, "--output", "json", "ecs", "publish-image", "--ecr-url", "123456789012.dkr.ecr.us-east-1.amazonaws.com/app")
	if err != nil {
		t.Fatalf("execute ecs publish-image failure path: %v", err)
	}
	if !strings.Contains(output, "failed:") || !strings.Contains(output, "skipped:") {
		t.Fatalf("unexpected ecs failure output: %s", output)
	}
}

func TestCoverageBoostRootAndOrgHelpers(t *testing.T) {
	// Exercise root.Execute wrapper.
	oldArgs := os.Args
	os.Args = []string{"awstbx", "--version"}
	t.Cleanup(func() { os.Args = oldArgs })
	if err := Execute(); err != nil {
		t.Fatalf("Execute --version: %v", err)
	}

	// Exercise default example fallback paths.
	fallbackLeaf := &cobra.Command{Use: "leaf"}
	fallbackParent := &cobra.Command{Use: "parent"}
	fallbackParent.AddCommand(fallbackLeaf)
	example := defaultCommandExample(fallbackParent)
	if !strings.Contains(example, "parent --output table") {
		t.Fatalf("unexpected fallback example: %s", example)
	}

	// Exercise org helper branches.
	accounts := []organizationtypes.Account{{Id: ptr("222222222222")}, {Id: ptr("111111111111")}}
	sortAccountsByID(accounts)
	if pointerToString(accounts[0].Id) != "111111111111" {
		t.Fatalf("unexpected account sort order: %#v", accounts)
	}
	if _, err := ssoPrincipalTypeFromString("invalid"); err == nil {
		t.Fatal("expected principal type validation error")
	}
	if typ, err := ssoPrincipalTypeFromString("user"); err != nil || typ != ssoadmintypes.PrincipalTypeUser {
		t.Fatalf("unexpected principal type parse result: %v %v", typ, err)
	}

	// Cover CFN matcher helper.
	resource := cloudformationtypes.StackResourceSummary{
		LogicalResourceId:  ptr("BucketResource"),
		PhysicalResourceId: ptr("bucket-123"),
		ResourceType:       ptr("AWS::S3::Bucket"),
	}
	if !stackResourceMatches(resource, "bucketresource", true) {
		t.Fatal("expected exact resource match")
	}
	if !stackResourceMatches(resource, "s3::bucket", false) {
		t.Fatal("expected fuzzy resource match")
	}
	if stackResourceMatches(resource, "ddb", false) {
		t.Fatal("unexpected false-positive resource match")
	}
}

func TestCoverageBoostSageMakerKMSAndEFSBranches(t *testing.T) {
	// SageMaker cleanup validation + cancelled prompt branch.
	if _, err := executeCommand(t, "sagemaker", "cleanup-spaces", "--spaces", "one"); err == nil {
		t.Fatal("expected cleanup-spaces validation error when --domain-id is missing")
	}

	sageMakerClient := &mockMilestone5SageMakerClient{
		listSpacesFn: func(_ context.Context, _ *sagemaker.ListSpacesInput, _ ...func(*sagemaker.Options)) (*sagemaker.ListSpacesOutput, error) {
			return &sagemaker.ListSpacesOutput{Spaces: []sagemakertypes.SpaceDetails{{SpaceName: ptr("space-a"), Status: sagemakertypes.SpaceStatusInService}}}, nil
		},
	}
	withMockMilestone5SageMakerDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) sageMakerAPI { return sageMakerClient },
	)
	cancelOut, err := executeCommandWithInput(t, "n\n", "--output", "json", "sagemaker", "cleanup-spaces", "--domain-id", "d-123")
	if err != nil {
		t.Fatalf("execute cleanup-spaces cancellation: %v", err)
	}
	if !strings.Contains(cancelOut, "cancelled") {
		t.Fatalf("unexpected cleanup-spaces cancel output: %s", cancelOut)
	}

	// KMS validation and no-target branch.
	if _, err := executeCommand(t, "kms", "delete-keys"); err == nil {
		t.Fatal("expected kms mode validation error")
	}
	if _, err := executeCommand(t, "kms", "delete-keys", "--unused", "--filter-tag", "env=dev"); err == nil {
		t.Fatal("expected kms mutually-exclusive flag validation error")
	}

	kmsNoTargetClient := &mockMilestone5KMSClient{
		listKeysFn: func(_ context.Context, _ *kms.ListKeysInput, _ ...func(*kms.Options)) (*kms.ListKeysOutput, error) {
			return &kms.ListKeysOutput{Keys: []kmstypes.KeyListEntry{{KeyId: ptr("key-2")}}}, nil
		},
		describeKeyFn: func(_ context.Context, _ *kms.DescribeKeyInput, _ ...func(*kms.Options)) (*kms.DescribeKeyOutput, error) {
			return &kms.DescribeKeyOutput{
				KeyMetadata: &kmstypes.KeyMetadata{
					KeyId:      ptr("key-2"),
					KeyManager: kmstypes.KeyManagerTypeCustomer,
					KeyState:   kmstypes.KeyStateEnabled,
				},
			}, nil
		},
		listResourceTagsFn: func(_ context.Context, _ *kms.ListResourceTagsInput, _ ...func(*kms.Options)) (*kms.ListResourceTagsOutput, error) {
			return &kms.ListResourceTagsOutput{Tags: []kmstypes.Tag{{TagKey: ptr("team"), TagValue: ptr("platform")}}}, nil
		},
	}
	withMockMilestone5KMSDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) kmsAPI { return kmsNoTargetClient },
	)
	kmsOut, err := executeCommand(t, "--output", "json", "kms", "delete-keys", "--filter-tag", "env=dev", "--dry-run")
	if err != nil {
		t.Fatalf("execute kms no-target branch: %v", err)
	}
	if !strings.Contains(kmsOut, "[]") {
		t.Fatalf("unexpected kms no-target output: %s", kmsOut)
	}

	// EFS no-target branch with filter tag mismatch.
	efsNoTargetClient := &mockMilestone5EFSClient{
		describeFileSystemsFn: func(_ context.Context, _ *efs.DescribeFileSystemsInput, _ ...func(*efs.Options)) (*efs.DescribeFileSystemsOutput, error) {
			return &efs.DescribeFileSystemsOutput{FileSystems: []efstypes.FileSystemDescription{{FileSystemId: ptr("fs-2")}}}, nil
		},
		listTagsForResourceFn: func(_ context.Context, _ *efs.ListTagsForResourceInput, _ ...func(*efs.Options)) (*efs.ListTagsForResourceOutput, error) {
			return &efs.ListTagsForResourceOutput{Tags: []efstypes.Tag{{Key: ptr("env"), Value: ptr("prod")}}}, nil
		},
	}
	withMockMilestone5EFSDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) efsAPI { return efsNoTargetClient },
	)
	efsOut, err := executeCommand(t, "--output", "json", "--dry-run", "efs", "delete-filesystems", "--filter-tag", "env=dev")
	if err != nil {
		t.Fatalf("execute efs no-target branch: %v", err)
	}
	if !strings.Contains(efsOut, "[]") {
		t.Fatalf("unexpected efs no-target output: %s", efsOut)
	}
}

func TestCoverageBoostRuntimeAndDefaultExamples(t *testing.T) {
	// newServiceRuntime success and error branches.
	root := NewRootCommand()
	root.SetContext(context.Background())
	cmd, _, findErr := root.Find([]string{"ec2", "list-eips"})
	if findErr != nil {
		t.Fatalf("find subcommand: %v", findErr)
	}

	_, _, _, err := newServiceRuntime(cmd, func(_, _ string) (awssdk.Config, error) {
		return awssdk.Config{}, errors.New("load failed")
	}, func(awssdk.Config) struct{} { return struct{}{} })
	if err == nil {
		t.Fatal("expected newServiceRuntime loader error")
	}

	runtime, _, client, err := newServiceRuntime(cmd, func(_, _ string) (awssdk.Config, error) {
		return awssdk.Config{Region: "us-east-1"}, nil
	}, func(cfg awssdk.Config) string { return cfg.Region })
	if err != nil || client != "us-east-1" || runtime.Options.OutputFormat == "" {
		t.Fatalf("unexpected newServiceRuntime success result: runtime=%+v client=%q err=%v", runtime, client, err)
	}

	// defaultCommandExample map, subcommand, and fallback cases.
	rootExample := defaultCommandExample(NewRootCommand())
	if !strings.Contains(rootExample, "awstbx ec2 list-eips") {
		t.Fatalf("unexpected root example: %s", rootExample)
	}

	parent := &cobra.Command{Use: "custom"}
	child := &cobra.Command{Use: "child", RunE: func(*cobra.Command, []string) error { return nil }}
	parent.AddCommand(child)
	if got := defaultCommandExample(parent); !strings.Contains(got, "custom --help") {
		t.Fatalf("unexpected subcommand default example: %s", got)
	}

	leaf := &cobra.Command{Use: "leaf", RunE: func(*cobra.Command, []string) error { return nil }}
	if got := defaultCommandExample(leaf); !strings.Contains(got, "leaf --output table") {
		t.Fatalf("unexpected leaf default example: %s", got)
	}

	// newServiceGroupCommand RunE/help path.
	groupCmd := newServiceGroupCommand("demo", "Demo service")
	groupCmd.SetArgs(nil)
	if err := groupCmd.Execute(); err != nil {
		t.Fatalf("execute service group help: %v", err)
	}
}

func TestCoverageBoostCFNWaitForOperationBranches(t *testing.T) {
	successClient := &mockCFNClient{
		describeStackSetOperation: func(_ context.Context, _ *cloudformation.DescribeStackSetOperationInput, _ ...func(*cloudformation.Options)) (*cloudformation.DescribeStackSetOperationOutput, error) {
			return &cloudformation.DescribeStackSetOperationOutput{
				StackSetOperation: &cloudformationtypes.StackSetOperation{
					Status: cloudformationtypes.StackSetOperationStatusSucceeded,
				},
			}, nil
		},
	}
	if err := waitForStackSetOperation(context.Background(), successClient, "stackset", "op-1"); err != nil {
		t.Fatalf("waitForStackSetOperation success path: %v", err)
	}

	failClient := &mockCFNClient{
		describeStackSetOperation: func(_ context.Context, _ *cloudformation.DescribeStackSetOperationInput, _ ...func(*cloudformation.Options)) (*cloudformation.DescribeStackSetOperationOutput, error) {
			return &cloudformation.DescribeStackSetOperationOutput{
				StackSetOperation: &cloudformationtypes.StackSetOperation{
					Status:       cloudformationtypes.StackSetOperationStatusFailed,
					StatusReason: ptr("dependency failure"),
				},
			}, nil
		},
	}
	if err := waitForStackSetOperation(context.Background(), failClient, "stackset", "op-2"); err == nil || !strings.Contains(err.Error(), "dependency failure") {
		t.Fatalf("expected failed operation error, got %v", err)
	}
}

func TestCoverageBoostOrgAssignmentWaitBranches(t *testing.T) {
	successClient := &mockOrgSSOAdminClient{
		describeDeletionStatusFn: func(_ context.Context, _ *ssoadmin.DescribeAccountAssignmentDeletionStatusInput, _ ...func(*ssoadmin.Options)) (*ssoadmin.DescribeAccountAssignmentDeletionStatusOutput, error) {
			return &ssoadmin.DescribeAccountAssignmentDeletionStatusOutput{
				AccountAssignmentDeletionStatus: &ssoadmintypes.AccountAssignmentOperationStatus{
					Status: ssoadmintypes.StatusValuesSucceeded,
				},
			}, nil
		},
	}
	successOut := &ssoadmin.DeleteAccountAssignmentOutput{
		AccountAssignmentDeletionStatus: &ssoadmintypes.AccountAssignmentOperationStatus{
			RequestId: ptr("req-success"),
		},
	}
	if err := waitForOrgAssignmentDeletion(context.Background(), successClient, "arn:aws:sso:::instance/ssoins-123", successOut); err != nil {
		t.Fatalf("waitForOrgAssignmentDeletion success path: %v", err)
	}

	failClient := &mockOrgSSOAdminClient{
		describeDeletionStatusFn: func(_ context.Context, _ *ssoadmin.DescribeAccountAssignmentDeletionStatusInput, _ ...func(*ssoadmin.Options)) (*ssoadmin.DescribeAccountAssignmentDeletionStatusOutput, error) {
			return &ssoadmin.DescribeAccountAssignmentDeletionStatusOutput{
				AccountAssignmentDeletionStatus: &ssoadmintypes.AccountAssignmentOperationStatus{
					Status:        ssoadmintypes.StatusValuesFailed,
					FailureReason: ptr("delete failed"),
				},
			}, nil
		},
	}
	failOut := &ssoadmin.DeleteAccountAssignmentOutput{
		AccountAssignmentDeletionStatus: &ssoadmintypes.AccountAssignmentOperationStatus{
			RequestId: ptr("req-fail"),
		},
	}
	if err := waitForOrgAssignmentDeletion(context.Background(), failClient, "arn:aws:sso:::instance/ssoins-123", failOut); err == nil || !strings.Contains(err.Error(), "delete failed") {
		t.Fatalf("expected waitForOrgAssignmentDeletion failure error, got %v", err)
	}

	if err := waitForOrgAssignmentDeletion(context.Background(), successClient, "arn:aws:sso:::instance/ssoins-123", &ssoadmin.DeleteAccountAssignmentOutput{}); err == nil {
		t.Fatal("expected missing request id error")
	}

	emptyClient := &mockOrgSSOAdminClient{
		describeDeletionStatusFn: func(_ context.Context, _ *ssoadmin.DescribeAccountAssignmentDeletionStatusInput, _ ...func(*ssoadmin.Options)) (*ssoadmin.DescribeAccountAssignmentDeletionStatusOutput, error) {
			return &ssoadmin.DescribeAccountAssignmentDeletionStatusOutput{}, nil
		},
	}
	if err := waitForOrgAssignmentDeletion(context.Background(), emptyClient, "arn:aws:sso:::instance/ssoins-123", failOut); err == nil {
		t.Fatal("expected empty deletion status error")
	}
}

func TestCoverageBoostS3HelpersAndBucketChecks(t *testing.T) {
	now := time.Now().UTC()
	if !s3ObjectLastModified(s3types.Object{LastModified: &now}).Equal(now) {
		t.Fatal("expected s3ObjectLastModified to return timestamp")
	}
	if !s3ObjectLastModified(s3types.Object{}).IsZero() {
		t.Fatal("expected zero timestamp for nil LastModified")
	}
	if s3ObjectSize(s3types.Object{Size: ptr(int64(8))}) != 8 {
		t.Fatal("expected s3ObjectSize to return object size")
	}
	if s3ObjectSize(s3types.Object{}) != 0 {
		t.Fatal("expected zero size for nil size pointer")
	}

	deleteCalls := 0
	client := &mockS3Client{
		deleteObjectsFn: func(_ context.Context, _ *s3.DeleteObjectsInput, _ ...func(*s3.Options)) (*s3.DeleteObjectsOutput, error) {
			deleteCalls++
			return &s3.DeleteObjectsOutput{}, nil
		},
	}
	if err := deleteS3ObjectBatch(context.Background(), client, "bucket-a", nil); err != nil {
		t.Fatalf("deleteS3ObjectBatch empty batch: %v", err)
	}
	if err := deleteS3ObjectBatch(context.Background(), client, "bucket-a", []s3types.ObjectIdentifier{{Key: ptr("k1")}}); err != nil {
		t.Fatalf("deleteS3ObjectBatch non-empty batch: %v", err)
	}
	if deleteCalls != 1 {
		t.Fatalf("expected one deleteObjects call, got %d", deleteCalls)
	}

	checkClient := &mockS3Client{
		listObjectsV2Fn: func(_ context.Context, _ *s3.ListObjectsV2Input, _ ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
			return &s3.ListObjectsV2Output{Contents: []s3types.Object{{Key: ptr("k1")}}}, nil
		},
	}
	empty, err := isS3BucketEmptyAndUnversioned(context.Background(), checkClient, "bucket-a")
	if err != nil || empty {
		t.Fatalf("expected non-empty bucket result, got empty=%t err=%v", empty, err)
	}

	versionClient := &mockS3Client{
		listObjectsV2Fn: func(_ context.Context, _ *s3.ListObjectsV2Input, _ ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
			return &s3.ListObjectsV2Output{}, nil
		},
		getBucketVersioningFn: func(_ context.Context, _ *s3.GetBucketVersioningInput, _ ...func(*s3.Options)) (*s3.GetBucketVersioningOutput, error) {
			return &s3.GetBucketVersioningOutput{Status: s3types.BucketVersioningStatusEnabled}, nil
		},
	}
	empty, err = isS3BucketEmptyAndUnversioned(context.Background(), versionClient, "bucket-a")
	if err != nil || empty {
		t.Fatalf("expected versioned bucket to be non-empty for deletion semantics, got empty=%t err=%v", empty, err)
	}
}

func TestCoverageBoostCompletionShellVariants(t *testing.T) {
	for _, shell := range []string{"fish", "powershell"} {
		if _, err := executeCommand(t, "completion", shell); err != nil {
			t.Fatalf("completion %s failed: %v", shell, err)
		}
	}

	// Directly invoke RunE to cover the unsupported shell fallback branch.
	cmd := newCompletionCommand()
	cmd.SetOut(io.Discard)
	root := &cobra.Command{Use: "awstbx"}
	root.AddCommand(cmd)
	cmd.SetArgs([]string{"unsupported"})
	if err := cmd.RunE(cmd, []string{"unsupported"}); err == nil {
		t.Fatal("expected unsupported shell error")
	}
}

func TestCoverageBoostRootExecutionBranches(t *testing.T) {
	if _, err := executeCommand(t, "--output", "xml"); err == nil {
		t.Fatal("expected invalid output format error")
	}

	out, err := executeCommand(t)
	if err != nil {
		t.Fatalf("root help execution failed: %v", err)
	}
	if !strings.Contains(out, "Available Commands:") {
		t.Fatalf("unexpected root help output: %s", out)
	}
}

func TestCoverageBoostLegacyFlagsRejected(t *testing.T) {
	cases := [][]string{
		{"appstream", "delete-image", "--name", "img-a"},
		{"cfn", "delete-stackset", "--name", "stackset-a"},
		{"cloudwatch", "delete-log-groups", "--keep", "30d"},
		{"ec2", "delete-security-groups", "--tag", "env=dev"},
		{"kms", "delete-keys", "--tag", "env=dev"},
		{"org", "set-alternate-contact", "--contacts-file", "contacts.json"},
		{"s3", "search-objects", "--bucket", "my-bucket", "--keys", "foo"},
	}

	for _, args := range cases {
		_, err := executeCommand(t, args...)
		if err == nil {
			t.Fatalf("expected legacy flag rejection for args: %v", args)
		}
		if !strings.Contains(err.Error(), "unknown flag") {
			t.Fatalf("expected unknown flag error for args=%v, got %v", args, err)
		}
	}
}
