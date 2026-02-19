package cli

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/appstream"
	appstreamtypes "github.com/aws/aws-sdk-go-v2/service/appstream/types"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	cloudformationtypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	route53types "github.com/aws/aws-sdk-go-v2/service/route53/types"
)

type mockAppStreamClient struct {
	describeImagePermissionsFn func(context.Context, *appstream.DescribeImagePermissionsInput, ...func(*appstream.Options)) (*appstream.DescribeImagePermissionsOutput, error)
	deleteImagePermissionsFn   func(context.Context, *appstream.DeleteImagePermissionsInput, ...func(*appstream.Options)) (*appstream.DeleteImagePermissionsOutput, error)
	deleteImageFn              func(context.Context, *appstream.DeleteImageInput, ...func(*appstream.Options)) (*appstream.DeleteImageOutput, error)
}

func (m *mockAppStreamClient) DescribeImagePermissions(ctx context.Context, in *appstream.DescribeImagePermissionsInput, optFns ...func(*appstream.Options)) (*appstream.DescribeImagePermissionsOutput, error) {
	if m.describeImagePermissionsFn == nil {
		return nil, errors.New("DescribeImagePermissions not mocked")
	}
	return m.describeImagePermissionsFn(ctx, in, optFns...)
}

func (m *mockAppStreamClient) DeleteImagePermissions(ctx context.Context, in *appstream.DeleteImagePermissionsInput, optFns ...func(*appstream.Options)) (*appstream.DeleteImagePermissionsOutput, error) {
	if m.deleteImagePermissionsFn == nil {
		return nil, errors.New("DeleteImagePermissions not mocked")
	}
	return m.deleteImagePermissionsFn(ctx, in, optFns...)
}

func (m *mockAppStreamClient) DeleteImage(ctx context.Context, in *appstream.DeleteImageInput, optFns ...func(*appstream.Options)) (*appstream.DeleteImageOutput, error) {
	if m.deleteImageFn == nil {
		return nil, errors.New("DeleteImage not mocked")
	}
	return m.deleteImageFn(ctx, in, optFns...)
}

func withMockAppStreamDeps(t *testing.T, loader func(string, string) (awssdk.Config, error), newClient func(awssdk.Config) appStreamAPI) {
	t.Helper()

	oldLoader := appStreamLoadAWSConfig
	oldNewClient := appStreamNewClient

	appStreamLoadAWSConfig = loader
	appStreamNewClient = newClient

	t.Cleanup(func() {
		appStreamLoadAWSConfig = oldLoader
		appStreamNewClient = oldNewClient
	})
}

type mockCFNClient struct {
	deleteStackInstancesFn    func(context.Context, *cloudformation.DeleteStackInstancesInput, ...func(*cloudformation.Options)) (*cloudformation.DeleteStackInstancesOutput, error)
	deleteStackSetFn          func(context.Context, *cloudformation.DeleteStackSetInput, ...func(*cloudformation.Options)) (*cloudformation.DeleteStackSetOutput, error)
	describeStackSetOperation func(context.Context, *cloudformation.DescribeStackSetOperationInput, ...func(*cloudformation.Options)) (*cloudformation.DescribeStackSetOperationOutput, error)
	describeStacksFn          func(context.Context, *cloudformation.DescribeStacksInput, ...func(*cloudformation.Options)) (*cloudformation.DescribeStacksOutput, error)
	listStackInstancesFn      func(context.Context, *cloudformation.ListStackInstancesInput, ...func(*cloudformation.Options)) (*cloudformation.ListStackInstancesOutput, error)
	listStackResourcesFn      func(context.Context, *cloudformation.ListStackResourcesInput, ...func(*cloudformation.Options)) (*cloudformation.ListStackResourcesOutput, error)
}

func (m *mockCFNClient) DeleteStackInstances(ctx context.Context, in *cloudformation.DeleteStackInstancesInput, optFns ...func(*cloudformation.Options)) (*cloudformation.DeleteStackInstancesOutput, error) {
	if m.deleteStackInstancesFn == nil {
		return nil, errors.New("DeleteStackInstances not mocked")
	}
	return m.deleteStackInstancesFn(ctx, in, optFns...)
}

func (m *mockCFNClient) DeleteStackSet(ctx context.Context, in *cloudformation.DeleteStackSetInput, optFns ...func(*cloudformation.Options)) (*cloudformation.DeleteStackSetOutput, error) {
	if m.deleteStackSetFn == nil {
		return nil, errors.New("DeleteStackSet not mocked")
	}
	return m.deleteStackSetFn(ctx, in, optFns...)
}

func (m *mockCFNClient) DescribeStackSetOperation(ctx context.Context, in *cloudformation.DescribeStackSetOperationInput, optFns ...func(*cloudformation.Options)) (*cloudformation.DescribeStackSetOperationOutput, error) {
	if m.describeStackSetOperation == nil {
		return nil, errors.New("DescribeStackSetOperation not mocked")
	}
	return m.describeStackSetOperation(ctx, in, optFns...)
}

func (m *mockCFNClient) DescribeStacks(ctx context.Context, in *cloudformation.DescribeStacksInput, optFns ...func(*cloudformation.Options)) (*cloudformation.DescribeStacksOutput, error) {
	if m.describeStacksFn == nil {
		return nil, errors.New("DescribeStacks not mocked")
	}
	return m.describeStacksFn(ctx, in, optFns...)
}

func (m *mockCFNClient) ListStackInstances(ctx context.Context, in *cloudformation.ListStackInstancesInput, optFns ...func(*cloudformation.Options)) (*cloudformation.ListStackInstancesOutput, error) {
	if m.listStackInstancesFn == nil {
		return nil, errors.New("ListStackInstances not mocked")
	}
	return m.listStackInstancesFn(ctx, in, optFns...)
}

func (m *mockCFNClient) ListStackResources(ctx context.Context, in *cloudformation.ListStackResourcesInput, optFns ...func(*cloudformation.Options)) (*cloudformation.ListStackResourcesOutput, error) {
	if m.listStackResourcesFn == nil {
		return nil, errors.New("ListStackResources not mocked")
	}
	return m.listStackResourcesFn(ctx, in, optFns...)
}

func withMockCFNDeps(t *testing.T, loader func(string, string) (awssdk.Config, error), newClient func(awssdk.Config) cfnAPI) {
	t.Helper()

	oldLoader := cfnLoadAWSConfig
	oldNewClient := cfnNewClient
	oldSleep := cfnSleep

	cfnLoadAWSConfig = loader
	cfnNewClient = newClient
	cfnSleep = func(_ time.Duration) {}

	t.Cleanup(func() {
		cfnLoadAWSConfig = oldLoader
		cfnNewClient = oldNewClient
		cfnSleep = oldSleep
	})
}

type mockR53Client struct {
	createHealthCheckFn     func(context.Context, *route53.CreateHealthCheckInput, ...func(*route53.Options)) (*route53.CreateHealthCheckOutput, error)
	changeTagsForResourceFn func(context.Context, *route53.ChangeTagsForResourceInput, ...func(*route53.Options)) (*route53.ChangeTagsForResourceOutput, error)
}

func (m *mockR53Client) CreateHealthCheck(ctx context.Context, in *route53.CreateHealthCheckInput, optFns ...func(*route53.Options)) (*route53.CreateHealthCheckOutput, error) {
	if m.createHealthCheckFn == nil {
		return nil, errors.New("CreateHealthCheck not mocked")
	}
	return m.createHealthCheckFn(ctx, in, optFns...)
}

func (m *mockR53Client) ChangeTagsForResource(ctx context.Context, in *route53.ChangeTagsForResourceInput, optFns ...func(*route53.Options)) (*route53.ChangeTagsForResourceOutput, error) {
	if m.changeTagsForResourceFn == nil {
		return nil, errors.New("ChangeTagsForResource not mocked")
	}
	return m.changeTagsForResourceFn(ctx, in, optFns...)
}

func withMockR53Deps(t *testing.T, loader func(string, string) (awssdk.Config, error), newClient func(awssdk.Config) r53API) {
	t.Helper()

	oldLoader := r53LoadAWSConfig
	oldNewClient := r53NewClient

	r53LoadAWSConfig = loader
	r53NewClient = newClient

	t.Cleanup(func() {
		r53LoadAWSConfig = oldLoader
		r53NewClient = oldNewClient
	})
}

func TestMilestone5AppStreamDeleteImageRequiresName(t *testing.T) {
	_, err := executeCommand(t, "appstream", "delete-image")
	if err == nil || !strings.Contains(err.Error(), "--name is required") {
		t.Fatalf("expected required name error, got %v", err)
	}
}

func TestMilestone5AppStreamDeleteImageDryRun(t *testing.T) {
	deletedPermissions := 0
	deletedImages := 0

	client := &mockAppStreamClient{
		describeImagePermissionsFn: func(_ context.Context, _ *appstream.DescribeImagePermissionsInput, _ ...func(*appstream.Options)) (*appstream.DescribeImagePermissionsOutput, error) {
			return &appstream.DescribeImagePermissionsOutput{SharedImagePermissionsList: []appstreamtypes.SharedImagePermissions{
				{SharedAccountId: ptr("111111111111")},
				{SharedAccountId: ptr("222222222222")},
			}}, nil
		},
		deleteImagePermissionsFn: func(_ context.Context, _ *appstream.DeleteImagePermissionsInput, _ ...func(*appstream.Options)) (*appstream.DeleteImagePermissionsOutput, error) {
			deletedPermissions++
			return &appstream.DeleteImagePermissionsOutput{}, nil
		},
		deleteImageFn: func(_ context.Context, _ *appstream.DeleteImageInput, _ ...func(*appstream.Options)) (*appstream.DeleteImageOutput, error) {
			deletedImages++
			return &appstream.DeleteImageOutput{}, nil
		},
	}

	withMockAppStreamDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) appStreamAPI { return client },
	)

	output, err := executeCommand(t, "--output", "json", "--dry-run", "appstream", "delete-image", "--name", "image-a")
	if err != nil {
		t.Fatalf("execute appstream delete-image dry-run: %v", err)
	}
	if deletedPermissions != 0 || deletedImages != 0 {
		t.Fatalf("expected no deletion calls in dry-run, got permissions=%d image=%d", deletedPermissions, deletedImages)
	}
	if !strings.Contains(output, "would-unshare") || !strings.Contains(output, "would-delete") {
		t.Fatalf("unexpected output: %s", output)
	}
}

func TestMilestone5AppStreamDeleteImageNoConfirmExecutes(t *testing.T) {
	deletedPermissions := 0
	deletedImages := 0

	client := &mockAppStreamClient{
		describeImagePermissionsFn: func(_ context.Context, _ *appstream.DescribeImagePermissionsInput, _ ...func(*appstream.Options)) (*appstream.DescribeImagePermissionsOutput, error) {
			return &appstream.DescribeImagePermissionsOutput{SharedImagePermissionsList: []appstreamtypes.SharedImagePermissions{{SharedAccountId: ptr("111111111111")}}}, nil
		},
		deleteImagePermissionsFn: func(_ context.Context, _ *appstream.DeleteImagePermissionsInput, _ ...func(*appstream.Options)) (*appstream.DeleteImagePermissionsOutput, error) {
			deletedPermissions++
			return &appstream.DeleteImagePermissionsOutput{}, nil
		},
		deleteImageFn: func(_ context.Context, _ *appstream.DeleteImageInput, _ ...func(*appstream.Options)) (*appstream.DeleteImageOutput, error) {
			deletedImages++
			return &appstream.DeleteImageOutput{}, nil
		},
	}

	withMockAppStreamDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) appStreamAPI { return client },
	)

	output, err := executeCommand(t, "--output", "json", "--no-confirm", "appstream", "delete-image", "--name", "image-a")
	if err != nil {
		t.Fatalf("execute appstream delete-image --no-confirm: %v", err)
	}
	if deletedPermissions != 1 || deletedImages != 1 {
		t.Fatalf("expected delete calls, got permissions=%d image=%d", deletedPermissions, deletedImages)
	}
	if !strings.Contains(output, "unshared") || !strings.Contains(output, "deleted") {
		t.Fatalf("unexpected output: %s", output)
	}
}

func TestMilestone5CFNDeleteStackSetRequiresName(t *testing.T) {
	_, err := executeCommand(t, "cfn", "delete-stackset")
	if err == nil || !strings.Contains(err.Error(), "--name is required") {
		t.Fatalf("expected required name error, got %v", err)
	}
}

func TestMilestone5CFNDeleteStackSetDryRun(t *testing.T) {
	deleteInstancesCalls := 0
	deleteStackSetCalls := 0

	client := &mockCFNClient{
		listStackInstancesFn: func(_ context.Context, _ *cloudformation.ListStackInstancesInput, _ ...func(*cloudformation.Options)) (*cloudformation.ListStackInstancesOutput, error) {
			return &cloudformation.ListStackInstancesOutput{Summaries: []cloudformationtypes.StackInstanceSummary{{Account: ptr("111111111111"), Region: ptr("us-east-1")}}}, nil
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

	withMockCFNDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) cfnAPI { return client },
	)

	output, err := executeCommand(t, "--output", "json", "--dry-run", "cfn", "delete-stackset", "--name", "stackset-a")
	if err != nil {
		t.Fatalf("execute cfn delete-stackset dry-run: %v", err)
	}
	if deleteInstancesCalls != 0 || deleteStackSetCalls != 0 {
		t.Fatalf("expected no delete calls in dry-run, got instances=%d stackset=%d", deleteInstancesCalls, deleteStackSetCalls)
	}
	if !strings.Contains(output, "would-delete-stack-instance") || !strings.Contains(output, "would-delete-stackset") {
		t.Fatalf("unexpected output: %s", output)
	}
}

func TestMilestone5CFNDeleteStackSetNoConfirmExecutes(t *testing.T) {
	deleteInstancesCalls := 0
	describeOperationCalls := 0
	deleteStackSetCalls := 0

	client := &mockCFNClient{
		listStackInstancesFn: func(_ context.Context, _ *cloudformation.ListStackInstancesInput, _ ...func(*cloudformation.Options)) (*cloudformation.ListStackInstancesOutput, error) {
			return &cloudformation.ListStackInstancesOutput{Summaries: []cloudformationtypes.StackInstanceSummary{{Account: ptr("111111111111"), Region: ptr("us-east-1")}}}, nil
		},
		deleteStackInstancesFn: func(_ context.Context, _ *cloudformation.DeleteStackInstancesInput, _ ...func(*cloudformation.Options)) (*cloudformation.DeleteStackInstancesOutput, error) {
			deleteInstancesCalls++
			return &cloudformation.DeleteStackInstancesOutput{OperationId: ptr("op-1")}, nil
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

	withMockCFNDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) cfnAPI { return client },
	)

	output, err := executeCommand(t, "--output", "json", "--no-confirm", "cfn", "delete-stackset", "--name", "stackset-a")
	if err != nil {
		t.Fatalf("execute cfn delete-stackset --no-confirm: %v", err)
	}
	if deleteInstancesCalls != 1 || describeOperationCalls != 1 || deleteStackSetCalls != 1 {
		t.Fatalf("unexpected API call counts instances=%d describe=%d stackset=%d", deleteInstancesCalls, describeOperationCalls, deleteStackSetCalls)
	}
	if !strings.Contains(output, "deleted-stack-instance") || !strings.Contains(output, "deleted-stackset") {
		t.Fatalf("unexpected output: %s", output)
	}
}

func TestMilestone5CFNDeleteStackSetWaitsBeyondLegacyTimeout(t *testing.T) {
	describeOperationCalls := 0
	client := &mockCFNClient{
		listStackInstancesFn: func(_ context.Context, _ *cloudformation.ListStackInstancesInput, _ ...func(*cloudformation.Options)) (*cloudformation.ListStackInstancesOutput, error) {
			return &cloudformation.ListStackInstancesOutput{Summaries: []cloudformationtypes.StackInstanceSummary{{Account: ptr("111111111111"), Region: ptr("us-east-1")}}}, nil
		},
		deleteStackInstancesFn: func(_ context.Context, _ *cloudformation.DeleteStackInstancesInput, _ ...func(*cloudformation.Options)) (*cloudformation.DeleteStackInstancesOutput, error) {
			return &cloudformation.DeleteStackInstancesOutput{OperationId: ptr("op-1")}, nil
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

	withMockCFNDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) cfnAPI { return client },
	)

	output, err := executeCommand(t, "--output", "json", "--no-confirm", "cfn", "delete-stackset", "--name", "stackset-a")
	if err != nil {
		t.Fatalf("execute cfn delete-stackset with long-running operation: %v", err)
	}
	if describeOperationCalls != 200 {
		t.Fatalf("expected operation polling to continue until success, got %d calls", describeOperationCalls)
	}
	if !strings.Contains(output, "deleted-stack-instance") || !strings.Contains(output, "deleted-stackset") {
		t.Fatalf("unexpected output: %s", output)
	}
}

func TestMilestone5CFNFindStackByResourceRequiresResource(t *testing.T) {
	_, err := executeCommand(t, "cfn", "find-stack-by-resource")
	if err == nil || !strings.Contains(err.Error(), "--resource is required") {
		t.Fatalf("expected required resource error, got %v", err)
	}
}

func TestMilestone5CFNFindStackByResourceMatchesNestedWhenRequested(t *testing.T) {
	client := &mockCFNClient{
		describeStacksFn: func(_ context.Context, _ *cloudformation.DescribeStacksInput, _ ...func(*cloudformation.Options)) (*cloudformation.DescribeStacksOutput, error) {
			return &cloudformation.DescribeStacksOutput{Stacks: []cloudformationtypes.Stack{
				{StackName: ptr("parent-stack")},
				{StackName: ptr("nested-stack"), ParentId: ptr("arn:aws:cloudformation:us-east-1:123:stack/parent-stack/1")},
			}}, nil
		},
		listStackResourcesFn: func(_ context.Context, in *cloudformation.ListStackResourcesInput, _ ...func(*cloudformation.Options)) (*cloudformation.ListStackResourcesOutput, error) {
			if pointerToString(in.StackName) == "parent-stack" {
				return &cloudformation.ListStackResourcesOutput{StackResourceSummaries: []cloudformationtypes.StackResourceSummary{{LogicalResourceId: ptr("AppBucket"), ResourceType: ptr("AWS::S3::Bucket"), ResourceStatus: cloudformationtypes.ResourceStatusCreateComplete}}}, nil
			}
			return &cloudformation.ListStackResourcesOutput{StackResourceSummaries: []cloudformationtypes.StackResourceSummary{{LogicalResourceId: ptr("NestedBucket"), ResourceType: ptr("AWS::S3::Bucket"), ResourceStatus: cloudformationtypes.ResourceStatusCreateComplete}}}, nil
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

	withMockCFNDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) cfnAPI { return client },
	)

	output, err := executeCommand(t, "--output", "json", "cfn", "find-stack-by-resource", "--resource", "AWS::S3::Bucket", "--exact", "--include-nested")
	if err != nil {
		t.Fatalf("execute cfn find-stack-by-resource: %v", err)
	}
	if !strings.Contains(output, "parent-stack") || !strings.Contains(output, "nested-stack") {
		t.Fatalf("expected both parent and nested matches, output=%s", output)
	}
}

func TestMilestone5R53CreateHealthChecksRequiresDomains(t *testing.T) {
	_, err := executeCommand(t, "r53", "create-health-checks")
	if err == nil || !strings.Contains(err.Error(), "--domains is required") {
		t.Fatalf("expected required domains error, got %v", err)
	}
}

func TestMilestone5R53CreateHealthChecksDryRun(t *testing.T) {
	createCalls := 0
	tagCalls := 0

	client := &mockR53Client{
		createHealthCheckFn: func(_ context.Context, _ *route53.CreateHealthCheckInput, _ ...func(*route53.Options)) (*route53.CreateHealthCheckOutput, error) {
			createCalls++
			return &route53.CreateHealthCheckOutput{}, nil
		},
		changeTagsForResourceFn: func(_ context.Context, _ *route53.ChangeTagsForResourceInput, _ ...func(*route53.Options)) (*route53.ChangeTagsForResourceOutput, error) {
			tagCalls++
			return &route53.ChangeTagsForResourceOutput{}, nil
		},
	}

	withMockR53Deps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) r53API { return client },
	)

	output, err := executeCommand(t, "--output", "json", "--dry-run", "r53", "create-health-checks", "--domains", "example.com,api.example.com")
	if err != nil {
		t.Fatalf("execute r53 create-health-checks dry-run: %v", err)
	}
	if createCalls != 0 || tagCalls != 0 {
		t.Fatalf("expected no API calls in dry-run, got create=%d tag=%d", createCalls, tagCalls)
	}
	if !strings.Contains(output, "would-create") || !strings.Contains(output, "api.example.com") || !strings.Contains(output, "example.com") {
		t.Fatalf("unexpected output: %s", output)
	}
}

func TestMilestone5R53CreateHealthChecksNoConfirmExecutes(t *testing.T) {
	createCalls := 0
	tagCalls := 0

	client := &mockR53Client{
		createHealthCheckFn: func(_ context.Context, in *route53.CreateHealthCheckInput, _ ...func(*route53.Options)) (*route53.CreateHealthCheckOutput, error) {
			createCalls++
			domain := pointerToString(in.HealthCheckConfig.FullyQualifiedDomainName)
			return &route53.CreateHealthCheckOutput{HealthCheck: &route53types.HealthCheck{Id: ptr("hc-" + strings.ReplaceAll(domain, ".", "-"))}}, nil
		},
		changeTagsForResourceFn: func(_ context.Context, _ *route53.ChangeTagsForResourceInput, _ ...func(*route53.Options)) (*route53.ChangeTagsForResourceOutput, error) {
			tagCalls++
			return &route53.ChangeTagsForResourceOutput{}, nil
		},
	}

	withMockR53Deps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) r53API { return client },
	)

	output, err := executeCommand(t, "--output", "json", "--no-confirm", "r53", "create-health-checks", "--domains", "example.com,api.example.com")
	if err != nil {
		t.Fatalf("execute r53 create-health-checks --no-confirm: %v", err)
	}
	if createCalls != 2 || tagCalls != 2 {
		t.Fatalf("unexpected API call counts create=%d tag=%d", createCalls, tagCalls)
	}
	if !strings.Contains(output, "created") || !strings.Contains(output, "hc-api-example-com") || !strings.Contains(output, "hc-example-com") {
		t.Fatalf("unexpected output: %s", output)
	}
}
