package ecs

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/towardsthecloud/aws-toolbox/internal/cliutil"
)

// ---------------------------------------------------------------------------
// Mock types
// ---------------------------------------------------------------------------

type mockRunner struct {
	calls   []mockCommandCall
	results []mockRunnerResult
}

type mockCommandCall struct {
	binary string
	args   []string
	stdin  string
}

type mockRunnerResult struct {
	output string
	err    error
}

func (m *mockRunner) Run(_ context.Context, binary string, args []string, stdin string) (string, error) {
	idx := len(m.calls)
	m.calls = append(m.calls, mockCommandCall{binary: binary, args: append([]string(nil), args...), stdin: stdin})

	if idx < len(m.results) {
		return m.results[idx].output, m.results[idx].err
	}

	// Default: aws returns password, everything else succeeds
	if binary == "aws" {
		return "mock-password", nil
	}
	return "", nil
}

type mockClient struct {
	deleteTaskDefinitionsFn func(context.Context, *ecs.DeleteTaskDefinitionsInput, ...func(*ecs.Options)) (*ecs.DeleteTaskDefinitionsOutput, error)
	listTaskDefinitionsFn   func(context.Context, *ecs.ListTaskDefinitionsInput, ...func(*ecs.Options)) (*ecs.ListTaskDefinitionsOutput, error)
}

func (m *mockClient) DeleteTaskDefinitions(ctx context.Context, in *ecs.DeleteTaskDefinitionsInput, optFns ...func(*ecs.Options)) (*ecs.DeleteTaskDefinitionsOutput, error) {
	if m.deleteTaskDefinitionsFn == nil {
		return nil, errors.New("DeleteTaskDefinitions not mocked")
	}
	return m.deleteTaskDefinitionsFn(ctx, in, optFns...)
}

func (m *mockClient) ListTaskDefinitions(ctx context.Context, in *ecs.ListTaskDefinitionsInput, optFns ...func(*ecs.Options)) (*ecs.ListTaskDefinitionsOutput, error) {
	if m.listTaskDefinitionsFn == nil {
		return nil, errors.New("ListTaskDefinitions not mocked")
	}
	return m.listTaskDefinitionsFn(ctx, in, optFns...)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

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

func withMockRunner(t *testing.T, r *mockRunner) {
	t.Helper()
	oldRunner := runner
	runner = r
	t.Cleanup(func() { runner = oldRunner })
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

func defaultLoader(_, _ string) (awssdk.Config, error) {
	return awssdk.Config{Region: "us-east-1"}, nil
}

func clientFactory(c API) func(awssdk.Config) API {
	return func(awssdk.Config) API { return c }
}

// ---------------------------------------------------------------------------
// Tests: publish-image
// ---------------------------------------------------------------------------

func TestPublishImageRunsPipeline(t *testing.T) {
	r := &mockRunner{}
	withMockRunner(t, r)

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

	if len(r.calls) != 5 {
		t.Fatalf("expected 5 command executions, got %d", len(r.calls))
	}

	expected := []string{
		"aws ecr get-login-password",
		"docker login --username AWS --password-stdin 123456789012.dkr.ecr.us-east-1.amazonaws.com",
		"docker build -t awstbx-ecs-publish:v1 -f Dockerfile .",
		"docker tag awstbx-ecs-publish:v1 123456789012.dkr.ecr.us-east-1.amazonaws.com/my-app:v1",
		"docker push 123456789012.dkr.ecr.us-east-1.amazonaws.com/my-app:v1",
	}

	for i, want := range expected {
		got := r.calls[i].binary + " " + strings.Join(r.calls[i].args, " ")
		if got != want {
			t.Fatalf("unexpected command %d: got %q want %q", i, got, want)
		}
	}

	if r.calls[1].stdin == "" {
		t.Fatal("expected docker login stdin to contain password")
	}
	if !strings.Contains(output, "\"step\": \"push\"") || !strings.Contains(output, "\"action\": \"completed\"") {
		t.Fatalf("unexpected output: %s", output)
	}
}

func TestPublishImageRequiresECRURL(t *testing.T) {
	if _, err := executeCommand(t, "ecs", "publish-image"); err == nil || !strings.Contains(err.Error(), "--ecr-url is required") {
		t.Fatalf("expected required flag validation error, got %v", err)
	}
}

func TestPublishImageDryRun(t *testing.T) {
	r := &mockRunner{}
	withMockRunner(t, r)

	output, err := executeCommand(
		t,
		"--output", "json", "--dry-run",
		"ecs", "publish-image",
		"--ecr-url", "123456789012.dkr.ecr.us-east-1.amazonaws.com/my-app",
		"--tag", "v1",
	)
	if err != nil {
		t.Fatalf("execute ecs publish-image dry-run: %v", err)
	}

	if len(r.calls) != 0 {
		t.Fatalf("expected no commands in dry-run, got %d", len(r.calls))
	}

	if !strings.Contains(output, "would-run") {
		t.Fatalf("expected would-run in dry-run output, got: %s", output)
	}
}

func TestPublishImageAWSLoginFails(t *testing.T) {
	r := &mockRunner{
		results: []mockRunnerResult{
			{output: "", err: errors.New("aws ecr get-login-password: no credentials")},
		},
	}
	withMockRunner(t, r)

	output, err := executeCommand(
		t,
		"--output", "json",
		"ecs", "publish-image",
		"--ecr-url", "123456789012.dkr.ecr.us-east-1.amazonaws.com/my-app",
		"--tag", "v1",
	)
	if err != nil {
		t.Fatalf("command should not return error on login failure (writes table): %v", err)
	}

	if len(r.calls) != 1 {
		t.Fatalf("expected only 1 command call (aws), got %d", len(r.calls))
	}

	if !strings.Contains(output, "failed:") {
		t.Fatalf("expected failed action in output, got: %s", output)
	}
	if !strings.Contains(output, "skipped:") {
		t.Fatalf("expected skipped actions in output, got: %s", output)
	}
}

func TestPublishImageDockerLoginFails(t *testing.T) {
	r := &mockRunner{
		results: []mockRunnerResult{
			{output: "mock-password", err: nil},
			{output: "", err: errors.New("docker login: unauthorized")},
		},
	}
	withMockRunner(t, r)

	output, err := executeCommand(
		t,
		"--output", "json",
		"ecs", "publish-image",
		"--ecr-url", "123456789012.dkr.ecr.us-east-1.amazonaws.com/my-app",
		"--tag", "v1",
	)
	if err != nil {
		t.Fatalf("command should not return error on docker login failure: %v", err)
	}

	if len(r.calls) != 2 {
		t.Fatalf("expected 2 command calls, got %d", len(r.calls))
	}

	if !strings.Contains(output, "failed:") {
		t.Fatalf("expected failed action in output, got: %s", output)
	}
	if !strings.Contains(output, "skipped:") {
		t.Fatalf("expected skipped actions in output, got: %s", output)
	}
}

func TestPublishImageBuildFails(t *testing.T) {
	r := &mockRunner{
		results: []mockRunnerResult{
			{output: "mock-password", err: nil},                              // aws ecr get-login-password
			{output: "Login Succeeded", err: nil},                            // docker login
			{output: "", err: errors.New("docker build: compilation error")}, // docker build
		},
	}
	withMockRunner(t, r)

	output, err := executeCommand(
		t,
		"--output", "json",
		"ecs", "publish-image",
		"--ecr-url", "123456789012.dkr.ecr.us-east-1.amazonaws.com/my-app",
		"--tag", "v1",
	)
	if err != nil {
		t.Fatalf("command should not return error on build failure: %v", err)
	}

	if len(r.calls) != 3 {
		t.Fatalf("expected 3 command calls, got %d", len(r.calls))
	}

	if !strings.Contains(output, "failed:") {
		t.Fatalf("expected failed action for build step, got: %s", output)
	}
	if !strings.Contains(output, "skipped:") {
		t.Fatalf("expected skipped actions for tag/push, got: %s", output)
	}
}

func TestPublishImageTagFails(t *testing.T) {
	r := &mockRunner{
		results: []mockRunnerResult{
			{output: "mock-password", err: nil},      // aws ecr get-login-password
			{output: "Login Succeeded", err: nil},    // docker login
			{output: "", err: nil},                   // docker build
			{output: "", err: errors.New("tag err")}, // docker tag
		},
	}
	withMockRunner(t, r)

	output, err := executeCommand(
		t,
		"--output", "json",
		"ecs", "publish-image",
		"--ecr-url", "123456789012.dkr.ecr.us-east-1.amazonaws.com/my-app",
		"--tag", "v1",
	)
	if err != nil {
		t.Fatalf("command should not return error on tag failure: %v", err)
	}

	if len(r.calls) != 4 {
		t.Fatalf("expected 4 command calls, got %d", len(r.calls))
	}

	if !strings.Contains(output, "failed:") {
		t.Fatalf("expected failed action for tag step, got: %s", output)
	}
	// push should be skipped
	if !strings.Contains(output, "skipped:") {
		t.Fatalf("expected skipped action for push step, got: %s", output)
	}
}

func TestPublishImagePushFails(t *testing.T) {
	r := &mockRunner{
		results: []mockRunnerResult{
			{output: "mock-password", err: nil},       // aws ecr get-login-password
			{output: "Login Succeeded", err: nil},     // docker login
			{output: "", err: nil},                    // docker build
			{output: "", err: nil},                    // docker tag
			{output: "", err: errors.New("push err")}, // docker push
		},
	}
	withMockRunner(t, r)

	output, err := executeCommand(
		t,
		"--output", "json",
		"ecs", "publish-image",
		"--ecr-url", "123456789012.dkr.ecr.us-east-1.amazonaws.com/my-app",
		"--tag", "v1",
	)
	if err != nil {
		t.Fatalf("command should not return error on push failure: %v", err)
	}

	if !strings.Contains(output, "failed:") {
		t.Fatalf("expected failed action for push step, got: %s", output)
	}
}

func TestPublishImageWithProfileAndRegion(t *testing.T) {
	r := &mockRunner{}
	withMockRunner(t, r)

	_, err := executeCommand(
		t,
		"--output", "json", "--profile", "myprofile", "--region", "eu-west-1",
		"ecs", "publish-image",
		"--ecr-url", "123456789012.dkr.ecr.eu-west-1.amazonaws.com/my-app",
		"--tag", "v1",
	)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}

	// The first call (aws) should include --profile and --region
	if len(r.calls) == 0 {
		t.Fatal("expected at least 1 call")
	}
	awsCall := r.calls[0]
	awsArgs := strings.Join(awsCall.args, " ")
	if !strings.Contains(awsArgs, "--profile myprofile") {
		t.Fatalf("expected aws call to contain --profile, got: %s", awsArgs)
	}
	if !strings.Contains(awsArgs, "--region eu-west-1") {
		t.Fatalf("expected aws call to contain --region, got: %s", awsArgs)
	}
}

func TestPublishImageECRURLWithoutSlash(t *testing.T) {
	r := &mockRunner{}
	withMockRunner(t, r)

	_, err := executeCommand(
		t,
		"--output", "json",
		"ecs", "publish-image",
		"--ecr-url", "123456789012.dkr.ecr.us-east-1.amazonaws.com",
		"--tag", "v1",
	)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}

	// docker login should use the full URL as registry when no slash
	if len(r.calls) < 2 {
		t.Fatal("expected at least 2 calls")
	}
	loginArgs := strings.Join(r.calls[1].args, " ")
	if !strings.Contains(loginArgs, "123456789012.dkr.ecr.us-east-1.amazonaws.com") {
		t.Fatalf("expected registry in docker login args, got: %s", loginArgs)
	}
}

func TestPublishImageTagWithSlash(t *testing.T) {
	r := &mockRunner{}
	withMockRunner(t, r)

	_, err := executeCommand(
		t,
		"--output", "json",
		"ecs", "publish-image",
		"--ecr-url", "123456789012.dkr.ecr.us-east-1.amazonaws.com/my-app",
		"--tag", "feature/branch",
	)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}

	// The local image ref should have slashes replaced with dashes
	if len(r.calls) < 3 {
		t.Fatal("expected at least 3 calls")
	}
	buildArgs := strings.Join(r.calls[2].args, " ")
	if !strings.Contains(buildArgs, "awstbx-ecs-publish:feature-branch") {
		t.Fatalf("expected sanitized local tag, got: %s", buildArgs)
	}
}

func TestPublishImageValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "empty dockerfile",
			args:    []string{"ecs", "publish-image", "--ecr-url", "url", "--dockerfile", "  ", "--tag", "v1"},
			wantErr: "--dockerfile is required",
		},
		{
			name:    "empty tag",
			args:    []string{"ecs", "publish-image", "--ecr-url", "url", "--tag", "  "},
			wantErr: "--tag is required",
		},
		{
			name:    "empty context",
			args:    []string{"ecs", "publish-image", "--ecr-url", "url", "--tag", "v1", "--context", "  "},
			wantErr: "--context is required",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := executeCommand(t, tc.args...)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("expected error containing %q, got %v", tc.wantErr, err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Tests: delete-task-definitions
// ---------------------------------------------------------------------------

func TestDeleteTaskDefinitionsConfigLoadError(t *testing.T) {
	withMockDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{}, errors.New("bad creds") },
		clientFactory(&mockClient{}),
	)

	_, err := executeCommand(t, "--output", "json", "ecs", "delete-task-definitions")
	if err == nil || !strings.Contains(err.Error(), "bad creds") {
		t.Fatalf("expected config load error, got %v", err)
	}
}

func TestDeleteTaskDefinitionsListError(t *testing.T) {
	client := &mockClient{
		listTaskDefinitionsFn: func(_ context.Context, _ *ecs.ListTaskDefinitionsInput, _ ...func(*ecs.Options)) (*ecs.ListTaskDefinitionsOutput, error) {
			return nil, errors.New("service unavailable")
		},
	}

	withMockDeps(t, defaultLoader, clientFactory(client))

	_, err := executeCommand(t, "--output", "json", "ecs", "delete-task-definitions")
	if err == nil || !strings.Contains(err.Error(), "list inactive task definitions") {
		t.Fatalf("expected list error, got %v", err)
	}
}

func TestDeleteTaskDefinitionsEmptyList(t *testing.T) {
	client := &mockClient{
		listTaskDefinitionsFn: func(_ context.Context, _ *ecs.ListTaskDefinitionsInput, _ ...func(*ecs.Options)) (*ecs.ListTaskDefinitionsOutput, error) {
			return &ecs.ListTaskDefinitionsOutput{}, nil
		},
	}

	withMockDeps(t, defaultLoader, clientFactory(client))

	output, err := executeCommand(t, "--output", "json", "ecs", "delete-task-definitions")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}

	// Empty list should produce output without errors
	if strings.Contains(output, "pending") || strings.Contains(output, "deleted") {
		t.Fatalf("expected no pending/deleted actions for empty list, got: %s", output)
	}
}

func TestDeleteTaskDefinitionsDryRun(t *testing.T) {
	deleteCalls := 0
	client := &mockClient{
		listTaskDefinitionsFn: func(_ context.Context, _ *ecs.ListTaskDefinitionsInput, _ ...func(*ecs.Options)) (*ecs.ListTaskDefinitionsOutput, error) {
			return &ecs.ListTaskDefinitionsOutput{
				TaskDefinitionArns: []string{
					"arn:aws:ecs:us-east-1:123456789012:task-definition/my-task:1",
					"arn:aws:ecs:us-east-1:123456789012:task-definition/my-task:2",
				},
			}, nil
		},
		deleteTaskDefinitionsFn: func(_ context.Context, _ *ecs.DeleteTaskDefinitionsInput, _ ...func(*ecs.Options)) (*ecs.DeleteTaskDefinitionsOutput, error) {
			deleteCalls++
			return &ecs.DeleteTaskDefinitionsOutput{}, nil
		},
	}

	withMockDeps(t, defaultLoader, clientFactory(client))

	output, err := executeCommand(t, "--output", "json", "--dry-run", "ecs", "delete-task-definitions")
	if err != nil {
		t.Fatalf("execute dry-run: %v", err)
	}

	if deleteCalls != 0 {
		t.Fatalf("expected no delete calls in dry-run, got %d", deleteCalls)
	}
	if !strings.Contains(output, "would-delete") {
		t.Fatalf("expected would-delete in dry-run output, got: %s", output)
	}
}

func TestDeleteTaskDefinitionsNoConfirmExecutes(t *testing.T) {
	deleteCalls := 0
	client := &mockClient{
		listTaskDefinitionsFn: func(_ context.Context, _ *ecs.ListTaskDefinitionsInput, _ ...func(*ecs.Options)) (*ecs.ListTaskDefinitionsOutput, error) {
			return &ecs.ListTaskDefinitionsOutput{
				TaskDefinitionArns: []string{
					"arn:aws:ecs:us-east-1:123456789012:task-definition/my-task:1",
				},
			}, nil
		},
		deleteTaskDefinitionsFn: func(_ context.Context, in *ecs.DeleteTaskDefinitionsInput, _ ...func(*ecs.Options)) (*ecs.DeleteTaskDefinitionsOutput, error) {
			deleteCalls++
			return &ecs.DeleteTaskDefinitionsOutput{}, nil
		},
	}

	withMockDeps(t, defaultLoader, clientFactory(client))

	output, err := executeCommand(t, "--output", "json", "--no-confirm", "ecs", "delete-task-definitions")
	if err != nil {
		t.Fatalf("execute --no-confirm: %v", err)
	}

	if deleteCalls != 1 {
		t.Fatalf("expected 1 delete call, got %d", deleteCalls)
	}
	if !strings.Contains(output, "deleted") {
		t.Fatalf("expected deleted in output, got: %s", output)
	}
}

func TestDeleteTaskDefinitionsDeleteError(t *testing.T) {
	client := &mockClient{
		listTaskDefinitionsFn: func(_ context.Context, _ *ecs.ListTaskDefinitionsInput, _ ...func(*ecs.Options)) (*ecs.ListTaskDefinitionsOutput, error) {
			return &ecs.ListTaskDefinitionsOutput{
				TaskDefinitionArns: []string{
					"arn:aws:ecs:us-east-1:123456789012:task-definition/my-task:1",
				},
			}, nil
		},
		deleteTaskDefinitionsFn: func(_ context.Context, _ *ecs.DeleteTaskDefinitionsInput, _ ...func(*ecs.Options)) (*ecs.DeleteTaskDefinitionsOutput, error) {
			return nil, errors.New("access denied")
		},
	}

	withMockDeps(t, defaultLoader, clientFactory(client))

	output, err := executeCommand(t, "--output", "json", "--no-confirm", "ecs", "delete-task-definitions")
	if err != nil {
		t.Fatalf("execute should not return error (writes table): %v", err)
	}

	if !strings.Contains(output, "failed:") {
		t.Fatalf("expected failed action in output, got: %s", output)
	}
}

func TestDeleteTaskDefinitionsDeleteWithFailures(t *testing.T) {
	reason := "TASK_DEFINITION_IN_USE"
	detail := "cannot delete active definition"
	client := &mockClient{
		listTaskDefinitionsFn: func(_ context.Context, _ *ecs.ListTaskDefinitionsInput, _ ...func(*ecs.Options)) (*ecs.ListTaskDefinitionsOutput, error) {
			return &ecs.ListTaskDefinitionsOutput{
				TaskDefinitionArns: []string{
					"arn:aws:ecs:us-east-1:123456789012:task-definition/my-task:1",
				},
			}, nil
		},
		deleteTaskDefinitionsFn: func(_ context.Context, _ *ecs.DeleteTaskDefinitionsInput, _ ...func(*ecs.Options)) (*ecs.DeleteTaskDefinitionsOutput, error) {
			return &ecs.DeleteTaskDefinitionsOutput{
				Failures: []ecstypes.Failure{
					{
						Reason: &reason,
						Detail: &detail,
					},
				},
			}, nil
		},
	}

	withMockDeps(t, defaultLoader, clientFactory(client))

	output, err := executeCommand(t, "--output", "json", "--no-confirm", "ecs", "delete-task-definitions")
	if err != nil {
		t.Fatalf("execute should not return error: %v", err)
	}

	if !strings.Contains(output, "failed:") {
		t.Fatalf("expected failed action in output, got: %s", output)
	}
	if !strings.Contains(output, "TASK_DEFINITION_IN_USE") {
		t.Fatalf("expected failure reason in output, got: %s", output)
	}
}

func TestDeleteTaskDefinitionsMultipleARNsSorted(t *testing.T) {
	var deletedARNs []string
	client := &mockClient{
		listTaskDefinitionsFn: func(_ context.Context, _ *ecs.ListTaskDefinitionsInput, _ ...func(*ecs.Options)) (*ecs.ListTaskDefinitionsOutput, error) {
			return &ecs.ListTaskDefinitionsOutput{
				TaskDefinitionArns: []string{
					"arn:aws:ecs:us-east-1:123456789012:task-definition/z-task:1",
					"arn:aws:ecs:us-east-1:123456789012:task-definition/a-task:1",
					"arn:aws:ecs:us-east-1:123456789012:task-definition/m-task:1",
				},
			}, nil
		},
		deleteTaskDefinitionsFn: func(_ context.Context, in *ecs.DeleteTaskDefinitionsInput, _ ...func(*ecs.Options)) (*ecs.DeleteTaskDefinitionsOutput, error) {
			deletedARNs = append(deletedARNs, in.TaskDefinitions[0])
			return &ecs.DeleteTaskDefinitionsOutput{}, nil
		},
	}

	withMockDeps(t, defaultLoader, clientFactory(client))

	_, err := executeCommand(t, "--output", "json", "--no-confirm", "ecs", "delete-task-definitions")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}

	if len(deletedARNs) != 3 {
		t.Fatalf("expected 3 deletes, got %d", len(deletedARNs))
	}

	// Verify sorted order
	if !strings.Contains(deletedARNs[0], "a-task") ||
		!strings.Contains(deletedARNs[1], "m-task") ||
		!strings.Contains(deletedARNs[2], "z-task") {
		t.Fatalf("expected sorted order, got: %v", deletedARNs)
	}
}

// ---------------------------------------------------------------------------
// Tests: listInactiveTaskDefinitionARNs (pagination)
// ---------------------------------------------------------------------------

func TestListInactiveTaskDefinitionARNsSinglePage(t *testing.T) {
	client := &mockClient{
		listTaskDefinitionsFn: func(_ context.Context, in *ecs.ListTaskDefinitionsInput, _ ...func(*ecs.Options)) (*ecs.ListTaskDefinitionsOutput, error) {
			if in.Status != ecstypes.TaskDefinitionStatusInactive {
				t.Fatalf("expected INACTIVE status filter, got: %v", in.Status)
			}
			return &ecs.ListTaskDefinitionsOutput{
				TaskDefinitionArns: []string{"arn:aws:ecs:us-east-1:123:task-definition/task:1"},
			}, nil
		},
	}

	arns, err := listInactiveTaskDefinitionARNs(context.Background(), client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(arns) != 1 || arns[0] != "arn:aws:ecs:us-east-1:123:task-definition/task:1" {
		t.Fatalf("unexpected arns: %v", arns)
	}
}

func TestListInactiveTaskDefinitionARNsMultiplePages(t *testing.T) {
	callCount := 0
	client := &mockClient{
		listTaskDefinitionsFn: func(_ context.Context, in *ecs.ListTaskDefinitionsInput, _ ...func(*ecs.Options)) (*ecs.ListTaskDefinitionsOutput, error) {
			callCount++
			switch callCount {
			case 1:
				if in.NextToken != nil {
					t.Fatal("expected nil token on first call")
				}
				next := "page2"
				return &ecs.ListTaskDefinitionsOutput{
					TaskDefinitionArns: []string{"arn:1"},
					NextToken:          &next,
				}, nil
			case 2:
				if in.NextToken == nil || *in.NextToken != "page2" {
					t.Fatal("expected page2 token on second call")
				}
				next := "page3"
				return &ecs.ListTaskDefinitionsOutput{
					TaskDefinitionArns: []string{"arn:2", "arn:3"},
					NextToken:          &next,
				}, nil
			case 3:
				return &ecs.ListTaskDefinitionsOutput{
					TaskDefinitionArns: []string{"arn:4"},
				}, nil
			default:
				t.Fatal("too many calls")
				return nil, nil
			}
		},
	}

	arns, err := listInactiveTaskDefinitionARNs(context.Background(), client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(arns) != 4 {
		t.Fatalf("expected 4 arns, got %d: %v", len(arns), arns)
	}
	if callCount != 3 {
		t.Fatalf("expected 3 API calls, got %d", callCount)
	}
}

func TestListInactiveTaskDefinitionARNsAPIError(t *testing.T) {
	client := &mockClient{
		listTaskDefinitionsFn: func(_ context.Context, _ *ecs.ListTaskDefinitionsInput, _ ...func(*ecs.Options)) (*ecs.ListTaskDefinitionsOutput, error) {
			return nil, errors.New("throttling")
		},
	}

	_, err := listInactiveTaskDefinitionARNs(context.Background(), client)
	if err == nil || !strings.Contains(err.Error(), "throttling") {
		t.Fatalf("expected throttling error, got: %v", err)
	}
}

func TestListInactiveTaskDefinitionARNsEmptyResult(t *testing.T) {
	client := &mockClient{
		listTaskDefinitionsFn: func(_ context.Context, _ *ecs.ListTaskDefinitionsInput, _ ...func(*ecs.Options)) (*ecs.ListTaskDefinitionsOutput, error) {
			return &ecs.ListTaskDefinitionsOutput{}, nil
		},
	}

	arns, err := listInactiveTaskDefinitionARNs(context.Background(), client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(arns) != 0 {
		t.Fatalf("expected 0 arns, got %d", len(arns))
	}
}

// ---------------------------------------------------------------------------
// Tests: failureReason
// ---------------------------------------------------------------------------

func TestFailureReasonWithReasonAndDetail(t *testing.T) {
	reason := "ACCESS_DENIED"
	detail := "not authorized"
	got := failureReason(ecstypes.Failure{Reason: &reason, Detail: &detail})
	want := "ACCESS_DENIED: not authorized"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestFailureReasonWithReasonOnly(t *testing.T) {
	reason := "MISSING_RESOURCE"
	got := failureReason(ecstypes.Failure{Reason: &reason})
	if got != "MISSING_RESOURCE" {
		t.Fatalf("got %q, want %q", got, "MISSING_RESOURCE")
	}
}

func TestFailureReasonWithDetailOnly(t *testing.T) {
	detail := "some detail"
	got := failureReason(ecstypes.Failure{Detail: &detail})
	if got != "unknown failure: some detail" {
		t.Fatalf("got %q, want %q", got, "unknown failure: some detail")
	}
}

func TestFailureReasonEmpty(t *testing.T) {
	got := failureReason(ecstypes.Failure{})
	if got != "unknown failure" {
		t.Fatalf("got %q, want %q", got, "unknown failure")
	}
}

// ---------------------------------------------------------------------------
// Tests: markRowsSkipped
// ---------------------------------------------------------------------------

func TestMarkRowsSkippedFromMiddle(t *testing.T) {
	rows := [][]string{
		{"login", "cmd", "completed"},
		{"build", "cmd", "pending"},
		{"tag", "cmd", "pending"},
		{"push", "cmd", "pending"},
	}

	markRowsSkipped(rows, 1)

	if rows[0][2] != "completed" {
		t.Fatalf("row 0 should remain completed, got %q", rows[0][2])
	}
	for i := 1; i < len(rows); i++ {
		if !strings.Contains(rows[i][2], "skipped:") {
			t.Fatalf("row %d should be skipped, got %q", i, rows[i][2])
		}
	}
}

func TestMarkRowsSkippedFromEnd(t *testing.T) {
	rows := [][]string{
		{"login", "cmd", "completed"},
		{"build", "cmd", "completed"},
		{"tag", "cmd", "completed"},
		{"push", "cmd", "pending"},
	}

	markRowsSkipped(rows, 3)

	if !strings.Contains(rows[3][2], "skipped:") {
		t.Fatalf("row 3 should be skipped, got %q", rows[3][2])
	}
}

func TestMarkRowsSkippedStartBeyondLength(t *testing.T) {
	rows := [][]string{
		{"login", "cmd", "completed"},
	}

	// Should not panic when start >= len(rows)
	markRowsSkipped(rows, 5)

	if rows[0][2] != "completed" {
		t.Fatalf("row 0 should remain completed, got %q", rows[0][2])
	}
}

func TestMarkRowsSkippedSkipsNonPending(t *testing.T) {
	rows := [][]string{
		{"login", "cmd", "completed"},
		{"build", "cmd", "failed:error"},
		{"tag", "cmd", "pending"},
		{"push", "cmd", "pending"},
	}

	markRowsSkipped(rows, 1)

	// row 1 is "failed:error", not "pending", so should not be changed
	if rows[1][2] != "failed:error" {
		t.Fatalf("row 1 should remain failed, got %q", rows[1][2])
	}
	if !strings.Contains(rows[2][2], "skipped:") {
		t.Fatalf("row 2 should be skipped, got %q", rows[2][2])
	}
	if !strings.Contains(rows[3][2], "skipped:") {
		t.Fatalf("row 3 should be skipped, got %q", rows[3][2])
	}
}
