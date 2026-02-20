package cloudwatch

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
	"github.com/towardsthecloud/aws-toolbox/internal/cliutil"
)

// API is the subset of the CloudWatch Logs client used by this package.
type API interface {
	DeleteLogGroup(context.Context, *cloudwatchlogs.DeleteLogGroupInput, ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DeleteLogGroupOutput, error)
	DescribeLogGroups(context.Context, *cloudwatchlogs.DescribeLogGroupsInput, ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeLogGroupsOutput, error)
	PutRetentionPolicy(context.Context, *cloudwatchlogs.PutRetentionPolicyInput, ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.PutRetentionPolicyOutput, error)
}

var loadAWSConfig = awstbxaws.LoadAWSConfig
var newClient = func(cfg awssdk.Config) API {
	return cloudwatchlogs.NewFromConfig(cfg)
}

// NewCommand returns the cloudwatch service group command.
func NewCommand() *cobra.Command {
	cmd := cliutil.NewServiceGroupCommand("cloudwatch", "Manage CloudWatch resources")

	cmd.AddCommand(newCountLogGroupsCommand())
	cmd.AddCommand(newDeleteLogGroupsCommand())
	cmd.AddCommand(newListLogGroupsCommand())
	cmd.AddCommand(newSetRetentionCommand())

	return cmd
}

func newCountLogGroupsCommand() *cobra.Command {
	return &cobra.Command{
		Use:          "count-log-groups",
		Short:        "Count CloudWatch log groups",
		RunE:         runCountLogGroups,
		SilenceUsage: true,
	}
}

func newDeleteLogGroupsCommand() *cobra.Command {
	var retentionDays int
	var nameContains string

	cmd := &cobra.Command{
		Use:   "delete-log-groups",
		Short: "Delete log groups by age and/or name pattern",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runDeleteLogGroups(cmd, retentionDays, nameContains)
		},
		SilenceUsage: true,
	}
	cmd.Flags().IntVar(&retentionDays, "retention-days", 0, "Delete log groups older than this many days (0 disables age filter)")
	cmd.Flags().StringVar(&nameContains, "filter-name-contains", "", "Only target log groups containing this text")

	return cmd
}

func newListLogGroupsCommand() *cobra.Command {
	return &cobra.Command{
		Use:          "list-log-groups",
		Short:        "List log groups with creation details",
		RunE:         runListLogGroups,
		SilenceUsage: true,
	}
}

func newSetRetentionCommand() *cobra.Command {
	var retentionDays int
	var printCounts bool

	cmd := &cobra.Command{
		Use:   "set-retention",
		Short: "Set or inspect log group retention",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runSetRetention(cmd, retentionDays, printCounts)
		},
		SilenceUsage: true,
	}
	cmd.Flags().IntVar(&retentionDays, "retention-days", 0, "Target retention in days for all log groups")
	cmd.Flags().BoolVar(&printCounts, "print-retention-counts", false, "Print count of log groups by retention value")

	return cmd
}

func listLogGroups(ctx context.Context, client API) ([]cloudwatchlogstypes.LogGroup, error) {
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
		return cliutil.PointerToString(groups[i].LogGroupName) < cliutil.PointerToString(groups[j].LogGroupName)
	})
}
