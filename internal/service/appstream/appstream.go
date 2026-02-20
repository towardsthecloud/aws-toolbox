package appstream

import (
	"context"
	"fmt"
	"sort"
	"strings"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/appstream"
	appstreamtypes "github.com/aws/aws-sdk-go-v2/service/appstream/types"
	"github.com/spf13/cobra"
	awstbxaws "github.com/towardsthecloud/aws-toolbox/internal/aws"
	"github.com/towardsthecloud/aws-toolbox/internal/cliutil"
)

// API is the subset of the AppStream client used by this package.
type API interface {
	DeleteImage(context.Context, *appstream.DeleteImageInput, ...func(*appstream.Options)) (*appstream.DeleteImageOutput, error)
	DeleteImagePermissions(context.Context, *appstream.DeleteImagePermissionsInput, ...func(*appstream.Options)) (*appstream.DeleteImagePermissionsOutput, error)
	DescribeImagePermissions(context.Context, *appstream.DescribeImagePermissionsInput, ...func(*appstream.Options)) (*appstream.DescribeImagePermissionsOutput, error)
}

var loadAWSConfig = awstbxaws.LoadAWSConfig
var newClient = func(cfg awssdk.Config) API {
	return appstream.NewFromConfig(cfg)
}

// NewCommand returns the appstream service group command.
func NewCommand() *cobra.Command {
	cmd := cliutil.NewServiceGroupCommand("appstream", "Manage AppStream resources")
	cmd.AddCommand(newDeleteImageCommand())

	return cmd
}

func newDeleteImageCommand() *cobra.Command {
	var imageName string

	cmd := &cobra.Command{
		Use:   "delete-image",
		Short: "Unshare and delete an AppStream image",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runDeleteImage(cmd, imageName)
		},
		SilenceUsage: true,
	}
	cmd.Flags().StringVar(&imageName, "image-name", "", "Private AppStream image name")

	return cmd
}

func runDeleteImage(cmd *cobra.Command, name string) error {
	imageName := strings.TrimSpace(name)
	if imageName == "" {
		return fmt.Errorf("--image-name is required")
	}

	runtime, _, client, err := cliutil.NewServiceRuntime(cmd, loadAWSConfig, newClient)
	if err != nil {
		return err
	}

	permissions, err := listImagePermissions(cmd.Context(), client, imageName)
	if err != nil {
		return fmt.Errorf("list image permissions: %s", awstbxaws.FormatUserError(err))
	}

	accounts := uniqueSharedAccountIDs(permissions)
	rows := make([][]string, 0, len(accounts)+1)
	for _, accountID := range accounts {
		action := cliutil.ActionWouldDelete
		if !runtime.Options.DryRun {
			action = cliutil.ActionPending
		}
		rows = append(rows, []string{imageName, accountID, "image-permission", action})
	}

	imageAction := cliutil.ActionWouldDelete
	if !runtime.Options.DryRun {
		imageAction = cliutil.ActionPending
	}
	rows = append(rows, []string{imageName, "", "image", imageAction})
	imageRowIndex := len(rows) - 1

	if runtime.Options.DryRun {
		return cliutil.WriteDataset(cmd, runtime, []string{"image_name", "shared_account_id", "resource", "action"}, rows)
	}

	ok, err := runtime.Prompter.Confirm(
		fmt.Sprintf("Delete AppStream image %q and revoke sharing with %d account(s)", imageName, len(accounts)),
		runtime.Options.NoConfirm,
	)
	if err != nil {
		return err
	}
	if !ok {
		for i := range rows {
			rows[i][3] = cliutil.ActionCancelled
		}
		return cliutil.WriteDataset(cmd, runtime, []string{"image_name", "shared_account_id", "resource", "action"}, rows)
	}

	permissionFailure := false
	for i, accountID := range accounts {
		_, deleteErr := client.DeleteImagePermissions(cmd.Context(), &appstream.DeleteImagePermissionsInput{
			Name:            cliutil.Ptr(imageName),
			SharedAccountId: cliutil.Ptr(accountID),
		})
		if deleteErr != nil {
			rows[i][3] = cliutil.FailedActionMessage(awstbxaws.FormatUserError(deleteErr))
			permissionFailure = true
			continue
		}
		rows[i][3] = cliutil.ActionDeleted
	}

	if permissionFailure {
		rows[imageRowIndex][3] = cliutil.SkippedActionMessage("permission cleanup failed")
		return cliutil.WriteDataset(cmd, runtime, []string{"image_name", "shared_account_id", "resource", "action"}, rows)
	}

	_, err = client.DeleteImage(cmd.Context(), &appstream.DeleteImageInput{Name: cliutil.Ptr(imageName)})
	if err != nil {
		rows[imageRowIndex][3] = cliutil.FailedActionMessage(awstbxaws.FormatUserError(err))
		return cliutil.WriteDataset(cmd, runtime, []string{"image_name", "shared_account_id", "resource", "action"}, rows)
	}

	rows[imageRowIndex][3] = cliutil.ActionDeleted
	return cliutil.WriteDataset(cmd, runtime, []string{"image_name", "shared_account_id", "resource", "action"}, rows)
}

func listImagePermissions(ctx context.Context, client API, imageName string) ([]appstreamtypes.SharedImagePermissions, error) {
	return awstbxaws.CollectAllPages(ctx, func(callCtx context.Context, nextToken *string) (awstbxaws.PageResult[appstreamtypes.SharedImagePermissions], error) {
		page, err := client.DescribeImagePermissions(callCtx, &appstream.DescribeImagePermissionsInput{
			Name:      cliutil.Ptr(imageName),
			NextToken: nextToken,
		})
		if err != nil {
			return awstbxaws.PageResult[appstreamtypes.SharedImagePermissions]{}, err
		}
		return awstbxaws.PageResult[appstreamtypes.SharedImagePermissions]{
			Items:     page.SharedImagePermissionsList,
			NextToken: page.NextToken,
		}, nil
	})
}

func uniqueSharedAccountIDs(permissions []appstreamtypes.SharedImagePermissions) []string {
	seen := make(map[string]struct{}, len(permissions))
	accounts := make([]string, 0, len(permissions))

	for _, item := range permissions {
		accountID := strings.TrimSpace(cliutil.PointerToString(item.SharedAccountId))
		if accountID == "" {
			continue
		}
		if _, exists := seen[accountID]; exists {
			continue
		}
		seen[accountID] = struct{}{}
		accounts = append(accounts, accountID)
	}

	sort.Strings(accounts)
	return accounts
}
