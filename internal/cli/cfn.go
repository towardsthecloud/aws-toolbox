package cli

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	cloudformationtypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	"github.com/spf13/cobra"
	awstbxaws "github.com/towardsthecloud/aws-toolbox/internal/aws"
)

type cfnAPI interface {
	DeleteStackInstances(context.Context, *cloudformation.DeleteStackInstancesInput, ...func(*cloudformation.Options)) (*cloudformation.DeleteStackInstancesOutput, error)
	DeleteStackSet(context.Context, *cloudformation.DeleteStackSetInput, ...func(*cloudformation.Options)) (*cloudformation.DeleteStackSetOutput, error)
	DescribeStackSetOperation(context.Context, *cloudformation.DescribeStackSetOperationInput, ...func(*cloudformation.Options)) (*cloudformation.DescribeStackSetOperationOutput, error)
	DescribeStacks(context.Context, *cloudformation.DescribeStacksInput, ...func(*cloudformation.Options)) (*cloudformation.DescribeStacksOutput, error)
	ListStackInstances(context.Context, *cloudformation.ListStackInstancesInput, ...func(*cloudformation.Options)) (*cloudformation.ListStackInstancesOutput, error)
	ListStackResources(context.Context, *cloudformation.ListStackResourcesInput, ...func(*cloudformation.Options)) (*cloudformation.ListStackResourcesOutput, error)
}

type stackInstanceTarget struct {
	Account string
	Region  string
}

var cfnLoadAWSConfig = awstbxaws.LoadAWSConfig
var cfnNewClient = func(cfg awssdk.Config) cfnAPI {
	return cloudformation.NewFromConfig(cfg)
}
var cfnSleep = time.Sleep

func newCFNCommand() *cobra.Command {
	cmd := newServiceGroupCommand("cfn", "Manage CloudFormation resources")

	cmd.AddCommand(newCFNDeleteStackSetCommand())
	cmd.AddCommand(newCFNFindStackByResourceCommand())

	return cmd
}

func newCFNDeleteStackSetCommand() *cobra.Command {
	var name string

	cmd := &cobra.Command{
		Use:   "delete-stackset",
		Short: "Delete a stack set after removing all stack instances",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runCFNDeleteStackSet(cmd, name)
		},
		SilenceUsage: true,
	}
	cmd.Flags().StringVar(&name, "name", "", "CloudFormation stack set name")

	return cmd
}

func newCFNFindStackByResourceCommand() *cobra.Command {
	var resource string
	var exact bool
	var includeNested bool

	cmd := &cobra.Command{
		Use:   "find-stack-by-resource",
		Short: "Find stacks that contain a matching resource",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runCFNFindStackByResource(cmd, resource, exact, includeNested)
		},
		SilenceUsage: true,
	}
	cmd.Flags().StringVar(&resource, "resource", "", "Resource identifier, logical ID, physical ID, or type to match")
	cmd.Flags().BoolVar(&exact, "exact", false, "Require an exact match")
	cmd.Flags().BoolVar(&includeNested, "include-nested", false, "Include nested stacks in the search")

	return cmd
}

func runCFNDeleteStackSet(cmd *cobra.Command, name string) error {
	stackSetName := strings.TrimSpace(name)
	if stackSetName == "" {
		return fmt.Errorf("--name is required")
	}

	runtime, err := newCommandRuntime(cmd)
	if err != nil {
		return err
	}

	cfg, err := cfnLoadAWSConfig(runtime.Options.Profile, runtime.Options.Region)
	if err != nil {
		return fmt.Errorf("load AWS config: %w", err)
	}
	client := cfnNewClient(cfg)

	targets, err := listStackInstanceTargets(cmd.Context(), client, stackSetName)
	if err != nil {
		return fmt.Errorf("list stack set instances: %s", awstbxaws.FormatUserError(err))
	}

	rows := make([][]string, 0, len(targets)+1)
	for _, target := range targets {
		action := "would-delete-stack-instance"
		if !runtime.Options.DryRun {
			action = "pending"
		}
		rows = append(rows, []string{stackSetName, target.Account, target.Region, "stack-instance", action})
	}

	stackSetAction := "would-delete-stackset"
	if !runtime.Options.DryRun {
		stackSetAction = "pending"
	}
	rows = append(rows, []string{stackSetName, "", "", "stackset", stackSetAction})
	stackSetRow := len(rows) - 1

	if runtime.Options.DryRun {
		return writeDataset(cmd, runtime, []string{"stackset_name", "account", "region", "resource", "action"}, rows)
	}

	ok, err := runtime.Prompter.Confirm(
		fmt.Sprintf("Delete stack set %q and %d stack instance(s)", stackSetName, len(targets)),
		runtime.Options.NoConfirm,
	)
	if err != nil {
		return err
	}
	if !ok {
		for i := range rows {
			rows[i][4] = "cancelled"
		}
		return writeDataset(cmd, runtime, []string{"stackset_name", "account", "region", "resource", "action"}, rows)
	}

	instanceFailure := false
	for i, target := range targets {
		opID, deleteErr := deleteStackSetInstanceTarget(cmd.Context(), client, stackSetName, target)
		if deleteErr != nil {
			rows[i][4] = "failed: " + awstbxaws.FormatUserError(deleteErr)
			instanceFailure = true
			continue
		}

		if opID != "" {
			waitErr := waitForStackSetOperation(cmd.Context(), client, stackSetName, opID)
			if waitErr != nil {
				rows[i][4] = "failed: " + awstbxaws.FormatUserError(waitErr)
				instanceFailure = true
				continue
			}
		}

		rows[i][4] = "deleted-stack-instance"
	}

	if instanceFailure {
		rows[stackSetRow][4] = "skipped: stack instance deletion failed"
		return writeDataset(cmd, runtime, []string{"stackset_name", "account", "region", "resource", "action"}, rows)
	}

	_, err = client.DeleteStackSet(cmd.Context(), &cloudformation.DeleteStackSetInput{StackSetName: ptr(stackSetName)})
	if err != nil {
		rows[stackSetRow][4] = "failed: " + awstbxaws.FormatUserError(err)
		return writeDataset(cmd, runtime, []string{"stackset_name", "account", "region", "resource", "action"}, rows)
	}

	rows[stackSetRow][4] = "deleted-stackset"
	return writeDataset(cmd, runtime, []string{"stackset_name", "account", "region", "resource", "action"}, rows)
}

func runCFNFindStackByResource(cmd *cobra.Command, resource string, exact, includeNested bool) error {
	query := strings.TrimSpace(resource)
	if query == "" {
		return fmt.Errorf("--resource is required")
	}

	runtime, err := newCommandRuntime(cmd)
	if err != nil {
		return err
	}

	cfg, err := cfnLoadAWSConfig(runtime.Options.Profile, runtime.Options.Region)
	if err != nil {
		return fmt.Errorf("load AWS config: %w", err)
	}
	client := cfnNewClient(cfg)

	stacks, err := listStacksForSearch(cmd.Context(), client, includeNested)
	if err != nil {
		return fmt.Errorf("list stacks: %s", awstbxaws.FormatUserError(err))
	}

	rows := make([][]string, 0)
	for _, stack := range stacks {
		resources, listErr := listStackResources(cmd.Context(), client, pointerToString(stack.StackName))
		if listErr != nil {
			return fmt.Errorf("list resources for stack %s: %s", pointerToString(stack.StackName), awstbxaws.FormatUserError(listErr))
		}

		for _, item := range resources {
			if !stackResourceMatches(item, query, exact) {
				continue
			}
			rows = append(rows, []string{
				pointerToString(stack.StackName),
				pointerToString(item.LogicalResourceId),
				pointerToString(item.PhysicalResourceId),
				pointerToString(item.ResourceType),
				string(item.ResourceStatus),
			})
		}
	}

	sort.Slice(rows, func(i, j int) bool {
		if rows[i][0] == rows[j][0] {
			return rows[i][1] < rows[j][1]
		}
		return rows[i][0] < rows[j][0]
	})

	return writeDataset(cmd, runtime, []string{"stack_name", "logical_id", "physical_id", "resource_type", "status"}, rows)
}

func listStackInstanceTargets(ctx context.Context, client cfnAPI, stackSetName string) ([]stackInstanceTarget, error) {
	targets := make([]stackInstanceTarget, 0)
	seen := make(map[string]struct{})
	var nextToken *string

	for {
		page, err := client.ListStackInstances(ctx, &cloudformation.ListStackInstancesInput{
			StackSetName: ptr(stackSetName),
			NextToken:    nextToken,
		})
		if err != nil {
			return nil, err
		}

		for _, item := range page.Summaries {
			account := strings.TrimSpace(pointerToString(item.Account))
			region := strings.TrimSpace(pointerToString(item.Region))
			if account == "" || region == "" {
				continue
			}

			key := account + "|" + region
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			targets = append(targets, stackInstanceTarget{Account: account, Region: region})
		}

		if page.NextToken == nil || *page.NextToken == "" {
			break
		}
		nextToken = page.NextToken
	}

	sort.Slice(targets, func(i, j int) bool {
		if targets[i].Account == targets[j].Account {
			return targets[i].Region < targets[j].Region
		}
		return targets[i].Account < targets[j].Account
	})

	return targets, nil
}

func deleteStackSetInstanceTarget(ctx context.Context, client cfnAPI, stackSetName string, target stackInstanceTarget) (string, error) {
	resp, err := client.DeleteStackInstances(ctx, &cloudformation.DeleteStackInstancesInput{
		StackSetName: ptr(stackSetName),
		Accounts:     []string{target.Account},
		Regions:      []string{target.Region},
		RetainStacks: ptr(false),
	})
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(pointerToString(resp.OperationId)), nil
}

func waitForStackSetOperation(ctx context.Context, client cfnAPI, stackSetName, operationID string) error {
	const maxAttempts = 360
	const pollInterval = 5 * time.Second
	for range maxAttempts {
		resp, err := client.DescribeStackSetOperation(ctx, &cloudformation.DescribeStackSetOperationInput{
			StackSetName: ptr(stackSetName),
			OperationId:  ptr(operationID),
		})
		if err != nil {
			return err
		}

		status := resp.StackSetOperation.Status
		switch status {
		case cloudformationtypes.StackSetOperationStatusSucceeded:
			return nil
		case cloudformationtypes.StackSetOperationStatusFailed, cloudformationtypes.StackSetOperationStatusStopped:
			reason := strings.TrimSpace(pointerToString(resp.StackSetOperation.StatusReason))
			if reason != "" {
				return fmt.Errorf("stack set operation %s: %s", status, reason)
			}
			return fmt.Errorf("stack set operation %s", status)
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			cfnSleep(pollInterval)
		}
	}

	return fmt.Errorf("timed out waiting for stack set operation %s", operationID)
}

func listStacksForSearch(ctx context.Context, client cfnAPI, includeNested bool) ([]cloudformationtypes.Stack, error) {
	stacks := make([]cloudformationtypes.Stack, 0)
	var nextToken *string

	for {
		page, err := client.DescribeStacks(ctx, &cloudformation.DescribeStacksInput{NextToken: nextToken})
		if err != nil {
			return nil, err
		}

		for _, stack := range page.Stacks {
			if stack.StackStatus == cloudformationtypes.StackStatusDeleteComplete {
				continue
			}
			if !includeNested && stack.ParentId != nil {
				continue
			}
			stacks = append(stacks, stack)
		}

		if page.NextToken == nil || *page.NextToken == "" {
			break
		}
		nextToken = page.NextToken
	}

	sort.Slice(stacks, func(i, j int) bool {
		return pointerToString(stacks[i].StackName) < pointerToString(stacks[j].StackName)
	})

	return stacks, nil
}

func listStackResources(ctx context.Context, client cfnAPI, stackName string) ([]cloudformationtypes.StackResourceSummary, error) {
	resources := make([]cloudformationtypes.StackResourceSummary, 0)
	var nextToken *string

	for {
		page, err := client.ListStackResources(ctx, &cloudformation.ListStackResourcesInput{
			StackName: ptr(stackName),
			NextToken: nextToken,
		})
		if err != nil {
			return nil, err
		}

		resources = append(resources, page.StackResourceSummaries...)
		if page.NextToken == nil || *page.NextToken == "" {
			break
		}
		nextToken = page.NextToken
	}

	return resources, nil
}

func stackResourceMatches(resource cloudformationtypes.StackResourceSummary, query string, exact bool) bool {
	if exact {
		for _, candidate := range []string{
			pointerToString(resource.LogicalResourceId),
			pointerToString(resource.PhysicalResourceId),
			pointerToString(resource.ResourceType),
		} {
			if strings.EqualFold(strings.TrimSpace(candidate), query) {
				return true
			}
		}
		return false
	}

	needle := strings.ToLower(query)
	for _, candidate := range []string{
		pointerToString(resource.LogicalResourceId),
		pointerToString(resource.PhysicalResourceId),
		pointerToString(resource.ResourceType),
	} {
		if strings.Contains(strings.ToLower(candidate), needle) {
			return true
		}
	}

	return false
}
