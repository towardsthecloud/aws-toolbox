package cloudwatch

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	cloudwatchlogstypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	"github.com/spf13/cobra"
	awstbxaws "github.com/towardsthecloud/aws-toolbox/internal/aws"
	"github.com/towardsthecloud/aws-toolbox/internal/cliutil"
)

func runCountLogGroups(cmd *cobra.Command, _ []string) error {
	runtime, _, client, err := cliutil.NewServiceRuntime(cmd, loadAWSConfig, newClient)
	if err != nil {
		return err
	}

	groups, err := listLogGroups(cmd.Context(), client)
	if err != nil {
		return fmt.Errorf("list log groups: %s", awstbxaws.FormatUserError(err))
	}

	rows := [][]string{{"total_log_groups", fmt.Sprintf("%d", len(groups))}}
	return cliutil.WriteDataset(cmd, runtime, []string{"metric", "value"}, rows)
}

func runListLogGroups(cmd *cobra.Command, _ []string) error {
	runtime, _, client, err := cliutil.NewServiceRuntime(cmd, loadAWSConfig, newClient)
	if err != nil {
		return err
	}

	groups, err := listLogGroups(cmd.Context(), client)
	if err != nil {
		return fmt.Errorf("list log groups: %s", awstbxaws.FormatUserError(err))
	}

	now := time.Now().UTC()
	sort.Slice(groups, func(i, j int) bool {
		left := logGroupCreatedAt(groups[i])
		right := logGroupCreatedAt(groups[j])
		if left.Equal(right) {
			return cliutil.PointerToString(groups[i].LogGroupName) < cliutil.PointerToString(groups[j].LogGroupName)
		}
		return left.After(right)
	})

	rows := make([][]string, 0, len(groups))
	for _, group := range groups {
		createdAt := logGroupCreatedAt(group)
		ageDays := 0
		createdAtText := "unknown"
		if !createdAt.IsZero() {
			ageDays = int(now.Sub(createdAt).Hours() / 24)
			createdAtText = createdAt.Format(time.RFC3339)
		}

		rows = append(rows, []string{
			cliutil.PointerToString(group.LogGroupName),
			createdAtText,
			fmt.Sprintf("%d", ageDays),
			retentionToString(group.RetentionInDays),
		})
	}

	return cliutil.WriteDataset(cmd, runtime, []string{"log_group", "created_at", "age_days", "retention_days"}, rows)
}

func runDeleteLogGroups(cmd *cobra.Command, retentionDays int, nameContains string) error {
	if retentionDays < 0 {
		return fmt.Errorf("--retention-days must be >= 0")
	}

	runtime, _, client, err := cliutil.NewServiceRuntime(cmd, loadAWSConfig, newClient)
	if err != nil {
		return err
	}

	groups, err := listLogGroups(cmd.Context(), client)
	if err != nil {
		return fmt.Errorf("list log groups: %s", awstbxaws.FormatUserError(err))
	}

	now := time.Now().UTC()
	targets := make([]cloudwatchlogstypes.LogGroup, 0)
	for _, group := range groups {
		name := cliutil.PointerToString(group.LogGroupName)
		if name == "" {
			continue
		}
		if nameContains != "" && !strings.Contains(name, nameContains) {
			continue
		}
		if retentionDays > 0 {
			createdAt := logGroupCreatedAt(group)
			if !createdAt.IsZero() {
				ageDays := int(now.Sub(createdAt).Hours() / 24)
				if ageDays <= retentionDays {
					continue
				}
			} else {
				continue
			}
		}
		targets = append(targets, group)
	}

	sortLogGroupsByName(targets)

	rows := make([][]string, 0, len(targets))
	for _, target := range targets {
		action := cliutil.ActionWouldDelete
		if !runtime.Options.DryRun {
			action = cliutil.ActionPending
		}
		rows = append(rows, []string{cliutil.PointerToString(target.LogGroupName), retentionToString(target.RetentionInDays), action})
	}

	return cliutil.RunDestructiveActionPlan(cmd, runtime, cliutil.DestructiveActionPlan{
		Headers:       []string{"log_group", "retention_days", "action"},
		Rows:          rows,
		ActionColumn:  2,
		ConfirmPrompt: fmt.Sprintf("Delete %d CloudWatch log group(s)", len(targets)),
		Execute: func(rowIndex int) string {
			_, deleteErr := client.DeleteLogGroup(cmd.Context(), &cloudwatchlogs.DeleteLogGroupInput{LogGroupName: targets[rowIndex].LogGroupName})
			if deleteErr != nil {
				return cliutil.FailedActionMessage(awstbxaws.FormatUserError(deleteErr))
			}
			return cliutil.ActionDeleted
		},
	})
}

func runSetRetention(cmd *cobra.Command, retentionDays int, printCounts bool) error {
	if retentionDays < 0 {
		return fmt.Errorf("--retention-days must be >= 0")
	}
	if printCounts && retentionDays > 0 {
		return fmt.Errorf("--print-retention-counts cannot be combined with --retention-days")
	}
	if !printCounts && retentionDays == 0 {
		return fmt.Errorf("set either --retention-days or --print-retention-counts")
	}

	runtime, _, client, err := cliutil.NewServiceRuntime(cmd, loadAWSConfig, newClient)
	if err != nil {
		return err
	}

	groups, err := listLogGroups(cmd.Context(), client)
	if err != nil {
		return fmt.Errorf("list log groups: %s", awstbxaws.FormatUserError(err))
	}

	if printCounts {
		return writeRetentionCounts(cmd, runtime, groups)
	}

	targetRetention := int32(retentionDays)
	targets := make([]cloudwatchlogstypes.LogGroup, 0)
	for _, group := range groups {
		if group.LogGroupName == nil {
			continue
		}
		if group.RetentionInDays != nil && *group.RetentionInDays == targetRetention {
			continue
		}
		targets = append(targets, group)
	}
	sortLogGroupsByName(targets)

	rows := make([][]string, 0, len(targets))
	for _, target := range targets {
		action := "would-update"
		if !runtime.Options.DryRun {
			action = "pending"
		}
		rows = append(rows, []string{
			cliutil.PointerToString(target.LogGroupName),
			retentionToString(target.RetentionInDays),
			fmt.Sprintf("%d", targetRetention),
			action,
		})
	}

	if len(targets) == 0 {
		return cliutil.WriteDataset(cmd, runtime, []string{"log_group", "current_retention_days", "target_retention_days", "action"}, rows)
	}

	if !runtime.Options.DryRun {
		ok, confirmErr := runtime.Prompter.Confirm(
			fmt.Sprintf("Update retention policy for %d log group(s)", len(targets)),
			runtime.Options.NoConfirm,
		)
		if confirmErr != nil {
			return confirmErr
		}
		if !ok {
			for i := range rows {
				rows[i][3] = "cancelled"
			}
			return cliutil.WriteDataset(cmd, runtime, []string{"log_group", "current_retention_days", "target_retention_days", "action"}, rows)
		}

		for i, target := range targets {
			_, updateErr := client.PutRetentionPolicy(cmd.Context(), &cloudwatchlogs.PutRetentionPolicyInput{
				LogGroupName:    target.LogGroupName,
				RetentionInDays: cliutil.Ptr(targetRetention),
			})
			if updateErr != nil {
				rows[i][3] = "failed: " + awstbxaws.FormatUserError(updateErr)
				continue
			}
			rows[i][3] = "updated"
		}
	}

	return cliutil.WriteDataset(cmd, runtime, []string{"log_group", "current_retention_days", "target_retention_days", "action"}, rows)
}

func writeRetentionCounts(cmd *cobra.Command, runtime cliutil.CommandRuntime, groups []cloudwatchlogstypes.LogGroup) error {
	counts := make(map[string]int)
	for _, group := range groups {
		key := retentionToString(group.RetentionInDays)
		counts[key]++
	}

	keys := make([]string, 0, len(counts))
	for key := range counts {
		keys = append(keys, key)
	}

	sort.Slice(keys, func(i, j int) bool {
		if keys[i] == "not_set" {
			return true
		}
		if keys[j] == "not_set" {
			return false
		}

		left, leftErr := strconv.Atoi(keys[i])
		right, rightErr := strconv.Atoi(keys[j])
		if leftErr != nil || rightErr != nil {
			return keys[i] < keys[j]
		}
		return left < right
	})

	rows := make([][]string, 0, len(keys))
	for _, key := range keys {
		rows = append(rows, []string{key, fmt.Sprintf("%d", counts[key])})
	}

	return cliutil.WriteDataset(cmd, runtime, []string{"retention_days", "count"}, rows)
}
