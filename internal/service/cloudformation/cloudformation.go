package cloudformation

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
	"github.com/towardsthecloud/aws-toolbox/internal/cliutil"
)

type API interface {
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

var loadAWSConfig = awstbxaws.LoadAWSConfig
var newClient = func(cfg awssdk.Config) API {
	return cloudformation.NewFromConfig(cfg)
}
var sleep = time.Sleep

func NewCommand() *cobra.Command {
	cmd := cliutil.NewServiceGroupCommand("cloudformation", "Manage CloudFormation resources")

	cmd.AddCommand(newDeleteStackSetCommand())
	cmd.AddCommand(newFindStackByResourceCommand())

	return cmd
}

func newDeleteStackSetCommand() *cobra.Command {
	var stackSetName string

	cmd := &cobra.Command{
		Use:   "delete-stackset",
		Short: "Delete a stack set after removing all stack instances",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runDeleteStackSet(cmd, stackSetName)
		},
		SilenceUsage: true,
	}
	cmd.Flags().StringVar(&stackSetName, "stackset-name", "", "CloudFormation stack set name")

	return cmd
}

func newFindStackByResourceCommand() *cobra.Command {
	var resource string
	var exact bool
	var includeNested bool

	cmd := &cobra.Command{
		Use:   "find-stack-by-resource",
		Short: "Find stacks that contain a matching resource",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runFindStackByResource(cmd, resource, exact, includeNested)
		},
		SilenceUsage: true,
	}
	cmd.Flags().StringVar(&resource, "resource", "", "Resource identifier, logical ID, physical ID, or type to match")
	cmd.Flags().BoolVar(&exact, "exact", false, "Require an exact match")
	cmd.Flags().BoolVar(&includeNested, "include-nested", false, "Include nested stacks in the search")

	return cmd
}

func runDeleteStackSet(cmd *cobra.Command, name string) error {
	stackSetName := strings.TrimSpace(name)
	if stackSetName == "" {
		return fmt.Errorf("--stackset-name is required")
	}

	runtime, _, client, err := cliutil.NewServiceRuntime(cmd, loadAWSConfig, newClient)
	if err != nil {
		return err
	}

	targets, err := listStackInstanceTargets(cmd.Context(), client, stackSetName)
	if err != nil {
		return fmt.Errorf("list stack set instances: %s", awstbxaws.FormatUserError(err))
	}

	rows := make([][]string, 0, len(targets)+1)
	for _, target := range targets {
		action := cliutil.ActionWouldDelete
		if !runtime.Options.DryRun {
			action = cliutil.ActionPending
		}
		rows = append(rows, []string{stackSetName, target.Account, target.Region, "stack-instance", action})
	}

	stackSetAction := cliutil.ActionWouldDelete
	if !runtime.Options.DryRun {
		stackSetAction = cliutil.ActionPending
	}
	rows = append(rows, []string{stackSetName, "", "", "stackset", stackSetAction})
	stackSetRow := len(rows) - 1

	if runtime.Options.DryRun {
		return cliutil.WriteDataset(cmd, runtime, []string{"stackset_name", "account", "region", "resource", "action"}, rows)
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
			rows[i][4] = cliutil.ActionCancelled
		}
		return cliutil.WriteDataset(cmd, runtime, []string{"stackset_name", "account", "region", "resource", "action"}, rows)
	}

	instanceFailure := false
	for i, target := range targets {
		opID, deleteErr := deleteStackSetInstanceTarget(cmd.Context(), client, stackSetName, target)
		if deleteErr != nil {
			rows[i][4] = cliutil.FailedActionMessage(awstbxaws.FormatUserError(deleteErr))
			instanceFailure = true
			continue
		}

		if opID != "" {
			waitErr := waitForStackSetOperation(cmd.Context(), client, stackSetName, opID)
			if waitErr != nil {
				rows[i][4] = cliutil.FailedActionMessage(awstbxaws.FormatUserError(waitErr))
				instanceFailure = true
				continue
			}
		}

		rows[i][4] = cliutil.ActionDeleted
	}

	if instanceFailure {
		rows[stackSetRow][4] = cliutil.SkippedActionMessage("stack instance deletion failed")
		return cliutil.WriteDataset(cmd, runtime, []string{"stackset_name", "account", "region", "resource", "action"}, rows)
	}

	_, err = client.DeleteStackSet(cmd.Context(), &cloudformation.DeleteStackSetInput{StackSetName: cliutil.Ptr(stackSetName)})
	if err != nil {
		rows[stackSetRow][4] = cliutil.FailedActionMessage(awstbxaws.FormatUserError(err))
		return cliutil.WriteDataset(cmd, runtime, []string{"stackset_name", "account", "region", "resource", "action"}, rows)
	}

	rows[stackSetRow][4] = cliutil.ActionDeleted
	return cliutil.WriteDataset(cmd, runtime, []string{"stackset_name", "account", "region", "resource", "action"}, rows)
}

func runFindStackByResource(cmd *cobra.Command, resource string, exact, includeNested bool) error {
	query := strings.TrimSpace(resource)
	if query == "" {
		return fmt.Errorf("--resource is required")
	}

	runtime, _, client, err := cliutil.NewServiceRuntime(cmd, loadAWSConfig, newClient)
	if err != nil {
		return err
	}

	stacks, err := listStacksForSearch(cmd.Context(), client, includeNested)
	if err != nil {
		return fmt.Errorf("list stacks: %s", awstbxaws.FormatUserError(err))
	}

	rows := make([][]string, 0)
	for _, stack := range stacks {
		resources, listErr := listStackResources(cmd.Context(), client, cliutil.PointerToString(stack.StackName))
		if listErr != nil {
			return fmt.Errorf("list resources for stack %s: %s", cliutil.PointerToString(stack.StackName), awstbxaws.FormatUserError(listErr))
		}

		for _, item := range resources {
			if !stackResourceMatches(item, query, exact) {
				continue
			}
			rows = append(rows, []string{
				cliutil.PointerToString(stack.StackName),
				cliutil.PointerToString(item.LogicalResourceId),
				cliutil.PointerToString(item.PhysicalResourceId),
				cliutil.PointerToString(item.ResourceType),
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

	return cliutil.WriteDataset(cmd, runtime, []string{"stack_name", "logical_id", "physical_id", "resource_type", "status"}, rows)
}

func listStackInstanceTargets(ctx context.Context, client API, stackSetName string) ([]stackInstanceTarget, error) {
	targets := make([]stackInstanceTarget, 0)
	seen := make(map[string]struct{})
	items, err := awstbxaws.CollectAllPages(ctx, func(callCtx context.Context, nextToken *string) (awstbxaws.PageResult[cloudformationtypes.StackInstanceSummary], error) {
		page, listErr := client.ListStackInstances(callCtx, &cloudformation.ListStackInstancesInput{
			StackSetName: cliutil.Ptr(stackSetName),
			NextToken:    nextToken,
		})
		if listErr != nil {
			return awstbxaws.PageResult[cloudformationtypes.StackInstanceSummary]{}, listErr
		}
		return awstbxaws.PageResult[cloudformationtypes.StackInstanceSummary]{
			Items:     page.Summaries,
			NextToken: page.NextToken,
		}, nil
	})
	if err != nil {
		return nil, err
	}

	for _, item := range items {
		account := strings.TrimSpace(cliutil.PointerToString(item.Account))
		region := strings.TrimSpace(cliutil.PointerToString(item.Region))
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

	sort.Slice(targets, func(i, j int) bool {
		if targets[i].Account == targets[j].Account {
			return targets[i].Region < targets[j].Region
		}
		return targets[i].Account < targets[j].Account
	})

	return targets, nil
}

func deleteStackSetInstanceTarget(ctx context.Context, client API, stackSetName string, target stackInstanceTarget) (string, error) {
	resp, err := client.DeleteStackInstances(ctx, &cloudformation.DeleteStackInstancesInput{
		StackSetName: cliutil.Ptr(stackSetName),
		Accounts:     []string{target.Account},
		Regions:      []string{target.Region},
		RetainStacks: cliutil.Ptr(false),
	})
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(cliutil.PointerToString(resp.OperationId)), nil
}

func waitForStackSetOperation(ctx context.Context, client API, stackSetName, operationID string) error {
	const maxAttempts = 360
	const pollInterval = 5 * time.Second
	for range maxAttempts {
		resp, err := client.DescribeStackSetOperation(ctx, &cloudformation.DescribeStackSetOperationInput{
			StackSetName: cliutil.Ptr(stackSetName),
			OperationId:  cliutil.Ptr(operationID),
		})
		if err != nil {
			return err
		}

		status := resp.StackSetOperation.Status
		switch status {
		case cloudformationtypes.StackSetOperationStatusSucceeded:
			return nil
		case cloudformationtypes.StackSetOperationStatusFailed, cloudformationtypes.StackSetOperationStatusStopped:
			reason := strings.TrimSpace(cliutil.PointerToString(resp.StackSetOperation.StatusReason))
			if reason != "" {
				return fmt.Errorf("stack set operation %s: %s", status, reason)
			}
			return fmt.Errorf("stack set operation %s", status)
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			sleep(pollInterval)
		}
	}

	return fmt.Errorf("timed out waiting for stack set operation %s", operationID)
}

func listStacksForSearch(ctx context.Context, client API, includeNested bool) ([]cloudformationtypes.Stack, error) {
	allStacks, err := awstbxaws.CollectAllPages(ctx, func(callCtx context.Context, nextToken *string) (awstbxaws.PageResult[cloudformationtypes.Stack], error) {
		page, listErr := client.DescribeStacks(callCtx, &cloudformation.DescribeStacksInput{NextToken: nextToken})
		if listErr != nil {
			return awstbxaws.PageResult[cloudformationtypes.Stack]{}, listErr
		}
		return awstbxaws.PageResult[cloudformationtypes.Stack]{
			Items:     page.Stacks,
			NextToken: page.NextToken,
		}, nil
	})
	if err != nil {
		return nil, err
	}

	stacks := make([]cloudformationtypes.Stack, 0, len(allStacks))
	for _, stack := range allStacks {
		if stack.StackStatus == cloudformationtypes.StackStatusDeleteComplete {
			continue
		}
		if !includeNested && stack.ParentId != nil {
			continue
		}
		stacks = append(stacks, stack)
	}

	sort.Slice(stacks, func(i, j int) bool {
		return cliutil.PointerToString(stacks[i].StackName) < cliutil.PointerToString(stacks[j].StackName)
	})

	return stacks, nil
}

func listStackResources(ctx context.Context, client API, stackName string) ([]cloudformationtypes.StackResourceSummary, error) {
	resources, err := awstbxaws.CollectAllPages(ctx, func(callCtx context.Context, nextToken *string) (awstbxaws.PageResult[cloudformationtypes.StackResourceSummary], error) {
		page, listErr := client.ListStackResources(callCtx, &cloudformation.ListStackResourcesInput{
			StackName: cliutil.Ptr(stackName),
			NextToken: nextToken,
		})
		if listErr != nil {
			return awstbxaws.PageResult[cloudformationtypes.StackResourceSummary]{}, listErr
		}
		return awstbxaws.PageResult[cloudformationtypes.StackResourceSummary]{
			Items:     page.StackResourceSummaries,
			NextToken: page.NextToken,
		}, nil
	})
	if err != nil {
		return nil, err
	}

	return resources, nil
}

func stackResourceMatches(resource cloudformationtypes.StackResourceSummary, query string, exact bool) bool {
	if exact {
		for _, candidate := range []string{
			cliutil.PointerToString(resource.LogicalResourceId),
			cliutil.PointerToString(resource.PhysicalResourceId),
			cliutil.PointerToString(resource.ResourceType),
		} {
			if strings.EqualFold(strings.TrimSpace(candidate), query) {
				return true
			}
		}
		return false
	}

	needle := strings.ToLower(query)
	for _, candidate := range []string{
		cliutil.PointerToString(resource.LogicalResourceId),
		cliutil.PointerToString(resource.PhysicalResourceId),
		cliutil.PointerToString(resource.ResourceType),
	} {
		if strings.Contains(strings.ToLower(candidate), needle) {
			return true
		}
	}

	return false
}
