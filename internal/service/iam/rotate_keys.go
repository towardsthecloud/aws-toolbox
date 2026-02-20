package iam

import (
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/spf13/cobra"
	awstbxaws "github.com/towardsthecloud/aws-toolbox/internal/aws"
	"github.com/towardsthecloud/aws-toolbox/internal/cliutil"
)

func runRotateKeys(cmd *cobra.Command, username, keyID string, disable, deleteKey bool) error {
	user := strings.TrimSpace(username)
	key := strings.TrimSpace(keyID)

	if user == "" {
		return fmt.Errorf("--username is required")
	}
	if disable && deleteKey {
		return fmt.Errorf("set at most one of --disable or --delete")
	}
	if (disable || deleteKey) && key == "" {
		return fmt.Errorf("--key is required with --disable or --delete")
	}

	runtime, _, client, err := cliutil.NewServiceRuntime(cmd, loadAWSConfig, newClient)
	if err != nil {
		return err
	}

	keys, err := listAccessKeys(cmd.Context(), client, user)
	if err != nil {
		return fmt.Errorf("list access keys for %s: %s", user, awstbxaws.FormatUserError(err))
	}

	activeCount := 0
	inactiveCount := 0
	for _, metadata := range keys {
		if metadata.Status == iamtypes.StatusTypeActive {
			activeCount++
		}
		if metadata.Status == iamtypes.StatusTypeInactive {
			inactiveCount++
		}
	}

	action := "would-create"
	if disable {
		action = "would-disable"
	}
	if deleteKey {
		action = "would-delete"
	}
	if !runtime.Options.DryRun {
		action = "pending"
	}

	row := []string{user, key, fmt.Sprintf("%d", activeCount), fmt.Sprintf("%d", inactiveCount), action, ""}
	headers := []string{"username", "key_id", "active_keys", "inactive_keys", "action", "secret_access_key"}

	if !disable && !deleteKey && len(keys) >= 2 {
		row[4] = cliutil.FailedActionMessage("user already has 2 keys")
		return cliutil.WriteDataset(cmd, runtime, headers, [][]string{row})
	}

	if runtime.Options.DryRun {
		return cliutil.WriteDataset(cmd, runtime, headers, [][]string{row})
	}

	ok, confirmErr := runtime.Prompter.Confirm(
		fmt.Sprintf("Apply IAM key change for user %q", user),
		runtime.Options.NoConfirm,
	)
	if confirmErr != nil {
		return confirmErr
	}
	if !ok {
		row[4] = "cancelled"
		return cliutil.WriteDataset(cmd, runtime, headers, [][]string{row})
	}

	if disable {
		_, updateErr := client.UpdateAccessKey(cmd.Context(), &iam.UpdateAccessKeyInput{
			UserName:    cliutil.Ptr(user),
			AccessKeyId: cliutil.Ptr(key),
			Status:      iamtypes.StatusTypeInactive,
		})
		if updateErr != nil {
			row[4] = cliutil.FailedActionMessage(awstbxaws.FormatUserError(updateErr))
			return cliutil.WriteDataset(cmd, runtime, headers, [][]string{row})
		}
		row[4] = "disabled"
		return cliutil.WriteDataset(cmd, runtime, headers, [][]string{row})
	}

	if deleteKey {
		_, deleteErr := client.DeleteAccessKey(cmd.Context(), &iam.DeleteAccessKeyInput{
			UserName:    cliutil.Ptr(user),
			AccessKeyId: cliutil.Ptr(key),
		})
		if deleteErr != nil {
			row[4] = cliutil.FailedActionMessage(awstbxaws.FormatUserError(deleteErr))
			return cliutil.WriteDataset(cmd, runtime, headers, [][]string{row})
		}
		row[4] = "deleted"
		return cliutil.WriteDataset(cmd, runtime, headers, [][]string{row})
	}

	createOut, createErr := client.CreateAccessKey(cmd.Context(), &iam.CreateAccessKeyInput{UserName: cliutil.Ptr(user)})
	if createErr != nil {
		row[4] = cliutil.FailedActionMessage(awstbxaws.FormatUserError(createErr))
		return cliutil.WriteDataset(cmd, runtime, headers, [][]string{row})
	}

	row[1] = cliutil.PointerToString(createOut.AccessKey.AccessKeyId)
	row[5] = cliutil.PointerToString(createOut.AccessKey.SecretAccessKey)
	row[4] = "created"

	return cliutil.WriteDataset(cmd, runtime, headers, [][]string{row})
}
