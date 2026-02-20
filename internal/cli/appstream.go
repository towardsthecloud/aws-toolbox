package cli

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
)

type appStreamAPI interface {
	DeleteImage(context.Context, *appstream.DeleteImageInput, ...func(*appstream.Options)) (*appstream.DeleteImageOutput, error)
	DeleteImagePermissions(context.Context, *appstream.DeleteImagePermissionsInput, ...func(*appstream.Options)) (*appstream.DeleteImagePermissionsOutput, error)
	DescribeImagePermissions(context.Context, *appstream.DescribeImagePermissionsInput, ...func(*appstream.Options)) (*appstream.DescribeImagePermissionsOutput, error)
}

var appStreamLoadAWSConfig = awstbxaws.LoadAWSConfig
var appStreamNewClient = func(cfg awssdk.Config) appStreamAPI {
	return appstream.NewFromConfig(cfg)
}

func newAppStreamCommand() *cobra.Command {
	cmd := newServiceGroupCommand("appstream", "Manage AppStream resources")
	cmd.AddCommand(newAppStreamDeleteImageCommand())

	return cmd
}

func newAppStreamDeleteImageCommand() *cobra.Command {
	var imageName string

	cmd := &cobra.Command{
		Use:   "delete-image",
		Short: "Unshare and delete an AppStream image",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runAppStreamDeleteImage(cmd, imageName)
		},
		SilenceUsage: true,
	}
	cmd.Flags().StringVar(&imageName, "image-name", "", "Private AppStream image name")

	return cmd
}

func runAppStreamDeleteImage(cmd *cobra.Command, name string) error {
	imageName := strings.TrimSpace(name)
	if imageName == "" {
		return fmt.Errorf("--image-name is required")
	}

	runtime, _, client, err := newServiceRuntime(cmd, appStreamLoadAWSConfig, appStreamNewClient)
	if err != nil {
		return err
	}

	permissions, err := listAppStreamImagePermissions(cmd.Context(), client, imageName)
	if err != nil {
		return fmt.Errorf("list image permissions: %s", awstbxaws.FormatUserError(err))
	}

	accounts := uniqueSharedAccountIDs(permissions)
	rows := make([][]string, 0, len(accounts)+1)
	for _, accountID := range accounts {
		action := actionWouldDelete
		if !runtime.Options.DryRun {
			action = actionPending
		}
		rows = append(rows, []string{imageName, accountID, "image-permission", action})
	}

	imageAction := actionWouldDelete
	if !runtime.Options.DryRun {
		imageAction = actionPending
	}
	rows = append(rows, []string{imageName, "", "image", imageAction})
	imageRowIndex := len(rows) - 1

	if runtime.Options.DryRun {
		return writeDataset(cmd, runtime, []string{"image_name", "shared_account_id", "resource", "action"}, rows)
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
			rows[i][3] = actionCancelled
		}
		return writeDataset(cmd, runtime, []string{"image_name", "shared_account_id", "resource", "action"}, rows)
	}

	permissionFailure := false
	for i, accountID := range accounts {
		_, deleteErr := client.DeleteImagePermissions(cmd.Context(), &appstream.DeleteImagePermissionsInput{
			Name:            ptr(imageName),
			SharedAccountId: ptr(accountID),
		})
		if deleteErr != nil {
			rows[i][3] = failedActionMessage(awstbxaws.FormatUserError(deleteErr))
			permissionFailure = true
			continue
		}
		rows[i][3] = actionDeleted
	}

	if permissionFailure {
		rows[imageRowIndex][3] = skippedActionMessage("permission cleanup failed")
		return writeDataset(cmd, runtime, []string{"image_name", "shared_account_id", "resource", "action"}, rows)
	}

	_, err = client.DeleteImage(cmd.Context(), &appstream.DeleteImageInput{Name: ptr(imageName)})
	if err != nil {
		rows[imageRowIndex][3] = failedActionMessage(awstbxaws.FormatUserError(err))
		return writeDataset(cmd, runtime, []string{"image_name", "shared_account_id", "resource", "action"}, rows)
	}

	rows[imageRowIndex][3] = actionDeleted
	return writeDataset(cmd, runtime, []string{"image_name", "shared_account_id", "resource", "action"}, rows)
}

func listAppStreamImagePermissions(ctx context.Context, client appStreamAPI, imageName string) ([]appstreamtypes.SharedImagePermissions, error) {
	return awstbxaws.CollectAllPages(ctx, func(callCtx context.Context, nextToken *string) (awstbxaws.PageResult[appstreamtypes.SharedImagePermissions], error) {
		page, err := client.DescribeImagePermissions(callCtx, &appstream.DescribeImagePermissionsInput{
			Name:      ptr(imageName),
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
		accountID := strings.TrimSpace(pointerToString(item.SharedAccountId))
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
