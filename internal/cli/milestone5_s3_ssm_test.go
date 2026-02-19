package cli

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
)

type mockS3Client struct {
	deleteBucketFn        func(context.Context, *s3.DeleteBucketInput, ...func(*s3.Options)) (*s3.DeleteBucketOutput, error)
	deleteObjectsFn       func(context.Context, *s3.DeleteObjectsInput, ...func(*s3.Options)) (*s3.DeleteObjectsOutput, error)
	getBucketVersioningFn func(context.Context, *s3.GetBucketVersioningInput, ...func(*s3.Options)) (*s3.GetBucketVersioningOutput, error)
	getObjectFn           func(context.Context, *s3.GetObjectInput, ...func(*s3.Options)) (*s3.GetObjectOutput, error)
	listBucketsFn         func(context.Context, *s3.ListBucketsInput, ...func(*s3.Options)) (*s3.ListBucketsOutput, error)
	listObjectVersionsFn  func(context.Context, *s3.ListObjectVersionsInput, ...func(*s3.Options)) (*s3.ListObjectVersionsOutput, error)
	listObjectsV2Fn       func(context.Context, *s3.ListObjectsV2Input, ...func(*s3.Options)) (*s3.ListObjectsV2Output, error)
}

func (m *mockS3Client) DeleteBucket(ctx context.Context, in *s3.DeleteBucketInput, optFns ...func(*s3.Options)) (*s3.DeleteBucketOutput, error) {
	if m.deleteBucketFn == nil {
		return nil, errors.New("DeleteBucket not mocked")
	}
	return m.deleteBucketFn(ctx, in, optFns...)
}

func (m *mockS3Client) DeleteObjects(ctx context.Context, in *s3.DeleteObjectsInput, optFns ...func(*s3.Options)) (*s3.DeleteObjectsOutput, error) {
	if m.deleteObjectsFn == nil {
		return nil, errors.New("DeleteObjects not mocked")
	}
	return m.deleteObjectsFn(ctx, in, optFns...)
}

func (m *mockS3Client) GetBucketVersioning(ctx context.Context, in *s3.GetBucketVersioningInput, optFns ...func(*s3.Options)) (*s3.GetBucketVersioningOutput, error) {
	if m.getBucketVersioningFn == nil {
		return nil, errors.New("GetBucketVersioning not mocked")
	}
	return m.getBucketVersioningFn(ctx, in, optFns...)
}

func (m *mockS3Client) GetObject(ctx context.Context, in *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	if m.getObjectFn == nil {
		return nil, errors.New("GetObject not mocked")
	}
	return m.getObjectFn(ctx, in, optFns...)
}

func (m *mockS3Client) ListBuckets(ctx context.Context, in *s3.ListBucketsInput, optFns ...func(*s3.Options)) (*s3.ListBucketsOutput, error) {
	if m.listBucketsFn == nil {
		return nil, errors.New("ListBuckets not mocked")
	}
	return m.listBucketsFn(ctx, in, optFns...)
}

func (m *mockS3Client) ListObjectVersions(ctx context.Context, in *s3.ListObjectVersionsInput, optFns ...func(*s3.Options)) (*s3.ListObjectVersionsOutput, error) {
	if m.listObjectVersionsFn == nil {
		return nil, errors.New("ListObjectVersions not mocked")
	}
	return m.listObjectVersionsFn(ctx, in, optFns...)
}

func (m *mockS3Client) ListObjectsV2(ctx context.Context, in *s3.ListObjectsV2Input, optFns ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
	if m.listObjectsV2Fn == nil {
		return nil, errors.New("ListObjectsV2 not mocked")
	}
	return m.listObjectsV2Fn(ctx, in, optFns...)
}

type mockSSMClient struct {
	deleteParameterFn func(context.Context, *ssm.DeleteParameterInput, ...func(*ssm.Options)) (*ssm.DeleteParameterOutput, error)
	putParameterFn    func(context.Context, *ssm.PutParameterInput, ...func(*ssm.Options)) (*ssm.PutParameterOutput, error)
}

func (m *mockSSMClient) DeleteParameter(ctx context.Context, in *ssm.DeleteParameterInput, optFns ...func(*ssm.Options)) (*ssm.DeleteParameterOutput, error) {
	if m.deleteParameterFn == nil {
		return nil, errors.New("DeleteParameter not mocked")
	}
	return m.deleteParameterFn(ctx, in, optFns...)
}

func (m *mockSSMClient) PutParameter(ctx context.Context, in *ssm.PutParameterInput, optFns ...func(*ssm.Options)) (*ssm.PutParameterOutput, error) {
	if m.putParameterFn == nil {
		return nil, errors.New("PutParameter not mocked")
	}
	return m.putParameterFn(ctx, in, optFns...)
}

func withMockS3Deps(t *testing.T, loader func(string, string) (awssdk.Config, error), factory func(awssdk.Config) s3API) {
	t.Helper()

	oldLoader := s3LoadAWSConfig
	oldFactory := s3NewClient

	s3LoadAWSConfig = loader
	s3NewClient = factory

	t.Cleanup(func() {
		s3LoadAWSConfig = oldLoader
		s3NewClient = oldFactory
	})
}

func withMockSSMDeps(t *testing.T, loader func(string, string) (awssdk.Config, error), factory func(awssdk.Config) ssmAPI) {
	t.Helper()

	oldLoader := ssmLoadAWSConfig
	oldFactory := ssmNewClient

	ssmLoadAWSConfig = loader
	ssmNewClient = factory

	t.Cleanup(func() {
		ssmLoadAWSConfig = oldLoader
		ssmNewClient = oldFactory
	})
}

func TestMilestone5S3SearchObjectsSupportsMultipleKeys(t *testing.T) {
	now := time.Now().UTC()
	client := &mockS3Client{
		listObjectsV2Fn: func(_ context.Context, in *s3.ListObjectsV2Input, _ ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
			if pointerToString(in.Bucket) != "my-bucket" {
				t.Fatalf("unexpected bucket: %s", pointerToString(in.Bucket))
			}
			return &s3.ListObjectsV2Output{
				Contents: []s3types.Object{
					{Key: ptr("foo"), LastModified: &now, Size: ptr(int64(123))},
					{Key: ptr("baz"), LastModified: &now, Size: ptr(int64(456))},
				},
			}, nil
		},
	}

	withMockS3Deps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) s3API { return client },
	)

	output, err := executeCommand(t, "--output", "json", "s3", "search-objects", "--bucket", "my-bucket", "--keys", "foo,bar")
	if err != nil {
		t.Fatalf("execute s3 search-objects: %v", err)
	}

	for _, expected := range []string{`"query_key": "foo"`, `"exists": "true"`, `"query_key": "bar"`, `"exists": "false"`} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected output to contain %q\n%s", expected, output)
		}
	}
}

func TestMilestone5SSMImportParametersReadsFromInputFile(t *testing.T) {
	inputPath := filepath.Join(t.TempDir(), "params.json")
	content := `[
  {"Name":"/service/foo","Type":"String","Value":"one"},
  {"Name":"/service/bar","Type":"SecureString","Value":"two","Overwrite":true}
]`
	if err := os.WriteFile(inputPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write params file: %v", err)
	}

	putCalls := make([]string, 0)
	client := &mockSSMClient{
		putParameterFn: func(_ context.Context, in *ssm.PutParameterInput, _ ...func(*ssm.Options)) (*ssm.PutParameterOutput, error) {
			putCalls = append(putCalls, pointerToString(in.Name))
			return &ssm.PutParameterOutput{Version: int64(1)}, nil
		},
	}

	withMockSSMDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) ssmAPI { return client },
	)

	output, err := executeCommand(t, "--output", "json", "ssm", "import-parameters", "--input-file", inputPath)
	if err != nil {
		t.Fatalf("execute ssm import-parameters: %v", err)
	}

	if len(putCalls) != 2 {
		t.Fatalf("expected 2 put calls, got %d", len(putCalls))
	}
	for _, expected := range []string{"/service/foo", "/service/bar", `"action": "imported"`} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected output to contain %q\n%s", expected, output)
		}
	}
}

func TestMilestone5S3SearchObjectsRequiresBucket(t *testing.T) {
	output, err := executeCommand(t, "s3", "search-objects", "--keys", "foo")
	if err == nil {
		t.Fatalf("expected error, got nil and output=%s", output)
	}
	if !strings.Contains(err.Error(), "--bucket is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMilestone5SSMImportParametersRequiresInputFile(t *testing.T) {
	output, err := executeCommand(t, "ssm", "import-parameters")
	if err == nil {
		t.Fatalf("expected error, got nil and output=%s", output)
	}
	if !strings.Contains(err.Error(), "--input-file is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMilestone5S3DeleteBucketsDryRunDoesNotDelete(t *testing.T) {
	deleteCalls := 0
	client := &mockS3Client{
		listBucketsFn: func(_ context.Context, _ *s3.ListBucketsInput, _ ...func(*s3.Options)) (*s3.ListBucketsOutput, error) {
			return &s3.ListBucketsOutput{
				Buckets: []s3types.Bucket{{Name: ptr("my-empty-bucket")}},
			}, nil
		},
		listObjectsV2Fn: func(_ context.Context, in *s3.ListObjectsV2Input, _ ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
			if in.MaxKeys == nil || *in.MaxKeys != 1 {
				t.Fatalf("expected MaxKeys=1 for emptiness check, got %v", in.MaxKeys)
			}
			return &s3.ListObjectsV2Output{}, nil
		},
		getBucketVersioningFn: func(_ context.Context, _ *s3.GetBucketVersioningInput, _ ...func(*s3.Options)) (*s3.GetBucketVersioningOutput, error) {
			return &s3.GetBucketVersioningOutput{}, nil
		},
		deleteBucketFn: func(_ context.Context, _ *s3.DeleteBucketInput, _ ...func(*s3.Options)) (*s3.DeleteBucketOutput, error) {
			deleteCalls++
			return &s3.DeleteBucketOutput{}, nil
		},
	}

	withMockS3Deps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) s3API { return client },
	)

	output, err := executeCommand(t, "--output", "json", "--dry-run", "s3", "delete-buckets", "--empty")
	if err != nil {
		t.Fatalf("execute s3 delete-buckets --dry-run: %v", err)
	}

	if deleteCalls != 0 {
		t.Fatalf("expected no delete calls in dry-run, got %d", deleteCalls)
	}
	if !strings.Contains(output, "would-delete") {
		t.Fatalf("expected dry-run action in output: %s", output)
	}
}

func TestMilestone5SSMImportParametersParsesTypeCaseInsensitive(t *testing.T) {
	inputPath := filepath.Join(t.TempDir(), "params-lower.json")
	content := `[
  {"name":"/service/sample","type":"stringlist","value":"one,two,three"}
]`
	if err := os.WriteFile(inputPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write params file: %v", err)
	}

	client := &mockSSMClient{
		putParameterFn: func(_ context.Context, in *ssm.PutParameterInput, _ ...func(*ssm.Options)) (*ssm.PutParameterOutput, error) {
			if in.Type != ssmtypes.ParameterTypeStringList {
				t.Fatalf("unexpected type: %s", in.Type)
			}
			return &ssm.PutParameterOutput{Version: int64(1)}, nil
		},
	}

	withMockSSMDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) ssmAPI { return client },
	)

	if _, err := executeCommand(t, "--output", "json", "ssm", "import-parameters", "--input-file", inputPath); err != nil {
		t.Fatalf("execute ssm import-parameters: %v", err)
	}
}
