package efs

import (
	"context"
	"fmt"
	"sort"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/efs"
	efstypes "github.com/aws/aws-sdk-go-v2/service/efs/types"
	"github.com/spf13/cobra"
	awstbxaws "github.com/towardsthecloud/aws-toolbox/internal/aws"
	"github.com/towardsthecloud/aws-toolbox/internal/cliutil"
)

// API is the subset of the EFS client used by this package.
type API interface {
	DeleteFileSystem(context.Context, *efs.DeleteFileSystemInput, ...func(*efs.Options)) (*efs.DeleteFileSystemOutput, error)
	DeleteMountTarget(context.Context, *efs.DeleteMountTargetInput, ...func(*efs.Options)) (*efs.DeleteMountTargetOutput, error)
	DescribeFileSystems(context.Context, *efs.DescribeFileSystemsInput, ...func(*efs.Options)) (*efs.DescribeFileSystemsOutput, error)
	DescribeMountTargets(context.Context, *efs.DescribeMountTargetsInput, ...func(*efs.Options)) (*efs.DescribeMountTargetsOutput, error)
	ListTagsForResource(context.Context, *efs.ListTagsForResourceInput, ...func(*efs.Options)) (*efs.ListTagsForResourceOutput, error)
}

var loadAWSConfig = awstbxaws.LoadAWSConfig
var newClient = func(cfg awssdk.Config) API {
	return efs.NewFromConfig(cfg)
}
var sleep = time.Sleep

// NewCommand returns the efs service group command.
func NewCommand() *cobra.Command {
	cmd := cliutil.NewServiceGroupCommand("efs", "Manage EFS resources")
	cmd.AddCommand(newDeleteFilesystemsCommand())
	return cmd
}

func newDeleteFilesystemsCommand() *cobra.Command {
	var tagFilter string

	cmd := &cobra.Command{
		Use:   "delete-filesystems",
		Short: "Delete EFS file systems and mount targets",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runDeleteFilesystems(cmd, tagFilter)
		},
		SilenceUsage: true,
	}
	cmd.Flags().StringVar(&tagFilter, "filter-tag", "", "Optional tag filter in KEY=VALUE form")

	return cmd
}

type deleteTarget struct {
	fileSystemID   string
	mountTargetIDs []string
}

func runDeleteFilesystems(cmd *cobra.Command, tagFilter string) error {
	tagKey, tagValue, err := cliutil.ParseTagFilter(tagFilter)
	if err != nil {
		return err
	}

	runtime, _, client, err := cliutil.NewServiceRuntime(cmd, loadAWSConfig, newClient)
	if err != nil {
		return err
	}

	fileSystems, err := listFileSystems(cmd.Context(), client)
	if err != nil {
		return fmt.Errorf("list EFS file systems: %s", awstbxaws.FormatUserError(err))
	}

	targets := make([]deleteTarget, 0, len(fileSystems))
	for _, fs := range fileSystems {
		fileSystemID := cliutil.PointerToString(fs.FileSystemId)
		if fileSystemID == "" {
			continue
		}

		if tagKey != "" {
			match, tagErr := fileSystemMatchesTag(cmd.Context(), client, fileSystemID, tagKey, tagValue)
			if tagErr != nil {
				return fmt.Errorf("list tags for file system %s: %s", fileSystemID, awstbxaws.FormatUserError(tagErr))
			}
			if !match {
				continue
			}
		}

		mountTargetIDs, mountErr := listMountTargetIDs(cmd.Context(), client, fileSystemID)
		if mountErr != nil {
			return fmt.Errorf("list mount targets for %s: %s", fileSystemID, awstbxaws.FormatUserError(mountErr))
		}

		targets = append(targets, deleteTarget{fileSystemID: fileSystemID, mountTargetIDs: mountTargetIDs})
	}

	sort.Slice(targets, func(i, j int) bool {
		return targets[i].fileSystemID < targets[j].fileSystemID
	})

	rows := make([][]string, 0, len(targets))
	for _, target := range targets {
		action := cliutil.ActionWouldDelete
		if !runtime.Options.DryRun {
			action = cliutil.ActionPending
		}

		rows = append(rows, []string{target.fileSystemID, fmt.Sprintf("%d", len(target.mountTargetIDs)), action})
	}

	if len(targets) == 0 || runtime.Options.DryRun {
		return cliutil.WriteDataset(cmd, runtime, []string{"file_system_id", "mount_targets", "action"}, rows)
	}

	ok, confirmErr := runtime.Prompter.Confirm(
		fmt.Sprintf("Delete %d EFS file system(s)", len(targets)),
		runtime.Options.NoConfirm,
	)
	if confirmErr != nil {
		return confirmErr
	}
	if !ok {
		for i := range rows {
			rows[i][2] = cliutil.ActionCancelled
		}
		return cliutil.WriteDataset(cmd, runtime, []string{"file_system_id", "mount_targets", "action"}, rows)
	}

	for i, target := range targets {
		deleteFailed := false
		for _, mountTargetID := range target.mountTargetIDs {
			_, deleteErr := client.DeleteMountTarget(cmd.Context(), &efs.DeleteMountTargetInput{MountTargetId: cliutil.Ptr(mountTargetID)})
			if deleteErr != nil {
				rows[i][2] = cliutil.FailedActionMessage(awstbxaws.FormatUserError(deleteErr))
				deleteFailed = true
				break
			}
		}
		if deleteFailed {
			continue
		}
		if len(target.mountTargetIDs) > 0 {
			waitErr := waitForMountTargetsDeleted(cmd.Context(), client, target.fileSystemID)
			if waitErr != nil {
				rows[i][2] = cliutil.FailedActionMessage(awstbxaws.FormatUserError(waitErr))
				continue
			}
		}

		_, deleteErr := client.DeleteFileSystem(cmd.Context(), &efs.DeleteFileSystemInput{FileSystemId: cliutil.Ptr(target.fileSystemID)})
		if deleteErr != nil {
			rows[i][2] = cliutil.FailedActionMessage(awstbxaws.FormatUserError(deleteErr))
			continue
		}
		rows[i][2] = cliutil.ActionDeleted
	}

	return cliutil.WriteDataset(cmd, runtime, []string{"file_system_id", "mount_targets", "action"}, rows)
}

func listFileSystems(ctx context.Context, client API) ([]efstypes.FileSystemDescription, error) {
	return awstbxaws.CollectAllPages(ctx, func(callCtx context.Context, marker *string) (awstbxaws.PageResult[efstypes.FileSystemDescription], error) {
		page, err := client.DescribeFileSystems(callCtx, &efs.DescribeFileSystemsInput{Marker: marker})
		if err != nil {
			return awstbxaws.PageResult[efstypes.FileSystemDescription]{}, err
		}
		return awstbxaws.PageResult[efstypes.FileSystemDescription]{
			Items:     page.FileSystems,
			NextToken: page.NextMarker,
		}, nil
	})
}

func listMountTargetIDs(ctx context.Context, client API, fileSystemID string) ([]string, error) {
	mountTargets, err := awstbxaws.CollectAllPages(ctx, func(callCtx context.Context, marker *string) (awstbxaws.PageResult[efstypes.MountTargetDescription], error) {
		page, listErr := client.DescribeMountTargets(callCtx, &efs.DescribeMountTargetsInput{FileSystemId: cliutil.Ptr(fileSystemID), Marker: marker})
		if listErr != nil {
			return awstbxaws.PageResult[efstypes.MountTargetDescription]{}, listErr
		}
		return awstbxaws.PageResult[efstypes.MountTargetDescription]{
			Items:     page.MountTargets,
			NextToken: page.NextMarker,
		}, nil
	})
	if err != nil {
		return nil, err
	}

	ids := make([]string, 0, len(mountTargets))
	for _, mountTarget := range mountTargets {
		mountTargetID := cliutil.PointerToString(mountTarget.MountTargetId)
		if mountTargetID != "" {
			ids = append(ids, mountTargetID)
		}
	}

	sort.Strings(ids)
	return ids, nil
}

func waitForMountTargetsDeleted(ctx context.Context, client API, fileSystemID string) error {
	const maxAttempts = 120
	const pollInterval = 5 * time.Second

	for range maxAttempts {
		mountTargetIDs, err := listMountTargetIDs(ctx, client, fileSystemID)
		if err != nil {
			return fmt.Errorf("list mount targets for %s: %w", fileSystemID, err)
		}
		if len(mountTargetIDs) == 0 {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			sleep(pollInterval)
		}
	}

	return fmt.Errorf("timed out waiting for EFS mount targets to delete for file system %s", fileSystemID)
}

func fileSystemMatchesTag(ctx context.Context, client API, fileSystemID, tagKey, tagValue string) (bool, error) {
	tags, err := awstbxaws.CollectAllPages(ctx, func(callCtx context.Context, nextToken *string) (awstbxaws.PageResult[efstypes.Tag], error) {
		page, listErr := client.ListTagsForResource(callCtx, &efs.ListTagsForResourceInput{ResourceId: cliutil.Ptr(fileSystemID), NextToken: nextToken})
		if listErr != nil {
			return awstbxaws.PageResult[efstypes.Tag]{}, listErr
		}
		return awstbxaws.PageResult[efstypes.Tag]{
			Items:     page.Tags,
			NextToken: page.NextToken,
		}, nil
	})
	if err != nil {
		return false, err
	}

	for _, tag := range tags {
		if cliutil.PointerToString(tag.Key) == tagKey && cliutil.PointerToString(tag.Value) == tagValue {
			return true, nil
		}
	}

	return false, nil
}
