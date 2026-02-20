package cloudwatch

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	cloudwatchlogstypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	"github.com/towardsthecloud/aws-toolbox/internal/cliutil"
)

type mockClient struct {
	describeLogGroupsFn func(context.Context, *cloudwatchlogs.DescribeLogGroupsInput, ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeLogGroupsOutput, error)
	deleteLogGroupFn    func(context.Context, *cloudwatchlogs.DeleteLogGroupInput, ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DeleteLogGroupOutput, error)
	putRetentionFn      func(context.Context, *cloudwatchlogs.PutRetentionPolicyInput, ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.PutRetentionPolicyOutput, error)
}

func (m *mockClient) DescribeLogGroups(ctx context.Context, in *cloudwatchlogs.DescribeLogGroupsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeLogGroupsOutput, error) {
	if m.describeLogGroupsFn == nil {
		return nil, errors.New("DescribeLogGroups not mocked")
	}
	return m.describeLogGroupsFn(ctx, in, optFns...)
}

func (m *mockClient) DeleteLogGroup(ctx context.Context, in *cloudwatchlogs.DeleteLogGroupInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DeleteLogGroupOutput, error) {
	if m.deleteLogGroupFn == nil {
		return nil, errors.New("DeleteLogGroup not mocked")
	}
	return m.deleteLogGroupFn(ctx, in, optFns...)
}

func (m *mockClient) PutRetentionPolicy(ctx context.Context, in *cloudwatchlogs.PutRetentionPolicyInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.PutRetentionPolicyOutput, error) {
	if m.putRetentionFn == nil {
		return nil, errors.New("PutRetentionPolicy not mocked")
	}
	return m.putRetentionFn(ctx, in, optFns...)
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
	return executeCommandWithInput(t, "", args...)
}

func executeCommandWithInput(t *testing.T, input string, args ...string) (string, error) {
	t.Helper()

	root := cliutil.NewTestRootCommand(NewCommand())
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetIn(strings.NewReader(input))
	root.SetArgs(args)

	err := root.Execute()
	return buf.String(), err
}

func TestCloudWatchCountLogGroups(t *testing.T) {
	client := &mockClient{
		describeLogGroupsFn: func(_ context.Context, _ *cloudwatchlogs.DescribeLogGroupsInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeLogGroupsOutput, error) {
			return &cloudwatchlogs.DescribeLogGroupsOutput{LogGroups: []cloudwatchlogstypes.LogGroup{{}, {}, {}}}, nil
		},
	}

	withMockDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) API { return client },
	)

	output, err := executeCommand(t, "--output", "json", "cloudwatch", "count-log-groups")
	if err != nil {
		t.Fatalf("execute count-log-groups: %v", err)
	}
	if !strings.Contains(output, "total_log_groups") || !strings.Contains(output, "3") {
		t.Fatalf("unexpected output: %s", output)
	}
}

func TestCloudWatchListLogGroups(t *testing.T) {
	created := time.Now().UTC().Add(-48 * time.Hour).UnixMilli()
	client := &mockClient{
		describeLogGroupsFn: func(_ context.Context, _ *cloudwatchlogs.DescribeLogGroupsInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeLogGroupsOutput, error) {
			return &cloudwatchlogs.DescribeLogGroupsOutput{LogGroups: []cloudwatchlogstypes.LogGroup{{
				LogGroupName:    cliutil.Ptr("/aws/lambda/service"),
				CreationTime:    &created,
				RetentionInDays: cliutil.Ptr(int32(14)),
			}}}, nil
		},
	}

	withMockDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) API { return client },
	)

	output, err := executeCommand(t, "--output", "text", "cloudwatch", "list-log-groups")
	if err != nil {
		t.Fatalf("execute list-log-groups: %v", err)
	}
	if !strings.Contains(output, "/aws/lambda/service") || !strings.Contains(output, "retention_days=14") {
		t.Fatalf("unexpected output: %s", output)
	}
}

func TestCloudWatchDeleteLogGroupsDryRunWithRetentionAndNameFilter(t *testing.T) {
	old := time.Now().UTC().AddDate(0, 0, -40).UnixMilli()
	recent := time.Now().UTC().AddDate(0, 0, -5).UnixMilli()
	client := &mockClient{
		describeLogGroupsFn: func(_ context.Context, _ *cloudwatchlogs.DescribeLogGroupsInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeLogGroupsOutput, error) {
			return &cloudwatchlogs.DescribeLogGroupsOutput{LogGroups: []cloudwatchlogstypes.LogGroup{
				{LogGroupName: cliutil.Ptr("/aws/lambda/delete-me"), CreationTime: &old, RetentionInDays: cliutil.Ptr(int32(7))},
				{LogGroupName: cliutil.Ptr("/aws/lambda/keep-me"), CreationTime: &recent, RetentionInDays: cliutil.Ptr(int32(7))},
				{LogGroupName: cliutil.Ptr("/aws/rds/delete-me"), CreationTime: &old, RetentionInDays: cliutil.Ptr(int32(7))},
			}}, nil
		},
	}

	withMockDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) API { return client },
	)

	output, err := executeCommand(t, "--output", "json", "--dry-run", "cloudwatch", "delete-log-groups", "--retention-days", "30", "--filter-name-contains", "lambda")
	if err != nil {
		t.Fatalf("execute delete-log-groups: %v", err)
	}

	if !strings.Contains(output, "/aws/lambda/delete-me") || strings.Contains(output, "/aws/lambda/keep-me") || strings.Contains(output, "/aws/rds/delete-me") {
		t.Fatalf("unexpected output: %s", output)
	}
	if !strings.Contains(output, "would-delete") {
		t.Fatalf("expected dry-run action in output: %s", output)
	}
}

func TestCloudWatchDeleteLogGroupsExecutesWhenNoConfirm(t *testing.T) {
	deleted := 0
	old := time.Now().UTC().AddDate(0, 0, -45).UnixMilli()
	client := &mockClient{
		describeLogGroupsFn: func(_ context.Context, _ *cloudwatchlogs.DescribeLogGroupsInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeLogGroupsOutput, error) {
			return &cloudwatchlogs.DescribeLogGroupsOutput{LogGroups: []cloudwatchlogstypes.LogGroup{{LogGroupName: cliutil.Ptr("/aws/lambda/delete-me"), CreationTime: &old}}}, nil
		},
		deleteLogGroupFn: func(_ context.Context, _ *cloudwatchlogs.DeleteLogGroupInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DeleteLogGroupOutput, error) {
			deleted++
			return &cloudwatchlogs.DeleteLogGroupOutput{}, nil
		},
	}

	withMockDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) API { return client },
	)

	output, err := executeCommand(t, "--output", "json", "--no-confirm", "cloudwatch", "delete-log-groups", "--retention-days", "30", "--filter-name-contains", "lambda")
	if err != nil {
		t.Fatalf("execute delete-log-groups: %v", err)
	}
	if deleted != 1 || !strings.Contains(output, "deleted") {
		t.Fatalf("unexpected output: %s", output)
	}
}

func TestCloudWatchSetRetentionPrintCounts(t *testing.T) {
	client := &mockClient{
		describeLogGroupsFn: func(_ context.Context, _ *cloudwatchlogs.DescribeLogGroupsInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeLogGroupsOutput, error) {
			return &cloudwatchlogs.DescribeLogGroupsOutput{LogGroups: []cloudwatchlogstypes.LogGroup{
				{LogGroupName: cliutil.Ptr("a"), RetentionInDays: cliutil.Ptr(int32(7))},
				{LogGroupName: cliutil.Ptr("b"), RetentionInDays: cliutil.Ptr(int32(7))},
				{LogGroupName: cliutil.Ptr("c")},
			}}, nil
		},
	}

	withMockDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) API { return client },
	)

	output, err := executeCommand(t, "--output", "table", "cloudwatch", "set-retention", "--print-retention-counts")
	if err != nil {
		t.Fatalf("execute set-retention --print-retention-counts: %v", err)
	}

	if !strings.Contains(output, "not_set") || !strings.Contains(output, "7") {
		t.Fatalf("unexpected output: %s", output)
	}
}

func TestCloudWatchSetRetentionDryRun(t *testing.T) {
	client := &mockClient{
		describeLogGroupsFn: func(_ context.Context, _ *cloudwatchlogs.DescribeLogGroupsInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeLogGroupsOutput, error) {
			return &cloudwatchlogs.DescribeLogGroupsOutput{LogGroups: []cloudwatchlogstypes.LogGroup{
				{LogGroupName: cliutil.Ptr("/aws/lambda/api"), RetentionInDays: cliutil.Ptr(int32(7))},
				{LogGroupName: cliutil.Ptr("/aws/lambda/worker")},
			}}, nil
		},
		putRetentionFn: func(_ context.Context, _ *cloudwatchlogs.PutRetentionPolicyInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.PutRetentionPolicyOutput, error) {
			return &cloudwatchlogs.PutRetentionPolicyOutput{}, nil
		},
	}

	withMockDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) API { return client },
	)

	output, err := executeCommand(t, "--output", "json", "--dry-run", "cloudwatch", "set-retention", "--retention-days", "30")
	if err != nil {
		t.Fatalf("execute set-retention --retention-days: %v", err)
	}

	if !strings.Contains(output, "would-update") || !strings.Contains(output, "/aws/lambda/api") {
		t.Fatalf("unexpected output: %s", output)
	}
}

func TestCloudWatchSetRetentionExecutesWhenNoConfirm(t *testing.T) {
	updated := 0
	client := &mockClient{
		describeLogGroupsFn: func(_ context.Context, _ *cloudwatchlogs.DescribeLogGroupsInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeLogGroupsOutput, error) {
			return &cloudwatchlogs.DescribeLogGroupsOutput{LogGroups: []cloudwatchlogstypes.LogGroup{{LogGroupName: cliutil.Ptr("/aws/lambda/api"), RetentionInDays: cliutil.Ptr(int32(7))}}}, nil
		},
		putRetentionFn: func(_ context.Context, _ *cloudwatchlogs.PutRetentionPolicyInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.PutRetentionPolicyOutput, error) {
			updated++
			return &cloudwatchlogs.PutRetentionPolicyOutput{}, nil
		},
	}

	withMockDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) API { return client },
	)

	output, err := executeCommand(t, "--output", "json", "--no-confirm", "cloudwatch", "set-retention", "--retention-days", "30")
	if err != nil {
		t.Fatalf("execute set-retention --retention-days: %v", err)
	}
	if updated != 1 || !strings.Contains(output, "updated") {
		t.Fatalf("unexpected output: %s", output)
	}
}

func TestCloudWatchCountLogGroupsAllOutputFormats(t *testing.T) {
	client := &mockClient{
		describeLogGroupsFn: func(_ context.Context, _ *cloudwatchlogs.DescribeLogGroupsInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeLogGroupsOutput, error) {
			return &cloudwatchlogs.DescribeLogGroupsOutput{LogGroups: []cloudwatchlogstypes.LogGroup{{}}}, nil
		},
	}

	withMockDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) API { return client },
	)

	for _, format := range []string{"table", "json", "text"} {
		output, err := executeCommand(t, "--output", format, "cloudwatch", "count-log-groups")
		if err != nil {
			t.Fatalf("execute count-log-groups (%s): %v", format, err)
		}
		if !strings.Contains(output, "total_log_groups") {
			t.Fatalf("expected metric in output for format=%s: %s", format, output)
		}
	}
}

func TestCloudWatchValidationAndHelpers(t *testing.T) {
	client := &mockClient{
		describeLogGroupsFn: func(_ context.Context, _ *cloudwatchlogs.DescribeLogGroupsInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeLogGroupsOutput, error) {
			return &cloudwatchlogs.DescribeLogGroupsOutput{}, nil
		},
	}
	withMockDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) API { return client },
	)

	if _, err := executeCommand(t, "cloudwatch", "set-retention"); err == nil {
		t.Fatal("expected missing-flag error")
	}
	if _, err := executeCommand(t, "cloudwatch", "delete-log-groups", "--retention-days", "-1"); err == nil {
		t.Fatal("expected retention-days validation error")
	}
	if _, err := executeCommand(t, "cloudwatch", "set-retention", "--retention-days", "30", "--print-retention-counts"); err == nil {
		t.Fatal("expected incompatible flags error")
	}
}

func TestCloudWatchDeleteLogGroupsCancelledPrompt(t *testing.T) {
	old := time.Now().UTC().AddDate(0, 0, -45).UnixMilli()
	client := &mockClient{
		describeLogGroupsFn: func(_ context.Context, _ *cloudwatchlogs.DescribeLogGroupsInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeLogGroupsOutput, error) {
			return &cloudwatchlogs.DescribeLogGroupsOutput{LogGroups: []cloudwatchlogstypes.LogGroup{{LogGroupName: cliutil.Ptr("/aws/lambda/delete-me"), CreationTime: &old}}}, nil
		},
	}

	withMockDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) API { return client },
	)

	output, err := executeCommandWithInput(t, "n\n", "--output", "json", "cloudwatch", "delete-log-groups", "--retention-days", "30")
	if err != nil {
		t.Fatalf("execute delete-log-groups with prompt: %v", err)
	}
	if !strings.Contains(output, "cancelled") {
		t.Fatalf("expected cancelled action: %s", output)
	}
}

func TestCloudWatchSetRetentionNoTargets(t *testing.T) {
	client := &mockClient{
		describeLogGroupsFn: func(_ context.Context, _ *cloudwatchlogs.DescribeLogGroupsInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeLogGroupsOutput, error) {
			return &cloudwatchlogs.DescribeLogGroupsOutput{LogGroups: []cloudwatchlogstypes.LogGroup{
				{LogGroupName: cliutil.Ptr("/aws/lambda/api"), RetentionInDays: cliutil.Ptr(int32(30))},
			}}, nil
		},
	}

	withMockDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) API { return client },
	)

	output, err := executeCommand(t, "--output", "json", "cloudwatch", "set-retention", "--retention-days", "30")
	if err != nil {
		t.Fatalf("execute set-retention no-targets: %v", err)
	}
	if !strings.Contains(output, "[]") {
		t.Fatalf("expected empty result set: %s", output)
	}
}

func TestCloudWatchSetRetentionCancelledPrompt(t *testing.T) {
	client := &mockClient{
		describeLogGroupsFn: func(_ context.Context, _ *cloudwatchlogs.DescribeLogGroupsInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeLogGroupsOutput, error) {
			return &cloudwatchlogs.DescribeLogGroupsOutput{LogGroups: []cloudwatchlogstypes.LogGroup{
				{LogGroupName: cliutil.Ptr("/aws/lambda/api"), RetentionInDays: cliutil.Ptr(int32(7))},
			}}, nil
		},
	}

	withMockDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) API { return client },
	)

	output, err := executeCommandWithInput(t, "n\n", "--output", "json", "cloudwatch", "set-retention", "--retention-days", "30")
	if err != nil {
		t.Fatalf("execute set-retention with prompt: %v", err)
	}
	if !strings.Contains(output, "cancelled") {
		t.Fatalf("expected cancelled action: %s", output)
	}
}

func TestCloudWatchCountLogGroupsError(t *testing.T) {
	client := &mockClient{
		describeLogGroupsFn: func(_ context.Context, _ *cloudwatchlogs.DescribeLogGroupsInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeLogGroupsOutput, error) {
			return nil, errors.New("boom")
		},
	}
	withMockDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) API { return client },
	)

	if _, err := executeCommand(t, "cloudwatch", "count-log-groups"); err == nil {
		t.Fatal("expected error")
	}
}
