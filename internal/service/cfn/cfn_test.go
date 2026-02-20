package cfn

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	cloudformationtypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	"github.com/towardsthecloud/aws-toolbox/internal/cliutil"
)

type mockClient struct {
	deleteStackInstancesFn    func(context.Context, *cloudformation.DeleteStackInstancesInput, ...func(*cloudformation.Options)) (*cloudformation.DeleteStackInstancesOutput, error)
	deleteStackSetFn          func(context.Context, *cloudformation.DeleteStackSetInput, ...func(*cloudformation.Options)) (*cloudformation.DeleteStackSetOutput, error)
	describeStackSetOperation func(context.Context, *cloudformation.DescribeStackSetOperationInput, ...func(*cloudformation.Options)) (*cloudformation.DescribeStackSetOperationOutput, error)
	describeStacksFn          func(context.Context, *cloudformation.DescribeStacksInput, ...func(*cloudformation.Options)) (*cloudformation.DescribeStacksOutput, error)
	listStackInstancesFn      func(context.Context, *cloudformation.ListStackInstancesInput, ...func(*cloudformation.Options)) (*cloudformation.ListStackInstancesOutput, error)
	listStackResourcesFn      func(context.Context, *cloudformation.ListStackResourcesInput, ...func(*cloudformation.Options)) (*cloudformation.ListStackResourcesOutput, error)
}

func (m *mockClient) DeleteStackInstances(ctx context.Context, in *cloudformation.DeleteStackInstancesInput, optFns ...func(*cloudformation.Options)) (*cloudformation.DeleteStackInstancesOutput, error) {
	if m.deleteStackInstancesFn == nil {
		return nil, errors.New("DeleteStackInstances not mocked")
	}
	return m.deleteStackInstancesFn(ctx, in, optFns...)
}

func (m *mockClient) DeleteStackSet(ctx context.Context, in *cloudformation.DeleteStackSetInput, optFns ...func(*cloudformation.Options)) (*cloudformation.DeleteStackSetOutput, error) {
	if m.deleteStackSetFn == nil {
		return nil, errors.New("DeleteStackSet not mocked")
	}
	return m.deleteStackSetFn(ctx, in, optFns...)
}

func (m *mockClient) DescribeStackSetOperation(ctx context.Context, in *cloudformation.DescribeStackSetOperationInput, optFns ...func(*cloudformation.Options)) (*cloudformation.DescribeStackSetOperationOutput, error) {
	if m.describeStackSetOperation == nil {
		return nil, errors.New("DescribeStackSetOperation not mocked")
	}
	return m.describeStackSetOperation(ctx, in, optFns...)
}

func (m *mockClient) DescribeStacks(ctx context.Context, in *cloudformation.DescribeStacksInput, optFns ...func(*cloudformation.Options)) (*cloudformation.DescribeStacksOutput, error) {
	if m.describeStacksFn == nil {
		return nil, errors.New("DescribeStacks not mocked")
	}
	return m.describeStacksFn(ctx, in, optFns...)
}

func (m *mockClient) ListStackInstances(ctx context.Context, in *cloudformation.ListStackInstancesInput, optFns ...func(*cloudformation.Options)) (*cloudformation.ListStackInstancesOutput, error) {
	if m.listStackInstancesFn == nil {
		return nil, errors.New("ListStackInstances not mocked")
	}
	return m.listStackInstancesFn(ctx, in, optFns...)
}

func (m *mockClient) ListStackResources(ctx context.Context, in *cloudformation.ListStackResourcesInput, optFns ...func(*cloudformation.Options)) (*cloudformation.ListStackResourcesOutput, error) {
	if m.listStackResourcesFn == nil {
		return nil, errors.New("ListStackResources not mocked")
	}
	return m.listStackResourcesFn(ctx, in, optFns...)
}

func withMockDeps(t *testing.T, loader func(string, string) (awssdk.Config, error), nc func(awssdk.Config) API) {
	t.Helper()

	oldLoader := loadAWSConfig
	oldNewClient := newClient
	oldSleep := sleep

	loadAWSConfig = loader
	newClient = nc
	sleep = func(_ time.Duration) {}

	t.Cleanup(func() {
		loadAWSConfig = oldLoader
		newClient = oldNewClient
		sleep = oldSleep
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

func TestDeleteStackSetRequiresName(t *testing.T) {
	_, err := executeCommand(t, "cfn", "delete-stackset")
	if err == nil || !strings.Contains(err.Error(), "--stackset-name is required") {
		t.Fatalf("expected required name error, got %v", err)
	}
}

func TestDeleteStackSetDryRun(t *testing.T) {
	deleteInstancesCalls := 0
	deleteStackSetCalls := 0

	client := &mockClient{
		listStackInstancesFn: func(_ context.Context, _ *cloudformation.ListStackInstancesInput, _ ...func(*cloudformation.Options)) (*cloudformation.ListStackInstancesOutput, error) {
			return &cloudformation.ListStackInstancesOutput{Summaries: []cloudformationtypes.StackInstanceSummary{{Account: cliutil.Ptr("111111111111"), Region: cliutil.Ptr("us-east-1")}}}, nil
		},
		deleteStackInstancesFn: func(_ context.Context, _ *cloudformation.DeleteStackInstancesInput, _ ...func(*cloudformation.Options)) (*cloudformation.DeleteStackInstancesOutput, error) {
			deleteInstancesCalls++
			return &cloudformation.DeleteStackInstancesOutput{}, nil
		},
		deleteStackSetFn: func(_ context.Context, _ *cloudformation.DeleteStackSetInput, _ ...func(*cloudformation.Options)) (*cloudformation.DeleteStackSetOutput, error) {
			deleteStackSetCalls++
			return &cloudformation.DeleteStackSetOutput{}, nil
		},
		describeStackSetOperation: func(_ context.Context, _ *cloudformation.DescribeStackSetOperationInput, _ ...func(*cloudformation.Options)) (*cloudformation.DescribeStackSetOperationOutput, error) {
			return &cloudformation.DescribeStackSetOperationOutput{}, nil
		},
		describeStacksFn: func(_ context.Context, _ *cloudformation.DescribeStacksInput, _ ...func(*cloudformation.Options)) (*cloudformation.DescribeStacksOutput, error) {
			return &cloudformation.DescribeStacksOutput{}, nil
		},
		listStackResourcesFn: func(_ context.Context, _ *cloudformation.ListStackResourcesInput, _ ...func(*cloudformation.Options)) (*cloudformation.ListStackResourcesOutput, error) {
			return &cloudformation.ListStackResourcesOutput{}, nil
		},
	}

	withMockDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) API { return client },
	)

	output, err := executeCommand(t, "--output", "json", "--dry-run", "cfn", "delete-stackset", "--stackset-name", "stackset-a")
	if err != nil {
		t.Fatalf("execute cfn delete-stackset dry-run: %v", err)
	}
	if deleteInstancesCalls != 0 || deleteStackSetCalls != 0 {
		t.Fatalf("expected no delete calls in dry-run, got instances=%d stackset=%d", deleteInstancesCalls, deleteStackSetCalls)
	}
	if !strings.Contains(output, "would-delete") {
		t.Fatalf("unexpected output: %s", output)
	}
}

func TestDeleteStackSetNoConfirmExecutes(t *testing.T) {
	deleteInstancesCalls := 0
	describeOperationCalls := 0
	deleteStackSetCalls := 0

	client := &mockClient{
		listStackInstancesFn: func(_ context.Context, _ *cloudformation.ListStackInstancesInput, _ ...func(*cloudformation.Options)) (*cloudformation.ListStackInstancesOutput, error) {
			return &cloudformation.ListStackInstancesOutput{Summaries: []cloudformationtypes.StackInstanceSummary{{Account: cliutil.Ptr("111111111111"), Region: cliutil.Ptr("us-east-1")}}}, nil
		},
		deleteStackInstancesFn: func(_ context.Context, _ *cloudformation.DeleteStackInstancesInput, _ ...func(*cloudformation.Options)) (*cloudformation.DeleteStackInstancesOutput, error) {
			deleteInstancesCalls++
			return &cloudformation.DeleteStackInstancesOutput{OperationId: cliutil.Ptr("op-1")}, nil
		},
		describeStackSetOperation: func(_ context.Context, _ *cloudformation.DescribeStackSetOperationInput, _ ...func(*cloudformation.Options)) (*cloudformation.DescribeStackSetOperationOutput, error) {
			describeOperationCalls++
			return &cloudformation.DescribeStackSetOperationOutput{StackSetOperation: &cloudformationtypes.StackSetOperation{Status: cloudformationtypes.StackSetOperationStatusSucceeded}}, nil
		},
		deleteStackSetFn: func(_ context.Context, _ *cloudformation.DeleteStackSetInput, _ ...func(*cloudformation.Options)) (*cloudformation.DeleteStackSetOutput, error) {
			deleteStackSetCalls++
			return &cloudformation.DeleteStackSetOutput{}, nil
		},
		describeStacksFn: func(_ context.Context, _ *cloudformation.DescribeStacksInput, _ ...func(*cloudformation.Options)) (*cloudformation.DescribeStacksOutput, error) {
			return &cloudformation.DescribeStacksOutput{}, nil
		},
		listStackResourcesFn: func(_ context.Context, _ *cloudformation.ListStackResourcesInput, _ ...func(*cloudformation.Options)) (*cloudformation.ListStackResourcesOutput, error) {
			return &cloudformation.ListStackResourcesOutput{}, nil
		},
	}

	withMockDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) API { return client },
	)

	output, err := executeCommand(t, "--output", "json", "--no-confirm", "cfn", "delete-stackset", "--stackset-name", "stackset-a")
	if err != nil {
		t.Fatalf("execute cfn delete-stackset --no-confirm: %v", err)
	}
	if deleteInstancesCalls != 1 || describeOperationCalls != 1 || deleteStackSetCalls != 1 {
		t.Fatalf("unexpected API call counts instances=%d describe=%d stackset=%d", deleteInstancesCalls, describeOperationCalls, deleteStackSetCalls)
	}
	if !strings.Contains(output, "deleted") {
		t.Fatalf("unexpected output: %s", output)
	}
}

func TestDeleteStackSetWaitsBeyondLegacyTimeout(t *testing.T) {
	describeOperationCalls := 0
	client := &mockClient{
		listStackInstancesFn: func(_ context.Context, _ *cloudformation.ListStackInstancesInput, _ ...func(*cloudformation.Options)) (*cloudformation.ListStackInstancesOutput, error) {
			return &cloudformation.ListStackInstancesOutput{Summaries: []cloudformationtypes.StackInstanceSummary{{Account: cliutil.Ptr("111111111111"), Region: cliutil.Ptr("us-east-1")}}}, nil
		},
		deleteStackInstancesFn: func(_ context.Context, _ *cloudformation.DeleteStackInstancesInput, _ ...func(*cloudformation.Options)) (*cloudformation.DeleteStackInstancesOutput, error) {
			return &cloudformation.DeleteStackInstancesOutput{OperationId: cliutil.Ptr("op-1")}, nil
		},
		describeStackSetOperation: func(_ context.Context, _ *cloudformation.DescribeStackSetOperationInput, _ ...func(*cloudformation.Options)) (*cloudformation.DescribeStackSetOperationOutput, error) {
			describeOperationCalls++
			status := cloudformationtypes.StackSetOperationStatusRunning
			if describeOperationCalls >= 200 {
				status = cloudformationtypes.StackSetOperationStatusSucceeded
			}
			return &cloudformation.DescribeStackSetOperationOutput{
				StackSetOperation: &cloudformationtypes.StackSetOperation{Status: status},
			}, nil
		},
		deleteStackSetFn: func(_ context.Context, _ *cloudformation.DeleteStackSetInput, _ ...func(*cloudformation.Options)) (*cloudformation.DeleteStackSetOutput, error) {
			return &cloudformation.DeleteStackSetOutput{}, nil
		},
		describeStacksFn: func(_ context.Context, _ *cloudformation.DescribeStacksInput, _ ...func(*cloudformation.Options)) (*cloudformation.DescribeStacksOutput, error) {
			return &cloudformation.DescribeStacksOutput{}, nil
		},
		listStackResourcesFn: func(_ context.Context, _ *cloudformation.ListStackResourcesInput, _ ...func(*cloudformation.Options)) (*cloudformation.ListStackResourcesOutput, error) {
			return &cloudformation.ListStackResourcesOutput{}, nil
		},
	}

	withMockDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) API { return client },
	)

	output, err := executeCommand(t, "--output", "json", "--no-confirm", "cfn", "delete-stackset", "--stackset-name", "stackset-a")
	if err != nil {
		t.Fatalf("execute cfn delete-stackset with long-running operation: %v", err)
	}
	if describeOperationCalls != 200 {
		t.Fatalf("expected operation polling to continue until success, got %d calls", describeOperationCalls)
	}
	if !strings.Contains(output, "deleted") {
		t.Fatalf("unexpected output: %s", output)
	}
}

func TestFindStackByResourceRequiresResource(t *testing.T) {
	_, err := executeCommand(t, "cfn", "find-stack-by-resource")
	if err == nil || !strings.Contains(err.Error(), "--resource is required") {
		t.Fatalf("expected required resource error, got %v", err)
	}
}

func TestFindStackByResourceMatchesNestedWhenRequested(t *testing.T) {
	client := &mockClient{
		describeStacksFn: func(_ context.Context, _ *cloudformation.DescribeStacksInput, _ ...func(*cloudformation.Options)) (*cloudformation.DescribeStacksOutput, error) {
			return &cloudformation.DescribeStacksOutput{Stacks: []cloudformationtypes.Stack{
				{StackName: cliutil.Ptr("parent-stack")},
				{StackName: cliutil.Ptr("nested-stack"), ParentId: cliutil.Ptr("arn:aws:cloudformation:us-east-1:123:stack/parent-stack/1")},
			}}, nil
		},
		listStackResourcesFn: func(_ context.Context, in *cloudformation.ListStackResourcesInput, _ ...func(*cloudformation.Options)) (*cloudformation.ListStackResourcesOutput, error) {
			if cliutil.PointerToString(in.StackName) == "parent-stack" {
				return &cloudformation.ListStackResourcesOutput{StackResourceSummaries: []cloudformationtypes.StackResourceSummary{{LogicalResourceId: cliutil.Ptr("AppBucket"), ResourceType: cliutil.Ptr("AWS::S3::Bucket"), ResourceStatus: cloudformationtypes.ResourceStatusCreateComplete}}}, nil
			}
			return &cloudformation.ListStackResourcesOutput{StackResourceSummaries: []cloudformationtypes.StackResourceSummary{{LogicalResourceId: cliutil.Ptr("NestedBucket"), ResourceType: cliutil.Ptr("AWS::S3::Bucket"), ResourceStatus: cloudformationtypes.ResourceStatusCreateComplete}}}, nil
		},
		listStackInstancesFn: func(_ context.Context, _ *cloudformation.ListStackInstancesInput, _ ...func(*cloudformation.Options)) (*cloudformation.ListStackInstancesOutput, error) {
			return &cloudformation.ListStackInstancesOutput{}, nil
		},
		deleteStackInstancesFn: func(_ context.Context, _ *cloudformation.DeleteStackInstancesInput, _ ...func(*cloudformation.Options)) (*cloudformation.DeleteStackInstancesOutput, error) {
			return &cloudformation.DeleteStackInstancesOutput{}, nil
		},
		describeStackSetOperation: func(_ context.Context, _ *cloudformation.DescribeStackSetOperationInput, _ ...func(*cloudformation.Options)) (*cloudformation.DescribeStackSetOperationOutput, error) {
			return &cloudformation.DescribeStackSetOperationOutput{}, nil
		},
		deleteStackSetFn: func(_ context.Context, _ *cloudformation.DeleteStackSetInput, _ ...func(*cloudformation.Options)) (*cloudformation.DeleteStackSetOutput, error) {
			return &cloudformation.DeleteStackSetOutput{}, nil
		},
	}

	withMockDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) API { return client },
	)

	output, err := executeCommand(t, "--output", "json", "cfn", "find-stack-by-resource", "--resource", "AWS::S3::Bucket", "--exact", "--include-nested")
	if err != nil {
		t.Fatalf("execute cfn find-stack-by-resource: %v", err)
	}
	if !strings.Contains(output, "parent-stack") || !strings.Contains(output, "nested-stack") {
		t.Fatalf("expected both parent and nested matches, output=%s", output)
	}
}

// defaultMockLoader returns a mock AWS config loader that always succeeds.
func defaultMockLoader() func(string, string) (awssdk.Config, error) {
	return func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil }
}

// defaultMockClientFactory returns a factory that always returns the given client.
func defaultMockClientFactory(c *mockClient) func(awssdk.Config) API {
	return func(awssdk.Config) API { return c }
}

// --- runDeleteStackSet: user declines confirmation ---

func TestDeleteStackSetUserDeclines(t *testing.T) {
	client := &mockClient{
		listStackInstancesFn: func(_ context.Context, _ *cloudformation.ListStackInstancesInput, _ ...func(*cloudformation.Options)) (*cloudformation.ListStackInstancesOutput, error) {
			return &cloudformation.ListStackInstancesOutput{
				Summaries: []cloudformationtypes.StackInstanceSummary{
					{Account: cliutil.Ptr("111111111111"), Region: cliutil.Ptr("us-east-1")},
				},
			}, nil
		},
	}

	withMockDeps(t, defaultMockLoader(), defaultMockClientFactory(client))

	// Without --no-confirm, stdin is empty so the prompter reads EOF and returns false (declined).
	output, err := executeCommand(t, "--output", "json", "cfn", "delete-stackset", "--stackset-name", "my-stackset")
	if err != nil {
		t.Fatalf("expected no error when user declines, got %v", err)
	}
	if !strings.Contains(output, "cancelled") {
		t.Fatalf("expected cancelled action in output, got: %s", output)
	}
}

// --- runDeleteStackSet: listStackInstances error ---

func TestDeleteStackSetListInstancesError(t *testing.T) {
	client := &mockClient{
		listStackInstancesFn: func(_ context.Context, _ *cloudformation.ListStackInstancesInput, _ ...func(*cloudformation.Options)) (*cloudformation.ListStackInstancesOutput, error) {
			return nil, errors.New("access denied")
		},
	}

	withMockDeps(t, defaultMockLoader(), defaultMockClientFactory(client))

	_, err := executeCommand(t, "--output", "json", "--no-confirm", "cfn", "delete-stackset", "--stackset-name", "my-stackset")
	if err == nil || !strings.Contains(err.Error(), "list stack set instances") {
		t.Fatalf("expected list instances error, got %v", err)
	}
}

// --- runDeleteStackSet: instance deletion error ---

func TestDeleteStackSetInstanceDeletionError(t *testing.T) {
	client := &mockClient{
		listStackInstancesFn: func(_ context.Context, _ *cloudformation.ListStackInstancesInput, _ ...func(*cloudformation.Options)) (*cloudformation.ListStackInstancesOutput, error) {
			return &cloudformation.ListStackInstancesOutput{
				Summaries: []cloudformationtypes.StackInstanceSummary{
					{Account: cliutil.Ptr("111111111111"), Region: cliutil.Ptr("us-east-1")},
				},
			}, nil
		},
		deleteStackInstancesFn: func(_ context.Context, _ *cloudformation.DeleteStackInstancesInput, _ ...func(*cloudformation.Options)) (*cloudformation.DeleteStackInstancesOutput, error) {
			return nil, errors.New("cannot delete instance")
		},
	}

	withMockDeps(t, defaultMockLoader(), defaultMockClientFactory(client))

	output, err := executeCommand(t, "--output", "json", "--no-confirm", "cfn", "delete-stackset", "--stackset-name", "my-stackset")
	if err != nil {
		t.Fatalf("command should not return error, got %v", err)
	}
	if !strings.Contains(output, "failed:") {
		t.Fatalf("expected failed action in output, got: %s", output)
	}
	if !strings.Contains(output, "skipped:") {
		t.Fatalf("expected skipped stackset action in output, got: %s", output)
	}
}

// --- runDeleteStackSet: wait operation failure ---

func TestDeleteStackSetWaitOperationFailure(t *testing.T) {
	client := &mockClient{
		listStackInstancesFn: func(_ context.Context, _ *cloudformation.ListStackInstancesInput, _ ...func(*cloudformation.Options)) (*cloudformation.ListStackInstancesOutput, error) {
			return &cloudformation.ListStackInstancesOutput{
				Summaries: []cloudformationtypes.StackInstanceSummary{
					{Account: cliutil.Ptr("222222222222"), Region: cliutil.Ptr("eu-west-1")},
				},
			}, nil
		},
		deleteStackInstancesFn: func(_ context.Context, _ *cloudformation.DeleteStackInstancesInput, _ ...func(*cloudformation.Options)) (*cloudformation.DeleteStackInstancesOutput, error) {
			return &cloudformation.DeleteStackInstancesOutput{OperationId: cliutil.Ptr("op-fail")}, nil
		},
		describeStackSetOperation: func(_ context.Context, _ *cloudformation.DescribeStackSetOperationInput, _ ...func(*cloudformation.Options)) (*cloudformation.DescribeStackSetOperationOutput, error) {
			return &cloudformation.DescribeStackSetOperationOutput{
				StackSetOperation: &cloudformationtypes.StackSetOperation{
					Status:       cloudformationtypes.StackSetOperationStatusFailed,
					StatusReason: cliutil.Ptr("deployment failed"),
				},
			}, nil
		},
	}

	withMockDeps(t, defaultMockLoader(), defaultMockClientFactory(client))

	output, err := executeCommand(t, "--output", "json", "--no-confirm", "cfn", "delete-stackset", "--stackset-name", "my-stackset")
	if err != nil {
		t.Fatalf("command should not return error, got %v", err)
	}
	if !strings.Contains(output, "failed:") {
		t.Fatalf("expected failed action in output, got: %s", output)
	}
	if !strings.Contains(output, "skipped:") {
		t.Fatalf("expected skipped stackset action, got: %s", output)
	}
}

// --- runDeleteStackSet: DeleteStackSet API error ---

func TestDeleteStackSetDeleteStackSetError(t *testing.T) {
	client := &mockClient{
		listStackInstancesFn: func(_ context.Context, _ *cloudformation.ListStackInstancesInput, _ ...func(*cloudformation.Options)) (*cloudformation.ListStackInstancesOutput, error) {
			return &cloudformation.ListStackInstancesOutput{
				Summaries: []cloudformationtypes.StackInstanceSummary{
					{Account: cliutil.Ptr("111111111111"), Region: cliutil.Ptr("us-east-1")},
				},
			}, nil
		},
		deleteStackInstancesFn: func(_ context.Context, _ *cloudformation.DeleteStackInstancesInput, _ ...func(*cloudformation.Options)) (*cloudformation.DeleteStackInstancesOutput, error) {
			return &cloudformation.DeleteStackInstancesOutput{OperationId: cliutil.Ptr("op-ok")}, nil
		},
		describeStackSetOperation: func(_ context.Context, _ *cloudformation.DescribeStackSetOperationInput, _ ...func(*cloudformation.Options)) (*cloudformation.DescribeStackSetOperationOutput, error) {
			return &cloudformation.DescribeStackSetOperationOutput{
				StackSetOperation: &cloudformationtypes.StackSetOperation{
					Status: cloudformationtypes.StackSetOperationStatusSucceeded,
				},
			}, nil
		},
		deleteStackSetFn: func(_ context.Context, _ *cloudformation.DeleteStackSetInput, _ ...func(*cloudformation.Options)) (*cloudformation.DeleteStackSetOutput, error) {
			return nil, errors.New("stackset in use")
		},
	}

	withMockDeps(t, defaultMockLoader(), defaultMockClientFactory(client))

	output, err := executeCommand(t, "--output", "json", "--no-confirm", "cfn", "delete-stackset", "--stackset-name", "my-stackset")
	if err != nil {
		t.Fatalf("command should not return error, got %v", err)
	}
	if !strings.Contains(output, "failed:") {
		t.Fatalf("expected failed action for stackset, got: %s", output)
	}
	if !strings.Contains(output, "deleted") {
		t.Fatalf("expected instances to show deleted, got: %s", output)
	}
}

// --- runDeleteStackSet: no instances, direct stackset deletion ---

func TestDeleteStackSetNoInstances(t *testing.T) {
	deleteStackSetCalls := 0
	client := &mockClient{
		listStackInstancesFn: func(_ context.Context, _ *cloudformation.ListStackInstancesInput, _ ...func(*cloudformation.Options)) (*cloudformation.ListStackInstancesOutput, error) {
			return &cloudformation.ListStackInstancesOutput{}, nil
		},
		deleteStackSetFn: func(_ context.Context, _ *cloudformation.DeleteStackSetInput, _ ...func(*cloudformation.Options)) (*cloudformation.DeleteStackSetOutput, error) {
			deleteStackSetCalls++
			return &cloudformation.DeleteStackSetOutput{}, nil
		},
	}

	withMockDeps(t, defaultMockLoader(), defaultMockClientFactory(client))

	output, err := executeCommand(t, "--output", "json", "--no-confirm", "cfn", "delete-stackset", "--stackset-name", "empty-stackset")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if deleteStackSetCalls != 1 {
		t.Fatalf("expected DeleteStackSet to be called, got %d calls", deleteStackSetCalls)
	}
	if !strings.Contains(output, "deleted") {
		t.Fatalf("expected deleted in output, got: %s", output)
	}
}

// --- runDeleteStackSet: dry-run with no instances ---

func TestDeleteStackSetDryRunNoInstances(t *testing.T) {
	client := &mockClient{
		listStackInstancesFn: func(_ context.Context, _ *cloudformation.ListStackInstancesInput, _ ...func(*cloudformation.Options)) (*cloudformation.ListStackInstancesOutput, error) {
			return &cloudformation.ListStackInstancesOutput{}, nil
		},
	}

	withMockDeps(t, defaultMockLoader(), defaultMockClientFactory(client))

	output, err := executeCommand(t, "--output", "json", "--dry-run", "cfn", "delete-stackset", "--stackset-name", "empty-stackset")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, "would-delete") {
		t.Fatalf("expected would-delete in output, got: %s", output)
	}
}

// --- runDeleteStackSet: empty operation ID (no wait needed) ---

func TestDeleteStackSetEmptyOperationID(t *testing.T) {
	describeCalls := 0
	client := &mockClient{
		listStackInstancesFn: func(_ context.Context, _ *cloudformation.ListStackInstancesInput, _ ...func(*cloudformation.Options)) (*cloudformation.ListStackInstancesOutput, error) {
			return &cloudformation.ListStackInstancesOutput{
				Summaries: []cloudformationtypes.StackInstanceSummary{
					{Account: cliutil.Ptr("111111111111"), Region: cliutil.Ptr("us-east-1")},
				},
			}, nil
		},
		deleteStackInstancesFn: func(_ context.Context, _ *cloudformation.DeleteStackInstancesInput, _ ...func(*cloudformation.Options)) (*cloudformation.DeleteStackInstancesOutput, error) {
			// Return empty operation ID
			return &cloudformation.DeleteStackInstancesOutput{}, nil
		},
		describeStackSetOperation: func(_ context.Context, _ *cloudformation.DescribeStackSetOperationInput, _ ...func(*cloudformation.Options)) (*cloudformation.DescribeStackSetOperationOutput, error) {
			describeCalls++
			return &cloudformation.DescribeStackSetOperationOutput{
				StackSetOperation: &cloudformationtypes.StackSetOperation{Status: cloudformationtypes.StackSetOperationStatusSucceeded},
			}, nil
		},
		deleteStackSetFn: func(_ context.Context, _ *cloudformation.DeleteStackSetInput, _ ...func(*cloudformation.Options)) (*cloudformation.DeleteStackSetOutput, error) {
			return &cloudformation.DeleteStackSetOutput{}, nil
		},
	}

	withMockDeps(t, defaultMockLoader(), defaultMockClientFactory(client))

	output, err := executeCommand(t, "--output", "json", "--no-confirm", "cfn", "delete-stackset", "--stackset-name", "my-stackset")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if describeCalls != 0 {
		t.Fatalf("expected no DescribeStackSetOperation calls when operationId is empty, got %d", describeCalls)
	}
	if !strings.Contains(output, "deleted") {
		t.Fatalf("expected deleted in output, got: %s", output)
	}
}

// --- waitForStackSetOperation unit tests ---

func TestWaitForStackSetOperationFailedWithReason(t *testing.T) {
	oldSleep := sleep
	sleep = func(_ time.Duration) {}
	defer func() { sleep = oldSleep }()

	client := &mockClient{
		describeStackSetOperation: func(_ context.Context, _ *cloudformation.DescribeStackSetOperationInput, _ ...func(*cloudformation.Options)) (*cloudformation.DescribeStackSetOperationOutput, error) {
			return &cloudformation.DescribeStackSetOperationOutput{
				StackSetOperation: &cloudformationtypes.StackSetOperation{
					Status:       cloudformationtypes.StackSetOperationStatusFailed,
					StatusReason: cliutil.Ptr("quota exceeded"),
				},
			}, nil
		},
	}

	err := waitForStackSetOperation(context.Background(), client, "my-stackset", "op-1")
	if err == nil || !strings.Contains(err.Error(), "quota exceeded") {
		t.Fatalf("expected error with reason, got %v", err)
	}
	if !strings.Contains(err.Error(), "FAILED") {
		t.Fatalf("expected FAILED in error, got %v", err)
	}
}

func TestWaitForStackSetOperationFailedWithoutReason(t *testing.T) {
	oldSleep := sleep
	sleep = func(_ time.Duration) {}
	defer func() { sleep = oldSleep }()

	client := &mockClient{
		describeStackSetOperation: func(_ context.Context, _ *cloudformation.DescribeStackSetOperationInput, _ ...func(*cloudformation.Options)) (*cloudformation.DescribeStackSetOperationOutput, error) {
			return &cloudformation.DescribeStackSetOperationOutput{
				StackSetOperation: &cloudformationtypes.StackSetOperation{
					Status: cloudformationtypes.StackSetOperationStatusFailed,
				},
			}, nil
		},
	}

	err := waitForStackSetOperation(context.Background(), client, "my-stackset", "op-1")
	if err == nil || !strings.Contains(err.Error(), "FAILED") {
		t.Fatalf("expected error with FAILED status, got %v", err)
	}
}

func TestWaitForStackSetOperationStopped(t *testing.T) {
	oldSleep := sleep
	sleep = func(_ time.Duration) {}
	defer func() { sleep = oldSleep }()

	client := &mockClient{
		describeStackSetOperation: func(_ context.Context, _ *cloudformation.DescribeStackSetOperationInput, _ ...func(*cloudformation.Options)) (*cloudformation.DescribeStackSetOperationOutput, error) {
			return &cloudformation.DescribeStackSetOperationOutput{
				StackSetOperation: &cloudformationtypes.StackSetOperation{
					Status:       cloudformationtypes.StackSetOperationStatusStopped,
					StatusReason: cliutil.Ptr("user cancelled"),
				},
			}, nil
		},
	}

	err := waitForStackSetOperation(context.Background(), client, "my-stackset", "op-1")
	if err == nil || !strings.Contains(err.Error(), "STOPPED") {
		t.Fatalf("expected error with STOPPED status, got %v", err)
	}
}

func TestWaitForStackSetOperationDescribeError(t *testing.T) {
	oldSleep := sleep
	sleep = func(_ time.Duration) {}
	defer func() { sleep = oldSleep }()

	client := &mockClient{
		describeStackSetOperation: func(_ context.Context, _ *cloudformation.DescribeStackSetOperationInput, _ ...func(*cloudformation.Options)) (*cloudformation.DescribeStackSetOperationOutput, error) {
			return nil, errors.New("throttled")
		},
	}

	err := waitForStackSetOperation(context.Background(), client, "my-stackset", "op-1")
	if err == nil || !strings.Contains(err.Error(), "throttled") {
		t.Fatalf("expected throttled error, got %v", err)
	}
}

func TestWaitForStackSetOperationContextCancelled(t *testing.T) {
	oldSleep := sleep
	sleep = func(_ time.Duration) {}
	defer func() { sleep = oldSleep }()

	ctx, cancel := context.WithCancel(context.Background())
	calls := 0
	client := &mockClient{
		describeStackSetOperation: func(_ context.Context, _ *cloudformation.DescribeStackSetOperationInput, _ ...func(*cloudformation.Options)) (*cloudformation.DescribeStackSetOperationOutput, error) {
			calls++
			if calls >= 2 {
				cancel()
			}
			return &cloudformation.DescribeStackSetOperationOutput{
				StackSetOperation: &cloudformationtypes.StackSetOperation{
					Status: cloudformationtypes.StackSetOperationStatusRunning,
				},
			}, nil
		},
	}

	err := waitForStackSetOperation(ctx, client, "my-stackset", "op-1")
	if err == nil {
		t.Fatalf("expected context cancelled error, got nil")
	}
}

func TestWaitForStackSetOperationSuccess(t *testing.T) {
	oldSleep := sleep
	sleep = func(_ time.Duration) {}
	defer func() { sleep = oldSleep }()

	calls := 0
	client := &mockClient{
		describeStackSetOperation: func(_ context.Context, _ *cloudformation.DescribeStackSetOperationInput, _ ...func(*cloudformation.Options)) (*cloudformation.DescribeStackSetOperationOutput, error) {
			calls++
			status := cloudformationtypes.StackSetOperationStatusRunning
			if calls >= 3 {
				status = cloudformationtypes.StackSetOperationStatusSucceeded
			}
			return &cloudformation.DescribeStackSetOperationOutput{
				StackSetOperation: &cloudformationtypes.StackSetOperation{Status: status},
			}, nil
		},
	}

	err := waitForStackSetOperation(context.Background(), client, "my-stackset", "op-1")
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if calls != 3 {
		t.Fatalf("expected 3 polling calls, got %d", calls)
	}
}

// --- stackResourceMatches unit tests ---

func TestStackResourceMatchesSubstringLogicalID(t *testing.T) {
	resource := cloudformationtypes.StackResourceSummary{
		LogicalResourceId:  cliutil.Ptr("MyAppBucketResource"),
		PhysicalResourceId: cliutil.Ptr("my-app-bucket-12345"),
		ResourceType:       cliutil.Ptr("AWS::S3::Bucket"),
	}

	if !stackResourceMatches(resource, "Bucket", false) {
		t.Fatal("expected substring match on logical ID")
	}
}

func TestStackResourceMatchesSubstringPhysicalID(t *testing.T) {
	resource := cloudformationtypes.StackResourceSummary{
		LogicalResourceId:  cliutil.Ptr("Table"),
		PhysicalResourceId: cliutil.Ptr("my-dynamo-table-xyz"),
		ResourceType:       cliutil.Ptr("AWS::DynamoDB::Table"),
	}

	if !stackResourceMatches(resource, "dynamo-table", false) {
		t.Fatal("expected substring match on physical ID")
	}
}

func TestStackResourceMatchesSubstringResourceType(t *testing.T) {
	resource := cloudformationtypes.StackResourceSummary{
		LogicalResourceId:  cliutil.Ptr("Queue"),
		PhysicalResourceId: cliutil.Ptr("https://sqs.us-east-1.amazonaws.com/123/my-queue"),
		ResourceType:       cliutil.Ptr("AWS::SQS::Queue"),
	}

	if !stackResourceMatches(resource, "sqs", false) {
		t.Fatal("expected case-insensitive substring match on resource type")
	}
}

func TestStackResourceMatchesNoMatch(t *testing.T) {
	resource := cloudformationtypes.StackResourceSummary{
		LogicalResourceId:  cliutil.Ptr("MyBucket"),
		PhysicalResourceId: cliutil.Ptr("bucket-123"),
		ResourceType:       cliutil.Ptr("AWS::S3::Bucket"),
	}

	if stackResourceMatches(resource, "Lambda", false) {
		t.Fatal("expected no match for Lambda")
	}
}

func TestStackResourceMatchesExactLogicalID(t *testing.T) {
	resource := cloudformationtypes.StackResourceSummary{
		LogicalResourceId:  cliutil.Ptr("MyBucket"),
		PhysicalResourceId: cliutil.Ptr("bucket-123"),
		ResourceType:       cliutil.Ptr("AWS::S3::Bucket"),
	}

	if !stackResourceMatches(resource, "MyBucket", true) {
		t.Fatal("expected exact match on logical ID")
	}
}

func TestStackResourceMatchesExactPhysicalID(t *testing.T) {
	resource := cloudformationtypes.StackResourceSummary{
		LogicalResourceId:  cliutil.Ptr("MyBucket"),
		PhysicalResourceId: cliutil.Ptr("bucket-123"),
		ResourceType:       cliutil.Ptr("AWS::S3::Bucket"),
	}

	if !stackResourceMatches(resource, "bucket-123", true) {
		t.Fatal("expected exact match on physical ID")
	}
}

func TestStackResourceMatchesExactNoMatch(t *testing.T) {
	resource := cloudformationtypes.StackResourceSummary{
		LogicalResourceId:  cliutil.Ptr("MyBucket"),
		PhysicalResourceId: cliutil.Ptr("bucket-123"),
		ResourceType:       cliutil.Ptr("AWS::S3::Bucket"),
	}

	if stackResourceMatches(resource, "Bucket", true) {
		t.Fatal("expected no exact match for partial string")
	}
}

func TestStackResourceMatchesCaseInsensitiveExact(t *testing.T) {
	resource := cloudformationtypes.StackResourceSummary{
		LogicalResourceId:  cliutil.Ptr("MyBucket"),
		PhysicalResourceId: cliutil.Ptr("bucket-123"),
		ResourceType:       cliutil.Ptr("AWS::S3::Bucket"),
	}

	if !stackResourceMatches(resource, "mybucket", true) {
		t.Fatal("expected case-insensitive exact match")
	}
}

// --- listStackInstanceTargets: deduplication and empty filtering ---

func TestListStackInstanceTargetsDedup(t *testing.T) {
	client := &mockClient{
		listStackInstancesFn: func(_ context.Context, _ *cloudformation.ListStackInstancesInput, _ ...func(*cloudformation.Options)) (*cloudformation.ListStackInstancesOutput, error) {
			return &cloudformation.ListStackInstancesOutput{
				Summaries: []cloudformationtypes.StackInstanceSummary{
					{Account: cliutil.Ptr("111111111111"), Region: cliutil.Ptr("us-east-1")},
					{Account: cliutil.Ptr("111111111111"), Region: cliutil.Ptr("us-east-1")}, // duplicate
					{Account: cliutil.Ptr("222222222222"), Region: cliutil.Ptr("eu-west-1")},
				},
			}, nil
		},
	}

	targets, err := listStackInstanceTargets(context.Background(), client, "my-stackset")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(targets) != 2 {
		t.Fatalf("expected 2 unique targets, got %d", len(targets))
	}
}

func TestListStackInstanceTargetsEmptyAccountOrRegion(t *testing.T) {
	client := &mockClient{
		listStackInstancesFn: func(_ context.Context, _ *cloudformation.ListStackInstancesInput, _ ...func(*cloudformation.Options)) (*cloudformation.ListStackInstancesOutput, error) {
			return &cloudformation.ListStackInstancesOutput{
				Summaries: []cloudformationtypes.StackInstanceSummary{
					{Account: cliutil.Ptr(""), Region: cliutil.Ptr("us-east-1")},
					{Account: cliutil.Ptr("111111111111"), Region: cliutil.Ptr("")},
					{Account: nil, Region: cliutil.Ptr("us-west-2")},
					{Account: cliutil.Ptr("333333333333"), Region: nil},
					{Account: cliutil.Ptr("444444444444"), Region: cliutil.Ptr("ap-southeast-1")},
				},
			}, nil
		},
	}

	targets, err := listStackInstanceTargets(context.Background(), client, "my-stackset")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(targets) != 1 {
		t.Fatalf("expected 1 valid target, got %d", len(targets))
	}
	if targets[0].Account != "444444444444" || targets[0].Region != "ap-southeast-1" {
		t.Fatalf("unexpected target: %+v", targets[0])
	}
}

func TestListStackInstanceTargetsSorted(t *testing.T) {
	client := &mockClient{
		listStackInstancesFn: func(_ context.Context, _ *cloudformation.ListStackInstancesInput, _ ...func(*cloudformation.Options)) (*cloudformation.ListStackInstancesOutput, error) {
			return &cloudformation.ListStackInstancesOutput{
				Summaries: []cloudformationtypes.StackInstanceSummary{
					{Account: cliutil.Ptr("222222222222"), Region: cliutil.Ptr("eu-west-1")},
					{Account: cliutil.Ptr("111111111111"), Region: cliutil.Ptr("us-west-2")},
					{Account: cliutil.Ptr("111111111111"), Region: cliutil.Ptr("us-east-1")},
				},
			}, nil
		},
	}

	targets, err := listStackInstanceTargets(context.Background(), client, "my-stackset")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(targets) != 3 {
		t.Fatalf("expected 3 targets, got %d", len(targets))
	}
	// Account 111 comes before 222; within 111, us-east-1 before us-west-2
	if targets[0].Account != "111111111111" || targets[0].Region != "us-east-1" {
		t.Fatalf("unexpected first target: %+v", targets[0])
	}
	if targets[1].Account != "111111111111" || targets[1].Region != "us-west-2" {
		t.Fatalf("unexpected second target: %+v", targets[1])
	}
	if targets[2].Account != "222222222222" || targets[2].Region != "eu-west-1" {
		t.Fatalf("unexpected third target: %+v", targets[2])
	}
}

func TestListStackInstanceTargetsError(t *testing.T) {
	client := &mockClient{
		listStackInstancesFn: func(_ context.Context, _ *cloudformation.ListStackInstancesInput, _ ...func(*cloudformation.Options)) (*cloudformation.ListStackInstancesOutput, error) {
			return nil, errors.New("service unavailable")
		},
	}

	_, err := listStackInstanceTargets(context.Background(), client, "my-stackset")
	if err == nil || !strings.Contains(err.Error(), "service unavailable") {
		t.Fatalf("expected error, got %v", err)
	}
}

// --- deleteStackSetInstanceTarget ---

func TestDeleteStackSetInstanceTargetError(t *testing.T) {
	client := &mockClient{
		deleteStackInstancesFn: func(_ context.Context, _ *cloudformation.DeleteStackInstancesInput, _ ...func(*cloudformation.Options)) (*cloudformation.DeleteStackInstancesOutput, error) {
			return nil, errors.New("permission denied")
		},
	}

	_, err := deleteStackSetInstanceTarget(context.Background(), client, "my-stackset", stackInstanceTarget{Account: "111111111111", Region: "us-east-1"})
	if err == nil || !strings.Contains(err.Error(), "permission denied") {
		t.Fatalf("expected error, got %v", err)
	}
}

func TestDeleteStackSetInstanceTargetSuccess(t *testing.T) {
	client := &mockClient{
		deleteStackInstancesFn: func(_ context.Context, in *cloudformation.DeleteStackInstancesInput, _ ...func(*cloudformation.Options)) (*cloudformation.DeleteStackInstancesOutput, error) {
			if len(in.Accounts) != 1 || in.Accounts[0] != "111111111111" {
				return nil, errors.New("unexpected account")
			}
			if len(in.Regions) != 1 || in.Regions[0] != "us-east-1" {
				return nil, errors.New("unexpected region")
			}
			return &cloudformation.DeleteStackInstancesOutput{OperationId: cliutil.Ptr("op-abc")}, nil
		},
	}

	opID, err := deleteStackSetInstanceTarget(context.Background(), client, "my-stackset", stackInstanceTarget{Account: "111111111111", Region: "us-east-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opID != "op-abc" {
		t.Fatalf("expected op-abc, got %s", opID)
	}
}

// --- listStacksForSearch ---

func TestListStacksForSearchFiltersDeleteComplete(t *testing.T) {
	client := &mockClient{
		describeStacksFn: func(_ context.Context, _ *cloudformation.DescribeStacksInput, _ ...func(*cloudformation.Options)) (*cloudformation.DescribeStacksOutput, error) {
			return &cloudformation.DescribeStacksOutput{
				Stacks: []cloudformationtypes.Stack{
					{StackName: cliutil.Ptr("active-stack"), StackStatus: cloudformationtypes.StackStatusCreateComplete},
					{StackName: cliutil.Ptr("deleted-stack"), StackStatus: cloudformationtypes.StackStatusDeleteComplete},
				},
			}, nil
		},
	}

	stacks, err := listStacksForSearch(context.Background(), client, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(stacks) != 1 {
		t.Fatalf("expected 1 stack after filtering, got %d", len(stacks))
	}
	if cliutil.PointerToString(stacks[0].StackName) != "active-stack" {
		t.Fatalf("expected active-stack, got %s", cliutil.PointerToString(stacks[0].StackName))
	}
}

func TestListStacksForSearchExcludesNestedByDefault(t *testing.T) {
	client := &mockClient{
		describeStacksFn: func(_ context.Context, _ *cloudformation.DescribeStacksInput, _ ...func(*cloudformation.Options)) (*cloudformation.DescribeStacksOutput, error) {
			return &cloudformation.DescribeStacksOutput{
				Stacks: []cloudformationtypes.Stack{
					{StackName: cliutil.Ptr("parent"), StackStatus: cloudformationtypes.StackStatusCreateComplete},
					{StackName: cliutil.Ptr("nested"), StackStatus: cloudformationtypes.StackStatusCreateComplete, ParentId: cliutil.Ptr("arn:parent")},
				},
			}, nil
		},
	}

	stacks, err := listStacksForSearch(context.Background(), client, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(stacks) != 1 {
		t.Fatalf("expected 1 non-nested stack, got %d", len(stacks))
	}
}

func TestListStacksForSearchIncludesNestedWhenRequested(t *testing.T) {
	client := &mockClient{
		describeStacksFn: func(_ context.Context, _ *cloudformation.DescribeStacksInput, _ ...func(*cloudformation.Options)) (*cloudformation.DescribeStacksOutput, error) {
			return &cloudformation.DescribeStacksOutput{
				Stacks: []cloudformationtypes.Stack{
					{StackName: cliutil.Ptr("parent"), StackStatus: cloudformationtypes.StackStatusCreateComplete},
					{StackName: cliutil.Ptr("nested"), StackStatus: cloudformationtypes.StackStatusCreateComplete, ParentId: cliutil.Ptr("arn:parent")},
				},
			}, nil
		},
	}

	stacks, err := listStacksForSearch(context.Background(), client, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(stacks) != 2 {
		t.Fatalf("expected 2 stacks with nested included, got %d", len(stacks))
	}
}

func TestListStacksForSearchError(t *testing.T) {
	client := &mockClient{
		describeStacksFn: func(_ context.Context, _ *cloudformation.DescribeStacksInput, _ ...func(*cloudformation.Options)) (*cloudformation.DescribeStacksOutput, error) {
			return nil, errors.New("throttled")
		},
	}

	_, err := listStacksForSearch(context.Background(), client, false)
	if err == nil || !strings.Contains(err.Error(), "throttled") {
		t.Fatalf("expected error, got %v", err)
	}
}

// --- listStackResources ---

func TestListStackResourcesError(t *testing.T) {
	client := &mockClient{
		listStackResourcesFn: func(_ context.Context, _ *cloudformation.ListStackResourcesInput, _ ...func(*cloudformation.Options)) (*cloudformation.ListStackResourcesOutput, error) {
			return nil, errors.New("stack not found")
		},
	}

	_, err := listStackResources(context.Background(), client, "nonexistent")
	if err == nil || !strings.Contains(err.Error(), "stack not found") {
		t.Fatalf("expected error, got %v", err)
	}
}

func TestListStackResourcesSuccess(t *testing.T) {
	client := &mockClient{
		listStackResourcesFn: func(_ context.Context, _ *cloudformation.ListStackResourcesInput, _ ...func(*cloudformation.Options)) (*cloudformation.ListStackResourcesOutput, error) {
			return &cloudformation.ListStackResourcesOutput{
				StackResourceSummaries: []cloudformationtypes.StackResourceSummary{
					{LogicalResourceId: cliutil.Ptr("Res1")},
					{LogicalResourceId: cliutil.Ptr("Res2")},
				},
			}, nil
		},
	}

	resources, err := listStackResources(context.Background(), client, "my-stack")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(resources))
	}
}

// --- runFindStackByResource: substring match and no match ---

func TestFindStackByResourceSubstringMatch(t *testing.T) {
	client := &mockClient{
		describeStacksFn: func(_ context.Context, _ *cloudformation.DescribeStacksInput, _ ...func(*cloudformation.Options)) (*cloudformation.DescribeStacksOutput, error) {
			return &cloudformation.DescribeStacksOutput{
				Stacks: []cloudformationtypes.Stack{
					{StackName: cliutil.Ptr("my-stack"), StackStatus: cloudformationtypes.StackStatusCreateComplete},
				},
			}, nil
		},
		listStackResourcesFn: func(_ context.Context, _ *cloudformation.ListStackResourcesInput, _ ...func(*cloudformation.Options)) (*cloudformation.ListStackResourcesOutput, error) {
			return &cloudformation.ListStackResourcesOutput{
				StackResourceSummaries: []cloudformationtypes.StackResourceSummary{
					{
						LogicalResourceId:  cliutil.Ptr("AppBucket"),
						PhysicalResourceId: cliutil.Ptr("my-app-bucket-12345"),
						ResourceType:       cliutil.Ptr("AWS::S3::Bucket"),
						ResourceStatus:     cloudformationtypes.ResourceStatusCreateComplete,
					},
					{
						LogicalResourceId:  cliutil.Ptr("AppFunction"),
						PhysicalResourceId: cliutil.Ptr("my-app-function"),
						ResourceType:       cliutil.Ptr("AWS::Lambda::Function"),
						ResourceStatus:     cloudformationtypes.ResourceStatusCreateComplete,
					},
				},
			}, nil
		},
	}

	withMockDeps(t, defaultMockLoader(), defaultMockClientFactory(client))

	output, err := executeCommand(t, "--output", "json", "cfn", "find-stack-by-resource", "--resource", "bucket")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, "AppBucket") {
		t.Fatalf("expected AppBucket in output, got: %s", output)
	}
	// Lambda should not match "bucket"
	if strings.Contains(output, "AppFunction") {
		t.Fatalf("did not expect AppFunction in output, got: %s", output)
	}
}

func TestFindStackByResourceNoMatches(t *testing.T) {
	client := &mockClient{
		describeStacksFn: func(_ context.Context, _ *cloudformation.DescribeStacksInput, _ ...func(*cloudformation.Options)) (*cloudformation.DescribeStacksOutput, error) {
			return &cloudformation.DescribeStacksOutput{
				Stacks: []cloudformationtypes.Stack{
					{StackName: cliutil.Ptr("my-stack"), StackStatus: cloudformationtypes.StackStatusCreateComplete},
				},
			}, nil
		},
		listStackResourcesFn: func(_ context.Context, _ *cloudformation.ListStackResourcesInput, _ ...func(*cloudformation.Options)) (*cloudformation.ListStackResourcesOutput, error) {
			return &cloudformation.ListStackResourcesOutput{
				StackResourceSummaries: []cloudformationtypes.StackResourceSummary{
					{
						LogicalResourceId:  cliutil.Ptr("AppBucket"),
						PhysicalResourceId: cliutil.Ptr("my-app-bucket"),
						ResourceType:       cliutil.Ptr("AWS::S3::Bucket"),
						ResourceStatus:     cloudformationtypes.ResourceStatusCreateComplete,
					},
				},
			}, nil
		},
	}

	withMockDeps(t, defaultMockLoader(), defaultMockClientFactory(client))

	output, err := executeCommand(t, "--output", "json", "cfn", "find-stack-by-resource", "--resource", "nonexistent-thing")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should succeed but with no matching rows
	if strings.Contains(output, "AppBucket") {
		t.Fatalf("expected no matches, got: %s", output)
	}
}

func TestFindStackByResourceListStacksError(t *testing.T) {
	client := &mockClient{
		describeStacksFn: func(_ context.Context, _ *cloudformation.DescribeStacksInput, _ ...func(*cloudformation.Options)) (*cloudformation.DescribeStacksOutput, error) {
			return nil, errors.New("service error")
		},
	}

	withMockDeps(t, defaultMockLoader(), defaultMockClientFactory(client))

	_, err := executeCommand(t, "--output", "json", "cfn", "find-stack-by-resource", "--resource", "something")
	if err == nil || !strings.Contains(err.Error(), "list stacks") {
		t.Fatalf("expected list stacks error, got %v", err)
	}
}

func TestFindStackByResourceListResourcesError(t *testing.T) {
	client := &mockClient{
		describeStacksFn: func(_ context.Context, _ *cloudformation.DescribeStacksInput, _ ...func(*cloudformation.Options)) (*cloudformation.DescribeStacksOutput, error) {
			return &cloudformation.DescribeStacksOutput{
				Stacks: []cloudformationtypes.Stack{
					{StackName: cliutil.Ptr("my-stack"), StackStatus: cloudformationtypes.StackStatusCreateComplete},
				},
			}, nil
		},
		listStackResourcesFn: func(_ context.Context, _ *cloudformation.ListStackResourcesInput, _ ...func(*cloudformation.Options)) (*cloudformation.ListStackResourcesOutput, error) {
			return nil, errors.New("access denied")
		},
	}

	withMockDeps(t, defaultMockLoader(), defaultMockClientFactory(client))

	_, err := executeCommand(t, "--output", "json", "cfn", "find-stack-by-resource", "--resource", "something")
	if err == nil || !strings.Contains(err.Error(), "list resources for stack") {
		t.Fatalf("expected list resources error, got %v", err)
	}
}

func TestFindStackByResourceExcludesNestedByDefault(t *testing.T) {
	client := &mockClient{
		describeStacksFn: func(_ context.Context, _ *cloudformation.DescribeStacksInput, _ ...func(*cloudformation.Options)) (*cloudformation.DescribeStacksOutput, error) {
			return &cloudformation.DescribeStacksOutput{
				Stacks: []cloudformationtypes.Stack{
					{StackName: cliutil.Ptr("parent-stack"), StackStatus: cloudformationtypes.StackStatusCreateComplete},
					{StackName: cliutil.Ptr("nested-stack"), StackStatus: cloudformationtypes.StackStatusCreateComplete, ParentId: cliutil.Ptr("arn:parent")},
				},
			}, nil
		},
		listStackResourcesFn: func(_ context.Context, in *cloudformation.ListStackResourcesInput, _ ...func(*cloudformation.Options)) (*cloudformation.ListStackResourcesOutput, error) {
			return &cloudformation.ListStackResourcesOutput{
				StackResourceSummaries: []cloudformationtypes.StackResourceSummary{
					{
						LogicalResourceId:  cliutil.Ptr("Bucket"),
						PhysicalResourceId: cliutil.Ptr("bucket-123"),
						ResourceType:       cliutil.Ptr("AWS::S3::Bucket"),
						ResourceStatus:     cloudformationtypes.ResourceStatusCreateComplete,
					},
				},
			}, nil
		},
	}

	withMockDeps(t, defaultMockLoader(), defaultMockClientFactory(client))

	output, err := executeCommand(t, "--output", "json", "cfn", "find-stack-by-resource", "--resource", "Bucket")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, "parent-stack") {
		t.Fatalf("expected parent-stack in output, got: %s", output)
	}
	if strings.Contains(output, "nested-stack") {
		t.Fatalf("did not expect nested-stack without --include-nested, got: %s", output)
	}
}

func TestFindStackByResourceMatchesPhysicalID(t *testing.T) {
	client := &mockClient{
		describeStacksFn: func(_ context.Context, _ *cloudformation.DescribeStacksInput, _ ...func(*cloudformation.Options)) (*cloudformation.DescribeStacksOutput, error) {
			return &cloudformation.DescribeStacksOutput{
				Stacks: []cloudformationtypes.Stack{
					{StackName: cliutil.Ptr("my-stack"), StackStatus: cloudformationtypes.StackStatusCreateComplete},
				},
			}, nil
		},
		listStackResourcesFn: func(_ context.Context, _ *cloudformation.ListStackResourcesInput, _ ...func(*cloudformation.Options)) (*cloudformation.ListStackResourcesOutput, error) {
			return &cloudformation.ListStackResourcesOutput{
				StackResourceSummaries: []cloudformationtypes.StackResourceSummary{
					{
						LogicalResourceId:  cliutil.Ptr("MyTable"),
						PhysicalResourceId: cliutil.Ptr("prod-users-table"),
						ResourceType:       cliutil.Ptr("AWS::DynamoDB::Table"),
						ResourceStatus:     cloudformationtypes.ResourceStatusCreateComplete,
					},
				},
			}, nil
		},
	}

	withMockDeps(t, defaultMockLoader(), defaultMockClientFactory(client))

	output, err := executeCommand(t, "--output", "json", "cfn", "find-stack-by-resource", "--resource", "prod-users-table", "--exact")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, "MyTable") {
		t.Fatalf("expected MyTable in output, got: %s", output)
	}
}

// --- runDeleteStackSet: multiple instances with partial failure ---

func TestDeleteStackSetMultipleInstancesPartialFailure(t *testing.T) {
	deleteCount := 0
	client := &mockClient{
		listStackInstancesFn: func(_ context.Context, _ *cloudformation.ListStackInstancesInput, _ ...func(*cloudformation.Options)) (*cloudformation.ListStackInstancesOutput, error) {
			return &cloudformation.ListStackInstancesOutput{
				Summaries: []cloudformationtypes.StackInstanceSummary{
					{Account: cliutil.Ptr("111111111111"), Region: cliutil.Ptr("us-east-1")},
					{Account: cliutil.Ptr("222222222222"), Region: cliutil.Ptr("eu-west-1")},
				},
			}, nil
		},
		deleteStackInstancesFn: func(_ context.Context, in *cloudformation.DeleteStackInstancesInput, _ ...func(*cloudformation.Options)) (*cloudformation.DeleteStackInstancesOutput, error) {
			deleteCount++
			// First instance succeeds, second fails
			if in.Accounts[0] == "222222222222" {
				return nil, errors.New("quota exceeded")
			}
			return &cloudformation.DeleteStackInstancesOutput{OperationId: cliutil.Ptr("op-1")}, nil
		},
		describeStackSetOperation: func(_ context.Context, _ *cloudformation.DescribeStackSetOperationInput, _ ...func(*cloudformation.Options)) (*cloudformation.DescribeStackSetOperationOutput, error) {
			return &cloudformation.DescribeStackSetOperationOutput{
				StackSetOperation: &cloudformationtypes.StackSetOperation{Status: cloudformationtypes.StackSetOperationStatusSucceeded},
			}, nil
		},
	}

	withMockDeps(t, defaultMockLoader(), defaultMockClientFactory(client))

	output, err := executeCommand(t, "--output", "json", "--no-confirm", "cfn", "delete-stackset", "--stackset-name", "my-stackset")
	if err != nil {
		t.Fatalf("command should not return error, got %v", err)
	}
	// First instance should be deleted, second should be failed
	if !strings.Contains(output, "deleted") {
		t.Fatalf("expected deleted for first instance, got: %s", output)
	}
	if !strings.Contains(output, "failed:") {
		t.Fatalf("expected failed for second instance, got: %s", output)
	}
	if !strings.Contains(output, "skipped:") {
		t.Fatalf("expected skipped for stackset, got: %s", output)
	}
}
