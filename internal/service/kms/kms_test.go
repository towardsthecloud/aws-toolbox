package kms

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	kmstypes "github.com/aws/aws-sdk-go-v2/service/kms/types"
	"github.com/towardsthecloud/aws-toolbox/internal/cliutil"
)

type mockClient struct {
	describeKeyFn         func(context.Context, *kms.DescribeKeyInput, ...func(*kms.Options)) (*kms.DescribeKeyOutput, error)
	listKeysFn            func(context.Context, *kms.ListKeysInput, ...func(*kms.Options)) (*kms.ListKeysOutput, error)
	listResourceTagsFn    func(context.Context, *kms.ListResourceTagsInput, ...func(*kms.Options)) (*kms.ListResourceTagsOutput, error)
	scheduleKeyDeletionFn func(context.Context, *kms.ScheduleKeyDeletionInput, ...func(*kms.Options)) (*kms.ScheduleKeyDeletionOutput, error)
}

func (m *mockClient) DescribeKey(ctx context.Context, in *kms.DescribeKeyInput, optFns ...func(*kms.Options)) (*kms.DescribeKeyOutput, error) {
	if m.describeKeyFn == nil {
		return nil, errors.New("DescribeKey not mocked")
	}
	return m.describeKeyFn(ctx, in, optFns...)
}

func (m *mockClient) ListKeys(ctx context.Context, in *kms.ListKeysInput, optFns ...func(*kms.Options)) (*kms.ListKeysOutput, error) {
	if m.listKeysFn == nil {
		return nil, errors.New("ListKeys not mocked")
	}
	return m.listKeysFn(ctx, in, optFns...)
}

func (m *mockClient) ListResourceTags(ctx context.Context, in *kms.ListResourceTagsInput, optFns ...func(*kms.Options)) (*kms.ListResourceTagsOutput, error) {
	if m.listResourceTagsFn == nil {
		return nil, errors.New("ListResourceTags not mocked")
	}
	return m.listResourceTagsFn(ctx, in, optFns...)
}

func (m *mockClient) ScheduleKeyDeletion(ctx context.Context, in *kms.ScheduleKeyDeletionInput, optFns ...func(*kms.Options)) (*kms.ScheduleKeyDeletionOutput, error) {
	if m.scheduleKeyDeletionFn == nil {
		return nil, errors.New("ScheduleKeyDeletion not mocked")
	}
	return m.scheduleKeyDeletionFn(ctx, in, optFns...)
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

// newStandardMockClient returns a mock with a single customer-managed disabled key.
func newStandardMockClient() *mockClient {
	return &mockClient{
		listKeysFn: func(_ context.Context, _ *kms.ListKeysInput, _ ...func(*kms.Options)) (*kms.ListKeysOutput, error) {
			return &kms.ListKeysOutput{Keys: []kmstypes.KeyListEntry{{KeyId: cliutil.Ptr("key-1")}}}, nil
		},
		describeKeyFn: func(_ context.Context, in *kms.DescribeKeyInput, _ ...func(*kms.Options)) (*kms.DescribeKeyOutput, error) {
			return &kms.DescribeKeyOutput{KeyMetadata: &kmstypes.KeyMetadata{
				KeyId:      in.KeyId,
				KeyManager: kmstypes.KeyManagerTypeCustomer,
				KeyState:   kmstypes.KeyStateDisabled,
			}}, nil
		},
		listResourceTagsFn: func(_ context.Context, _ *kms.ListResourceTagsInput, _ ...func(*kms.Options)) (*kms.ListResourceTagsOutput, error) {
			return &kms.ListResourceTagsOutput{Tags: []kmstypes.Tag{
				{TagKey: cliutil.Ptr("env"), TagValue: cliutil.Ptr("dev")},
			}}, nil
		},
		scheduleKeyDeletionFn: func(_ context.Context, _ *kms.ScheduleKeyDeletionInput, _ ...func(*kms.Options)) (*kms.ScheduleKeyDeletionOutput, error) {
			return &kms.ScheduleKeyDeletionOutput{}, nil
		},
	}
}

func standardLoader(_, _ string) (awssdk.Config, error) {
	return awssdk.Config{Region: "us-east-1"}, nil
}

func TestDeleteKeysDryRun(t *testing.T) {
	scheduled := 0
	client := &mockClient{
		listKeysFn: func(_ context.Context, _ *kms.ListKeysInput, _ ...func(*kms.Options)) (*kms.ListKeysOutput, error) {
			return &kms.ListKeysOutput{Keys: []kmstypes.KeyListEntry{{KeyId: cliutil.Ptr("key-1")}}}, nil
		},
		describeKeyFn: func(_ context.Context, in *kms.DescribeKeyInput, _ ...func(*kms.Options)) (*kms.DescribeKeyOutput, error) {
			if cliutil.PointerToString(in.KeyId) != "key-1" {
				t.Fatalf("unexpected key id: %s", cliutil.PointerToString(in.KeyId))
			}
			return &kms.DescribeKeyOutput{KeyMetadata: &kmstypes.KeyMetadata{KeyId: cliutil.Ptr("key-1"), KeyManager: kmstypes.KeyManagerTypeCustomer, KeyState: kmstypes.KeyStateDisabled}}, nil
		},
		scheduleKeyDeletionFn: func(_ context.Context, _ *kms.ScheduleKeyDeletionInput, _ ...func(*kms.Options)) (*kms.ScheduleKeyDeletionOutput, error) {
			scheduled++
			return &kms.ScheduleKeyDeletionOutput{}, nil
		},
	}

	withMockDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) API { return client },
	)

	output, err := executeCommand(t, "--output", "json", "--dry-run", "kms", "delete-keys", "--unused")
	if err != nil {
		t.Fatalf("execute kms delete-keys --unused --dry-run: %v", err)
	}

	if scheduled != 0 {
		t.Fatalf("expected 0 scheduled deletions in dry-run, got %d", scheduled)
	}
	if !strings.Contains(output, "would-delete") || !strings.Contains(output, "key-1") {
		t.Fatalf("unexpected output: %s", output)
	}
}

func TestDeleteKeysNoConfirm(t *testing.T) {
	client := newStandardMockClient()
	withMockDeps(t, standardLoader, func(awssdk.Config) API { return client })

	output, err := executeCommand(t, "--output", "json", "--no-confirm", "kms", "delete-keys", "--unused")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(output, "\"action\": \"deleted\"") {
		t.Fatalf("expected deleted action, got: %s", output)
	}
}

func TestDeleteKeysUserDeclinesConfirmation(t *testing.T) {
	client := newStandardMockClient()
	withMockDeps(t, standardLoader, func(awssdk.Config) API { return client })

	root := cliutil.NewTestRootCommand(NewCommand())
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetIn(strings.NewReader("n\n"))
	root.SetArgs([]string{"--output", "json", "kms", "delete-keys", "--unused"})

	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(buf.String(), "cancelled") {
		t.Fatalf("expected cancelled, got: %s", buf.String())
	}
}

func TestDeleteKeysNoFlagSet(t *testing.T) {
	client := newStandardMockClient()
	withMockDeps(t, standardLoader, func(awssdk.Config) API { return client })

	_, err := executeCommand(t, "--output", "json", "kms", "delete-keys")
	if err == nil {
		t.Fatal("expected error when neither --filter-tag nor --unused is set")
	}
	if !strings.Contains(err.Error(), "set one of") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeleteKeysBothFlagsSet(t *testing.T) {
	client := newStandardMockClient()
	withMockDeps(t, standardLoader, func(awssdk.Config) API { return client })

	_, err := executeCommand(t, "--output", "json", "kms", "delete-keys", "--filter-tag", "env=dev", "--unused")
	if err == nil {
		t.Fatal("expected error when both --filter-tag and --unused are set")
	}
	if !strings.Contains(err.Error(), "not both") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeleteKeysPendingDaysOutOfRange(t *testing.T) {
	client := newStandardMockClient()
	withMockDeps(t, standardLoader, func(awssdk.Config) API { return client })

	_, err := executeCommand(t, "--output", "json", "kms", "delete-keys", "--unused", "--pending-days", "3")
	if err == nil {
		t.Fatal("expected error for pending-days < 7")
	}
	if !strings.Contains(err.Error(), "--pending-days must be between 7 and 30") {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = executeCommand(t, "--output", "json", "kms", "delete-keys", "--unused", "--pending-days", "50")
	if err == nil {
		t.Fatal("expected error for pending-days > 30")
	}
}

func TestDeleteKeysInvalidTagFilter(t *testing.T) {
	client := newStandardMockClient()
	withMockDeps(t, standardLoader, func(awssdk.Config) API { return client })

	_, err := executeCommand(t, "--output", "json", "kms", "delete-keys", "--filter-tag", "invalid")
	if err == nil {
		t.Fatal("expected error for invalid tag filter")
	}
}

func TestDeleteKeysFilterTagMatch(t *testing.T) {
	client := newStandardMockClient()
	withMockDeps(t, standardLoader, func(awssdk.Config) API { return client })

	output, err := executeCommand(t, "--output", "json", "--dry-run", "kms", "delete-keys", "--filter-tag", "env=dev")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(output, "key-1") {
		t.Fatalf("expected key-1 in output: %s", output)
	}
	if !strings.Contains(output, "tag:env=dev") {
		t.Fatalf("expected mode tag:env=dev in output: %s", output)
	}
}

func TestDeleteKeysFilterTagNoMatch(t *testing.T) {
	client := newStandardMockClient()
	withMockDeps(t, standardLoader, func(awssdk.Config) API { return client })

	output, err := executeCommand(t, "--output", "json", "--dry-run", "kms", "delete-keys", "--filter-tag", "env=prod")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if strings.Contains(output, "key-1") {
		t.Fatalf("expected no key-1 in output (tag mismatch): %s", output)
	}
}

func TestDeleteKeysListKeysError(t *testing.T) {
	client := &mockClient{
		listKeysFn: func(_ context.Context, _ *kms.ListKeysInput, _ ...func(*kms.Options)) (*kms.ListKeysOutput, error) {
			return nil, errors.New("access denied")
		},
	}
	withMockDeps(t, standardLoader, func(awssdk.Config) API { return client })

	_, err := executeCommand(t, "--output", "json", "kms", "delete-keys", "--unused")
	if err == nil {
		t.Fatal("expected error from ListKeys failure")
	}
	if !strings.Contains(err.Error(), "list KMS keys") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeleteKeysScheduleDeletionError(t *testing.T) {
	client := newStandardMockClient()
	client.scheduleKeyDeletionFn = func(_ context.Context, _ *kms.ScheduleKeyDeletionInput, _ ...func(*kms.Options)) (*kms.ScheduleKeyDeletionOutput, error) {
		return nil, errors.New("deletion failed")
	}
	withMockDeps(t, standardLoader, func(awssdk.Config) API { return client })

	output, err := executeCommand(t, "--output", "json", "--no-confirm", "kms", "delete-keys", "--unused")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(output, "failed:") {
		t.Fatalf("expected failed action in output: %s", output)
	}
}

func TestDeleteKeysAWSConfigLoadError(t *testing.T) {
	withMockDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{}, errors.New("config error") },
		func(awssdk.Config) API { return &mockClient{} },
	)

	_, err := executeCommand(t, "--output", "json", "kms", "delete-keys", "--unused")
	if err == nil {
		t.Fatal("expected error from config load failure")
	}
}

func TestDeleteKeysTagListError(t *testing.T) {
	client := newStandardMockClient()
	client.listResourceTagsFn = func(_ context.Context, _ *kms.ListResourceTagsInput, _ ...func(*kms.Options)) (*kms.ListResourceTagsOutput, error) {
		return nil, errors.New("tags error")
	}
	withMockDeps(t, standardLoader, func(awssdk.Config) API { return client })

	_, err := executeCommand(t, "--output", "json", "kms", "delete-keys", "--filter-tag", "env=dev")
	if err == nil {
		t.Fatal("expected error from ListResourceTags failure")
	}
	if !strings.Contains(err.Error(), "list tags for key") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestListCustomerManagedKeysVariousStates(t *testing.T) {
	keyData := map[string]*kmstypes.KeyMetadata{
		"key-customer-enabled": {
			KeyId: cliutil.Ptr("key-customer-enabled"), KeyManager: kmstypes.KeyManagerTypeCustomer, KeyState: kmstypes.KeyStateEnabled,
		},
		"key-customer-disabled": {
			KeyId: cliutil.Ptr("key-customer-disabled"), KeyManager: kmstypes.KeyManagerTypeCustomer, KeyState: kmstypes.KeyStateDisabled,
		},
		"key-aws-managed": {
			KeyId: cliutil.Ptr("key-aws-managed"), KeyManager: kmstypes.KeyManagerTypeAws, KeyState: kmstypes.KeyStateEnabled,
		},
		"key-pending-deletion": {
			KeyId: cliutil.Ptr("key-pending-deletion"), KeyManager: kmstypes.KeyManagerTypeCustomer, KeyState: kmstypes.KeyStatePendingDeletion,
		},
		"key-nil-metadata": nil,
	}

	client := &mockClient{
		listKeysFn: func(_ context.Context, _ *kms.ListKeysInput, _ ...func(*kms.Options)) (*kms.ListKeysOutput, error) {
			keys := []kmstypes.KeyListEntry{
				{KeyId: cliutil.Ptr("key-customer-enabled")},
				{KeyId: cliutil.Ptr("key-customer-disabled")},
				{KeyId: cliutil.Ptr("key-aws-managed")},
				{KeyId: cliutil.Ptr("key-pending-deletion")},
				{KeyId: cliutil.Ptr("key-nil-metadata")},
				{KeyId: nil}, // nil key ID - should be skipped
			}
			return &kms.ListKeysOutput{Keys: keys}, nil
		},
		describeKeyFn: func(_ context.Context, in *kms.DescribeKeyInput, _ ...func(*kms.Options)) (*kms.DescribeKeyOutput, error) {
			keyID := cliutil.PointerToString(in.KeyId)
			meta, ok := keyData[keyID]
			if !ok {
				return nil, errors.New("key not found")
			}
			return &kms.DescribeKeyOutput{KeyMetadata: meta}, nil
		},
	}

	keys, err := listCustomerManagedKeys(context.Background(), client)
	if err != nil {
		t.Fatalf("listCustomerManagedKeys: %v", err)
	}

	// Should only include customer-managed keys NOT pending deletion
	if len(keys) != 2 {
		t.Fatalf("expected 2 customer-managed keys, got %d", len(keys))
	}
	ids := make(map[string]bool)
	for _, k := range keys {
		ids[cliutil.PointerToString(k.KeyId)] = true
	}
	if !ids["key-customer-enabled"] || !ids["key-customer-disabled"] {
		t.Fatalf("unexpected key IDs: %v", ids)
	}
}

func TestListCustomerManagedKeysDescribeError(t *testing.T) {
	client := &mockClient{
		listKeysFn: func(_ context.Context, _ *kms.ListKeysInput, _ ...func(*kms.Options)) (*kms.ListKeysOutput, error) {
			return &kms.ListKeysOutput{Keys: []kmstypes.KeyListEntry{{KeyId: cliutil.Ptr("key-1")}}}, nil
		},
		describeKeyFn: func(_ context.Context, _ *kms.DescribeKeyInput, _ ...func(*kms.Options)) (*kms.DescribeKeyOutput, error) {
			return nil, errors.New("describe error")
		},
	}

	_, err := listCustomerManagedKeys(context.Background(), client)
	if err == nil {
		t.Fatal("expected error from DescribeKey failure")
	}
}

func TestKeyMatchesTagEdgeCases(t *testing.T) {
	t.Run("no tags", func(t *testing.T) {
		client := &mockClient{
			listResourceTagsFn: func(_ context.Context, _ *kms.ListResourceTagsInput, _ ...func(*kms.Options)) (*kms.ListResourceTagsOutput, error) {
				return &kms.ListResourceTagsOutput{Tags: []kmstypes.Tag{}}, nil
			},
		}
		key := kmstypes.KeyMetadata{KeyId: cliutil.Ptr("key-1")}
		match, err := keyMatchesTag(context.Background(), client, key, "env", "dev")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if match {
			t.Fatal("expected no match with empty tags")
		}
	})

	t.Run("key matches", func(t *testing.T) {
		client := &mockClient{
			listResourceTagsFn: func(_ context.Context, _ *kms.ListResourceTagsInput, _ ...func(*kms.Options)) (*kms.ListResourceTagsOutput, error) {
				return &kms.ListResourceTagsOutput{Tags: []kmstypes.Tag{
					{TagKey: cliutil.Ptr("env"), TagValue: cliutil.Ptr("dev")},
				}}, nil
			},
		}
		key := kmstypes.KeyMetadata{KeyId: cliutil.Ptr("key-1")}
		match, err := keyMatchesTag(context.Background(), client, key, "env", "dev")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !match {
			t.Fatal("expected match")
		}
	})

	t.Run("key matches but value differs", func(t *testing.T) {
		client := &mockClient{
			listResourceTagsFn: func(_ context.Context, _ *kms.ListResourceTagsInput, _ ...func(*kms.Options)) (*kms.ListResourceTagsOutput, error) {
				return &kms.ListResourceTagsOutput{Tags: []kmstypes.Tag{
					{TagKey: cliutil.Ptr("env"), TagValue: cliutil.Ptr("prod")},
				}}, nil
			},
		}
		key := kmstypes.KeyMetadata{KeyId: cliutil.Ptr("key-1")}
		match, err := keyMatchesTag(context.Background(), client, key, "env", "dev")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if match {
			t.Fatal("expected no match when value differs")
		}
	})

	t.Run("nil tag key/value pointers", func(t *testing.T) {
		client := &mockClient{
			listResourceTagsFn: func(_ context.Context, _ *kms.ListResourceTagsInput, _ ...func(*kms.Options)) (*kms.ListResourceTagsOutput, error) {
				return &kms.ListResourceTagsOutput{Tags: []kmstypes.Tag{
					{TagKey: nil, TagValue: nil},
				}}, nil
			},
		}
		key := kmstypes.KeyMetadata{KeyId: cliutil.Ptr("key-1")}
		match, err := keyMatchesTag(context.Background(), client, key, "env", "dev")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if match {
			t.Fatal("expected no match with nil tag pointers")
		}
	})

	t.Run("ListResourceTags error", func(t *testing.T) {
		client := &mockClient{
			listResourceTagsFn: func(_ context.Context, _ *kms.ListResourceTagsInput, _ ...func(*kms.Options)) (*kms.ListResourceTagsOutput, error) {
				return nil, errors.New("tag list error")
			},
		}
		key := kmstypes.KeyMetadata{KeyId: cliutil.Ptr("key-1")}
		_, err := keyMatchesTag(context.Background(), client, key, "env", "dev")
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestDeleteKeysUnusedFiltersEnabledKeys(t *testing.T) {
	// With --unused, enabled keys should be excluded
	client := &mockClient{
		listKeysFn: func(_ context.Context, _ *kms.ListKeysInput, _ ...func(*kms.Options)) (*kms.ListKeysOutput, error) {
			return &kms.ListKeysOutput{Keys: []kmstypes.KeyListEntry{
				{KeyId: cliutil.Ptr("key-enabled")},
				{KeyId: cliutil.Ptr("key-disabled")},
			}}, nil
		},
		describeKeyFn: func(_ context.Context, in *kms.DescribeKeyInput, _ ...func(*kms.Options)) (*kms.DescribeKeyOutput, error) {
			keyID := cliutil.PointerToString(in.KeyId)
			state := kmstypes.KeyStateEnabled
			if keyID == "key-disabled" {
				state = kmstypes.KeyStateDisabled
			}
			return &kms.DescribeKeyOutput{KeyMetadata: &kmstypes.KeyMetadata{
				KeyId: in.KeyId, KeyManager: kmstypes.KeyManagerTypeCustomer, KeyState: state,
			}}, nil
		},
	}
	withMockDeps(t, standardLoader, func(awssdk.Config) API { return client })

	output, err := executeCommand(t, "--output", "json", "--dry-run", "kms", "delete-keys", "--unused")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if strings.Contains(output, "key-enabled") {
		t.Fatalf("enabled key should be excluded with --unused: %s", output)
	}
	if !strings.Contains(output, "key-disabled") {
		t.Fatalf("disabled key should be included with --unused: %s", output)
	}
}

func TestDeleteKeysEmptyResults(t *testing.T) {
	client := &mockClient{
		listKeysFn: func(_ context.Context, _ *kms.ListKeysInput, _ ...func(*kms.Options)) (*kms.ListKeysOutput, error) {
			return &kms.ListKeysOutput{Keys: []kmstypes.KeyListEntry{}}, nil
		},
	}
	withMockDeps(t, standardLoader, func(awssdk.Config) API { return client })

	_, err := executeCommand(t, "--output", "json", "--no-confirm", "kms", "delete-keys", "--unused")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
}

func TestDeleteKeysFilterTagWithNoConfirm(t *testing.T) {
	deleted := 0
	client := newStandardMockClient()
	client.scheduleKeyDeletionFn = func(_ context.Context, _ *kms.ScheduleKeyDeletionInput, _ ...func(*kms.Options)) (*kms.ScheduleKeyDeletionOutput, error) {
		deleted++
		return &kms.ScheduleKeyDeletionOutput{}, nil
	}
	withMockDeps(t, standardLoader, func(awssdk.Config) API { return client })

	output, err := executeCommand(t, "--output", "json", "--no-confirm", "kms", "delete-keys", "--filter-tag", "env=dev")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if deleted != 1 {
		t.Fatalf("expected 1 deletion, got %d", deleted)
	}
	if !strings.Contains(output, "\"action\": \"deleted\"") {
		t.Fatalf("expected deleted action: %s", output)
	}
}

func TestNewCommand(t *testing.T) {
	cmd := NewCommand()
	if cmd.Use != "kms" {
		t.Fatalf("expected 'kms' use, got %q", cmd.Use)
	}
	if !cmd.HasSubCommands() {
		t.Fatal("expected sub-commands")
	}
}

func TestListCustomerManagedKeysPagination(t *testing.T) {
	callCount := 0
	client := &mockClient{
		listKeysFn: func(_ context.Context, in *kms.ListKeysInput, _ ...func(*kms.Options)) (*kms.ListKeysOutput, error) {
			callCount++
			if callCount == 1 {
				return &kms.ListKeysOutput{
					Keys:       []kmstypes.KeyListEntry{{KeyId: cliutil.Ptr("key-page1")}},
					Truncated:  true,
					NextMarker: cliutil.Ptr("marker1"),
				}, nil
			}
			return &kms.ListKeysOutput{
				Keys: []kmstypes.KeyListEntry{{KeyId: cliutil.Ptr("key-page2")}},
			}, nil
		},
		describeKeyFn: func(_ context.Context, in *kms.DescribeKeyInput, _ ...func(*kms.Options)) (*kms.DescribeKeyOutput, error) {
			return &kms.DescribeKeyOutput{KeyMetadata: &kmstypes.KeyMetadata{
				KeyId: in.KeyId, KeyManager: kmstypes.KeyManagerTypeCustomer, KeyState: kmstypes.KeyStateEnabled,
			}}, nil
		},
	}

	keys, err := listCustomerManagedKeys(context.Background(), client)
	if err != nil {
		t.Fatalf("listCustomerManagedKeys: %v", err)
	}
	if len(keys) != 2 {
		t.Fatalf("expected 2 keys from pagination, got %d", len(keys))
	}
	if callCount != 2 {
		t.Fatalf("expected 2 ListKeys calls, got %d", callCount)
	}
}
