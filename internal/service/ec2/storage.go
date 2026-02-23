package ec2

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/spf13/cobra"
	awstbxaws "github.com/towardsthecloud/aws-toolbox/internal/aws"
	"github.com/towardsthecloud/aws-toolbox/internal/cliutil"
)

func runDeleteSnapshots(cmd *cobra.Command, retentionDays int) error {
	if retentionDays < 0 {
		return fmt.Errorf("--retention-days must be >= 0")
	}

	runtime, cfg, client, err := cliutil.NewServiceRuntime(cmd, loadAWSConfig, newClient)
	if err != nil {
		return err
	}

	snapshots, err := listSnapshots(cmd.Context(), client)
	if err != nil {
		return fmt.Errorf("list snapshots: %s", awstbxaws.FormatUserError(err))
	}

	usedSnapshots, err := listSnapshotIDsUsedByAMIs(cmd.Context(), client)
	if err != nil {
		return fmt.Errorf("list AMI snapshot references: %s", awstbxaws.FormatUserError(err))
	}

	var cutoff time.Time
	if retentionDays > 0 {
		cutoff = time.Now().UTC().AddDate(0, 0, -retentionDays)
	}

	// Cache volume existence checks so that multiple snapshots referencing the
	// same volume only trigger a single DescribeVolumes API call.
	checkedVolumes := make(map[string]bool)

	targets := make([]ec2types.Snapshot, 0)
	for _, snapshot := range snapshots {
		snapshotID := cliutil.PointerToString(snapshot.SnapshotId)
		if snapshotID == "" {
			continue
		}
		if _, used := usedSnapshots[snapshotID]; used {
			continue
		}
		if retentionDays > 0 && snapshot.StartTime != nil && snapshot.StartTime.After(cutoff) {
			continue
		}

		volumeID := cliutil.PointerToString(snapshot.VolumeId)
		if volumeID != "" {
			exists, ok := checkedVolumes[volumeID]
			if !ok {
				exists, err = volumeExists(cmd.Context(), client, volumeID)
				if err != nil {
					return err
				}
				checkedVolumes[volumeID] = exists
			}
			if exists {
				continue
			}
		}

		targets = append(targets, snapshot)
	}

	sort.Slice(targets, func(i, j int) bool {
		return cliutil.PointerToString(targets[i].SnapshotId) < cliutil.PointerToString(targets[j].SnapshotId)
	})

	rows := make([][]string, 0, len(targets))
	for _, snapshot := range targets {
		action := cliutil.ActionWouldDelete
		if !runtime.Options.DryRun {
			action = cliutil.ActionPending
		}
		rows = append(rows, []string{
			cliutil.PointerToString(snapshot.SnapshotId),
			cliutil.PointerToString(snapshot.VolumeId),
			cfg.Region,
			action,
		})
	}

	if len(targets) == 0 {
		return cliutil.WriteDataset(cmd, runtime, []string{"snapshot_id", "volume_id", "region", "action"}, rows)
	}

	if !runtime.Options.DryRun {
		ok, confirmErr := runtime.Prompter.Confirm(
			fmt.Sprintf("Delete %d orphaned snapshot(s)", len(targets)),
			runtime.Options.NoConfirm,
		)
		if confirmErr != nil {
			return confirmErr
		}
		if !ok {
			for i := range rows {
				rows[i][3] = cliutil.ActionCancelled
			}
			return cliutil.WriteDataset(cmd, runtime, []string{"snapshot_id", "volume_id", "region", "action"}, rows)
		}

		for i, snapshot := range targets {
			_, deleteErr := client.DeleteSnapshot(cmd.Context(), &ec2.DeleteSnapshotInput{SnapshotId: snapshot.SnapshotId})
			if deleteErr != nil {
				rows[i][3] = cliutil.FailedActionMessage(awstbxaws.FormatUserError(deleteErr))
				continue
			}
			rows[i][3] = cliutil.ActionDeleted
		}
	}

	return cliutil.WriteDataset(cmd, runtime, []string{"snapshot_id", "volume_id", "region", "action"}, rows)
}

func runDeleteVolumes(cmd *cobra.Command, _ []string) error {
	runtime, cfg, client, err := cliutil.NewServiceRuntime(cmd, loadAWSConfig, newClient)
	if err != nil {
		return err
	}

	volumes, err := listUnattachedVolumes(cmd.Context(), client)
	if err != nil {
		return fmt.Errorf("list volumes: %s", awstbxaws.FormatUserError(err))
	}

	sort.Slice(volumes, func(i, j int) bool {
		return cliutil.PointerToString(volumes[i].VolumeId) < cliutil.PointerToString(volumes[j].VolumeId)
	})

	rows := make([][]string, 0, len(volumes))
	for _, volume := range volumes {
		action := cliutil.ActionWouldDelete
		if !runtime.Options.DryRun {
			action = cliutil.ActionPending
		}
		rows = append(rows, []string{
			cliutil.PointerToString(volume.VolumeId),
			fmt.Sprintf("%d", cliutil.PointerToInt32(volume.Size)),
			cfg.Region,
			action,
		})
	}

	if len(volumes) == 0 {
		return cliutil.WriteDataset(cmd, runtime, []string{"volume_id", "size_gib", "region", "action"}, rows)
	}

	if !runtime.Options.DryRun {
		ok, confirmErr := runtime.Prompter.Confirm(
			fmt.Sprintf("Delete %d unattached volume(s)", len(volumes)),
			runtime.Options.NoConfirm,
		)
		if confirmErr != nil {
			return confirmErr
		}
		if !ok {
			for i := range rows {
				rows[i][3] = cliutil.ActionCancelled
			}
			return cliutil.WriteDataset(cmd, runtime, []string{"volume_id", "size_gib", "region", "action"}, rows)
		}

		for i, volume := range volumes {
			_, deleteErr := client.DeleteVolume(cmd.Context(), &ec2.DeleteVolumeInput{VolumeId: volume.VolumeId})
			if deleteErr != nil {
				rows[i][3] = cliutil.FailedActionMessage(awstbxaws.FormatUserError(deleteErr))
				continue
			}
			rows[i][3] = cliutil.ActionDeleted
		}
	}

	return cliutil.WriteDataset(cmd, runtime, []string{"volume_id", "size_gib", "region", "action"}, rows)
}

func listSnapshots(ctx context.Context, client API) ([]ec2types.Snapshot, error) {
	return awstbxaws.CollectAllPages(ctx, func(callCtx context.Context, nextToken *string) (awstbxaws.PageResult[ec2types.Snapshot], error) {
		page, err := client.DescribeSnapshots(callCtx, &ec2.DescribeSnapshotsInput{OwnerIds: []string{"self"}, NextToken: nextToken})
		if err != nil {
			return awstbxaws.PageResult[ec2types.Snapshot]{}, err
		}
		return awstbxaws.PageResult[ec2types.Snapshot]{
			Items:     page.Snapshots,
			NextToken: page.NextToken,
		}, nil
	})
}

func listSnapshotIDsUsedByAMIs(ctx context.Context, client API) (map[string]struct{}, error) {
	used := make(map[string]struct{})
	images, err := listOwnedImages(ctx, client)
	if err != nil {
		return nil, err
	}

	for _, image := range images {
		for _, mapping := range image.BlockDeviceMappings {
			if mapping.Ebs != nil && mapping.Ebs.SnapshotId != nil {
				used[*mapping.Ebs.SnapshotId] = struct{}{}
			}
		}
	}

	return used, nil
}

func volumeExists(ctx context.Context, client API, volumeID string) (bool, error) {
	output, err := client.DescribeVolumes(ctx, &ec2.DescribeVolumesInput{VolumeIds: []string{volumeID}})
	if err != nil {
		code := awsErrorCode(err)
		if strings.EqualFold(code, "InvalidVolume.NotFound") {
			return false, nil
		}
		return false, fmt.Errorf("check volume %s: %s", volumeID, awstbxaws.FormatUserError(err))
	}

	return len(output.Volumes) > 0, nil
}

func listUnattachedVolumes(ctx context.Context, client API) ([]ec2types.Volume, error) {
	return awstbxaws.CollectAllPages(ctx, func(callCtx context.Context, nextToken *string) (awstbxaws.PageResult[ec2types.Volume], error) {
		page, err := client.DescribeVolumes(callCtx, &ec2.DescribeVolumesInput{
			Filters:   []ec2types.Filter{{Name: cliutil.Ptr("status"), Values: []string{"available"}}},
			NextToken: nextToken,
		})
		if err != nil {
			return awstbxaws.PageResult[ec2types.Volume]{}, err
		}
		return awstbxaws.PageResult[ec2types.Volume]{
			Items:     page.Volumes,
			NextToken: page.NextToken,
		}, nil
	})
}
