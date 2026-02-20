package cli

import (
	"context"
	"fmt"
	"sort"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	cloudwatchlogstypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	"github.com/spf13/cobra"
	awstbxaws "github.com/towardsthecloud/aws-toolbox/internal/aws"
)

type cloudWatchLogsAPI interface {
	DeleteLogGroup(context.Context, *cloudwatchlogs.DeleteLogGroupInput, ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DeleteLogGroupOutput, error)
	DescribeLogGroups(context.Context, *cloudwatchlogs.DescribeLogGroupsInput, ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeLogGroupsOutput, error)
	PutRetentionPolicy(context.Context, *cloudwatchlogs.PutRetentionPolicyInput, ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.PutRetentionPolicyOutput, error)
}

var cloudWatchLoadAWSConfig = awstbxaws.LoadAWSConfig
var cloudWatchNewClient = func(cfg awssdk.Config) cloudWatchLogsAPI {
	return cloudwatchlogs.NewFromConfig(cfg)
}

func newCloudWatchCommand() *cobra.Command {
	cmd := newServiceGroupCommand("cloudwatch", "Manage CloudWatch resources")

	cmd.AddCommand(newCloudWatchCountLogGroupsCommand())
	cmd.AddCommand(newCloudWatchDeleteLogGroupsCommand())
	cmd.AddCommand(newCloudWatchListLogGroupsCommand())
	cmd.AddCommand(newCloudWatchSetRetentionCommand())

	return cmd
}

func newCloudWatchCountLogGroupsCommand() *cobra.Command {
	return &cobra.Command{
		Use:          "count-log-groups",
		Short:        "Count CloudWatch log groups",
		RunE:         runCloudWatchCountLogGroups,
		SilenceUsage: true,
	}
}

func newCloudWatchDeleteLogGroupsCommand() *cobra.Command {
	var retentionDays int
	var nameContains string

	cmd := &cobra.Command{
		Use:   "delete-log-groups",
		Short: "Delete log groups by age and/or name pattern",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runCloudWatchDeleteLogGroups(cmd, retentionDays, nameContains)
		},
		SilenceUsage: true,
	}
	cmd.Flags().IntVar(&retentionDays, "retention-days", 0, "Delete log groups older than this many days (0 disables age filter)")
	cmd.Flags().StringVar(&nameContains, "filter-name-contains", "", "Only target log groups containing this text")

	return cmd
}

func newCloudWatchListLogGroupsCommand() *cobra.Command {
	return &cobra.Command{
		Use:          "list-log-groups",
		Short:        "List log groups with creation details",
		RunE:         runCloudWatchListLogGroups,
		SilenceUsage: true,
	}
}

func newCloudWatchSetRetentionCommand() *cobra.Command {
	var retentionDays int
	var printCounts bool

	cmd := &cobra.Command{
		Use:   "set-retention",
		Short: "Set or inspect log group retention",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runCloudWatchSetRetention(cmd, retentionDays, printCounts)
		},
		SilenceUsage: true,
	}
	cmd.Flags().IntVar(&retentionDays, "retention-days", 0, "Target retention in days for all log groups")
	cmd.Flags().BoolVar(&printCounts, "print-retention-counts", false, "Print count of log groups by retention value")

	return cmd
}

func listLogGroups(ctx context.Context, client cloudWatchLogsAPI) ([]cloudwatchlogstypes.LogGroup, error) {
	groups := make([]cloudwatchlogstypes.LogGroup, 0)
	var nextToken *string
	for {
		page, err := client.DescribeLogGroups(ctx, &cloudwatchlogs.DescribeLogGroupsInput{NextToken: nextToken})
		if err != nil {
			return nil, err
		}

		groups = append(groups, page.LogGroups...)
		if page.NextToken == nil || *page.NextToken == "" {
			break
		}
		nextToken = page.NextToken
	}

	return groups, nil
}

func logGroupCreatedAt(group cloudwatchlogstypes.LogGroup) time.Time {
	if group.CreationTime == nil {
		return time.Time{}
	}
	return time.UnixMilli(*group.CreationTime).UTC()
}

func retentionToString(value *int32) string {
	if value == nil {
		return "not_set"
	}
	return fmt.Sprintf("%d", *value)
}

func sortLogGroupsByName(groups []cloudwatchlogstypes.LogGroup) {
	sort.Slice(groups, func(i, j int) bool {
		return pointerToString(groups[i].LogGroupName) < pointerToString(groups[j].LogGroupName)
	})
}
