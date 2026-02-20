package efs

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/efs"
	efstypes "github.com/aws/aws-sdk-go-v2/service/efs/types"
	"github.com/towardsthecloud/aws-toolbox/internal/cliutil"
)

type mockClient struct {
	deleteFileSystemFn     func(context.Context, *efs.DeleteFileSystemInput, ...func(*efs.Options)) (*efs.DeleteFileSystemOutput, error)
	deleteMountTargetFn    func(context.Context, *efs.DeleteMountTargetInput, ...func(*efs.Options)) (*efs.DeleteMountTargetOutput, error)
	describeFileSystemsFn  func(context.Context, *efs.DescribeFileSystemsInput, ...func(*efs.Options)) (*efs.DescribeFileSystemsOutput, error)
	describeMountTargetsFn func(context.Context, *efs.DescribeMountTargetsInput, ...func(*efs.Options)) (*efs.DescribeMountTargetsOutput, error)
	listTagsForResourceFn  func(context.Context, *efs.ListTagsForResourceInput, ...func(*efs.Options)) (*efs.ListTagsForResourceOutput, error)
}

func (m *mockClient) DeleteFileSystem(ctx context.Context, in *efs.DeleteFileSystemInput, optFns ...func(*efs.Options)) (*efs.DeleteFileSystemOutput, error) {
	if m.deleteFileSystemFn == nil {
		return nil, errors.New("DeleteFileSystem not mocked")
	}
	return m.deleteFileSystemFn(ctx, in, optFns...)
}

func (m *mockClient) DeleteMountTarget(ctx context.Context, in *efs.DeleteMountTargetInput, optFns ...func(*efs.Options)) (*efs.DeleteMountTargetOutput, error) {
	if m.deleteMountTargetFn == nil {
		return nil, errors.New("DeleteMountTarget not mocked")
	}
	return m.deleteMountTargetFn(ctx, in, optFns...)
}

func (m *mockClient) DescribeFileSystems(ctx context.Context, in *efs.DescribeFileSystemsInput, optFns ...func(*efs.Options)) (*efs.DescribeFileSystemsOutput, error) {
	if m.describeFileSystemsFn == nil {
		return nil, errors.New("DescribeFileSystems not mocked")
	}
	return m.describeFileSystemsFn(ctx, in, optFns...)
}

func (m *mockClient) DescribeMountTargets(ctx context.Context, in *efs.DescribeMountTargetsInput, optFns ...func(*efs.Options)) (*efs.DescribeMountTargetsOutput, error) {
	if m.describeMountTargetsFn == nil {
		return nil, errors.New("DescribeMountTargets not mocked")
	}
	return m.describeMountTargetsFn(ctx, in, optFns...)
}

func (m *mockClient) ListTagsForResource(ctx context.Context, in *efs.ListTagsForResourceInput, optFns ...func(*efs.Options)) (*efs.ListTagsForResourceOutput, error) {
	if m.listTagsForResourceFn == nil {
		return &efs.ListTagsForResourceOutput{}, nil
	}
	return m.listTagsForResourceFn(ctx, in, optFns...)
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

func mockSleep(t *testing.T) {
	t.Helper()
	oldSleep := sleep
	sleep = func(_ time.Duration) {}
	t.Cleanup(func() { sleep = oldSleep })
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

func standardLoader(_, _ string) (awssdk.Config, error) {
	return awssdk.Config{Region: "us-east-1"}, nil
}

func newStandardMockClient() *mockClient {
	return &mockClient{
		describeFileSystemsFn: func(_ context.Context, _ *efs.DescribeFileSystemsInput, _ ...func(*efs.Options)) (*efs.DescribeFileSystemsOutput, error) {
			return &efs.DescribeFileSystemsOutput{
				FileSystems: []efstypes.FileSystemDescription{
					{FileSystemId: cliutil.Ptr("fs-123")},
				},
			}, nil
		},
		describeMountTargetsFn: func(_ context.Context, _ *efs.DescribeMountTargetsInput, _ ...func(*efs.Options)) (*efs.DescribeMountTargetsOutput, error) {
			return &efs.DescribeMountTargetsOutput{}, nil
		},
		deleteMountTargetFn: func(_ context.Context, _ *efs.DeleteMountTargetInput, _ ...func(*efs.Options)) (*efs.DeleteMountTargetOutput, error) {
			return &efs.DeleteMountTargetOutput{}, nil
		},
		deleteFileSystemFn: func(_ context.Context, _ *efs.DeleteFileSystemInput, _ ...func(*efs.Options)) (*efs.DeleteFileSystemOutput, error) {
			return &efs.DeleteFileSystemOutput{}, nil
		},
		listTagsForResourceFn: func(_ context.Context, _ *efs.ListTagsForResourceInput, _ ...func(*efs.Options)) (*efs.ListTagsForResourceOutput, error) {
			return &efs.ListTagsForResourceOutput{Tags: []efstypes.Tag{
				{Key: cliutil.Ptr("env"), Value: cliutil.Ptr("dev")},
			}}, nil
		},
	}
}

func TestDeleteFilesystemsWaitsForMountTargets(t *testing.T) {
	describeMountTargetsCalls := 0
	deleteMountTargetCalls := 0
	deleteFileSystemCalls := 0

	client := &mockClient{
		describeFileSystemsFn: func(_ context.Context, _ *efs.DescribeFileSystemsInput, _ ...func(*efs.Options)) (*efs.DescribeFileSystemsOutput, error) {
			return &efs.DescribeFileSystemsOutput{
				FileSystems: []efstypes.FileSystemDescription{
					{FileSystemId: cliutil.Ptr("fs-123")},
				},
			}, nil
		},
		describeMountTargetsFn: func(_ context.Context, _ *efs.DescribeMountTargetsInput, _ ...func(*efs.Options)) (*efs.DescribeMountTargetsOutput, error) {
			describeMountTargetsCalls++
			if describeMountTargetsCalls < 3 {
				return &efs.DescribeMountTargetsOutput{
					MountTargets: []efstypes.MountTargetDescription{{MountTargetId: cliutil.Ptr("mt-1")}},
				}, nil
			}
			return &efs.DescribeMountTargetsOutput{}, nil
		},
		deleteMountTargetFn: func(_ context.Context, _ *efs.DeleteMountTargetInput, _ ...func(*efs.Options)) (*efs.DeleteMountTargetOutput, error) {
			deleteMountTargetCalls++
			return &efs.DeleteMountTargetOutput{}, nil
		},
		deleteFileSystemFn: func(_ context.Context, _ *efs.DeleteFileSystemInput, _ ...func(*efs.Options)) (*efs.DeleteFileSystemOutput, error) {
			deleteFileSystemCalls++
			return &efs.DeleteFileSystemOutput{}, nil
		},
	}

	withMockDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) API { return client },
	)

	mockSleep(t)

	output, err := executeCommand(t, "--output", "json", "--no-confirm", "efs", "delete-filesystems")
	if err != nil {
		t.Fatalf("execute efs delete-filesystems --no-confirm: %v", err)
	}

	if deleteMountTargetCalls != 1 || deleteFileSystemCalls != 1 {
		t.Fatalf("unexpected delete call counts mount-target=%d file-system=%d", deleteMountTargetCalls, deleteFileSystemCalls)
	}
	if describeMountTargetsCalls < 3 {
		t.Fatalf("expected mount-target polling before deleting file system, describe calls=%d", describeMountTargetsCalls)
	}
	if !strings.Contains(output, "\"action\": \"deleted\"") {
		t.Fatalf("unexpected output: %s", output)
	}
}

func TestDeleteFilesystemsDryRun(t *testing.T) {
	client := newStandardMockClient()
	withMockDeps(t, standardLoader, func(awssdk.Config) API { return client })

	output, err := executeCommand(t, "--output", "json", "--dry-run", "efs", "delete-filesystems")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(output, "would-delete") {
		t.Fatalf("expected would-delete in dry-run output: %s", output)
	}
	if !strings.Contains(output, "fs-123") {
		t.Fatalf("expected fs-123 in output: %s", output)
	}
}

func TestDeleteFilesystemsNoConfirmDirect(t *testing.T) {
	client := newStandardMockClient()
	withMockDeps(t, standardLoader, func(awssdk.Config) API { return client })

	output, err := executeCommand(t, "--output", "json", "--no-confirm", "efs", "delete-filesystems")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(output, "\"action\": \"deleted\"") {
		t.Fatalf("expected deleted action: %s", output)
	}
}

func TestDeleteFilesystemsUserDeclinesConfirmation(t *testing.T) {
	client := newStandardMockClient()
	withMockDeps(t, standardLoader, func(awssdk.Config) API { return client })

	root := cliutil.NewTestRootCommand(NewCommand())
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetIn(strings.NewReader("n\n"))
	root.SetArgs([]string{"--output", "json", "efs", "delete-filesystems"})

	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(buf.String(), "cancelled") {
		t.Fatalf("expected cancelled: %s", buf.String())
	}
}

func TestDeleteFilesystemsEmptyResults(t *testing.T) {
	client := &mockClient{
		describeFileSystemsFn: func(_ context.Context, _ *efs.DescribeFileSystemsInput, _ ...func(*efs.Options)) (*efs.DescribeFileSystemsOutput, error) {
			return &efs.DescribeFileSystemsOutput{FileSystems: []efstypes.FileSystemDescription{}}, nil
		},
	}
	withMockDeps(t, standardLoader, func(awssdk.Config) API { return client })

	_, err := executeCommand(t, "--output", "json", "--no-confirm", "efs", "delete-filesystems")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
}

func TestDeleteFilesystemsListError(t *testing.T) {
	client := &mockClient{
		describeFileSystemsFn: func(_ context.Context, _ *efs.DescribeFileSystemsInput, _ ...func(*efs.Options)) (*efs.DescribeFileSystemsOutput, error) {
			return nil, errors.New("describe error")
		},
	}
	withMockDeps(t, standardLoader, func(awssdk.Config) API { return client })

	_, err := executeCommand(t, "--output", "json", "efs", "delete-filesystems")
	if err == nil {
		t.Fatal("expected error from DescribeFileSystems failure")
	}
	if !strings.Contains(err.Error(), "list EFS file systems") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeleteFilesystemsAWSConfigError(t *testing.T) {
	withMockDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{}, errors.New("config error") },
		func(awssdk.Config) API { return &mockClient{} },
	)

	_, err := executeCommand(t, "--output", "json", "efs", "delete-filesystems")
	if err == nil {
		t.Fatal("expected error from config load failure")
	}
}

func TestDeleteFilesystemsInvalidTagFilter(t *testing.T) {
	client := newStandardMockClient()
	withMockDeps(t, standardLoader, func(awssdk.Config) API { return client })

	_, err := executeCommand(t, "--output", "json", "efs", "delete-filesystems", "--filter-tag", "invalid")
	if err == nil {
		t.Fatal("expected error for invalid tag filter")
	}
}

func TestDeleteFilesystemsFilterTagMatch(t *testing.T) {
	client := newStandardMockClient()
	withMockDeps(t, standardLoader, func(awssdk.Config) API { return client })

	output, err := executeCommand(t, "--output", "json", "--dry-run", "efs", "delete-filesystems", "--filter-tag", "env=dev")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(output, "fs-123") {
		t.Fatalf("expected fs-123 in output: %s", output)
	}
}

func TestDeleteFilesystemsFilterTagNoMatch(t *testing.T) {
	client := newStandardMockClient()
	withMockDeps(t, standardLoader, func(awssdk.Config) API { return client })

	output, err := executeCommand(t, "--output", "json", "--dry-run", "efs", "delete-filesystems", "--filter-tag", "env=prod")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if strings.Contains(output, "fs-123") {
		t.Fatalf("expected no fs-123 when tag does not match: %s", output)
	}
}

func TestDeleteFilesystemsFilterTagListError(t *testing.T) {
	client := newStandardMockClient()
	client.listTagsForResourceFn = func(_ context.Context, _ *efs.ListTagsForResourceInput, _ ...func(*efs.Options)) (*efs.ListTagsForResourceOutput, error) {
		return nil, errors.New("tags error")
	}
	withMockDeps(t, standardLoader, func(awssdk.Config) API { return client })

	_, err := executeCommand(t, "--output", "json", "efs", "delete-filesystems", "--filter-tag", "env=dev")
	if err == nil {
		t.Fatal("expected error from tag list failure")
	}
	if !strings.Contains(err.Error(), "list tags for file system") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeleteFilesystemsDeleteFileSystemError(t *testing.T) {
	client := newStandardMockClient()
	client.deleteFileSystemFn = func(_ context.Context, _ *efs.DeleteFileSystemInput, _ ...func(*efs.Options)) (*efs.DeleteFileSystemOutput, error) {
		return nil, errors.New("delete error")
	}
	withMockDeps(t, standardLoader, func(awssdk.Config) API { return client })

	output, err := executeCommand(t, "--output", "json", "--no-confirm", "efs", "delete-filesystems")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(output, "failed:") {
		t.Fatalf("expected failed action in output: %s", output)
	}
}

func TestDeleteFilesystemsDeleteMountTargetError(t *testing.T) {
	client := newStandardMockClient()
	client.describeMountTargetsFn = func(_ context.Context, _ *efs.DescribeMountTargetsInput, _ ...func(*efs.Options)) (*efs.DescribeMountTargetsOutput, error) {
		return &efs.DescribeMountTargetsOutput{
			MountTargets: []efstypes.MountTargetDescription{{MountTargetId: cliutil.Ptr("mt-1")}},
		}, nil
	}
	client.deleteMountTargetFn = func(_ context.Context, _ *efs.DeleteMountTargetInput, _ ...func(*efs.Options)) (*efs.DeleteMountTargetOutput, error) {
		return nil, errors.New("mount delete error")
	}
	withMockDeps(t, standardLoader, func(awssdk.Config) API { return client })

	output, err := executeCommand(t, "--output", "json", "--no-confirm", "efs", "delete-filesystems")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(output, "failed:") {
		t.Fatalf("expected failed action in output: %s", output)
	}
}

func TestDeleteFilesystemsMountTargetListError(t *testing.T) {
	client := &mockClient{
		describeFileSystemsFn: func(_ context.Context, _ *efs.DescribeFileSystemsInput, _ ...func(*efs.Options)) (*efs.DescribeFileSystemsOutput, error) {
			return &efs.DescribeFileSystemsOutput{
				FileSystems: []efstypes.FileSystemDescription{
					{FileSystemId: cliutil.Ptr("fs-123")},
				},
			}, nil
		},
		describeMountTargetsFn: func(_ context.Context, _ *efs.DescribeMountTargetsInput, _ ...func(*efs.Options)) (*efs.DescribeMountTargetsOutput, error) {
			return nil, errors.New("mount targets error")
		},
	}
	withMockDeps(t, standardLoader, func(awssdk.Config) API { return client })

	_, err := executeCommand(t, "--output", "json", "efs", "delete-filesystems")
	if err == nil {
		t.Fatal("expected error from mount target listing failure")
	}
	if !strings.Contains(err.Error(), "list mount targets") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestListFileSystemsPagination(t *testing.T) {
	callCount := 0
	client := &mockClient{
		describeFileSystemsFn: func(_ context.Context, in *efs.DescribeFileSystemsInput, _ ...func(*efs.Options)) (*efs.DescribeFileSystemsOutput, error) {
			callCount++
			if callCount == 1 {
				return &efs.DescribeFileSystemsOutput{
					FileSystems: []efstypes.FileSystemDescription{{FileSystemId: cliutil.Ptr("fs-page1")}},
					NextMarker:  cliutil.Ptr("marker1"),
				}, nil
			}
			return &efs.DescribeFileSystemsOutput{
				FileSystems: []efstypes.FileSystemDescription{{FileSystemId: cliutil.Ptr("fs-page2")}},
			}, nil
		},
	}

	fss, err := listFileSystems(context.Background(), client)
	if err != nil {
		t.Fatalf("listFileSystems: %v", err)
	}
	if len(fss) != 2 {
		t.Fatalf("expected 2 file systems from pagination, got %d", len(fss))
	}
	if callCount != 2 {
		t.Fatalf("expected 2 DescribeFileSystems calls, got %d", callCount)
	}
}

func TestFileSystemMatchesTagEdgeCases(t *testing.T) {
	t.Run("no tags", func(t *testing.T) {
		client := &mockClient{
			listTagsForResourceFn: func(_ context.Context, _ *efs.ListTagsForResourceInput, _ ...func(*efs.Options)) (*efs.ListTagsForResourceOutput, error) {
				return &efs.ListTagsForResourceOutput{Tags: []efstypes.Tag{}}, nil
			},
		}
		match, err := fileSystemMatchesTag(context.Background(), client, "fs-1", "env", "dev")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if match {
			t.Fatal("expected no match with empty tags")
		}
	})

	t.Run("tag matches", func(t *testing.T) {
		client := &mockClient{
			listTagsForResourceFn: func(_ context.Context, _ *efs.ListTagsForResourceInput, _ ...func(*efs.Options)) (*efs.ListTagsForResourceOutput, error) {
				return &efs.ListTagsForResourceOutput{Tags: []efstypes.Tag{
					{Key: cliutil.Ptr("env"), Value: cliutil.Ptr("dev")},
				}}, nil
			},
		}
		match, err := fileSystemMatchesTag(context.Background(), client, "fs-1", "env", "dev")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !match {
			t.Fatal("expected match")
		}
	})

	t.Run("value mismatch", func(t *testing.T) {
		client := &mockClient{
			listTagsForResourceFn: func(_ context.Context, _ *efs.ListTagsForResourceInput, _ ...func(*efs.Options)) (*efs.ListTagsForResourceOutput, error) {
				return &efs.ListTagsForResourceOutput{Tags: []efstypes.Tag{
					{Key: cliutil.Ptr("env"), Value: cliutil.Ptr("prod")},
				}}, nil
			},
		}
		match, err := fileSystemMatchesTag(context.Background(), client, "fs-1", "env", "dev")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if match {
			t.Fatal("expected no match when value differs")
		}
	})

	t.Run("nil key/value pointers", func(t *testing.T) {
		client := &mockClient{
			listTagsForResourceFn: func(_ context.Context, _ *efs.ListTagsForResourceInput, _ ...func(*efs.Options)) (*efs.ListTagsForResourceOutput, error) {
				return &efs.ListTagsForResourceOutput{Tags: []efstypes.Tag{
					{Key: nil, Value: nil},
				}}, nil
			},
		}
		match, err := fileSystemMatchesTag(context.Background(), client, "fs-1", "env", "dev")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if match {
			t.Fatal("expected no match with nil pointers")
		}
	})

	t.Run("list tags error", func(t *testing.T) {
		client := &mockClient{
			listTagsForResourceFn: func(_ context.Context, _ *efs.ListTagsForResourceInput, _ ...func(*efs.Options)) (*efs.ListTagsForResourceOutput, error) {
				return nil, errors.New("tags error")
			},
		}
		_, err := fileSystemMatchesTag(context.Background(), client, "fs-1", "env", "dev")
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestWaitForMountTargetsDeletedImmediate(t *testing.T) {
	mockSleep(t)

	client := &mockClient{
		describeMountTargetsFn: func(_ context.Context, _ *efs.DescribeMountTargetsInput, _ ...func(*efs.Options)) (*efs.DescribeMountTargetsOutput, error) {
			return &efs.DescribeMountTargetsOutput{}, nil
		},
	}

	err := waitForMountTargetsDeleted(context.Background(), client, "fs-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWaitForMountTargetsDeletedListError(t *testing.T) {
	mockSleep(t)

	client := &mockClient{
		describeMountTargetsFn: func(_ context.Context, _ *efs.DescribeMountTargetsInput, _ ...func(*efs.Options)) (*efs.DescribeMountTargetsOutput, error) {
			return nil, errors.New("list error")
		},
	}

	err := waitForMountTargetsDeleted(context.Background(), client, "fs-1")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestWaitForMountTargetsDeletedContextCancelled(t *testing.T) {
	mockSleep(t)

	ctx, cancel := context.WithCancel(context.Background())
	callCount := 0

	client := &mockClient{
		describeMountTargetsFn: func(_ context.Context, _ *efs.DescribeMountTargetsInput, _ ...func(*efs.Options)) (*efs.DescribeMountTargetsOutput, error) {
			callCount++
			if callCount == 2 {
				cancel()
			}
			return &efs.DescribeMountTargetsOutput{
				MountTargets: []efstypes.MountTargetDescription{{MountTargetId: cliutil.Ptr("mt-1")}},
			}, nil
		},
	}

	err := waitForMountTargetsDeleted(ctx, client, "fs-1")
	if err == nil {
		t.Fatal("expected context cancellation error")
	}
}

func TestDeleteFilesystemsWaitError(t *testing.T) {
	mockSleep(t)

	describeCalls := 0
	client := &mockClient{
		describeFileSystemsFn: func(_ context.Context, _ *efs.DescribeFileSystemsInput, _ ...func(*efs.Options)) (*efs.DescribeFileSystemsOutput, error) {
			return &efs.DescribeFileSystemsOutput{
				FileSystems: []efstypes.FileSystemDescription{{FileSystemId: cliutil.Ptr("fs-123")}},
			}, nil
		},
		describeMountTargetsFn: func(_ context.Context, _ *efs.DescribeMountTargetsInput, _ ...func(*efs.Options)) (*efs.DescribeMountTargetsOutput, error) {
			describeCalls++
			// First call is during target building (returns mount targets)
			if describeCalls == 1 {
				return &efs.DescribeMountTargetsOutput{
					MountTargets: []efstypes.MountTargetDescription{{MountTargetId: cliutil.Ptr("mt-1")}},
				}, nil
			}
			// During wait, return an error
			return nil, errors.New("wait list error")
		},
		deleteMountTargetFn: func(_ context.Context, _ *efs.DeleteMountTargetInput, _ ...func(*efs.Options)) (*efs.DeleteMountTargetOutput, error) {
			return &efs.DeleteMountTargetOutput{}, nil
		},
		deleteFileSystemFn: func(_ context.Context, _ *efs.DeleteFileSystemInput, _ ...func(*efs.Options)) (*efs.DeleteFileSystemOutput, error) {
			return &efs.DeleteFileSystemOutput{}, nil
		},
	}
	withMockDeps(t, standardLoader, func(awssdk.Config) API { return client })

	output, err := executeCommand(t, "--output", "json", "--no-confirm", "efs", "delete-filesystems")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(output, "failed:") {
		t.Fatalf("expected failed action: %s", output)
	}
}

func TestDeleteFilesystemsNilFileSystemID(t *testing.T) {
	client := &mockClient{
		describeFileSystemsFn: func(_ context.Context, _ *efs.DescribeFileSystemsInput, _ ...func(*efs.Options)) (*efs.DescribeFileSystemsOutput, error) {
			return &efs.DescribeFileSystemsOutput{
				FileSystems: []efstypes.FileSystemDescription{
					{FileSystemId: nil},
					{FileSystemId: cliutil.Ptr("")},
				},
			}, nil
		},
	}
	withMockDeps(t, standardLoader, func(awssdk.Config) API { return client })

	_, err := executeCommand(t, "--output", "json", "--dry-run", "efs", "delete-filesystems")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
}

func TestNewCommand(t *testing.T) {
	cmd := NewCommand()
	if cmd.Use != "efs" {
		t.Fatalf("expected 'efs' use, got %q", cmd.Use)
	}
	if !cmd.HasSubCommands() {
		t.Fatal("expected sub-commands")
	}
}

func TestDeleteFilesystemsMultipleMountTargets(t *testing.T) {
	mockSleep(t)

	deletedMounts := make(map[string]bool)
	client := &mockClient{
		describeFileSystemsFn: func(_ context.Context, _ *efs.DescribeFileSystemsInput, _ ...func(*efs.Options)) (*efs.DescribeFileSystemsOutput, error) {
			return &efs.DescribeFileSystemsOutput{
				FileSystems: []efstypes.FileSystemDescription{{FileSystemId: cliutil.Ptr("fs-multi")}},
			}, nil
		},
		describeMountTargetsFn: func(_ context.Context, _ *efs.DescribeMountTargetsInput, _ ...func(*efs.Options)) (*efs.DescribeMountTargetsOutput, error) {
			// If all mount targets deleted, return empty
			if len(deletedMounts) >= 2 {
				return &efs.DescribeMountTargetsOutput{}, nil
			}
			return &efs.DescribeMountTargetsOutput{
				MountTargets: []efstypes.MountTargetDescription{
					{MountTargetId: cliutil.Ptr("mt-a")},
					{MountTargetId: cliutil.Ptr("mt-b")},
				},
			}, nil
		},
		deleteMountTargetFn: func(_ context.Context, in *efs.DeleteMountTargetInput, _ ...func(*efs.Options)) (*efs.DeleteMountTargetOutput, error) {
			deletedMounts[cliutil.PointerToString(in.MountTargetId)] = true
			return &efs.DeleteMountTargetOutput{}, nil
		},
		deleteFileSystemFn: func(_ context.Context, _ *efs.DeleteFileSystemInput, _ ...func(*efs.Options)) (*efs.DeleteFileSystemOutput, error) {
			return &efs.DeleteFileSystemOutput{}, nil
		},
	}
	withMockDeps(t, standardLoader, func(awssdk.Config) API { return client })

	output, err := executeCommand(t, "--output", "json", "--no-confirm", "efs", "delete-filesystems")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(output, "\"action\": \"deleted\"") {
		t.Fatalf("expected deleted: %s", output)
	}
	if len(deletedMounts) != 2 {
		t.Fatalf("expected 2 mount targets deleted, got %d", len(deletedMounts))
	}
}
