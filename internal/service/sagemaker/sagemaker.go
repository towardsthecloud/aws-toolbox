package sagemaker

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sagemaker"
	sagemakertypes "github.com/aws/aws-sdk-go-v2/service/sagemaker/types"
	"github.com/spf13/cobra"
	awstbxaws "github.com/towardsthecloud/aws-toolbox/internal/aws"
	"github.com/towardsthecloud/aws-toolbox/internal/cliutil"
)

// API is the subset of the SageMaker client used by this package.
type API interface {
	DeleteApp(context.Context, *sagemaker.DeleteAppInput, ...func(*sagemaker.Options)) (*sagemaker.DeleteAppOutput, error)
	DeleteSpace(context.Context, *sagemaker.DeleteSpaceInput, ...func(*sagemaker.Options)) (*sagemaker.DeleteSpaceOutput, error)
	DeleteUserProfile(context.Context, *sagemaker.DeleteUserProfileInput, ...func(*sagemaker.Options)) (*sagemaker.DeleteUserProfileOutput, error)
	DescribeSpace(context.Context, *sagemaker.DescribeSpaceInput, ...func(*sagemaker.Options)) (*sagemaker.DescribeSpaceOutput, error)
	ListApps(context.Context, *sagemaker.ListAppsInput, ...func(*sagemaker.Options)) (*sagemaker.ListAppsOutput, error)
	ListDomains(context.Context, *sagemaker.ListDomainsInput, ...func(*sagemaker.Options)) (*sagemaker.ListDomainsOutput, error)
	ListSpaces(context.Context, *sagemaker.ListSpacesInput, ...func(*sagemaker.Options)) (*sagemaker.ListSpacesOutput, error)
}

var loadAWSConfig = awstbxaws.LoadAWSConfig
var newClient = func(cfg awssdk.Config) API {
	return sagemaker.NewFromConfig(cfg)
}
var sleep = time.Sleep

// NewCommand returns the sagemaker service group command.
func NewCommand() *cobra.Command {
	cmd := cliutil.NewServiceGroupCommand("sagemaker", "Manage SageMaker resources")

	cmd.AddCommand(newCleanupSpacesCommand())
	cmd.AddCommand(newDeleteUserProfileCommand())

	return cmd
}

func newCleanupSpacesCommand() *cobra.Command {
	var domainID string
	var spaceNames []string

	cmd := &cobra.Command{
		Use:   "cleanup-spaces",
		Short: "Delete SageMaker spaces",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runCleanupSpaces(cmd, domainID, spaceNames)
		},
		SilenceUsage: true,
	}
	cmd.Flags().StringVar(&domainID, "domain-id", "", "Optional SageMaker domain ID (defaults to all domains)")
	cmd.Flags().StringSliceVar(&spaceNames, "spaces", nil, "Optional comma-separated list of space names (requires --domain-id)")

	return cmd
}

func newDeleteUserProfileCommand() *cobra.Command {
	var domainID string
	var userProfile string

	cmd := &cobra.Command{
		Use:   "delete-user-profile",
		Short: "Delete a SageMaker user profile and dependencies",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runDeleteUserProfile(cmd, domainID, userProfile)
		},
		SilenceUsage: true,
	}
	cmd.Flags().StringVar(&domainID, "domain-id", "", "SageMaker domain ID")
	cmd.Flags().StringVar(&userProfile, "user-profile", "", "SageMaker user profile name")

	return cmd
}

type sageMakerSpaceTarget struct {
	domainID  string
	spaceName string
	status    string
}

type sageMakerDeleteOperation struct {
	step     string
	resource string
	execute  func(context.Context) error
	rowIndex int
}

func runCleanupSpaces(cmd *cobra.Command, domainID string, spaceNames []string) error {
	domain := strings.TrimSpace(domainID)
	if len(spaceNames) > 0 && domain == "" {
		return fmt.Errorf("--domain-id is required when --spaces is set")
	}

	runtime, _, client, err := cliutil.NewServiceRuntime(cmd, loadAWSConfig, newClient)
	if err != nil {
		return err
	}

	domainIDs := make([]string, 0)
	if domain != "" {
		domainIDs = append(domainIDs, domain)
	} else {
		domainIDs, err = listDomainIDs(cmd.Context(), client)
		if err != nil {
			return fmt.Errorf("list SageMaker domains: %s", awstbxaws.FormatUserError(err))
		}
	}

	requestedSpaces := make(map[string]struct{}, len(spaceNames))
	for _, name := range spaceNames {
		trimmed := strings.TrimSpace(name)
		if trimmed != "" {
			requestedSpaces[trimmed] = struct{}{}
		}
	}

	targets := make([]sageMakerSpaceTarget, 0)
	for _, currentDomainID := range domainIDs {
		spaces, listErr := listSpaces(cmd.Context(), client, currentDomainID)
		if listErr != nil {
			return fmt.Errorf("list spaces for domain %s: %s", currentDomainID, awstbxaws.FormatUserError(listErr))
		}

		for _, space := range spaces {
			spaceName := cliutil.PointerToString(space.SpaceName)
			if spaceName == "" {
				continue
			}
			if len(requestedSpaces) > 0 {
				if _, ok := requestedSpaces[spaceName]; !ok {
					continue
				}
			}

			status := string(space.Status)
			if strings.EqualFold(status, "Deleting") || strings.EqualFold(status, "Delete_Failed") {
				continue
			}

			targets = append(targets, sageMakerSpaceTarget{domainID: currentDomainID, spaceName: spaceName, status: status})
		}
	}

	sort.Slice(targets, func(i, j int) bool {
		if targets[i].domainID == targets[j].domainID {
			return targets[i].spaceName < targets[j].spaceName
		}
		return targets[i].domainID < targets[j].domainID
	})

	rows := make([][]string, 0, len(targets))
	for _, target := range targets {
		action := cliutil.ActionWouldDelete
		if !runtime.Options.DryRun {
			action = cliutil.ActionPending
		}
		rows = append(rows, []string{target.domainID, target.spaceName, target.status, action})
	}

	if len(targets) == 0 || runtime.Options.DryRun {
		return cliutil.WriteDataset(cmd, runtime, []string{"domain_id", "space_name", "status", "action"}, rows)
	}

	ok, confirmErr := runtime.Prompter.Confirm(
		fmt.Sprintf("Delete %d SageMaker space(s)", len(targets)),
		runtime.Options.NoConfirm,
	)
	if confirmErr != nil {
		return confirmErr
	}
	if !ok {
		for i := range rows {
			rows[i][3] = cliutil.ActionCancelled
		}
		return cliutil.WriteDataset(cmd, runtime, []string{"domain_id", "space_name", "status", "action"}, rows)
	}

	for i, target := range targets {
		_, deleteErr := client.DeleteSpace(cmd.Context(), &sagemaker.DeleteSpaceInput{
			DomainId:  cliutil.Ptr(target.domainID),
			SpaceName: cliutil.Ptr(target.spaceName),
		})
		if deleteErr != nil {
			rows[i][3] = cliutil.FailedActionMessage(awstbxaws.FormatUserError(deleteErr))
			continue
		}
		rows[i][3] = cliutil.ActionDeleted
	}

	return cliutil.WriteDataset(cmd, runtime, []string{"domain_id", "space_name", "status", "action"}, rows)
}

func runDeleteUserProfile(cmd *cobra.Command, domainID, userProfile string) error {
	domain := strings.TrimSpace(domainID)
	profile := strings.TrimSpace(userProfile)
	if domain == "" {
		return fmt.Errorf("--domain-id is required")
	}
	if profile == "" {
		return fmt.Errorf("--user-profile is required")
	}

	runtime, _, client, err := cliutil.NewServiceRuntime(cmd, loadAWSConfig, newClient)
	if err != nil {
		return err
	}

	rows := make([][]string, 0)
	operations := make([]sageMakerDeleteOperation, 0)
	addOperation := func(step, resource string, execute func(context.Context) error) {
		action := cliutil.ActionWouldDelete
		if !runtime.Options.DryRun {
			action = cliutil.ActionPending
		}
		rows = append(rows, []string{domain, profile, step, resource, action})
		operations = append(operations, sageMakerDeleteOperation{step: step, resource: resource, execute: execute, rowIndex: len(rows) - 1})
	}

	apps, err := listUserProfileApps(cmd.Context(), client, domain, profile, false)
	if err != nil {
		return fmt.Errorf("list apps for user profile %s: %s", profile, awstbxaws.FormatUserError(err))
	}
	for _, app := range apps {
		appName := cliutil.PointerToString(app.AppName)
		if appName == "" {
			continue
		}
		appType := app.AppType
		resource := appName + " (" + string(appType) + ")"
		addOperation("app", resource, func(callCtx context.Context) error {
			_, deleteErr := client.DeleteApp(callCtx, &sagemaker.DeleteAppInput{
				AppName:         cliutil.Ptr(appName),
				AppType:         appType,
				DomainId:        cliutil.Ptr(domain),
				UserProfileName: cliutil.Ptr(profile),
			})
			return deleteErr
		})
	}

	spaces, err := listUserProfileSpaces(cmd.Context(), client, domain, profile, false)
	if err != nil {
		return fmt.Errorf("list spaces for user profile %s: %s", profile, awstbxaws.FormatUserError(err))
	}
	for _, spaceName := range spaces {
		currentSpaceName := spaceName
		addOperation("space", currentSpaceName, func(callCtx context.Context) error {
			_, deleteErr := client.DeleteSpace(callCtx, &sagemaker.DeleteSpaceInput{
				DomainId:  cliutil.Ptr(domain),
				SpaceName: cliutil.Ptr(currentSpaceName),
			})
			return deleteErr
		})
	}

	addOperation("user-profile", profile, func(callCtx context.Context) error {
		_, deleteErr := client.DeleteUserProfile(callCtx, &sagemaker.DeleteUserProfileInput{
			DomainId:        cliutil.Ptr(domain),
			UserProfileName: cliutil.Ptr(profile),
		})
		return deleteErr
	})

	if runtime.Options.DryRun {
		return cliutil.WriteDataset(cmd, runtime, []string{"domain_id", "user_profile", "step", "resource", "action"}, rows)
	}

	ok, confirmErr := runtime.Prompter.Confirm(
		fmt.Sprintf("Delete SageMaker user profile %q and %d dependency item(s)", profile, len(operations)-1),
		runtime.Options.NoConfirm,
	)
	if confirmErr != nil {
		return confirmErr
	}
	if !ok {
		for i := range rows {
			if rows[i][4] == cliutil.ActionPending {
				rows[i][4] = cliutil.ActionCancelled
			}
		}
		return cliutil.WriteDataset(cmd, runtime, []string{"domain_id", "user_profile", "step", "resource", "action"}, rows)
	}

	var dependencyFailure bool
	var userProfileOperation *sageMakerDeleteOperation
	for i := range operations {
		operation := &operations[i]
		if operation.step == "user-profile" {
			userProfileOperation = operation
			continue
		}

		execErr := operation.execute(cmd.Context())
		if execErr != nil {
			rows[operation.rowIndex][4] = cliutil.FailedActionMessage(awstbxaws.FormatUserError(execErr))
			dependencyFailure = true
			continue
		}
		rows[operation.rowIndex][4] = cliutil.ActionDeleted
	}

	if userProfileOperation == nil {
		return cliutil.WriteDataset(cmd, runtime, []string{"domain_id", "user_profile", "step", "resource", "action"}, rows)
	}

	if dependencyFailure {
		rows[userProfileOperation.rowIndex][4] = cliutil.SkippedActionMessage("dependency cleanup failed")
		return cliutil.WriteDataset(cmd, runtime, []string{"domain_id", "user_profile", "step", "resource", "action"}, rows)
	}

	waitErr := waitForUserProfileDependenciesDeleted(cmd.Context(), client, domain, profile)
	if waitErr != nil {
		rows[userProfileOperation.rowIndex][4] = cliutil.FailedActionMessage(awstbxaws.FormatUserError(waitErr))
		return cliutil.WriteDataset(cmd, runtime, []string{"domain_id", "user_profile", "step", "resource", "action"}, rows)
	}

	execErr := userProfileOperation.execute(cmd.Context())
	if execErr != nil {
		rows[userProfileOperation.rowIndex][4] = cliutil.FailedActionMessage(awstbxaws.FormatUserError(execErr))
		return cliutil.WriteDataset(cmd, runtime, []string{"domain_id", "user_profile", "step", "resource", "action"}, rows)
	}
	rows[userProfileOperation.rowIndex][4] = cliutil.ActionDeleted

	return cliutil.WriteDataset(cmd, runtime, []string{"domain_id", "user_profile", "step", "resource", "action"}, rows)
}

func waitForUserProfileDependenciesDeleted(ctx context.Context, client API, domainID, userProfile string) error {
	const maxAttempts = 120
	const pollInterval = 5 * time.Second

	for range maxAttempts {
		apps, err := listUserProfileApps(ctx, client, domainID, userProfile, true)
		if err != nil {
			return fmt.Errorf("list apps for user profile %s: %w", userProfile, err)
		}

		spaces, err := listUserProfileSpaces(ctx, client, domainID, userProfile, true)
		if err != nil {
			return fmt.Errorf("list spaces for user profile %s: %w", userProfile, err)
		}

		if len(apps) == 0 && len(spaces) == 0 {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			sleep(pollInterval)
		}
	}

	return fmt.Errorf("timed out waiting for dependencies to delete for user profile %s", userProfile)
}

func listDomainIDs(ctx context.Context, client API) ([]string, error) {
	domains, err := awstbxaws.CollectAllPages(ctx, func(callCtx context.Context, nextToken *string) (awstbxaws.PageResult[sagemakertypes.DomainDetails], error) {
		page, listErr := client.ListDomains(callCtx, &sagemaker.ListDomainsInput{NextToken: nextToken})
		if listErr != nil {
			return awstbxaws.PageResult[sagemakertypes.DomainDetails]{}, listErr
		}
		return awstbxaws.PageResult[sagemakertypes.DomainDetails]{
			Items:     page.Domains,
			NextToken: page.NextToken,
		}, nil
	})
	if err != nil {
		return nil, err
	}

	items := make([]string, 0, len(domains))
	for _, domain := range domains {
		domainID := cliutil.PointerToString(domain.DomainId)
		if domainID != "" {
			items = append(items, domainID)
		}
	}

	sort.Strings(items)
	return items, nil
}

func listSpaces(ctx context.Context, client API, domainID string) ([]sagemakertypes.SpaceDetails, error) {
	return awstbxaws.CollectAllPages(ctx, func(callCtx context.Context, nextToken *string) (awstbxaws.PageResult[sagemakertypes.SpaceDetails], error) {
		page, err := client.ListSpaces(callCtx, &sagemaker.ListSpacesInput{DomainIdEquals: cliutil.Ptr(domainID), NextToken: nextToken})
		if err != nil {
			return awstbxaws.PageResult[sagemakertypes.SpaceDetails]{}, err
		}
		return awstbxaws.PageResult[sagemakertypes.SpaceDetails]{
			Items:     page.Spaces,
			NextToken: page.NextToken,
		}, nil
	})
}

func listUserProfileApps(ctx context.Context, client API, domainID, userProfile string, includeDeleting bool) ([]sagemakertypes.AppDetails, error) {
	apps, err := awstbxaws.CollectAllPages(ctx, func(callCtx context.Context, nextToken *string) (awstbxaws.PageResult[sagemakertypes.AppDetails], error) {
		page, listErr := client.ListApps(callCtx, &sagemaker.ListAppsInput{
			DomainIdEquals:        cliutil.Ptr(domainID),
			UserProfileNameEquals: cliutil.Ptr(userProfile),
			NextToken:             nextToken,
		})
		if listErr != nil {
			return awstbxaws.PageResult[sagemakertypes.AppDetails]{}, listErr
		}
		return awstbxaws.PageResult[sagemakertypes.AppDetails]{
			Items:     page.Apps,
			NextToken: page.NextToken,
		}, nil
	})
	if err != nil {
		return nil, err
	}

	items := make([]sagemakertypes.AppDetails, 0, len(apps))
	for _, app := range apps {
		status := string(app.Status)
		if strings.EqualFold(status, "Deleted") {
			continue
		}
		if !includeDeleting && strings.EqualFold(status, "Deleting") {
			continue
		}
		items = append(items, app)
	}

	sort.Slice(items, func(i, j int) bool {
		return cliutil.PointerToString(items[i].AppName) < cliutil.PointerToString(items[j].AppName)
	})
	return items, nil
}

func listUserProfileSpaces(ctx context.Context, client API, domainID, userProfile string, includeDeleting bool) ([]string, error) {
	spaces, err := listSpaces(ctx, client, domainID)
	if err != nil {
		return nil, err
	}

	items := make([]string, 0)
	for _, space := range spaces {
		spaceName := cliutil.PointerToString(space.SpaceName)
		if spaceName == "" {
			continue
		}

		detail, describeErr := client.DescribeSpace(ctx, &sagemaker.DescribeSpaceInput{
			DomainId:  cliutil.Ptr(domainID),
			SpaceName: cliutil.Ptr(spaceName),
		})
		if describeErr != nil {
			return nil, describeErr
		}
		if detail.OwnershipSettings == nil || cliutil.PointerToString(detail.OwnershipSettings.OwnerUserProfileName) != userProfile {
			continue
		}
		status := string(detail.Status)
		if strings.EqualFold(status, "Deleted") {
			continue
		}
		if !includeDeleting && strings.EqualFold(status, "Deleting") {
			continue
		}
		items = append(items, spaceName)
	}

	sort.Strings(items)
	return items, nil
}
