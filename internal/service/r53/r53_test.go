package r53

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	route53types "github.com/aws/aws-sdk-go-v2/service/route53/types"
	"github.com/towardsthecloud/aws-toolbox/internal/cliutil"
)

type mockClient struct {
	createHealthCheckFn     func(context.Context, *route53.CreateHealthCheckInput, ...func(*route53.Options)) (*route53.CreateHealthCheckOutput, error)
	changeTagsForResourceFn func(context.Context, *route53.ChangeTagsForResourceInput, ...func(*route53.Options)) (*route53.ChangeTagsForResourceOutput, error)
}

func (m *mockClient) CreateHealthCheck(ctx context.Context, in *route53.CreateHealthCheckInput, optFns ...func(*route53.Options)) (*route53.CreateHealthCheckOutput, error) {
	if m.createHealthCheckFn == nil {
		return nil, errors.New("CreateHealthCheck not mocked")
	}
	return m.createHealthCheckFn(ctx, in, optFns...)
}

func (m *mockClient) ChangeTagsForResource(ctx context.Context, in *route53.ChangeTagsForResourceInput, optFns ...func(*route53.Options)) (*route53.ChangeTagsForResourceOutput, error) {
	if m.changeTagsForResourceFn == nil {
		return nil, errors.New("ChangeTagsForResource not mocked")
	}
	return m.changeTagsForResourceFn(ctx, in, optFns...)
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

func TestCreateHealthChecksRequiresDomains(t *testing.T) {
	_, err := executeCommand(t, "r53", "create-health-checks")
	if err == nil || !strings.Contains(err.Error(), "--domains is required") {
		t.Fatalf("expected required domains error, got %v", err)
	}
}

func TestCreateHealthChecksDryRun(t *testing.T) {
	createCalls := 0
	tagCalls := 0

	client := &mockClient{
		createHealthCheckFn: func(_ context.Context, _ *route53.CreateHealthCheckInput, _ ...func(*route53.Options)) (*route53.CreateHealthCheckOutput, error) {
			createCalls++
			return &route53.CreateHealthCheckOutput{}, nil
		},
		changeTagsForResourceFn: func(_ context.Context, _ *route53.ChangeTagsForResourceInput, _ ...func(*route53.Options)) (*route53.ChangeTagsForResourceOutput, error) {
			tagCalls++
			return &route53.ChangeTagsForResourceOutput{}, nil
		},
	}

	withMockDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) API { return client },
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

func TestCreateHealthChecksNoConfirmExecutes(t *testing.T) {
	createCalls := 0
	tagCalls := 0

	client := &mockClient{
		createHealthCheckFn: func(_ context.Context, in *route53.CreateHealthCheckInput, _ ...func(*route53.Options)) (*route53.CreateHealthCheckOutput, error) {
			createCalls++
			domain := cliutil.PointerToString(in.HealthCheckConfig.FullyQualifiedDomainName)
			return &route53.CreateHealthCheckOutput{HealthCheck: &route53types.HealthCheck{Id: cliutil.Ptr("hc-" + strings.ReplaceAll(domain, ".", "-"))}}, nil
		},
		changeTagsForResourceFn: func(_ context.Context, _ *route53.ChangeTagsForResourceInput, _ ...func(*route53.Options)) (*route53.ChangeTagsForResourceOutput, error) {
			tagCalls++
			return &route53.ChangeTagsForResourceOutput{}, nil
		},
	}

	withMockDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) API { return client },
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
