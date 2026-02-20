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

func runDeleteAMIs(cmd *cobra.Command, retentionDays int, unusedOnly bool) error {
	if retentionDays < 0 {
		return fmt.Errorf("--retention-days must be >= 0")
	}
	if !unusedOnly && retentionDays == 0 {
		return fmt.Errorf("set at least one filter: --unused or --retention-days")
	}

	runtime, cfg, client, err := cliutil.NewServiceRuntime(cmd, loadAWSConfig, newClient)
	if err != nil {
		return err
	}

	ctx := cmd.Context()
	images, err := listOwnedImages(ctx, client)
	if err != nil {
		return fmt.Errorf("list AMIs: %s", awstbxaws.FormatUserError(err))
	}

	usedAMIIDs := map[string]struct{}{}
	if unusedOnly {
		usedAMIIDs, err = listUsedAMIIDs(ctx, client)
		if err != nil {
			return fmt.Errorf("list used AMIs: %s", awstbxaws.FormatUserError(err))
		}
	}

	var cutoff time.Time
	if retentionDays > 0 {
		cutoff = time.Now().UTC().AddDate(0, 0, -retentionDays)
	}

	targets := make([]ec2types.Image, 0)
	for _, image := range images {
		if image.ImageId == nil {
			continue
		}

		if unusedOnly {
			if _, inUse := usedAMIIDs[*image.ImageId]; inUse {
				continue
			}
		}

		if retentionDays > 0 {
			createdAt, parseErr := parseAWSDate(strings.TrimSpace(cliutil.PointerToString(image.CreationDate)))
			if parseErr != nil {
				return fmt.Errorf("parse CreationDate for %s: %w", cliutil.PointerToString(image.ImageId), parseErr)
			}
			if createdAt.After(cutoff) {
				continue
			}
		}

		targets = append(targets, image)
	}

	sort.Slice(targets, func(i, j int) bool {
		return cliutil.PointerToString(targets[i].ImageId) < cliutil.PointerToString(targets[j].ImageId)
	})

	rows := make([][]string, 0, len(targets))
	for _, image := range targets {
		action := cliutil.ActionWouldDelete
		if !runtime.Options.DryRun {
			action = cliutil.ActionPending
		}
		rows = append(rows, []string{
			cliutil.PointerToString(image.ImageId),
			cliutil.PointerToString(image.Name),
			cfg.Region,
			action,
		})
	}

	if len(targets) == 0 {
		return cliutil.WriteDataset(cmd, runtime, []string{"image_id", "name", "region", "action"}, rows)
	}

	if !runtime.Options.DryRun {
		ok, confirmErr := runtime.Prompter.Confirm(
			fmt.Sprintf("Deregister %d AMI(s)", len(targets)),
			runtime.Options.NoConfirm,
		)
		if confirmErr != nil {
			return confirmErr
		}
		if !ok {
			for i := range rows {
				rows[i][3] = cliutil.ActionCancelled
			}
			return cliutil.WriteDataset(cmd, runtime, []string{"image_id", "name", "region", "action"}, rows)
		}

		for i, image := range targets {
			_, deleteErr := client.DeregisterImage(ctx, &ec2.DeregisterImageInput{ImageId: image.ImageId})
			if deleteErr != nil {
				rows[i][3] = cliutil.FailedActionMessage(awstbxaws.FormatUserError(deleteErr))
				continue
			}
			rows[i][3] = cliutil.ActionDeleted
		}
	}

	return cliutil.WriteDataset(cmd, runtime, []string{"image_id", "name", "region", "action"}, rows)
}

func runListEIPs(cmd *cobra.Command, _ []string) error {
	runtime, cfg, client, err := cliutil.NewServiceRuntime(cmd, loadAWSConfig, newClient)
	if err != nil {
		return err
	}

	addresses, err := listAddresses(cmd.Context(), client)
	if err != nil {
		return fmt.Errorf("list addresses: %s", awstbxaws.FormatUserError(err))
	}

	rows := make([][]string, 0)
	for _, address := range addresses {
		if address.AssociationId != nil {
			continue
		}
		rows = append(rows, []string{
			cliutil.PointerToString(address.AllocationId),
			cliutil.PointerToString(address.PublicIp),
			cfg.Region,
			"available",
		})
	}

	sort.Slice(rows, func(i, j int) bool {
		return rows[i][0] < rows[j][0]
	})

	return cliutil.WriteDataset(cmd, runtime, []string{"allocation_id", "public_ip", "region", "status"}, rows)
}

func runDeleteEIPs(cmd *cobra.Command, _ []string) error {
	runtime, cfg, client, err := cliutil.NewServiceRuntime(cmd, loadAWSConfig, newClient)
	if err != nil {
		return err
	}

	addresses, err := listAddresses(cmd.Context(), client)
	if err != nil {
		return fmt.Errorf("list addresses: %s", awstbxaws.FormatUserError(err))
	}

	targets := make([]ec2types.Address, 0)
	for _, address := range addresses {
		if address.AssociationId != nil || address.AllocationId == nil {
			continue
		}
		targets = append(targets, address)
	}

	sort.Slice(targets, func(i, j int) bool {
		return cliutil.PointerToString(targets[i].AllocationId) < cliutil.PointerToString(targets[j].AllocationId)
	})

	rows := make([][]string, 0, len(targets))
	for _, address := range targets {
		action := cliutil.ActionWouldDelete
		if !runtime.Options.DryRun {
			action = cliutil.ActionPending
		}
		rows = append(rows, []string{cliutil.PointerToString(address.AllocationId), cliutil.PointerToString(address.PublicIp), cfg.Region, action})
	}

	if len(targets) == 0 {
		return cliutil.WriteDataset(cmd, runtime, []string{"allocation_id", "public_ip", "region", "action"}, rows)
	}

	if !runtime.Options.DryRun {
		ok, confirmErr := runtime.Prompter.Confirm(
			fmt.Sprintf("Release %d Elastic IP(s)", len(targets)),
			runtime.Options.NoConfirm,
		)
		if confirmErr != nil {
			return confirmErr
		}
		if !ok {
			for i := range rows {
				rows[i][3] = cliutil.ActionCancelled
			}
			return cliutil.WriteDataset(cmd, runtime, []string{"allocation_id", "public_ip", "region", "action"}, rows)
		}

		for i, address := range targets {
			_, releaseErr := client.ReleaseAddress(cmd.Context(), &ec2.ReleaseAddressInput{AllocationId: address.AllocationId})
			if releaseErr != nil {
				rows[i][3] = cliutil.FailedActionMessage(awstbxaws.FormatUserError(releaseErr))
				continue
			}
			rows[i][3] = cliutil.ActionDeleted
		}
	}

	return cliutil.WriteDataset(cmd, runtime, []string{"allocation_id", "public_ip", "region", "action"}, rows)
}

func listAddresses(ctx context.Context, client API) ([]ec2types.Address, error) {
	page, err := client.DescribeAddresses(ctx, &ec2.DescribeAddressesInput{})
	if err != nil {
		return nil, err
	}

	return page.Addresses, nil
}
