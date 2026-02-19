package cli

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
)

func runEC2DeleteSnapshots(cmd *cobra.Command, retentionDays int) error {
	if retentionDays < 0 {
		return fmt.Errorf("--retention-days must be >= 0")
	}

	runtime, err := newCommandRuntime(cmd)
	if err != nil {
		return err
	}

	cfg, err := ec2LoadAWSConfig(runtime.Options.Profile, runtime.Options.Region)
	if err != nil {
		return fmt.Errorf("load AWS config: %w", err)
	}
	client := ec2NewClient(cfg)

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

	targets := make([]ec2types.Snapshot, 0)
	for _, snapshot := range snapshots {
		snapshotID := pointerToString(snapshot.SnapshotId)
		if snapshotID == "" {
			continue
		}
		if _, used := usedSnapshots[snapshotID]; used {
			continue
		}
		if retentionDays > 0 && snapshot.StartTime != nil && snapshot.StartTime.After(cutoff) {
			continue
		}

		volumeID := pointerToString(snapshot.VolumeId)
		if volumeID != "" {
			exists, existsErr := volumeExists(cmd.Context(), client, volumeID)
			if existsErr != nil {
				return existsErr
			}
			if exists {
				continue
			}
		}

		targets = append(targets, snapshot)
	}

	sort.Slice(targets, func(i, j int) bool {
		return pointerToString(targets[i].SnapshotId) < pointerToString(targets[j].SnapshotId)
	})

	rows := make([][]string, 0, len(targets))
	for _, snapshot := range targets {
		action := "would-delete"
		if !runtime.Options.DryRun {
			action = "pending"
		}
		rows = append(rows, []string{
			pointerToString(snapshot.SnapshotId),
			pointerToString(snapshot.VolumeId),
			cfg.Region,
			action,
		})
	}

	if len(targets) == 0 {
		return writeDataset(cmd, runtime, []string{"snapshot_id", "volume_id", "region", "action"}, rows)
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
				rows[i][3] = "cancelled"
			}
			return writeDataset(cmd, runtime, []string{"snapshot_id", "volume_id", "region", "action"}, rows)
		}

		for i, snapshot := range targets {
			_, deleteErr := client.DeleteSnapshot(cmd.Context(), &ec2.DeleteSnapshotInput{SnapshotId: snapshot.SnapshotId})
			if deleteErr != nil {
				rows[i][3] = "failed: " + awstbxaws.FormatUserError(deleteErr)
				continue
			}
			rows[i][3] = "deleted"
		}
	}

	return writeDataset(cmd, runtime, []string{"snapshot_id", "volume_id", "region", "action"}, rows)
}

func runEC2DeleteVolumes(cmd *cobra.Command, _ []string) error {
	runtime, err := newCommandRuntime(cmd)
	if err != nil {
		return err
	}

	cfg, err := ec2LoadAWSConfig(runtime.Options.Profile, runtime.Options.Region)
	if err != nil {
		return fmt.Errorf("load AWS config: %w", err)
	}
	client := ec2NewClient(cfg)

	volumes, err := listUnattachedVolumes(cmd.Context(), client)
	if err != nil {
		return fmt.Errorf("list volumes: %s", awstbxaws.FormatUserError(err))
	}

	sort.Slice(volumes, func(i, j int) bool {
		return pointerToString(volumes[i].VolumeId) < pointerToString(volumes[j].VolumeId)
	})

	rows := make([][]string, 0, len(volumes))
	for _, volume := range volumes {
		action := "would-delete"
		if !runtime.Options.DryRun {
			action = "pending"
		}
		rows = append(rows, []string{
			pointerToString(volume.VolumeId),
			fmt.Sprintf("%d", pointerToInt32(volume.Size)),
			cfg.Region,
			action,
		})
	}

	if len(volumes) == 0 {
		return writeDataset(cmd, runtime, []string{"volume_id", "size_gib", "region", "action"}, rows)
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
				rows[i][3] = "cancelled"
			}
			return writeDataset(cmd, runtime, []string{"volume_id", "size_gib", "region", "action"}, rows)
		}

		for i, volume := range volumes {
			_, deleteErr := client.DeleteVolume(cmd.Context(), &ec2.DeleteVolumeInput{VolumeId: volume.VolumeId})
			if deleteErr != nil {
				rows[i][3] = "failed: " + awstbxaws.FormatUserError(deleteErr)
				continue
			}
			rows[i][3] = "deleted"
		}
	}

	return writeDataset(cmd, runtime, []string{"volume_id", "size_gib", "region", "action"}, rows)
}

func listSnapshots(ctx context.Context, client ec2API) ([]ec2types.Snapshot, error) {
	items := make([]ec2types.Snapshot, 0)
	var nextToken *string
	for {
		page, err := client.DescribeSnapshots(ctx, &ec2.DescribeSnapshotsInput{OwnerIds: []string{"self"}, NextToken: nextToken})
		if err != nil {
			return nil, err
		}
		items = append(items, page.Snapshots...)
		if page.NextToken == nil || *page.NextToken == "" {
			break
		}
		nextToken = page.NextToken
	}
	return items, nil
}

func listSnapshotIDsUsedByAMIs(ctx context.Context, client ec2API) (map[string]struct{}, error) {
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

func volumeExists(ctx context.Context, client ec2API, volumeID string) (bool, error) {
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

func listUnattachedVolumes(ctx context.Context, client ec2API) ([]ec2types.Volume, error) {
	items := make([]ec2types.Volume, 0)
	var nextToken *string
	for {
		page, err := client.DescribeVolumes(ctx, &ec2.DescribeVolumesInput{
			Filters:   []ec2types.Filter{{Name: ptr("status"), Values: []string{"available"}}},
			NextToken: nextToken,
		})
		if err != nil {
			return nil, err
		}
		items = append(items, page.Volumes...)
		if page.NextToken == nil || *page.NextToken == "" {
			break
		}
		nextToken = page.NextToken
	}
	return items, nil
}
