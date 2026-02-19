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
	var name string

	cmd := &cobra.Command{
		Use:   "delete-image",
		Short: "Unshare and delete an AppStream image",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runAppStreamDeleteImage(cmd, name)
		},
		SilenceUsage: true,
	}
	cmd.Flags().StringVar(&name, "name", "", "Private AppStream image name")

	return cmd
}

func runAppStreamDeleteImage(cmd *cobra.Command, name string) error {
	imageName := strings.TrimSpace(name)
	if imageName == "" {
		return fmt.Errorf("--name is required")
	}

	runtime, err := newCommandRuntime(cmd)
	if err != nil {
		return err
	}

	cfg, err := appStreamLoadAWSConfig(runtime.Options.Profile, runtime.Options.Region)
	if err != nil {
		return fmt.Errorf("load AWS config: %w", err)
	}
	client := appStreamNewClient(cfg)

	permissions, err := listAppStreamImagePermissions(cmd.Context(), client, imageName)
	if err != nil {
		return fmt.Errorf("list image permissions: %s", awstbxaws.FormatUserError(err))
	}

	accounts := uniqueSharedAccountIDs(permissions)
	rows := make([][]string, 0, len(accounts)+1)
	for _, accountID := range accounts {
		action := "would-unshare"
		if !runtime.Options.DryRun {
			action = "pending"
		}
		rows = append(rows, []string{imageName, accountID, "image-permission", action})
	}

	imageAction := "would-delete"
	if !runtime.Options.DryRun {
		imageAction = "pending"
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
			rows[i][3] = "cancelled"
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
			rows[i][3] = "failed: " + awstbxaws.FormatUserError(deleteErr)
			permissionFailure = true
			continue
		}
		rows[i][3] = "unshared"
	}

	if permissionFailure {
		rows[imageRowIndex][3] = "skipped: permission cleanup failed"
		return writeDataset(cmd, runtime, []string{"image_name", "shared_account_id", "resource", "action"}, rows)
	}

	_, err = client.DeleteImage(cmd.Context(), &appstream.DeleteImageInput{Name: ptr(imageName)})
	if err != nil {
		rows[imageRowIndex][3] = "failed: " + awstbxaws.FormatUserError(err)
		return writeDataset(cmd, runtime, []string{"image_name", "shared_account_id", "resource", "action"}, rows)
	}

	rows[imageRowIndex][3] = "deleted"
	return writeDataset(cmd, runtime, []string{"image_name", "shared_account_id", "resource", "action"}, rows)
}

func listAppStreamImagePermissions(ctx context.Context, client appStreamAPI, imageName string) ([]appstreamtypes.SharedImagePermissions, error) {
	permissions := make([]appstreamtypes.SharedImagePermissions, 0)
	var nextToken *string

	for {
		page, err := client.DescribeImagePermissions(ctx, &appstream.DescribeImagePermissionsInput{
			Name:      ptr(imageName),
			NextToken: nextToken,
		})
		if err != nil {
			return nil, err
		}

		permissions = append(permissions, page.SharedImagePermissionsList...)
		if page.NextToken == nil || *page.NextToken == "" {
			break
		}
		nextToken = page.NextToken
	}

	return permissions, nil
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
