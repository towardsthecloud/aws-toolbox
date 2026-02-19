package cli

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
)

type efsAPI interface {
	DeleteFileSystem(context.Context, *efs.DeleteFileSystemInput, ...func(*efs.Options)) (*efs.DeleteFileSystemOutput, error)
	DeleteMountTarget(context.Context, *efs.DeleteMountTargetInput, ...func(*efs.Options)) (*efs.DeleteMountTargetOutput, error)
	DescribeFileSystems(context.Context, *efs.DescribeFileSystemsInput, ...func(*efs.Options)) (*efs.DescribeFileSystemsOutput, error)
	DescribeMountTargets(context.Context, *efs.DescribeMountTargetsInput, ...func(*efs.Options)) (*efs.DescribeMountTargetsOutput, error)
	ListTagsForResource(context.Context, *efs.ListTagsForResourceInput, ...func(*efs.Options)) (*efs.ListTagsForResourceOutput, error)
}

var efsLoadAWSConfig = awstbxaws.LoadAWSConfig
var efsNewClient = func(cfg awssdk.Config) efsAPI {
	return efs.NewFromConfig(cfg)
}
var efsSleep = time.Sleep

func newEFSCommand() *cobra.Command {
	cmd := newServiceGroupCommand("efs", "Manage EFS resources")

	cmd.AddCommand(newEFSDeleteFilesystemsCommand())

	return cmd
}

func newEFSDeleteFilesystemsCommand() *cobra.Command {
	var tagFilter string

	cmd := &cobra.Command{
		Use:   "delete-filesystems",
		Short: "Delete EFS file systems and mount targets",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runEFSDeleteFilesystems(cmd, tagFilter)
		},
		SilenceUsage: true,
	}
	cmd.Flags().StringVar(&tagFilter, "tag", "", "Optional tag filter in KEY=VALUE form")

	return cmd
}

type efsDeleteTarget struct {
	fileSystemID   string
	mountTargetIDs []string
}

func runEFSDeleteFilesystems(cmd *cobra.Command, tagFilter string) error {
	tagKey, tagValue, err := parseTagFilter(tagFilter)
	if err != nil {
		return err
	}

	runtime, err := newCommandRuntime(cmd)
	if err != nil {
		return err
	}

	cfg, err := efsLoadAWSConfig(runtime.Options.Profile, runtime.Options.Region)
	if err != nil {
		return fmt.Errorf("load AWS config: %w", err)
	}
	client := efsNewClient(cfg)

	fileSystems, err := listEFSFileSystems(cmd.Context(), client)
	if err != nil {
		return fmt.Errorf("list EFS file systems: %s", awstbxaws.FormatUserError(err))
	}

	targets := make([]efsDeleteTarget, 0, len(fileSystems))
	for _, fs := range fileSystems {
		fileSystemID := pointerToString(fs.FileSystemId)
		if fileSystemID == "" {
			continue
		}

		if tagKey != "" {
			match, tagErr := efsFileSystemMatchesTag(cmd.Context(), client, fileSystemID, tagKey, tagValue)
			if tagErr != nil {
				return fmt.Errorf("list tags for file system %s: %s", fileSystemID, awstbxaws.FormatUserError(tagErr))
			}
			if !match {
				continue
			}
		}

		mountTargetIDs, mountErr := listEFSMountTargetIDs(cmd.Context(), client, fileSystemID)
		if mountErr != nil {
			return fmt.Errorf("list mount targets for %s: %s", fileSystemID, awstbxaws.FormatUserError(mountErr))
		}

		targets = append(targets, efsDeleteTarget{fileSystemID: fileSystemID, mountTargetIDs: mountTargetIDs})
	}

	sort.Slice(targets, func(i, j int) bool {
		return targets[i].fileSystemID < targets[j].fileSystemID
	})

	rows := make([][]string, 0, len(targets))
	for _, target := range targets {
		action := "would-delete"
		if !runtime.Options.DryRun {
			action = "pending"
		}

		rows = append(rows, []string{target.fileSystemID, fmt.Sprintf("%d", len(target.mountTargetIDs)), action})
	}

	if len(targets) == 0 || runtime.Options.DryRun {
		return writeDataset(cmd, runtime, []string{"file_system_id", "mount_targets", "action"}, rows)
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
			rows[i][2] = "cancelled"
		}
		return writeDataset(cmd, runtime, []string{"file_system_id", "mount_targets", "action"}, rows)
	}

	for i, target := range targets {
		deleteFailed := false
		for _, mountTargetID := range target.mountTargetIDs {
			_, deleteErr := client.DeleteMountTarget(cmd.Context(), &efs.DeleteMountTargetInput{MountTargetId: ptr(mountTargetID)})
			if deleteErr != nil {
				rows[i][2] = "failed: " + awstbxaws.FormatUserError(deleteErr)
				deleteFailed = true
				break
			}
		}
		if deleteFailed {
			continue
		}
		if len(target.mountTargetIDs) > 0 {
			waitErr := waitForEFSMountTargetsDeleted(cmd.Context(), client, target.fileSystemID)
			if waitErr != nil {
				rows[i][2] = "failed: " + awstbxaws.FormatUserError(waitErr)
				continue
			}
		}

		_, deleteErr := client.DeleteFileSystem(cmd.Context(), &efs.DeleteFileSystemInput{FileSystemId: ptr(target.fileSystemID)})
		if deleteErr != nil {
			rows[i][2] = "failed: " + awstbxaws.FormatUserError(deleteErr)
			continue
		}
		rows[i][2] = "deleted"
	}

	return writeDataset(cmd, runtime, []string{"file_system_id", "mount_targets", "action"}, rows)
}

func listEFSFileSystems(ctx context.Context, client efsAPI) ([]efstypes.FileSystemDescription, error) {
	items := make([]efstypes.FileSystemDescription, 0)
	var marker *string

	for {
		page, err := client.DescribeFileSystems(ctx, &efs.DescribeFileSystemsInput{Marker: marker})
		if err != nil {
			return nil, err
		}

		items = append(items, page.FileSystems...)
		if page.NextMarker == nil || *page.NextMarker == "" {
			break
		}
		marker = page.NextMarker
	}

	return items, nil
}

func listEFSMountTargetIDs(ctx context.Context, client efsAPI, fileSystemID string) ([]string, error) {
	ids := make([]string, 0)
	var marker *string

	for {
		page, err := client.DescribeMountTargets(ctx, &efs.DescribeMountTargetsInput{FileSystemId: ptr(fileSystemID), Marker: marker})
		if err != nil {
			return nil, err
		}

		for _, mountTarget := range page.MountTargets {
			mountTargetID := pointerToString(mountTarget.MountTargetId)
			if mountTargetID != "" {
				ids = append(ids, mountTargetID)
			}
		}

		if page.NextMarker == nil || *page.NextMarker == "" {
			break
		}
		marker = page.NextMarker
	}

	sort.Strings(ids)
	return ids, nil
}

func waitForEFSMountTargetsDeleted(ctx context.Context, client efsAPI, fileSystemID string) error {
	const maxAttempts = 120
	const pollInterval = 5 * time.Second

	for range maxAttempts {
		mountTargetIDs, err := listEFSMountTargetIDs(ctx, client, fileSystemID)
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
			efsSleep(pollInterval)
		}
	}

	return fmt.Errorf("timed out waiting for EFS mount targets to delete for file system %s", fileSystemID)
}

func efsFileSystemMatchesTag(ctx context.Context, client efsAPI, fileSystemID, tagKey, tagValue string) (bool, error) {
	var nextToken *string

	for {
		page, err := client.ListTagsForResource(ctx, &efs.ListTagsForResourceInput{ResourceId: ptr(fileSystemID), NextToken: nextToken})
		if err != nil {
			return false, err
		}

		for _, tag := range page.Tags {
			if pointerToString(tag.Key) == tagKey && pointerToString(tag.Value) == tagValue {
				return true, nil
			}
		}

		if page.NextToken == nil || *page.NextToken == "" {
			break
		}
		nextToken = page.NextToken
	}

	return false, nil
}
