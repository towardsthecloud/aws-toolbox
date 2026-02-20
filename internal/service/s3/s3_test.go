package s3

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/towardsthecloud/aws-toolbox/internal/cliutil"
)

type mockClient struct {
	deleteBucketFn        func(context.Context, *s3.DeleteBucketInput, ...func(*s3.Options)) (*s3.DeleteBucketOutput, error)
	deleteObjectsFn       func(context.Context, *s3.DeleteObjectsInput, ...func(*s3.Options)) (*s3.DeleteObjectsOutput, error)
	getBucketVersioningFn func(context.Context, *s3.GetBucketVersioningInput, ...func(*s3.Options)) (*s3.GetBucketVersioningOutput, error)
	getObjectFn           func(context.Context, *s3.GetObjectInput, ...func(*s3.Options)) (*s3.GetObjectOutput, error)
	listBucketsFn         func(context.Context, *s3.ListBucketsInput, ...func(*s3.Options)) (*s3.ListBucketsOutput, error)
	listObjectVersionsFn  func(context.Context, *s3.ListObjectVersionsInput, ...func(*s3.Options)) (*s3.ListObjectVersionsOutput, error)
	listObjectsV2Fn       func(context.Context, *s3.ListObjectsV2Input, ...func(*s3.Options)) (*s3.ListObjectsV2Output, error)
}

func (m *mockClient) DeleteBucket(ctx context.Context, in *s3.DeleteBucketInput, optFns ...func(*s3.Options)) (*s3.DeleteBucketOutput, error) {
	if m.deleteBucketFn == nil {
		return nil, errors.New("DeleteBucket not mocked")
	}
	return m.deleteBucketFn(ctx, in, optFns...)
}

func (m *mockClient) DeleteObjects(ctx context.Context, in *s3.DeleteObjectsInput, optFns ...func(*s3.Options)) (*s3.DeleteObjectsOutput, error) {
	if m.deleteObjectsFn == nil {
		return nil, errors.New("DeleteObjects not mocked")
	}
	return m.deleteObjectsFn(ctx, in, optFns...)
}

func (m *mockClient) GetBucketVersioning(ctx context.Context, in *s3.GetBucketVersioningInput, optFns ...func(*s3.Options)) (*s3.GetBucketVersioningOutput, error) {
	if m.getBucketVersioningFn == nil {
		return nil, errors.New("GetBucketVersioning not mocked")
	}
	return m.getBucketVersioningFn(ctx, in, optFns...)
}

func (m *mockClient) GetObject(ctx context.Context, in *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	if m.getObjectFn == nil {
		return nil, errors.New("GetObject not mocked")
	}
	return m.getObjectFn(ctx, in, optFns...)
}

func (m *mockClient) ListBuckets(ctx context.Context, in *s3.ListBucketsInput, optFns ...func(*s3.Options)) (*s3.ListBucketsOutput, error) {
	if m.listBucketsFn == nil {
		return nil, errors.New("ListBuckets not mocked")
	}
	return m.listBucketsFn(ctx, in, optFns...)
}

func (m *mockClient) ListObjectVersions(ctx context.Context, in *s3.ListObjectVersionsInput, optFns ...func(*s3.Options)) (*s3.ListObjectVersionsOutput, error) {
	if m.listObjectVersionsFn == nil {
		return nil, errors.New("ListObjectVersions not mocked")
	}
	return m.listObjectVersionsFn(ctx, in, optFns...)
}

func (m *mockClient) ListObjectsV2(ctx context.Context, in *s3.ListObjectsV2Input, optFns ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
	if m.listObjectsV2Fn == nil {
		return nil, errors.New("ListObjectsV2 not mocked")
	}
	return m.listObjectsV2Fn(ctx, in, optFns...)
}

func withMockDeps(t *testing.T, loader func(string, string) (awssdk.Config, error), factory func(awssdk.Config) API) {
	t.Helper()

	oldLoader := loadAWSConfig
	oldFactory := newClient

	loadAWSConfig = loader
	newClient = factory

	t.Cleanup(func() {
		loadAWSConfig = oldLoader
		newClient = oldFactory
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

func TestSearchObjectsSupportsMultipleKeys(t *testing.T) {
	now := time.Now().UTC()
	client := &mockClient{
		listObjectsV2Fn: func(_ context.Context, in *s3.ListObjectsV2Input, _ ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
			if cliutil.PointerToString(in.Bucket) != "my-bucket" {
				t.Fatalf("unexpected bucket: %s", cliutil.PointerToString(in.Bucket))
			}
			return &s3.ListObjectsV2Output{
				Contents: []s3types.Object{
					{Key: cliutil.Ptr("foo"), LastModified: &now, Size: cliutil.Ptr(int64(123))},
					{Key: cliutil.Ptr("baz"), LastModified: &now, Size: cliutil.Ptr(int64(456))},
				},
			}, nil
		},
	}

	withMockDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) API { return client },
	)

	output, err := executeCommand(t, "--output", "json", "s3", "search-objects", "--bucket-name", "my-bucket", "--keys", "foo,bar")
	if err != nil {
		t.Fatalf("execute s3 search-objects: %v", err)
	}

	for _, expected := range []string{`"query_key": "foo"`, `"exists": "true"`, `"query_key": "bar"`, `"exists": "false"`} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected output to contain %q\n%s", expected, output)
		}
	}
}

func TestSearchObjectsRequiresBucket(t *testing.T) {
	output, err := executeCommand(t, "s3", "search-objects", "--keys", "foo")
	if err == nil {
		t.Fatalf("expected error, got nil and output=%s", output)
	}
	if !strings.Contains(err.Error(), "--bucket-name is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeleteBucketsDryRunDoesNotDelete(t *testing.T) {
	deleteCalls := 0
	client := &mockClient{
		listBucketsFn: func(_ context.Context, _ *s3.ListBucketsInput, _ ...func(*s3.Options)) (*s3.ListBucketsOutput, error) {
			return &s3.ListBucketsOutput{
				Buckets: []s3types.Bucket{{Name: cliutil.Ptr("my-empty-bucket")}},
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

	withMockDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) API { return client },
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

// --- helpers ---

func mockLoader(_, _ string) (awssdk.Config, error) {
	return awssdk.Config{Region: "us-east-1"}, nil
}

func mockFactory(client API) func(awssdk.Config) API {
	return func(awssdk.Config) API { return client }
}

// nopReadCloser wraps a bytes.Reader as an io.ReadCloser.
type nopReadCloser struct{ io.Reader }

func (nopReadCloser) Close() error { return nil }

// ============================================================
// runDeleteBuckets tests
// ============================================================

func TestDeleteBucketsRequiresFilterOrEmpty(t *testing.T) {
	_, err := executeCommand(t, "s3", "delete-buckets")
	if err == nil {
		t.Fatal("expected error when neither --empty nor --filter-name-contains is set")
	}
	if !strings.Contains(err.Error(), "set --empty or --filter-name-contains") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeleteBucketsNoConfirmActuallyDeletes(t *testing.T) {
	deletedBuckets := make([]string, 0)
	client := &mockClient{
		listBucketsFn: func(_ context.Context, _ *s3.ListBucketsInput, _ ...func(*s3.Options)) (*s3.ListBucketsOutput, error) {
			return &s3.ListBucketsOutput{
				Buckets: []s3types.Bucket{
					{Name: cliutil.Ptr("test-bucket-alpha")},
					{Name: cliutil.Ptr("other-bucket")},
				},
			}, nil
		},
		listObjectsV2Fn: func(_ context.Context, _ *s3.ListObjectsV2Input, _ ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
			return &s3.ListObjectsV2Output{}, nil
		},
		listObjectVersionsFn: func(_ context.Context, _ *s3.ListObjectVersionsInput, _ ...func(*s3.Options)) (*s3.ListObjectVersionsOutput, error) {
			return &s3.ListObjectVersionsOutput{}, nil
		},
		deleteObjectsFn: func(_ context.Context, _ *s3.DeleteObjectsInput, _ ...func(*s3.Options)) (*s3.DeleteObjectsOutput, error) {
			return &s3.DeleteObjectsOutput{}, nil
		},
		deleteBucketFn: func(_ context.Context, in *s3.DeleteBucketInput, _ ...func(*s3.Options)) (*s3.DeleteBucketOutput, error) {
			deletedBuckets = append(deletedBuckets, cliutil.PointerToString(in.Bucket))
			return &s3.DeleteBucketOutput{}, nil
		},
	}

	withMockDeps(t, mockLoader, mockFactory(client))

	output, err := executeCommand(t, "--output", "json", "--no-confirm", "s3", "delete-buckets", "--filter-name-contains", "test-bucket")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}

	if len(deletedBuckets) != 1 || deletedBuckets[0] != "test-bucket-alpha" {
		t.Fatalf("expected exactly test-bucket-alpha deleted, got %v", deletedBuckets)
	}
	if !strings.Contains(output, "deleted") {
		t.Fatalf("expected 'deleted' in output: %s", output)
	}
}

func TestDeleteBucketsFilterNameContainsMatchesNoBuckets(t *testing.T) {
	client := &mockClient{
		listBucketsFn: func(_ context.Context, _ *s3.ListBucketsInput, _ ...func(*s3.Options)) (*s3.ListBucketsOutput, error) {
			return &s3.ListBucketsOutput{
				Buckets: []s3types.Bucket{{Name: cliutil.Ptr("alpha")}},
			}, nil
		},
	}

	withMockDeps(t, mockLoader, mockFactory(client))

	output, err := executeCommand(t, "--output", "json", "--dry-run", "s3", "delete-buckets", "--filter-name-contains", "no-match")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	// No rows should be emitted.
	if strings.Contains(output, "no-match") {
		t.Fatalf("expected no matching rows: %s", output)
	}
}

func TestDeleteBucketsListError(t *testing.T) {
	client := &mockClient{
		listBucketsFn: func(_ context.Context, _ *s3.ListBucketsInput, _ ...func(*s3.Options)) (*s3.ListBucketsOutput, error) {
			return nil, errors.New("access denied")
		},
	}

	withMockDeps(t, mockLoader, mockFactory(client))

	_, err := executeCommand(t, "--output", "json", "s3", "delete-buckets", "--filter-name-contains", "x")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "list buckets") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeleteBucketsEmptyCheckVersioningEnabled(t *testing.T) {
	client := &mockClient{
		listBucketsFn: func(_ context.Context, _ *s3.ListBucketsInput, _ ...func(*s3.Options)) (*s3.ListBucketsOutput, error) {
			return &s3.ListBucketsOutput{
				Buckets: []s3types.Bucket{{Name: cliutil.Ptr("versioned-bucket")}},
			}, nil
		},
		listObjectsV2Fn: func(_ context.Context, _ *s3.ListObjectsV2Input, _ ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
			return &s3.ListObjectsV2Output{}, nil // empty
		},
		getBucketVersioningFn: func(_ context.Context, _ *s3.GetBucketVersioningInput, _ ...func(*s3.Options)) (*s3.GetBucketVersioningOutput, error) {
			return &s3.GetBucketVersioningOutput{
				Status: s3types.BucketVersioningStatusEnabled,
			}, nil
		},
	}

	withMockDeps(t, mockLoader, mockFactory(client))

	output, err := executeCommand(t, "--output", "json", "--dry-run", "s3", "delete-buckets", "--empty")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	// Bucket has versioning enabled, so it should not appear in targets.
	if strings.Contains(output, "versioned-bucket") {
		t.Fatalf("versioned bucket should not appear in targets: %s", output)
	}
}

func TestDeleteBucketsEmptyCheckNotEmpty(t *testing.T) {
	client := &mockClient{
		listBucketsFn: func(_ context.Context, _ *s3.ListBucketsInput, _ ...func(*s3.Options)) (*s3.ListBucketsOutput, error) {
			return &s3.ListBucketsOutput{
				Buckets: []s3types.Bucket{{Name: cliutil.Ptr("non-empty")}},
			}, nil
		},
		listObjectsV2Fn: func(_ context.Context, _ *s3.ListObjectsV2Input, _ ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
			return &s3.ListObjectsV2Output{
				Contents: []s3types.Object{{Key: cliutil.Ptr("some-object")}},
			}, nil
		},
	}

	withMockDeps(t, mockLoader, mockFactory(client))

	output, err := executeCommand(t, "--output", "json", "--dry-run", "s3", "delete-buckets", "--empty")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if strings.Contains(output, "non-empty") {
		t.Fatalf("non-empty bucket should not appear: %s", output)
	}
}

func TestDeleteBucketsEmptyCheckListObjectsError(t *testing.T) {
	client := &mockClient{
		listBucketsFn: func(_ context.Context, _ *s3.ListBucketsInput, _ ...func(*s3.Options)) (*s3.ListBucketsOutput, error) {
			return &s3.ListBucketsOutput{
				Buckets: []s3types.Bucket{{Name: cliutil.Ptr("my-bucket")}},
			}, nil
		},
		listObjectsV2Fn: func(_ context.Context, _ *s3.ListObjectsV2Input, _ ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
			return nil, errors.New("list objects failed")
		},
	}

	withMockDeps(t, mockLoader, mockFactory(client))

	_, err := executeCommand(t, "--output", "json", "s3", "delete-buckets", "--empty")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "list objects for bucket") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeleteBucketsEmptyCheckVersioningError(t *testing.T) {
	client := &mockClient{
		listBucketsFn: func(_ context.Context, _ *s3.ListBucketsInput, _ ...func(*s3.Options)) (*s3.ListBucketsOutput, error) {
			return &s3.ListBucketsOutput{
				Buckets: []s3types.Bucket{{Name: cliutil.Ptr("my-bucket")}},
			}, nil
		},
		listObjectsV2Fn: func(_ context.Context, _ *s3.ListObjectsV2Input, _ ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
			return &s3.ListObjectsV2Output{}, nil
		},
		getBucketVersioningFn: func(_ context.Context, _ *s3.GetBucketVersioningInput, _ ...func(*s3.Options)) (*s3.GetBucketVersioningOutput, error) {
			return nil, errors.New("versioning check failed")
		},
	}

	withMockDeps(t, mockLoader, mockFactory(client))

	_, err := executeCommand(t, "--output", "json", "s3", "delete-buckets", "--empty")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "get versioning for bucket") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeleteBucketsDeleteBucketError(t *testing.T) {
	client := &mockClient{
		listBucketsFn: func(_ context.Context, _ *s3.ListBucketsInput, _ ...func(*s3.Options)) (*s3.ListBucketsOutput, error) {
			return &s3.ListBucketsOutput{
				Buckets: []s3types.Bucket{{Name: cliutil.Ptr("fail-bucket")}},
			}, nil
		},
		listObjectsV2Fn: func(_ context.Context, _ *s3.ListObjectsV2Input, _ ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
			return &s3.ListObjectsV2Output{}, nil
		},
		listObjectVersionsFn: func(_ context.Context, _ *s3.ListObjectVersionsInput, _ ...func(*s3.Options)) (*s3.ListObjectVersionsOutput, error) {
			return &s3.ListObjectVersionsOutput{}, nil
		},
		deleteBucketFn: func(_ context.Context, _ *s3.DeleteBucketInput, _ ...func(*s3.Options)) (*s3.DeleteBucketOutput, error) {
			return nil, errors.New("delete failed")
		},
	}

	withMockDeps(t, mockLoader, mockFactory(client))

	output, err := executeCommand(t, "--output", "json", "--no-confirm", "s3", "delete-buckets", "--filter-name-contains", "fail")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(output, "failed:") {
		t.Fatalf("expected failed action: %s", output)
	}
}

func TestDeleteBucketsClearObjectsError(t *testing.T) {
	client := &mockClient{
		listBucketsFn: func(_ context.Context, _ *s3.ListBucketsInput, _ ...func(*s3.Options)) (*s3.ListBucketsOutput, error) {
			return &s3.ListBucketsOutput{
				Buckets: []s3types.Bucket{{Name: cliutil.Ptr("my-bucket")}},
			}, nil
		},
		listObjectsV2Fn: func(_ context.Context, in *s3.ListObjectsV2Input, _ ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
			// When called with MaxKeys=1 for emptiness check, we don't hit this.
			// For the delete-all-objects phase, fail.
			if in.MaxKeys != nil && *in.MaxKeys == 1 {
				return &s3.ListObjectsV2Output{}, nil
			}
			return nil, errors.New("list objects for delete failed")
		},
		getBucketVersioningFn: func(_ context.Context, _ *s3.GetBucketVersioningInput, _ ...func(*s3.Options)) (*s3.GetBucketVersioningOutput, error) {
			return &s3.GetBucketVersioningOutput{}, nil
		},
	}

	withMockDeps(t, mockLoader, mockFactory(client))

	output, err := executeCommand(t, "--output", "json", "--no-confirm", "s3", "delete-buckets", "--empty")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(output, "failed:") {
		t.Fatalf("expected failed action: %s", output)
	}
}

func TestDeleteBucketsSkipsNilNameBuckets(t *testing.T) {
	client := &mockClient{
		listBucketsFn: func(_ context.Context, _ *s3.ListBucketsInput, _ ...func(*s3.Options)) (*s3.ListBucketsOutput, error) {
			return &s3.ListBucketsOutput{
				Buckets: []s3types.Bucket{
					{Name: nil},
					{Name: cliutil.Ptr("good-bucket")},
				},
			}, nil
		},
	}

	withMockDeps(t, mockLoader, mockFactory(client))

	output, err := executeCommand(t, "--output", "json", "--dry-run", "s3", "delete-buckets", "--filter-name-contains", "good")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(output, "good-bucket") {
		t.Fatalf("expected good-bucket in output: %s", output)
	}
}

func TestDeleteBucketsAWSConfigError(t *testing.T) {
	withMockDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{}, errors.New("config failed") },
		func(awssdk.Config) API { return &mockClient{} },
	)

	_, err := executeCommand(t, "s3", "delete-buckets", "--empty")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "config failed") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ============================================================
// deleteAllObjectsFromBucket tests
// ============================================================

func TestDeleteAllObjectsFromBucketWithVersions(t *testing.T) {
	deletedKeys := make([]string, 0)
	client := &mockClient{
		listObjectsV2Fn: func(_ context.Context, _ *s3.ListObjectsV2Input, _ ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
			return &s3.ListObjectsV2Output{
				Contents: []s3types.Object{
					{Key: cliutil.Ptr("obj1")},
					{Key: cliutil.Ptr("obj2")},
					{Key: nil}, // nil key should be skipped
				},
			}, nil
		},
		listObjectVersionsFn: func(_ context.Context, _ *s3.ListObjectVersionsInput, _ ...func(*s3.Options)) (*s3.ListObjectVersionsOutput, error) {
			return &s3.ListObjectVersionsOutput{
				Versions: []s3types.ObjectVersion{
					{Key: cliutil.Ptr("obj1"), VersionId: cliutil.Ptr("v1")},
					{Key: nil}, // nil key should be skipped
				},
				DeleteMarkers: []s3types.DeleteMarkerEntry{
					{Key: cliutil.Ptr("obj2"), VersionId: cliutil.Ptr("dm1")},
					{Key: nil}, // nil key should be skipped
				},
			}, nil
		},
		deleteObjectsFn: func(_ context.Context, in *s3.DeleteObjectsInput, _ ...func(*s3.Options)) (*s3.DeleteObjectsOutput, error) {
			for _, obj := range in.Delete.Objects {
				if obj.Key != nil {
					deletedKeys = append(deletedKeys, *obj.Key)
				}
			}
			return &s3.DeleteObjectsOutput{}, nil
		},
	}

	err := deleteAllObjectsFromBucket(context.Background(), client, "my-bucket")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Expect obj1, obj2 from regular objects + obj1 from versions + obj2 from delete markers
	if len(deletedKeys) != 4 {
		t.Fatalf("expected 4 deleted keys, got %d: %v", len(deletedKeys), deletedKeys)
	}
}

func TestDeleteAllObjectsFromBucketListVersionsError(t *testing.T) {
	client := &mockClient{
		listObjectsV2Fn: func(_ context.Context, _ *s3.ListObjectsV2Input, _ ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
			return &s3.ListObjectsV2Output{}, nil
		},
		listObjectVersionsFn: func(_ context.Context, _ *s3.ListObjectVersionsInput, _ ...func(*s3.Options)) (*s3.ListObjectVersionsOutput, error) {
			return nil, errors.New("list versions failed")
		},
	}

	err := deleteAllObjectsFromBucket(context.Background(), client, "my-bucket")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "list object versions") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeleteAllObjectsFromBucketDeleteObjectsError(t *testing.T) {
	client := &mockClient{
		listObjectsV2Fn: func(_ context.Context, _ *s3.ListObjectsV2Input, _ ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
			return &s3.ListObjectsV2Output{
				Contents: []s3types.Object{{Key: cliutil.Ptr("obj1")}},
			}, nil
		},
		deleteObjectsFn: func(_ context.Context, _ *s3.DeleteObjectsInput, _ ...func(*s3.Options)) (*s3.DeleteObjectsOutput, error) {
			return nil, errors.New("delete batch failed")
		},
	}

	err := deleteAllObjectsFromBucket(context.Background(), client, "my-bucket")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "delete objects from bucket") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeleteAllObjectsFromBucketDeleteVersionsError(t *testing.T) {
	client := &mockClient{
		listObjectsV2Fn: func(_ context.Context, _ *s3.ListObjectsV2Input, _ ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
			return &s3.ListObjectsV2Output{}, nil
		},
		listObjectVersionsFn: func(_ context.Context, _ *s3.ListObjectVersionsInput, _ ...func(*s3.Options)) (*s3.ListObjectVersionsOutput, error) {
			return &s3.ListObjectVersionsOutput{
				Versions: []s3types.ObjectVersion{{Key: cliutil.Ptr("obj1"), VersionId: cliutil.Ptr("v1")}},
			}, nil
		},
		deleteObjectsFn: func(_ context.Context, _ *s3.DeleteObjectsInput, _ ...func(*s3.Options)) (*s3.DeleteObjectsOutput, error) {
			return nil, errors.New("delete batch failed")
		},
	}

	err := deleteAllObjectsFromBucket(context.Background(), client, "my-bucket")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "delete objects from bucket") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeleteAllObjectsFromBucketPagination(t *testing.T) {
	objectPageCalls := 0
	versionPageCalls := 0
	client := &mockClient{
		listObjectsV2Fn: func(_ context.Context, in *s3.ListObjectsV2Input, _ ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
			objectPageCalls++
			if objectPageCalls == 1 {
				return &s3.ListObjectsV2Output{
					Contents:              []s3types.Object{{Key: cliutil.Ptr("obj1")}},
					NextContinuationToken: cliutil.Ptr("token1"),
				}, nil
			}
			return &s3.ListObjectsV2Output{
				Contents: []s3types.Object{{Key: cliutil.Ptr("obj2")}},
			}, nil
		},
		listObjectVersionsFn: func(_ context.Context, in *s3.ListObjectVersionsInput, _ ...func(*s3.Options)) (*s3.ListObjectVersionsOutput, error) {
			versionPageCalls++
			if versionPageCalls == 1 {
				return &s3.ListObjectVersionsOutput{
					Versions:            []s3types.ObjectVersion{{Key: cliutil.Ptr("obj1"), VersionId: cliutil.Ptr("v1")}},
					NextKeyMarker:       cliutil.Ptr("key-marker"),
					NextVersionIdMarker: cliutil.Ptr("vid-marker"),
				}, nil
			}
			return &s3.ListObjectVersionsOutput{}, nil
		},
		deleteObjectsFn: func(_ context.Context, _ *s3.DeleteObjectsInput, _ ...func(*s3.Options)) (*s3.DeleteObjectsOutput, error) {
			return &s3.DeleteObjectsOutput{}, nil
		},
	}

	err := deleteAllObjectsFromBucket(context.Background(), client, "my-bucket")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if objectPageCalls != 2 {
		t.Fatalf("expected 2 object page calls, got %d", objectPageCalls)
	}
	if versionPageCalls != 2 {
		t.Fatalf("expected 2 version page calls, got %d", versionPageCalls)
	}
}

func TestDeleteAllObjectsFromBucketListObjectsError(t *testing.T) {
	client := &mockClient{
		listObjectsV2Fn: func(_ context.Context, _ *s3.ListObjectsV2Input, _ ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
			return nil, errors.New("list failed")
		},
	}

	err := deleteAllObjectsFromBucket(context.Background(), client, "my-bucket")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "list objects for bucket") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ============================================================
// runDownloadBucket tests
// ============================================================

func TestDownloadBucketRequiresBucket(t *testing.T) {
	_, err := executeCommand(t, "s3", "download-bucket", "--prefix", "data/")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "--bucket-name is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDownloadBucketRequiresPrefix(t *testing.T) {
	_, err := executeCommand(t, "s3", "download-bucket", "--bucket-name", "my-bucket")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "--prefix is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDownloadBucketDryRun(t *testing.T) {
	now := time.Now().UTC()
	client := &mockClient{
		listObjectsV2Fn: func(_ context.Context, _ *s3.ListObjectsV2Input, _ ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
			return &s3.ListObjectsV2Output{
				Contents: []s3types.Object{
					{Key: cliutil.Ptr("data/file1.txt"), LastModified: &now, Size: cliutil.Ptr(int64(100))},
					{Key: cliutil.Ptr("data/sub/file2.txt"), LastModified: &now, Size: cliutil.Ptr(int64(200))},
				},
			}, nil
		},
	}

	withMockDeps(t, mockLoader, mockFactory(client))

	output, err := executeCommand(t, "--output", "json", "--dry-run", "s3", "download-bucket", "--bucket-name", "my-bucket", "--prefix", "data/")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(output, "would-download") {
		t.Fatalf("expected would-download in output: %s", output)
	}
	if !strings.Contains(output, "file1.txt") {
		t.Fatalf("expected file1.txt in output: %s", output)
	}
}

func TestDownloadBucketActualDownload(t *testing.T) {
	tmpDir := t.TempDir()
	now := time.Now().UTC()

	client := &mockClient{
		listObjectsV2Fn: func(_ context.Context, _ *s3.ListObjectsV2Input, _ ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
			return &s3.ListObjectsV2Output{
				Contents: []s3types.Object{
					{Key: cliutil.Ptr("prefix/hello.txt"), LastModified: &now, Size: cliutil.Ptr(int64(5))},
				},
			}, nil
		},
		getObjectFn: func(_ context.Context, in *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
			return &s3.GetObjectOutput{
				Body: nopReadCloser{bytes.NewReader([]byte("hello"))},
			}, nil
		},
	}

	withMockDeps(t, mockLoader, mockFactory(client))

	output, err := executeCommand(t, "--output", "json", "s3", "download-bucket", "--bucket-name", "my-bucket", "--prefix", "prefix/", "--output-dir", tmpDir)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(output, "downloaded") {
		t.Fatalf("expected downloaded in output: %s", output)
	}

	content, readErr := os.ReadFile(filepath.Join(tmpDir, "hello.txt"))
	if readErr != nil {
		t.Fatalf("read downloaded file: %v", readErr)
	}
	if string(content) != "hello" {
		t.Fatalf("expected 'hello', got %q", string(content))
	}
}

func TestDownloadBucketDownloadError(t *testing.T) {
	now := time.Now().UTC()
	client := &mockClient{
		listObjectsV2Fn: func(_ context.Context, _ *s3.ListObjectsV2Input, _ ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
			return &s3.ListObjectsV2Output{
				Contents: []s3types.Object{
					{Key: cliutil.Ptr("prefix/fail.txt"), LastModified: &now, Size: cliutil.Ptr(int64(5))},
				},
			}, nil
		},
		getObjectFn: func(_ context.Context, _ *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
			return nil, errors.New("download failed")
		},
	}

	withMockDeps(t, mockLoader, mockFactory(client))

	output, err := executeCommand(t, "--output", "json", "s3", "download-bucket", "--bucket-name", "my-bucket", "--prefix", "prefix/", "--output-dir", t.TempDir())
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(output, "failed:") {
		t.Fatalf("expected failed in output: %s", output)
	}
}

func TestDownloadBucketRejectsPathTraversal(t *testing.T) {
	now := time.Now().UTC()
	tmpDir := t.TempDir()
	outsideFileName := filepath.Base(tmpDir) + "-outside-test-traversal.txt"
	outsideFile := filepath.Join(filepath.Dir(tmpDir), outsideFileName)

	client := &mockClient{
		listObjectsV2Fn: func(_ context.Context, _ *s3.ListObjectsV2Input, _ ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
			return &s3.ListObjectsV2Output{
				Contents: []s3types.Object{
					{Key: cliutil.Ptr("prefix/../../" + outsideFileName), LastModified: &now, Size: cliutil.Ptr(int64(5))},
				},
			}, nil
		},
		getObjectFn: func(_ context.Context, _ *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
			t.Fatal("GetObject should not be called for traversal keys")
			return nil, nil
		},
	}

	withMockDeps(t, mockLoader, mockFactory(client))

	output, err := executeCommand(t, "--output", "json", "s3", "download-bucket", "--bucket-name", "my-bucket", "--prefix", "prefix/", "--output-dir", tmpDir)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(output, "failed:") || !strings.Contains(output, "invalid object key path") {
		t.Fatalf("expected traversal failure in output: %s", output)
	}
	if _, statErr := os.Stat(outsideFile); !os.IsNotExist(statErr) {
		t.Fatalf("expected %s not to exist; statErr=%v", outsideFile, statErr)
	}
}

func TestDownloadBucketListError(t *testing.T) {
	client := &mockClient{
		listObjectsV2Fn: func(_ context.Context, _ *s3.ListObjectsV2Input, _ ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
			return nil, errors.New("list error")
		},
	}

	withMockDeps(t, mockLoader, mockFactory(client))

	_, err := executeCommand(t, "--output", "json", "s3", "download-bucket", "--bucket-name", "my-bucket", "--prefix", "data/")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "list objects") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDownloadBucketAWSConfigError(t *testing.T) {
	withMockDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{}, errors.New("config failed") },
		func(awssdk.Config) API { return &mockClient{} },
	)

	_, err := executeCommand(t, "s3", "download-bucket", "--bucket-name", "b", "--prefix", "p")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "config failed") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDownloadBucketKeyEqualToPrefix(t *testing.T) {
	// When the key equals the prefix, relativeKey becomes empty and falls back to key.
	now := time.Now().UTC()
	tmpDir := t.TempDir()

	client := &mockClient{
		listObjectsV2Fn: func(_ context.Context, _ *s3.ListObjectsV2Input, _ ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
			return &s3.ListObjectsV2Output{
				Contents: []s3types.Object{
					{Key: cliutil.Ptr("data/"), LastModified: &now, Size: cliutil.Ptr(int64(0))},
				},
			}, nil
		},
	}

	withMockDeps(t, mockLoader, mockFactory(client))

	output, err := executeCommand(t, "--output", "json", "--dry-run", "s3", "download-bucket", "--bucket-name", "my-bucket", "--prefix", "data/", "--output-dir", tmpDir)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(output, "would-download") {
		t.Fatalf("expected would-download in output: %s", output)
	}
}

// ============================================================
// runListOldFiles tests
// ============================================================

func TestListOldFilesRequiresBucket(t *testing.T) {
	_, err := executeCommand(t, "s3", "list-old-files")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "--bucket-name is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestListOldFilesNegativeDays(t *testing.T) {
	_, err := executeCommand(t, "s3", "list-old-files", "--bucket-name", "b", "--older-than-days", "-1")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "--older-than-days must be >= 0") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestListOldFilesWithResults(t *testing.T) {
	oldDate := time.Now().UTC().AddDate(0, 0, -90)
	recentDate := time.Now().UTC().AddDate(0, 0, -10)

	client := &mockClient{
		listObjectsV2Fn: func(_ context.Context, _ *s3.ListObjectsV2Input, _ ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
			return &s3.ListObjectsV2Output{
				Contents: []s3types.Object{
					{Key: cliutil.Ptr("old-file.txt"), LastModified: &oldDate, Size: cliutil.Ptr(int64(500))},
					{Key: cliutil.Ptr("recent-file.txt"), LastModified: &recentDate, Size: cliutil.Ptr(int64(200))},
					{Key: cliutil.Ptr("nil-date-file.txt"), LastModified: nil, Size: cliutil.Ptr(int64(100))},
				},
			}, nil
		},
	}

	withMockDeps(t, mockLoader, mockFactory(client))

	output, err := executeCommand(t, "--output", "json", "s3", "list-old-files", "--bucket-name", "my-bucket", "--older-than-days", "60")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(output, "old-file.txt") {
		t.Fatalf("expected old-file.txt in output: %s", output)
	}
	if strings.Contains(output, "recent-file.txt") {
		t.Fatalf("recent file should not appear: %s", output)
	}
	if strings.Contains(output, "nil-date-file.txt") {
		t.Fatalf("nil-date file should not appear: %s", output)
	}
}

func TestListOldFilesEmptyResults(t *testing.T) {
	client := &mockClient{
		listObjectsV2Fn: func(_ context.Context, _ *s3.ListObjectsV2Input, _ ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
			return &s3.ListObjectsV2Output{}, nil
		},
	}

	withMockDeps(t, mockLoader, mockFactory(client))

	output, err := executeCommand(t, "--output", "json", "s3", "list-old-files", "--bucket-name", "my-bucket")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	// Should produce valid output with no rows.
	if strings.Contains(output, "old-file") {
		t.Fatalf("expected no results: %s", output)
	}
}

func TestListOldFilesListError(t *testing.T) {
	client := &mockClient{
		listObjectsV2Fn: func(_ context.Context, _ *s3.ListObjectsV2Input, _ ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
			return nil, errors.New("list error")
		},
	}

	withMockDeps(t, mockLoader, mockFactory(client))

	_, err := executeCommand(t, "s3", "list-old-files", "--bucket-name", "my-bucket")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "list objects") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestListOldFilesAWSConfigError(t *testing.T) {
	withMockDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{}, errors.New("config failed") },
		func(awssdk.Config) API { return &mockClient{} },
	)

	_, err := executeCommand(t, "s3", "list-old-files", "--bucket-name", "b")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "config failed") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestListOldFilesWithPrefix(t *testing.T) {
	oldDate := time.Now().UTC().AddDate(0, 0, -100)

	client := &mockClient{
		listObjectsV2Fn: func(_ context.Context, in *s3.ListObjectsV2Input, _ ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
			if cliutil.PointerToString(in.Prefix) != "logs/" {
				t.Fatalf("expected prefix 'logs/', got %q", cliutil.PointerToString(in.Prefix))
			}
			return &s3.ListObjectsV2Output{
				Contents: []s3types.Object{
					{Key: cliutil.Ptr("logs/app.log"), LastModified: &oldDate, Size: cliutil.Ptr(int64(1000))},
				},
			}, nil
		},
	}

	withMockDeps(t, mockLoader, mockFactory(client))

	output, err := executeCommand(t, "--output", "json", "s3", "list-old-files", "--bucket-name", "my-bucket", "--prefix", "logs/", "--older-than-days", "30")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(output, "logs/app.log") {
		t.Fatalf("expected logs/app.log in output: %s", output)
	}
}

func TestListOldFilesNilSize(t *testing.T) {
	oldDate := time.Now().UTC().AddDate(0, 0, -100)

	client := &mockClient{
		listObjectsV2Fn: func(_ context.Context, _ *s3.ListObjectsV2Input, _ ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
			return &s3.ListObjectsV2Output{
				Contents: []s3types.Object{
					{Key: cliutil.Ptr("nil-size.txt"), LastModified: &oldDate, Size: nil},
				},
			}, nil
		},
	}

	withMockDeps(t, mockLoader, mockFactory(client))

	output, err := executeCommand(t, "--output", "json", "s3", "list-old-files", "--bucket-name", "my-bucket", "--older-than-days", "30")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(output, `"size_bytes": "0"`) {
		t.Fatalf("expected size_bytes 0 for nil size: %s", output)
	}
}

// ============================================================
// runSearchObjects tests
// ============================================================

func TestSearchObjectsRequiresPrefixOrKeys(t *testing.T) {
	withMockDeps(t, mockLoader, mockFactory(&mockClient{}))

	_, err := executeCommand(t, "s3", "search-objects", "--bucket-name", "my-bucket")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "set --prefix and/or --keys") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSearchObjectsPrefixOnlyNoKeys(t *testing.T) {
	now := time.Now().UTC()
	client := &mockClient{
		listObjectsV2Fn: func(_ context.Context, _ *s3.ListObjectsV2Input, _ ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
			return &s3.ListObjectsV2Output{
				Contents: []s3types.Object{
					{Key: cliutil.Ptr("data/a.txt"), LastModified: &now, Size: cliutil.Ptr(int64(100))},
					{Key: cliutil.Ptr("data/b.txt"), LastModified: &now, Size: cliutil.Ptr(int64(200))},
				},
			}, nil
		},
	}

	withMockDeps(t, mockLoader, mockFactory(client))

	output, err := executeCommand(t, "--output", "json", "s3", "search-objects", "--bucket-name", "my-bucket", "--prefix", "data/")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(output, "data/a.txt") || !strings.Contains(output, "data/b.txt") {
		t.Fatalf("expected both files in output: %s", output)
	}
	// In prefix-only mode, all rows should have exists=true.
	if !strings.Contains(output, `"exists": "true"`) {
		t.Fatalf("expected exists true: %s", output)
	}
}

func TestSearchObjectsKeyWithPrefixMatch(t *testing.T) {
	now := time.Now().UTC()
	client := &mockClient{
		listObjectsV2Fn: func(_ context.Context, _ *s3.ListObjectsV2Input, _ ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
			return &s3.ListObjectsV2Output{
				Contents: []s3types.Object{
					{Key: cliutil.Ptr("prefix/file.txt"), LastModified: &now, Size: cliutil.Ptr(int64(100))},
				},
			}, nil
		},
	}

	withMockDeps(t, mockLoader, mockFactory(client))

	// Search for "file.txt" with prefix "prefix/" - should match prefix/file.txt.
	output, err := executeCommand(t, "--output", "json", "s3", "search-objects", "--bucket-name", "my-bucket", "--prefix", "prefix/", "--keys", "file.txt")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(output, `"exists": "true"`) {
		t.Fatalf("expected exists true: %s", output)
	}
	if !strings.Contains(output, "prefix/file.txt") {
		t.Fatalf("expected matched key prefix/file.txt: %s", output)
	}
}

func TestSearchObjectsListError(t *testing.T) {
	client := &mockClient{
		listObjectsV2Fn: func(_ context.Context, _ *s3.ListObjectsV2Input, _ ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
			return nil, errors.New("list error")
		},
	}

	withMockDeps(t, mockLoader, mockFactory(client))

	_, err := executeCommand(t, "s3", "search-objects", "--bucket-name", "my-bucket", "--prefix", "x")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "list objects") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSearchObjectsAWSConfigError(t *testing.T) {
	withMockDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{}, errors.New("config failed") },
		func(awssdk.Config) API { return &mockClient{} },
	)

	_, err := executeCommand(t, "s3", "search-objects", "--bucket-name", "b", "--keys", "k")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "config failed") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSearchObjectsNilLastModified(t *testing.T) {
	client := &mockClient{
		listObjectsV2Fn: func(_ context.Context, _ *s3.ListObjectsV2Input, _ ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
			return &s3.ListObjectsV2Output{
				Contents: []s3types.Object{
					{Key: cliutil.Ptr("no-date.txt"), LastModified: nil, Size: cliutil.Ptr(int64(100))},
				},
			}, nil
		},
	}

	withMockDeps(t, mockLoader, mockFactory(client))

	output, err := executeCommand(t, "--output", "json", "s3", "search-objects", "--bucket-name", "my-bucket", "--keys", "no-date.txt")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(output, `"exists": "true"`) {
		t.Fatalf("expected exists true: %s", output)
	}
	// last_modified should be empty string.
	if !strings.Contains(output, `"last_modified": ""`) {
		t.Fatalf("expected empty last_modified: %s", output)
	}
}

func TestSearchObjectsPrefixOnlyNilLastModified(t *testing.T) {
	client := &mockClient{
		listObjectsV2Fn: func(_ context.Context, _ *s3.ListObjectsV2Input, _ ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
			return &s3.ListObjectsV2Output{
				Contents: []s3types.Object{
					{Key: cliutil.Ptr("nil-time.txt"), LastModified: nil, Size: nil},
				},
			}, nil
		},
	}

	withMockDeps(t, mockLoader, mockFactory(client))

	output, err := executeCommand(t, "--output", "json", "s3", "search-objects", "--bucket-name", "my-bucket", "--prefix", "nil")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(output, "nil-time.txt") {
		t.Fatalf("expected nil-time.txt in output: %s", output)
	}
}

// ============================================================
// Helper function tests
// ============================================================

func TestNormalizeKeyQueriesDedup(t *testing.T) {
	result := normalizeKeyQueries([]string{"a,b,a", "c, b , d"})
	if len(result) != 4 {
		t.Fatalf("expected 4 unique keys, got %d: %v", len(result), result)
	}
	expected := []string{"a", "b", "c", "d"}
	for i, key := range expected {
		if result[i] != key {
			t.Fatalf("expected %q at index %d, got %q", key, i, result[i])
		}
	}
}

func TestNormalizeKeyQueriesEmpty(t *testing.T) {
	result := normalizeKeyQueries([]string{",,,", "  ", ""})
	if len(result) != 0 {
		t.Fatalf("expected empty result, got %v", result)
	}
}

func TestNormalizeKeyQueriesNil(t *testing.T) {
	result := normalizeKeyQueries(nil)
	if len(result) != 0 {
		t.Fatalf("expected empty result, got %v", result)
	}
}

func TestObjectKeyNil(t *testing.T) {
	obj := s3types.Object{Key: nil}
	if got := objectKey(obj); got != "" {
		t.Fatalf("expected empty string, got %q", got)
	}
}

func TestObjectLastModifiedNil(t *testing.T) {
	obj := s3types.Object{LastModified: nil}
	if got := objectLastModified(obj); !got.IsZero() {
		t.Fatalf("expected zero time, got %v", got)
	}
}

func TestObjectLastModifiedValue(t *testing.T) {
	ts := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	obj := s3types.Object{LastModified: &ts}
	if got := objectLastModified(obj); !got.Equal(ts) {
		t.Fatalf("expected %v, got %v", ts, got)
	}
}

func TestObjectSizeNil(t *testing.T) {
	obj := s3types.Object{Size: nil}
	if got := objectSize(obj); got != 0 {
		t.Fatalf("expected 0, got %d", got)
	}
}

func TestObjectSizeValue(t *testing.T) {
	obj := s3types.Object{Size: cliutil.Ptr(int64(42))}
	if got := objectSize(obj); got != 42 {
		t.Fatalf("expected 42, got %d", got)
	}
}

func TestSortObjectsByKey(t *testing.T) {
	objects := []s3types.Object{
		{Key: cliutil.Ptr("c")},
		{Key: cliutil.Ptr("a")},
		{Key: cliutil.Ptr("b")},
	}
	sortObjectsByKey(objects)
	if objectKey(objects[0]) != "a" || objectKey(objects[1]) != "b" || objectKey(objects[2]) != "c" {
		t.Fatalf("unexpected order: %s %s %s", objectKey(objects[0]), objectKey(objects[1]), objectKey(objects[2]))
	}
}

func TestDeleteObjectBatchEmpty(t *testing.T) {
	client := &mockClient{}
	err := deleteObjectBatch(context.Background(), client, "my-bucket", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeleteObjectBatchError(t *testing.T) {
	client := &mockClient{
		deleteObjectsFn: func(_ context.Context, _ *s3.DeleteObjectsInput, _ ...func(*s3.Options)) (*s3.DeleteObjectsOutput, error) {
			return nil, errors.New("delete failed")
		},
	}

	err := deleteObjectBatch(context.Background(), client, "my-bucket", []s3types.ObjectIdentifier{
		{Key: cliutil.Ptr("obj1")},
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "delete objects from bucket") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeleteObjectBatchSuccess(t *testing.T) {
	client := &mockClient{
		deleteObjectsFn: func(_ context.Context, in *s3.DeleteObjectsInput, _ ...func(*s3.Options)) (*s3.DeleteObjectsOutput, error) {
			if len(in.Delete.Objects) != 2 {
				t.Fatalf("expected 2 objects, got %d", len(in.Delete.Objects))
			}
			return &s3.DeleteObjectsOutput{}, nil
		},
	}

	err := deleteObjectBatch(context.Background(), client, "my-bucket", []s3types.ObjectIdentifier{
		{Key: cliutil.Ptr("obj1")},
		{Key: cliutil.Ptr("obj2")},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDownloadObjectSuccess(t *testing.T) {
	tmpDir := t.TempDir()
	targetPath := filepath.Join(tmpDir, "sub", "file.txt")

	client := &mockClient{
		getObjectFn: func(_ context.Context, _ *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
			return &s3.GetObjectOutput{
				Body: nopReadCloser{bytes.NewReader([]byte("content"))},
			}, nil
		},
	}

	err := downloadObject(context.Background(), client, "my-bucket", "key", targetPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, readErr := os.ReadFile(targetPath)
	if readErr != nil {
		t.Fatalf("read file: %v", readErr)
	}
	if string(data) != "content" {
		t.Fatalf("expected 'content', got %q", string(data))
	}
}

func TestDownloadObjectGetError(t *testing.T) {
	client := &mockClient{
		getObjectFn: func(_ context.Context, _ *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
			return nil, errors.New("get failed")
		},
	}

	err := downloadObject(context.Background(), client, "my-bucket", "key", "/tmp/whatever.txt")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "download key") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ============================================================
// listObjects pagination test
// ============================================================

func TestListObjectsPagination(t *testing.T) {
	callCount := 0
	client := &mockClient{
		listObjectsV2Fn: func(_ context.Context, in *s3.ListObjectsV2Input, _ ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
			callCount++
			if callCount == 1 {
				return &s3.ListObjectsV2Output{
					Contents:              []s3types.Object{{Key: cliutil.Ptr("obj1")}},
					NextContinuationToken: cliutil.Ptr("token1"),
				}, nil
			}
			return &s3.ListObjectsV2Output{
				Contents: []s3types.Object{{Key: cliutil.Ptr("obj2")}},
			}, nil
		},
	}

	objects, err := listObjects(context.Background(), client, "my-bucket", "prefix/")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(objects) != 2 {
		t.Fatalf("expected 2 objects, got %d", len(objects))
	}
	if callCount != 2 {
		t.Fatalf("expected 2 API calls, got %d", callCount)
	}
}

func TestListObjectsNoPrefix(t *testing.T) {
	client := &mockClient{
		listObjectsV2Fn: func(_ context.Context, in *s3.ListObjectsV2Input, _ ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
			if in.Prefix != nil {
				t.Fatalf("expected nil prefix, got %q", *in.Prefix)
			}
			return &s3.ListObjectsV2Output{}, nil
		},
	}

	_, err := listObjects(context.Background(), client, "my-bucket", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestListObjectsError(t *testing.T) {
	client := &mockClient{
		listObjectsV2Fn: func(_ context.Context, _ *s3.ListObjectsV2Input, _ ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
			return nil, errors.New("fail")
		},
	}

	_, err := listObjects(context.Background(), client, "my-bucket", "")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestListBucketsError(t *testing.T) {
	client := &mockClient{
		listBucketsFn: func(_ context.Context, _ *s3.ListBucketsInput, _ ...func(*s3.Options)) (*s3.ListBucketsOutput, error) {
			return nil, errors.New("access denied")
		},
	}

	_, err := listBuckets(context.Background(), client)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestListBucketsSuccess(t *testing.T) {
	client := &mockClient{
		listBucketsFn: func(_ context.Context, _ *s3.ListBucketsInput, _ ...func(*s3.Options)) (*s3.ListBucketsOutput, error) {
			return &s3.ListBucketsOutput{
				Buckets: []s3types.Bucket{
					{Name: cliutil.Ptr("bucket1")},
					{Name: cliutil.Ptr("bucket2")},
				},
			}, nil
		},
	}

	buckets, err := listBuckets(context.Background(), client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(buckets) != 2 {
		t.Fatalf("expected 2 buckets, got %d", len(buckets))
	}
}

// ============================================================
// isBucketEmptyAndUnversioned tests
// ============================================================

func TestIsBucketEmptyAndUnversionedTrue(t *testing.T) {
	client := &mockClient{
		listObjectsV2Fn: func(_ context.Context, _ *s3.ListObjectsV2Input, _ ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
			return &s3.ListObjectsV2Output{}, nil
		},
		getBucketVersioningFn: func(_ context.Context, _ *s3.GetBucketVersioningInput, _ ...func(*s3.Options)) (*s3.GetBucketVersioningOutput, error) {
			return &s3.GetBucketVersioningOutput{
				Status: s3types.BucketVersioningStatusSuspended,
			}, nil
		},
	}

	ok, err := isBucketEmptyAndUnversioned(context.Background(), client, "test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected true for empty + suspended versioning")
	}
}

func TestIsBucketEmptyAndUnversionedFalseWhenHasObjects(t *testing.T) {
	client := &mockClient{
		listObjectsV2Fn: func(_ context.Context, _ *s3.ListObjectsV2Input, _ ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
			return &s3.ListObjectsV2Output{
				Contents: []s3types.Object{{Key: cliutil.Ptr("obj")}},
			}, nil
		},
	}

	ok, err := isBucketEmptyAndUnversioned(context.Background(), client, "test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatal("expected false for non-empty bucket")
	}
}

func TestIsBucketEmptyAndUnversionedFalseWhenVersioned(t *testing.T) {
	client := &mockClient{
		listObjectsV2Fn: func(_ context.Context, _ *s3.ListObjectsV2Input, _ ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
			return &s3.ListObjectsV2Output{}, nil
		},
		getBucketVersioningFn: func(_ context.Context, _ *s3.GetBucketVersioningInput, _ ...func(*s3.Options)) (*s3.GetBucketVersioningOutput, error) {
			return &s3.GetBucketVersioningOutput{
				Status: s3types.BucketVersioningStatusEnabled,
			}, nil
		},
	}

	ok, err := isBucketEmptyAndUnversioned(context.Background(), client, "test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatal("expected false for versioned bucket")
	}
}
