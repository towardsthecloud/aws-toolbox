package kms

import (
	"context"
	"fmt"
	"sort"
	"strings"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	kmstypes "github.com/aws/aws-sdk-go-v2/service/kms/types"
	"github.com/spf13/cobra"
	awstbxaws "github.com/towardsthecloud/aws-toolbox/internal/aws"
	"github.com/towardsthecloud/aws-toolbox/internal/cliutil"
)

// API is the subset of the KMS client used by this package.
type API interface {
	DescribeKey(context.Context, *kms.DescribeKeyInput, ...func(*kms.Options)) (*kms.DescribeKeyOutput, error)
	ListKeys(context.Context, *kms.ListKeysInput, ...func(*kms.Options)) (*kms.ListKeysOutput, error)
	ListResourceTags(context.Context, *kms.ListResourceTagsInput, ...func(*kms.Options)) (*kms.ListResourceTagsOutput, error)
	ScheduleKeyDeletion(context.Context, *kms.ScheduleKeyDeletionInput, ...func(*kms.Options)) (*kms.ScheduleKeyDeletionOutput, error)
}

var loadAWSConfig = awstbxaws.LoadAWSConfig
var newClient = func(cfg awssdk.Config) API {
	return kms.NewFromConfig(cfg)
}

// NewCommand returns the kms service group command.
func NewCommand() *cobra.Command {
	cmd := cliutil.NewServiceGroupCommand("kms", "Manage KMS resources")
	cmd.AddCommand(newDeleteKeysCommand())
	return cmd
}

func newDeleteKeysCommand() *cobra.Command {
	var tagFilter string
	var unusedOnly bool
	var pendingDays int

	cmd := &cobra.Command{
		Use:   "delete-keys",
		Short: "Schedule KMS key deletion by tag or unused mode",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runDeleteKeys(cmd, tagFilter, unusedOnly, pendingDays)
		},
		SilenceUsage: true,
	}
	cmd.Flags().StringVar(&tagFilter, "filter-tag", "", "Tag filter in KEY=VALUE form")
	cmd.Flags().BoolVar(&unusedOnly, "unused", false, "Target disabled customer-managed keys")
	cmd.Flags().IntVar(&pendingDays, "pending-days", 7, "Days before deletion (7-30)")

	return cmd
}

func runDeleteKeys(cmd *cobra.Command, tagFilter string, unusedOnly bool, pendingDays int) error {
	if pendingDays < 7 || pendingDays > 30 {
		return fmt.Errorf("--pending-days must be between 7 and 30")
	}

	modeTag := strings.TrimSpace(tagFilter)
	if modeTag == "" && !unusedOnly {
		return fmt.Errorf("set one of --filter-tag or --unused")
	}
	if modeTag != "" && unusedOnly {
		return fmt.Errorf("set either --filter-tag or --unused, not both")
	}

	tagKey, tagValue, err := cliutil.ParseTagFilter(modeTag)
	if err != nil {
		return err
	}

	runtime, _, client, err := cliutil.NewServiceRuntime(cmd, loadAWSConfig, newClient)
	if err != nil {
		return err
	}

	keys, err := listCustomerManagedKeys(cmd.Context(), client)
	if err != nil {
		return fmt.Errorf("list KMS keys: %s", awstbxaws.FormatUserError(err))
	}

	targets := make([]kmstypes.KeyMetadata, 0, len(keys))
	for _, key := range keys {
		if modeTag != "" {
			match, matchErr := keyMatchesTag(cmd.Context(), client, key, tagKey, tagValue)
			if matchErr != nil {
				return fmt.Errorf("list tags for key %s: %s", cliutil.PointerToString(key.KeyId), awstbxaws.FormatUserError(matchErr))
			}
			if !match {
				continue
			}
		}

		if unusedOnly && key.KeyState != kmstypes.KeyStateDisabled {
			continue
		}

		targets = append(targets, key)
	}

	sort.Slice(targets, func(i, j int) bool {
		return cliutil.PointerToString(targets[i].KeyId) < cliutil.PointerToString(targets[j].KeyId)
	})

	mode := "unused"
	if modeTag != "" {
		mode = "tag:" + tagKey + "=" + tagValue
	}

	rows := make([][]string, 0, len(targets))
	for _, key := range targets {
		action := cliutil.ActionWouldDelete
		if !runtime.Options.DryRun {
			action = cliutil.ActionPending
		}
		rows = append(rows, []string{cliutil.PointerToString(key.KeyId), mode, string(key.KeyState), action})
	}

	if len(targets) == 0 || runtime.Options.DryRun {
		return cliutil.WriteDataset(cmd, runtime, []string{"key_id", "mode", "key_state", "action"}, rows)
	}

	ok, confirmErr := runtime.Prompter.Confirm(
		fmt.Sprintf("Schedule deletion for %d KMS key(s)", len(targets)),
		runtime.Options.NoConfirm,
	)
	if confirmErr != nil {
		return confirmErr
	}
	if !ok {
		for i := range rows {
			rows[i][3] = cliutil.ActionCancelled
		}
		return cliutil.WriteDataset(cmd, runtime, []string{"key_id", "mode", "key_state", "action"}, rows)
	}

	for i, key := range targets {
		_, deleteErr := client.ScheduleKeyDeletion(cmd.Context(), &kms.ScheduleKeyDeletionInput{
			KeyId:               key.KeyId,
			PendingWindowInDays: cliutil.Ptr(int32(pendingDays)),
		})
		if deleteErr != nil {
			rows[i][3] = cliutil.FailedActionMessage(awstbxaws.FormatUserError(deleteErr))
			continue
		}
		rows[i][3] = cliutil.ActionDeleted
	}

	return cliutil.WriteDataset(cmd, runtime, []string{"key_id", "mode", "key_state", "action"}, rows)
}

func listCustomerManagedKeys(ctx context.Context, client API) ([]kmstypes.KeyMetadata, error) {
	items := make([]kmstypes.KeyMetadata, 0)

	keys, err := awstbxaws.CollectAllPages(ctx, func(callCtx context.Context, marker *string) (awstbxaws.PageResult[kmstypes.KeyListEntry], error) {
		page, listErr := client.ListKeys(callCtx, &kms.ListKeysInput{Marker: marker})
		if listErr != nil {
			return awstbxaws.PageResult[kmstypes.KeyListEntry]{}, listErr
		}

		nextToken := page.NextMarker
		if !page.Truncated {
			nextToken = nil
		}

		return awstbxaws.PageResult[kmstypes.KeyListEntry]{
			Items:     page.Keys,
			NextToken: nextToken,
		}, nil
	})
	if err != nil {
		return nil, err
	}

	for _, key := range keys {
		keyID := cliutil.PointerToString(key.KeyId)
		if keyID == "" {
			continue
		}

		describeOut, describeErr := client.DescribeKey(ctx, &kms.DescribeKeyInput{KeyId: cliutil.Ptr(keyID)})
		if describeErr != nil {
			return nil, describeErr
		}
		if describeOut.KeyMetadata == nil {
			continue
		}
		if describeOut.KeyMetadata.KeyManager != kmstypes.KeyManagerTypeCustomer {
			continue
		}
		if describeOut.KeyMetadata.KeyState == kmstypes.KeyStatePendingDeletion {
			continue
		}

		items = append(items, *describeOut.KeyMetadata)
	}

	return items, nil
}

func keyMatchesTag(ctx context.Context, client API, key kmstypes.KeyMetadata, tagKey, tagValue string) (bool, error) {
	tags, err := awstbxaws.CollectAllPages(ctx, func(callCtx context.Context, marker *string) (awstbxaws.PageResult[kmstypes.Tag], error) {
		page, listErr := client.ListResourceTags(callCtx, &kms.ListResourceTagsInput{KeyId: key.KeyId, Marker: marker})
		if listErr != nil {
			return awstbxaws.PageResult[kmstypes.Tag]{}, listErr
		}

		nextToken := page.NextMarker
		if !page.Truncated {
			nextToken = nil
		}

		return awstbxaws.PageResult[kmstypes.Tag]{
			Items:     page.Tags,
			NextToken: nextToken,
		}, nil
	})
	if err != nil {
		return false, err
	}

	for _, tag := range tags {
		if cliutil.PointerToString(tag.TagKey) == tagKey && cliutil.PointerToString(tag.TagValue) == tagValue {
			return true, nil
		}
	}

	return false, nil
}
