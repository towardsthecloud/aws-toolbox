package appstream

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/appstream"
	appstreamtypes "github.com/aws/aws-sdk-go-v2/service/appstream/types"
	"github.com/towardsthecloud/aws-toolbox/internal/cliutil"
)

type mockClient struct {
	describeImagePermissionsFn func(context.Context, *appstream.DescribeImagePermissionsInput, ...func(*appstream.Options)) (*appstream.DescribeImagePermissionsOutput, error)
	deleteImagePermissionsFn   func(context.Context, *appstream.DeleteImagePermissionsInput, ...func(*appstream.Options)) (*appstream.DeleteImagePermissionsOutput, error)
	deleteImageFn              func(context.Context, *appstream.DeleteImageInput, ...func(*appstream.Options)) (*appstream.DeleteImageOutput, error)
}

func (m *mockClient) DescribeImagePermissions(ctx context.Context, in *appstream.DescribeImagePermissionsInput, optFns ...func(*appstream.Options)) (*appstream.DescribeImagePermissionsOutput, error) {
	if m.describeImagePermissionsFn == nil {
		return nil, errors.New("DescribeImagePermissions not mocked")
	}
	return m.describeImagePermissionsFn(ctx, in, optFns...)
}

func (m *mockClient) DeleteImagePermissions(ctx context.Context, in *appstream.DeleteImagePermissionsInput, optFns ...func(*appstream.Options)) (*appstream.DeleteImagePermissionsOutput, error) {
	if m.deleteImagePermissionsFn == nil {
		return nil, errors.New("DeleteImagePermissions not mocked")
	}
	return m.deleteImagePermissionsFn(ctx, in, optFns...)
}

func (m *mockClient) DeleteImage(ctx context.Context, in *appstream.DeleteImageInput, optFns ...func(*appstream.Options)) (*appstream.DeleteImageOutput, error) {
	if m.deleteImageFn == nil {
		return nil, errors.New("DeleteImage not mocked")
	}
	return m.deleteImageFn(ctx, in, optFns...)
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

func TestDeleteImageRequiresName(t *testing.T) {
	_, err := executeCommand(t, "appstream", "delete-image")
	if err == nil || !strings.Contains(err.Error(), "--image-name is required") {
		t.Fatalf("expected required name error, got %v", err)
	}
}

func TestDeleteImageDryRun(t *testing.T) {
	deletedPermissions := 0
	deletedImages := 0

	client := &mockClient{
		describeImagePermissionsFn: func(_ context.Context, _ *appstream.DescribeImagePermissionsInput, _ ...func(*appstream.Options)) (*appstream.DescribeImagePermissionsOutput, error) {
			return &appstream.DescribeImagePermissionsOutput{SharedImagePermissionsList: []appstreamtypes.SharedImagePermissions{
				{SharedAccountId: cliutil.Ptr("111111111111")},
				{SharedAccountId: cliutil.Ptr("222222222222")},
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

	withMockDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) API { return client },
	)

	output, err := executeCommand(t, "--output", "json", "--dry-run", "appstream", "delete-image", "--image-name", "image-a")
	if err != nil {
		t.Fatalf("execute appstream delete-image dry-run: %v", err)
	}
	if deletedPermissions != 0 || deletedImages != 0 {
		t.Fatalf("expected no deletion calls in dry-run, got permissions=%d image=%d", deletedPermissions, deletedImages)
	}
	if !strings.Contains(output, "would-delete") {
		t.Fatalf("unexpected output: %s", output)
	}
}

func TestDeleteImageNoConfirmExecutes(t *testing.T) {
	deletedPermissions := 0
	deletedImages := 0

	client := &mockClient{
		describeImagePermissionsFn: func(_ context.Context, _ *appstream.DescribeImagePermissionsInput, _ ...func(*appstream.Options)) (*appstream.DescribeImagePermissionsOutput, error) {
			return &appstream.DescribeImagePermissionsOutput{SharedImagePermissionsList: []appstreamtypes.SharedImagePermissions{{SharedAccountId: cliutil.Ptr("111111111111")}}}, nil
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

	withMockDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) API { return client },
	)

	output, err := executeCommand(t, "--output", "json", "--no-confirm", "appstream", "delete-image", "--image-name", "image-a")
	if err != nil {
		t.Fatalf("execute appstream delete-image --no-confirm: %v", err)
	}
	if deletedPermissions != 1 || deletedImages != 1 {
		t.Fatalf("expected delete calls, got permissions=%d image=%d", deletedPermissions, deletedImages)
	}
	if !strings.Contains(output, "deleted") {
		t.Fatalf("unexpected output: %s", output)
	}
}

func TestDeleteImagePermissionFailureSkipsImage(t *testing.T) {
	client := &mockClient{
		describeImagePermissionsFn: func(_ context.Context, _ *appstream.DescribeImagePermissionsInput, _ ...func(*appstream.Options)) (*appstream.DescribeImagePermissionsOutput, error) {
			return &appstream.DescribeImagePermissionsOutput{SharedImagePermissionsList: []appstreamtypes.SharedImagePermissions{
				{SharedAccountId: cliutil.Ptr("111111111111")},
			}}, nil
		},
		deleteImagePermissionsFn: func(_ context.Context, _ *appstream.DeleteImagePermissionsInput, _ ...func(*appstream.Options)) (*appstream.DeleteImagePermissionsOutput, error) {
			return nil, errors.New("permission denied")
		},
		deleteImageFn: func(_ context.Context, _ *appstream.DeleteImageInput, _ ...func(*appstream.Options)) (*appstream.DeleteImageOutput, error) {
			t.Fatal("should not call DeleteImage when permissions cleanup fails")
			return nil, nil
		},
	}

	withMockDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) API { return client },
	)

	output, err := executeCommand(t, "--output", "json", "--no-confirm", "appstream", "delete-image", "--image-name", "image-a")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, "failed:") {
		t.Fatalf("expected failed action in output: %s", output)
	}
	if !strings.Contains(output, "skipped:") {
		t.Fatalf("expected skipped action in output: %s", output)
	}
}

func TestDeleteImageDeleteError(t *testing.T) {
	client := &mockClient{
		describeImagePermissionsFn: func(_ context.Context, _ *appstream.DescribeImagePermissionsInput, _ ...func(*appstream.Options)) (*appstream.DescribeImagePermissionsOutput, error) {
			return &appstream.DescribeImagePermissionsOutput{}, nil
		},
		deleteImageFn: func(_ context.Context, _ *appstream.DeleteImageInput, _ ...func(*appstream.Options)) (*appstream.DeleteImageOutput, error) {
			return nil, errors.New("image in use")
		},
	}

	withMockDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) API { return client },
	)

	output, err := executeCommand(t, "--output", "json", "--no-confirm", "appstream", "delete-image", "--image-name", "image-a")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, "failed:") {
		t.Fatalf("expected failed action in output: %s", output)
	}
}

func TestDeleteImageNoSharedPermissions(t *testing.T) {
	deletedImages := 0
	client := &mockClient{
		describeImagePermissionsFn: func(_ context.Context, _ *appstream.DescribeImagePermissionsInput, _ ...func(*appstream.Options)) (*appstream.DescribeImagePermissionsOutput, error) {
			return &appstream.DescribeImagePermissionsOutput{}, nil
		},
		deleteImageFn: func(_ context.Context, _ *appstream.DeleteImageInput, _ ...func(*appstream.Options)) (*appstream.DeleteImageOutput, error) {
			deletedImages++
			return &appstream.DeleteImageOutput{}, nil
		},
	}

	withMockDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) API { return client },
	)

	output, err := executeCommand(t, "--output", "json", "--no-confirm", "appstream", "delete-image", "--image-name", "image-a")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if deletedImages != 1 {
		t.Fatalf("expected image deleted, got %d", deletedImages)
	}
	if !strings.Contains(output, "deleted") {
		t.Fatalf("expected deleted in output: %s", output)
	}
}

func TestDeleteImageListPermissionsError(t *testing.T) {
	client := &mockClient{
		describeImagePermissionsFn: func(_ context.Context, _ *appstream.DescribeImagePermissionsInput, _ ...func(*appstream.Options)) (*appstream.DescribeImagePermissionsOutput, error) {
			return nil, errors.New("access denied")
		},
	}

	withMockDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) API { return client },
	)

	_, err := executeCommand(t, "--no-confirm", "appstream", "delete-image", "--image-name", "image-a")
	if err == nil || !strings.Contains(err.Error(), "list image permissions") {
		t.Fatalf("expected list permissions error, got %v", err)
	}
}
