package cli

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/spf13/cobra"
	awstbxaws "github.com/towardsthecloud/aws-toolbox/internal/aws"
)

func runEC2DeleteKeypairs(cmd *cobra.Command, allRegions bool) error {
	runtime, cfg, baseClient, err := newServiceRuntime(cmd, ec2LoadAWSConfig, ec2NewClient)
	if err != nil {
		return err
	}

	if !allRegions && cfg.Region == "" {
		return fmt.Errorf("resolve AWS region: set --region, AWS_REGION, or profile default region")
	}

	regions := []string{cfg.Region}
	if allRegions {
		regions, err = listRegions(cmd.Context(), baseClient)
		if err != nil {
			return fmt.Errorf("list regions: %s", awstbxaws.FormatUserError(err))
		}
	}

	targets := make([]keyPairTarget, 0)
	for _, region := range regions {
		if region == "" {
			continue
		}
		regionalClient := ec2NewRegionalClient(cfg, region)
		regionTargets, collectErr := collectUnusedKeyPairs(cmd.Context(), regionalClient, region)
		if collectErr != nil {
			return collectErr
		}
		targets = append(targets, regionTargets...)
	}

	sort.Slice(targets, func(i, j int) bool {
		if targets[i].Region == targets[j].Region {
			return targets[i].Name < targets[j].Name
		}
		return targets[i].Region < targets[j].Region
	})

	rows := make([][]string, 0, len(targets))
	for _, target := range targets {
		action := actionWouldDelete
		if !runtime.Options.DryRun {
			action = actionPending
		}
		rows = append(rows, []string{target.Name, target.Region, action})
	}

	if len(targets) == 0 {
		return writeDataset(cmd, runtime, []string{"key_name", "region", "action"}, rows)
	}

	if !runtime.Options.DryRun {
		ok, confirmErr := runtime.Prompter.Confirm(
			fmt.Sprintf("Delete %d unused key pair(s)", len(targets)),
			runtime.Options.NoConfirm,
		)
		if confirmErr != nil {
			return confirmErr
		}
		if !ok {
			for i := range rows {
				rows[i][2] = actionCancelled
			}
			return writeDataset(cmd, runtime, []string{"key_name", "region", "action"}, rows)
		}

		for i, target := range targets {
			client := ec2NewRegionalClient(cfg, target.Region)
			_, deleteErr := client.DeleteKeyPair(cmd.Context(), &ec2.DeleteKeyPairInput{KeyName: &target.Name})
			if deleteErr != nil {
				rows[i][2] = failedActionMessage(awstbxaws.FormatUserError(deleteErr))
				continue
			}
			rows[i][2] = actionDeleted
		}
	}

	return writeDataset(cmd, runtime, []string{"key_name", "region", "action"}, rows)
}

func runEC2DeleteSecurityGroups(
	cmd *cobra.Command,
	sshRules bool,
	tagFilter string,
	unusedOnly bool,
	securityGroupType string,
) error {
	if securityGroupType != "all" && securityGroupType != "ec2" && securityGroupType != "rds" && securityGroupType != "elb" {
		return fmt.Errorf("--type must be one of: all, ec2, rds, elb")
	}
	if !sshRules && !unusedOnly && tagFilter == "" {
		return fmt.Errorf("set at least one filter: --ssh-rules, --unused, or --filter-tag")
	}

	tagKey, tagValue, err := parseTagFilter(tagFilter)
	if err != nil {
		return err
	}

	runtime, cfg, client, err := newServiceRuntime(cmd, ec2LoadAWSConfig, ec2NewClient)
	if err != nil {
		return err
	}

	groups, err := listSecurityGroups(cmd.Context(), client)
	if err != nil {
		return fmt.Errorf("list security groups: %s", awstbxaws.FormatUserError(err))
	}

	usedGroups := map[string]struct{}{}
	if unusedOnly {
		usedGroups, err = listUsedSecurityGroups(cmd.Context(), client)
		if err != nil {
			return fmt.Errorf("list used security groups: %s", awstbxaws.FormatUserError(err))
		}
	}

	targets := make([]securityGroupTarget, 0)
	for _, group := range groups {
		groupID := pointerToString(group.GroupId)
		groupName := pointerToString(group.GroupName)
		if groupID == "" || strings.EqualFold(groupName, "default") {
			continue
		}
		if !matchesSecurityGroupType(groupName, securityGroupType) {
			continue
		}
		if unusedOnly {
			if _, inUse := usedGroups[groupID]; inUse {
				continue
			}
		}
		if tagKey != "" && !hasTagMatch(group.Tags, tagKey, tagValue) {
			continue
		}

		sshOnlyPermissions := ingressSSHRules(group.IpPermissions)
		if sshRules && len(sshOnlyPermissions) == 0 {
			continue
		}

		targets = append(targets, securityGroupTarget{
			GroupID:        groupID,
			GroupName:      groupName,
			SSHPermissions: sshOnlyPermissions,
		})
	}

	sort.Slice(targets, func(i, j int) bool {
		return targets[i].GroupID < targets[j].GroupID
	})

	rows := make([][]string, 0, len(targets))
	for _, target := range targets {
		action := actionWouldDelete
		if !runtime.Options.DryRun {
			action = actionPending
		}
		rows = append(rows, []string{target.GroupID, target.GroupName, cfg.Region, action})
	}

	if len(targets) == 0 {
		return writeDataset(cmd, runtime, []string{"group_id", "group_name", "region", "action"}, rows)
	}

	if !runtime.Options.DryRun {
		verb := "delete"
		if sshRules {
			verb = "revoke SSH rules from"
		}
		ok, confirmErr := runtime.Prompter.Confirm(
			fmt.Sprintf("Proceed to %s %d security group(s)", verb, len(targets)),
			runtime.Options.NoConfirm,
		)
		if confirmErr != nil {
			return confirmErr
		}
		if !ok {
			for i := range rows {
				rows[i][3] = actionCancelled
			}
			return writeDataset(cmd, runtime, []string{"group_id", "group_name", "region", "action"}, rows)
		}

		for i, target := range targets {
			var opErr error
			if sshRules {
				_, opErr = client.RevokeSecurityGroupIngress(cmd.Context(), &ec2.RevokeSecurityGroupIngressInput{
					GroupId:       &target.GroupID,
					IpPermissions: target.SSHPermissions,
				})
				if opErr == nil {
					rows[i][3] = actionDeleted
				}
			} else {
				_, opErr = client.DeleteSecurityGroup(cmd.Context(), &ec2.DeleteSecurityGroupInput{GroupId: &target.GroupID})
				if opErr == nil {
					rows[i][3] = actionDeleted
				}
			}

			if opErr != nil {
				rows[i][3] = failedActionMessage(awstbxaws.FormatUserError(opErr))
			}
		}
	}

	return writeDataset(cmd, runtime, []string{"group_id", "group_name", "region", "action"}, rows)
}

type keyPairTarget struct {
	Name   string
	Region string
}

type securityGroupTarget struct {
	GroupID        string
	GroupName      string
	SSHPermissions []ec2types.IpPermission
}

func collectUnusedKeyPairs(ctx context.Context, client ec2API, region string) ([]keyPairTarget, error) {
	keyPairs, err := listKeyPairs(ctx, client)
	if err != nil {
		return nil, fmt.Errorf("list key pairs (%s): %s", region, awstbxaws.FormatUserError(err))
	}

	usedKeys, err := listUsedKeyPairs(ctx, client)
	if err != nil {
		return nil, fmt.Errorf("list used key pairs (%s): %s", region, awstbxaws.FormatUserError(err))
	}

	targets := make([]keyPairTarget, 0)
	for _, keyPair := range keyPairs {
		name := pointerToString(keyPair.KeyName)
		if name == "" {
			continue
		}
		if _, used := usedKeys[name]; used {
			continue
		}
		targets = append(targets, keyPairTarget{Name: name, Region: region})
	}

	return targets, nil
}

func listKeyPairs(ctx context.Context, client ec2API) ([]ec2types.KeyPairInfo, error) {
	page, err := client.DescribeKeyPairs(ctx, &ec2.DescribeKeyPairsInput{})
	if err != nil {
		return nil, err
	}

	return page.KeyPairs, nil
}

func listUsedKeyPairs(ctx context.Context, client ec2API) (map[string]struct{}, error) {
	used := make(map[string]struct{})
	var nextToken *string
	for {
		page, err := client.DescribeInstances(ctx, &ec2.DescribeInstancesInput{NextToken: nextToken})
		if err != nil {
			return nil, err
		}
		for _, reservation := range page.Reservations {
			for _, instance := range reservation.Instances {
				if instance.KeyName != nil {
					used[*instance.KeyName] = struct{}{}
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

func listSecurityGroups(ctx context.Context, client ec2API) ([]ec2types.SecurityGroup, error) {
	groups := make([]ec2types.SecurityGroup, 0)
	var nextToken *string
	for {
		page, err := client.DescribeSecurityGroups(ctx, &ec2.DescribeSecurityGroupsInput{NextToken: nextToken})
		if err != nil {
			return nil, err
		}
		groups = append(groups, page.SecurityGroups...)
		if page.NextToken == nil || *page.NextToken == "" {
			break
		}
		nextToken = page.NextToken
	}
	return groups, nil
}

func listUsedSecurityGroups(ctx context.Context, client ec2API) (map[string]struct{}, error) {
	used := make(map[string]struct{})
	var nextToken *string
	for {
		page, err := client.DescribeNetworkInterfaces(ctx, &ec2.DescribeNetworkInterfacesInput{NextToken: nextToken})
		if err != nil {
			return nil, err
		}
		for _, networkInterface := range page.NetworkInterfaces {
			for _, group := range networkInterface.Groups {
				if group.GroupId != nil {
					used[*group.GroupId] = struct{}{}
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

func ingressSSHRules(permissions []ec2types.IpPermission) []ec2types.IpPermission {
	matches := make([]ec2types.IpPermission, 0)
	for _, permission := range permissions {
		if permission.FromPort == nil || permission.ToPort == nil || permission.IpProtocol == nil {
			continue
		}
		if *permission.FromPort == 22 && *permission.ToPort == 22 && *permission.IpProtocol == "tcp" {
			matches = append(matches, permission)
		}
	}
	return matches
}

func hasTagMatch(tags []ec2types.Tag, key, value string) bool {
	for _, tag := range tags {
		if tag.Key == nil || tag.Value == nil {
			continue
		}
		if *tag.Key == key && strings.Contains(*tag.Value, value) {
			return true
		}
	}
	return false
}
