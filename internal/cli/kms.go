package cli

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
)

type kmsAPI interface {
	DescribeKey(context.Context, *kms.DescribeKeyInput, ...func(*kms.Options)) (*kms.DescribeKeyOutput, error)
	ListKeys(context.Context, *kms.ListKeysInput, ...func(*kms.Options)) (*kms.ListKeysOutput, error)
	ListResourceTags(context.Context, *kms.ListResourceTagsInput, ...func(*kms.Options)) (*kms.ListResourceTagsOutput, error)
	ScheduleKeyDeletion(context.Context, *kms.ScheduleKeyDeletionInput, ...func(*kms.Options)) (*kms.ScheduleKeyDeletionOutput, error)
}

var kmsLoadAWSConfig = awstbxaws.LoadAWSConfig
var kmsNewClient = func(cfg awssdk.Config) kmsAPI {
	return kms.NewFromConfig(cfg)
}

func newKMSCommand() *cobra.Command {
	cmd := newServiceGroupCommand("kms", "Manage KMS resources")

	cmd.AddCommand(newKMSDeleteKeysCommand())

	return cmd
}

func newKMSDeleteKeysCommand() *cobra.Command {
	var tagFilter string
	var unusedOnly bool
	var pendingDays int

	cmd := &cobra.Command{
		Use:   "delete-keys",
		Short: "Schedule KMS key deletion by tag or unused mode",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runKMSDeleteKeys(cmd, tagFilter, unusedOnly, pendingDays)
		},
		SilenceUsage: true,
	}
	cmd.Flags().StringVar(&tagFilter, "filter-tag", "", "Tag filter in KEY=VALUE form")
	cmd.Flags().BoolVar(&unusedOnly, "unused", false, "Target disabled customer-managed keys")
	cmd.Flags().IntVar(&pendingDays, "pending-days", 7, "Days before deletion (7-30)")

	return cmd
}

func runKMSDeleteKeys(cmd *cobra.Command, tagFilter string, unusedOnly bool, pendingDays int) error {
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

	tagKey, tagValue, err := parseTagFilter(modeTag)
	if err != nil {
		return err
	}

	runtime, _, client, err := newServiceRuntime(cmd, kmsLoadAWSConfig, kmsNewClient)
	if err != nil {
		return err
	}

	keys, err := listCustomerManagedKMSKeys(cmd.Context(), client)
	if err != nil {
		return fmt.Errorf("list KMS keys: %s", awstbxaws.FormatUserError(err))
	}

	targets := make([]kmstypes.KeyMetadata, 0, len(keys))
	for _, key := range keys {
		if modeTag != "" {
			match, matchErr := kmsKeyMatchesTag(cmd.Context(), client, key, tagKey, tagValue)
			if matchErr != nil {
				return fmt.Errorf("list tags for key %s: %s", pointerToString(key.KeyId), awstbxaws.FormatUserError(matchErr))
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
		return pointerToString(targets[i].KeyId) < pointerToString(targets[j].KeyId)
	})

	mode := "unused"
	if modeTag != "" {
		mode = "tag:" + tagKey + "=" + tagValue
	}

	rows := make([][]string, 0, len(targets))
	for _, key := range targets {
		action := actionWouldDelete
		if !runtime.Options.DryRun {
			action = actionPending
		}
		rows = append(rows, []string{pointerToString(key.KeyId), mode, string(key.KeyState), action})
	}

	if len(targets) == 0 || runtime.Options.DryRun {
		return writeDataset(cmd, runtime, []string{"key_id", "mode", "key_state", "action"}, rows)
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
			rows[i][3] = actionCancelled
		}
		return writeDataset(cmd, runtime, []string{"key_id", "mode", "key_state", "action"}, rows)
	}

	for i, key := range targets {
		_, deleteErr := client.ScheduleKeyDeletion(cmd.Context(), &kms.ScheduleKeyDeletionInput{
			KeyId:               key.KeyId,
			PendingWindowInDays: ptr(int32(pendingDays)),
		})
		if deleteErr != nil {
			rows[i][3] = failedActionMessage(awstbxaws.FormatUserError(deleteErr))
			continue
		}
		rows[i][3] = actionDeleted
	}

	return writeDataset(cmd, runtime, []string{"key_id", "mode", "key_state", "action"}, rows)
}

func listCustomerManagedKMSKeys(ctx context.Context, client kmsAPI) ([]kmstypes.KeyMetadata, error) {
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
		keyID := pointerToString(key.KeyId)
		if keyID == "" {
			continue
		}

		describeOut, describeErr := client.DescribeKey(ctx, &kms.DescribeKeyInput{KeyId: ptr(keyID)})
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

func kmsKeyMatchesTag(ctx context.Context, client kmsAPI, key kmstypes.KeyMetadata, tagKey, tagValue string) (bool, error) {
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
		if pointerToString(tag.TagKey) == tagKey && pointerToString(tag.TagValue) == tagValue {
			return true, nil
		}
	}

	return false, nil
}
