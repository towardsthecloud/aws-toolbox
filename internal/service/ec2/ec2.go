package ec2

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/smithy-go"
	"github.com/spf13/cobra"
	awstbxaws "github.com/towardsthecloud/aws-toolbox/internal/aws"
	"github.com/towardsthecloud/aws-toolbox/internal/cliutil"
)

// API defines the subset of EC2 operations used by this package.
type API interface {
	DescribeAddresses(context.Context, *ec2.DescribeAddressesInput, ...func(*ec2.Options)) (*ec2.DescribeAddressesOutput, error)
	DescribeImages(context.Context, *ec2.DescribeImagesInput, ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error)
	DescribeInstances(context.Context, *ec2.DescribeInstancesInput, ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error)
	DescribeKeyPairs(context.Context, *ec2.DescribeKeyPairsInput, ...func(*ec2.Options)) (*ec2.DescribeKeyPairsOutput, error)
	DescribeNetworkInterfaces(context.Context, *ec2.DescribeNetworkInterfacesInput, ...func(*ec2.Options)) (*ec2.DescribeNetworkInterfacesOutput, error)
	DescribeRegions(context.Context, *ec2.DescribeRegionsInput, ...func(*ec2.Options)) (*ec2.DescribeRegionsOutput, error)
	DescribeSecurityGroups(context.Context, *ec2.DescribeSecurityGroupsInput, ...func(*ec2.Options)) (*ec2.DescribeSecurityGroupsOutput, error)
	DescribeSnapshots(context.Context, *ec2.DescribeSnapshotsInput, ...func(*ec2.Options)) (*ec2.DescribeSnapshotsOutput, error)
	DescribeVolumes(context.Context, *ec2.DescribeVolumesInput, ...func(*ec2.Options)) (*ec2.DescribeVolumesOutput, error)
	DeleteKeyPair(context.Context, *ec2.DeleteKeyPairInput, ...func(*ec2.Options)) (*ec2.DeleteKeyPairOutput, error)
	DeleteSecurityGroup(context.Context, *ec2.DeleteSecurityGroupInput, ...func(*ec2.Options)) (*ec2.DeleteSecurityGroupOutput, error)
	DeleteSnapshot(context.Context, *ec2.DeleteSnapshotInput, ...func(*ec2.Options)) (*ec2.DeleteSnapshotOutput, error)
	DeleteVolume(context.Context, *ec2.DeleteVolumeInput, ...func(*ec2.Options)) (*ec2.DeleteVolumeOutput, error)
	DeregisterImage(context.Context, *ec2.DeregisterImageInput, ...func(*ec2.Options)) (*ec2.DeregisterImageOutput, error)
	ReleaseAddress(context.Context, *ec2.ReleaseAddressInput, ...func(*ec2.Options)) (*ec2.ReleaseAddressOutput, error)
	RevokeSecurityGroupIngress(context.Context, *ec2.RevokeSecurityGroupIngressInput, ...func(*ec2.Options)) (*ec2.RevokeSecurityGroupIngressOutput, error)
}

var loadAWSConfig = awstbxaws.LoadAWSConfig
var newClient = func(cfg awssdk.Config) API {
	return ec2.NewFromConfig(cfg)
}
var newRegionalClient = func(cfg awssdk.Config, region string) API {
	regionalCfg := cfg
	regionalCfg.Region = region
	return ec2.NewFromConfig(regionalCfg)
}

// NewCommand returns the top-level ec2 cobra command with all subcommands.
func NewCommand() *cobra.Command {
	cmd := cliutil.NewServiceGroupCommand("ec2", "Manage EC2 resources")

	cmd.AddCommand(newDeleteAMIsCommand())
	cmd.AddCommand(newDeleteEIPsCommand())
	cmd.AddCommand(newDeleteKeypairsCommand())
	cmd.AddCommand(newDeleteSecurityGroupsCommand())
	cmd.AddCommand(newDeleteSnapshotsCommand())
	cmd.AddCommand(newDeleteVolumesCommand())
	cmd.AddCommand(newListEIPsCommand())

	return cmd
}

func newDeleteAMIsCommand() *cobra.Command {
	var retentionDays int
	var unusedOnly bool

	cmd := &cobra.Command{
		Use:   "delete-amis",
		Short: "Deregister stale AMIs",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runDeleteAMIs(cmd, retentionDays, unusedOnly)
		},
		SilenceUsage: true,
	}
	cmd.Flags().IntVar(&retentionDays, "retention-days", 0, "Only target AMIs older than this many days")
	cmd.Flags().BoolVar(&unusedOnly, "unused", false, "Only target AMIs not used by any EC2 instance")

	return cmd
}

func newDeleteEIPsCommand() *cobra.Command {
	return &cobra.Command{
		Use:          "delete-eips",
		Short:        "Release unused Elastic IPs",
		RunE:         runDeleteEIPs,
		SilenceUsage: true,
	}
}

func newDeleteKeypairsCommand() *cobra.Command {
	var allRegions bool

	cmd := &cobra.Command{
		Use:   "delete-keypairs",
		Short: "Delete unused EC2 key pairs",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runDeleteKeypairs(cmd, allRegions)
		},
		SilenceUsage: true,
	}
	cmd.Flags().BoolVar(&allRegions, "all-regions", false, "Scan all enabled regions")

	return cmd
}

func newDeleteSecurityGroupsCommand() *cobra.Command {
	var sshRules bool
	var tagFilter string
	var unusedOnly bool
	var securityGroupType string

	cmd := &cobra.Command{
		Use:   "delete-security-groups",
		Short: "Delete or harden security groups",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runDeleteSecurityGroups(cmd, sshRules, tagFilter, unusedOnly, securityGroupType)
		},
		SilenceUsage: true,
	}
	cmd.Flags().BoolVar(&sshRules, "ssh-rules", false, "Revoke inbound TCP/22 rules instead of deleting groups")
	cmd.Flags().StringVar(&tagFilter, "filter-tag", "", "Tag filter in KEY=VALUE form")
	cmd.Flags().BoolVar(&unusedOnly, "unused", false, "Only target security groups not attached to ENIs")
	cmd.Flags().StringVar(&securityGroupType, "type", "all", "Filter by naming convention: all|ec2|rds|elb")

	return cmd
}

func newDeleteSnapshotsCommand() *cobra.Command {
	var retentionDays int

	cmd := &cobra.Command{
		Use:   "delete-snapshots",
		Short: "Delete orphaned EBS snapshots",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runDeleteSnapshots(cmd, retentionDays)
		},
		SilenceUsage: true,
	}
	cmd.Flags().IntVar(&retentionDays, "retention-days", 0, "Only target snapshots older than this many days")

	return cmd
}

func newDeleteVolumesCommand() *cobra.Command {
	return &cobra.Command{
		Use:          "delete-volumes",
		Short:        "Delete unattached EBS volumes",
		RunE:         runDeleteVolumes,
		SilenceUsage: true,
	}
}

func newListEIPsCommand() *cobra.Command {
	return &cobra.Command{
		Use:          "list-eips",
		Short:        "List available Elastic IPs",
		RunE:         runListEIPs,
		SilenceUsage: true,
	}
}

func listOwnedImages(ctx context.Context, client API) ([]ec2types.Image, error) {
	images := make([]ec2types.Image, 0)
	var nextToken *string

	for {
		page, err := client.DescribeImages(ctx, &ec2.DescribeImagesInput{Owners: []string{"self"}, NextToken: nextToken})
		if err != nil {
			return nil, err
		}

		images = append(images, page.Images...)
		if page.NextToken == nil || *page.NextToken == "" {
			break
		}
		nextToken = page.NextToken
	}

	return images, nil
}

func listUsedAMIIDs(ctx context.Context, client API) (map[string]struct{}, error) {
	used := make(map[string]struct{})
	var nextToken *string

	for {
		page, err := client.DescribeInstances(ctx, &ec2.DescribeInstancesInput{NextToken: nextToken})
		if err != nil {
			return nil, err
		}

		for _, reservation := range page.Reservations {
			for _, instance := range reservation.Instances {
				if instance.ImageId != nil {
					used[*instance.ImageId] = struct{}{}
				}
			}
		}

		if page.NextToken == nil || *page.NextToken == "" {
			break
		}
		nextToken = page.NextToken
	}

	return used, nil
}

func parseAWSDate(value string) (time.Time, error) {
	layouts := []string{time.RFC3339Nano, time.RFC3339, "2006-01-02T15:04:05.000Z"}
	for _, layout := range layouts {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			return parsed, nil
		}
	}

	return time.Time{}, fmt.Errorf("unsupported timestamp %q", value)
}

func awsErrorCode(err error) string {
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		return apiErr.ErrorCode()
	}
	return ""
}

func matchesSecurityGroupType(groupName, securityGroupType string) bool {
	name := strings.ToLower(groupName)
	switch securityGroupType {
	case "all":
		return true
	case "ec2":
		return !strings.HasPrefix(name, "rds-") && !strings.HasPrefix(name, "elb-")
	case "rds":
		return strings.HasPrefix(name, "rds-")
	case "elb":
		return strings.HasPrefix(name, "elb-")
	default:
		return false
	}
}

func listRegions(ctx context.Context, client API) ([]string, error) {
	resp, err := client.DescribeRegions(ctx, &ec2.DescribeRegionsInput{})
	if err != nil {
		return nil, err
	}

	regions := make([]string, 0, len(resp.Regions))
	for _, region := range resp.Regions {
		if region.RegionName != nil {
			regions = append(regions, *region.RegionName)
		}
	}
	sort.Strings(regions)

	return regions, nil
}
