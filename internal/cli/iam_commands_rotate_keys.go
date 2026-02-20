package cli

import (
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/spf13/cobra"
	awstbxaws "github.com/towardsthecloud/aws-toolbox/internal/aws"
)

func runIAMRotateKeys(cmd *cobra.Command, username, keyID string, disable, deleteKey bool) error {
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

	runtime, _, client, err := newServiceRuntime(cmd, iamLoadAWSConfig, iamNewClient)
	if err != nil {
		return err
	}

	keys, err := listIAMAccessKeys(cmd.Context(), client, user)
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
		row[4] = failedActionMessage("user already has 2 keys")
		return writeDataset(cmd, runtime, headers, [][]string{row})
	}

	if runtime.Options.DryRun {
		return writeDataset(cmd, runtime, headers, [][]string{row})
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
		return writeDataset(cmd, runtime, headers, [][]string{row})
	}

	if disable {
		_, updateErr := client.UpdateAccessKey(cmd.Context(), &iam.UpdateAccessKeyInput{
			UserName:    ptr(user),
			AccessKeyId: ptr(key),
			Status:      iamtypes.StatusTypeInactive,
		})
		if updateErr != nil {
			row[4] = failedActionMessage(awstbxaws.FormatUserError(updateErr))
			return writeDataset(cmd, runtime, headers, [][]string{row})
		}
		row[4] = "disabled"
		return writeDataset(cmd, runtime, headers, [][]string{row})
	}

	if deleteKey {
		_, deleteErr := client.DeleteAccessKey(cmd.Context(), &iam.DeleteAccessKeyInput{
			UserName:    ptr(user),
			AccessKeyId: ptr(key),
		})
		if deleteErr != nil {
			row[4] = failedActionMessage(awstbxaws.FormatUserError(deleteErr))
			return writeDataset(cmd, runtime, headers, [][]string{row})
		}
		row[4] = "deleted"
		return writeDataset(cmd, runtime, headers, [][]string{row})
	}

	createOut, createErr := client.CreateAccessKey(cmd.Context(), &iam.CreateAccessKeyInput{UserName: ptr(user)})
	if createErr != nil {
		row[4] = failedActionMessage(awstbxaws.FormatUserError(createErr))
		return writeDataset(cmd, runtime, headers, [][]string{row})
	}

	row[1] = pointerToString(createOut.AccessKey.AccessKeyId)
	row[5] = pointerToString(createOut.AccessKey.SecretAccessKey)
	row[4] = "created"

	return writeDataset(cmd, runtime, headers, [][]string{row})
}
