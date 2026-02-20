package cli

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/spf13/cobra"
	awstbxaws "github.com/towardsthecloud/aws-toolbox/internal/aws"
)

type iamDeleteOperation struct {
	step          string
	resource      string
	dryRunAction  string
	successAction string
	execute       func(context.Context) error
	rowIndex      int
}

func runIAMDeleteUser(cmd *cobra.Command, username string) error {
	user := strings.TrimSpace(username)
	if user == "" {
		return fmt.Errorf("--username is required")
	}

	runtime, _, client, err := newServiceRuntime(cmd, iamLoadAWSConfig, iamNewClient)
	if err != nil {
		return err
	}
	ctx := cmd.Context()

	operations := make([]iamDeleteOperation, 0)
	rows := make([][]string, 0)
	addOperation := func(op iamDeleteOperation) {
		op.rowIndex = len(rows)
		operations = append(operations, op)

		action := op.dryRunAction
		if !runtime.Options.DryRun {
			action = actionPending
		}
		rows = append(rows, []string{user, op.step, op.resource, action})
	}
	addListFailure := func(step string, listErr error) {
		rows = append(rows, []string{user, step, "-", failedActionMessage(awstbxaws.FormatUserError(listErr))})
	}

	accessKeyIDs, listErr := listIAMAccessKeyIDs(ctx, client, user)
	if listErr != nil {
		addListFailure("access-key-list", listErr)
	} else {
		for _, keyID := range accessKeyIDs {
			currentKeyID := keyID
			addOperation(iamDeleteOperation{
				step:          "access-key",
				resource:      currentKeyID,
				dryRunAction:  actionWouldDelete,
				successAction: actionDeleted,
				execute: func(callCtx context.Context) error {
					_, err := client.DeleteAccessKey(callCtx, &iam.DeleteAccessKeyInput{
						UserName:    ptr(user),
						AccessKeyId: ptr(currentKeyID),
					})
					return err
				},
			})
		}
	}

	mfaDevices, listErr := listIAMMFADevices(ctx, client, user)
	if listErr != nil {
		addListFailure("mfa-device-list", listErr)
	} else {
		for _, serial := range mfaDevices {
			currentSerial := serial
			addOperation(iamDeleteOperation{
				step:          "mfa-device",
				resource:      currentSerial,
				dryRunAction:  actionWouldDelete,
				successAction: actionDeleted,
				execute: func(callCtx context.Context) error {
					_, err := client.DeactivateMFADevice(callCtx, &iam.DeactivateMFADeviceInput{
						UserName:     ptr(user),
						SerialNumber: ptr(currentSerial),
					})
					return err
				},
			})
		}
	}

	attachedPolicies, listErr := listIAMAttachedPolicies(ctx, client, user)
	if listErr != nil {
		addListFailure("attached-policy-list", listErr)
	} else {
		for _, policyARN := range attachedPolicies {
			currentPolicyARN := policyARN
			addOperation(iamDeleteOperation{
				step:          "attached-policy",
				resource:      currentPolicyARN,
				dryRunAction:  actionWouldDelete,
				successAction: actionDeleted,
				execute: func(callCtx context.Context) error {
					_, err := client.DetachUserPolicy(callCtx, &iam.DetachUserPolicyInput{
						UserName:  ptr(user),
						PolicyArn: ptr(currentPolicyARN),
					})
					return err
				},
			})
		}
	}

	inlinePolicies, listErr := listIAMInlinePolicies(ctx, client, user)
	if listErr != nil {
		addListFailure("inline-policy-list", listErr)
	} else {
		for _, policyName := range inlinePolicies {
			currentPolicyName := policyName
			addOperation(iamDeleteOperation{
				step:          "inline-policy",
				resource:      currentPolicyName,
				dryRunAction:  actionWouldDelete,
				successAction: actionDeleted,
				execute: func(callCtx context.Context) error {
					_, err := client.DeleteUserPolicy(callCtx, &iam.DeleteUserPolicyInput{
						UserName:   ptr(user),
						PolicyName: ptr(currentPolicyName),
					})
					return err
				},
			})
		}
	}

	groups, listErr := listIAMGroupsForUser(ctx, client, user)
	if listErr != nil {
		addListFailure("group-membership-list", listErr)
	} else {
		for _, groupName := range groups {
			currentGroupName := groupName
			addOperation(iamDeleteOperation{
				step:          "group-membership",
				resource:      currentGroupName,
				dryRunAction:  actionWouldDelete,
				successAction: actionDeleted,
				execute: func(callCtx context.Context) error {
					_, err := client.RemoveUserFromGroup(callCtx, &iam.RemoveUserFromGroupInput{
						UserName:  ptr(user),
						GroupName: ptr(currentGroupName),
					})
					return err
				},
			})
		}
	}

	addOperation(iamDeleteOperation{
		step:          "permissions-boundary",
		resource:      "-",
		dryRunAction:  actionWouldDelete,
		successAction: actionDeleted,
		execute: func(callCtx context.Context) error {
			_, err := client.DeleteUserPermissionsBoundary(callCtx, &iam.DeleteUserPermissionsBoundaryInput{UserName: ptr(user)})
			return err
		},
	})

	addOperation(iamDeleteOperation{
		step:          "login-profile",
		resource:      "-",
		dryRunAction:  actionWouldDelete,
		successAction: actionDeleted,
		execute: func(callCtx context.Context) error {
			_, err := client.DeleteLoginProfile(callCtx, &iam.DeleteLoginProfileInput{UserName: ptr(user)})
			return err
		},
	})

	signingCertIDs, listErr := listIAMSigningCertificates(ctx, client, user)
	if listErr != nil {
		addListFailure("signing-certificate-list", listErr)
	} else {
		for _, certificateID := range signingCertIDs {
			currentCertificateID := certificateID
			addOperation(iamDeleteOperation{
				step:          "signing-certificate",
				resource:      currentCertificateID,
				dryRunAction:  actionWouldDelete,
				successAction: actionDeleted,
				execute: func(callCtx context.Context) error {
					_, err := client.DeleteSigningCertificate(callCtx, &iam.DeleteSigningCertificateInput{
						UserName:      ptr(user),
						CertificateId: ptr(currentCertificateID),
					})
					return err
				},
			})
		}
	}

	sshKeyIDs, listErr := listIAMSSHPublicKeys(ctx, client, user)
	if listErr != nil {
		addListFailure("ssh-public-key-list", listErr)
	} else {
		for _, keyID := range sshKeyIDs {
			currentKeyID := keyID
			addOperation(iamDeleteOperation{
				step:          "ssh-public-key",
				resource:      currentKeyID,
				dryRunAction:  actionWouldDelete,
				successAction: actionDeleted,
				execute: func(callCtx context.Context) error {
					_, err := client.DeleteSSHPublicKey(callCtx, &iam.DeleteSSHPublicKeyInput{
						UserName:       ptr(user),
						SSHPublicKeyId: ptr(currentKeyID),
					})
					return err
				},
			})
		}
	}

	addOperation(iamDeleteOperation{
		step:          "user",
		resource:      user,
		dryRunAction:  actionWouldDelete,
		successAction: actionDeleted,
		execute: func(callCtx context.Context) error {
			_, err := client.DeleteUser(callCtx, &iam.DeleteUserInput{UserName: ptr(user)})
			return err
		},
	})

	if runtime.Options.DryRun {
		return writeDataset(cmd, runtime, []string{"username", "step", "resource", "action"}, rows)
	}

	ok, confirmErr := runtime.Prompter.Confirm(
		fmt.Sprintf("Cascade-delete IAM user %q", user),
		runtime.Options.NoConfirm,
	)
	if confirmErr != nil {
		return confirmErr
	}
	if !ok {
		for i := range rows {
			if rows[i][3] == actionPending {
				rows[i][3] = actionCancelled
			}
		}
		return writeDataset(cmd, runtime, []string{"username", "step", "resource", "action"}, rows)
	}

	for _, op := range operations {
		execErr := op.execute(ctx)
		if execErr == nil {
			rows[op.rowIndex][3] = op.successAction
			continue
		}
		if isIAMNoSuchEntity(execErr) {
			rows[op.rowIndex][3] = skippedActionMessage("not-found")
			continue
		}
		rows[op.rowIndex][3] = failedActionMessage(awstbxaws.FormatUserError(execErr))
	}

	return writeDataset(cmd, runtime, []string{"username", "step", "resource", "action"}, rows)
}

func listIAMAccessKeys(ctx context.Context, client iamAPI, username string) ([]iamtypes.AccessKeyMetadata, error) {
	keys := make([]iamtypes.AccessKeyMetadata, 0)
	var marker *string
	for {
		page, err := client.ListAccessKeys(ctx, &iam.ListAccessKeysInput{
			UserName: ptr(username),
			Marker:   marker,
		})
		if err != nil {
			return nil, err
		}
		keys = append(keys, page.AccessKeyMetadata...)
		if !page.IsTruncated || page.Marker == nil || *page.Marker == "" {
			break
		}
		marker = page.Marker
	}
	return keys, nil
}

func listIAMAccessKeyIDs(ctx context.Context, client iamAPI, username string) ([]string, error) {
	keys, err := listIAMAccessKeys(ctx, client, username)
	if err != nil {
		return nil, err
	}

	ids := make([]string, 0, len(keys))
	for _, key := range keys {
		id := pointerToString(key.AccessKeyId)
		if id != "" {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)

	return ids, nil
}

func listIAMMFADevices(ctx context.Context, client iamAPI, username string) ([]string, error) {
	items := make([]string, 0)
	var marker *string
	for {
		page, err := client.ListMFADevices(ctx, &iam.ListMFADevicesInput{
			UserName: ptr(username),
			Marker:   marker,
		})
		if err != nil {
			return nil, err
		}
		for _, device := range page.MFADevices {
			serial := pointerToString(device.SerialNumber)
			if serial != "" {
				items = append(items, serial)
			}
		}
		if !page.IsTruncated || page.Marker == nil || *page.Marker == "" {
			break
		}
		marker = page.Marker
	}
	sort.Strings(items)
	return items, nil
}

func listIAMAttachedPolicies(ctx context.Context, client iamAPI, username string) ([]string, error) {
	items := make([]string, 0)
	var marker *string
	for {
		page, err := client.ListAttachedUserPolicies(ctx, &iam.ListAttachedUserPoliciesInput{
			UserName: ptr(username),
			Marker:   marker,
		})
		if err != nil {
			return nil, err
		}
		for _, policy := range page.AttachedPolicies {
			arn := pointerToString(policy.PolicyArn)
			if arn != "" {
				items = append(items, arn)
			}
		}
		if !page.IsTruncated || page.Marker == nil || *page.Marker == "" {
			break
		}
		marker = page.Marker
	}
	sort.Strings(items)
	return items, nil
}

func listIAMInlinePolicies(ctx context.Context, client iamAPI, username string) ([]string, error) {
	items := make([]string, 0)
	var marker *string
	for {
		page, err := client.ListUserPolicies(ctx, &iam.ListUserPoliciesInput{
			UserName: ptr(username),
			Marker:   marker,
		})
		if err != nil {
			return nil, err
		}
		items = append(items, page.PolicyNames...)
		if !page.IsTruncated || page.Marker == nil || *page.Marker == "" {
			break
		}
		marker = page.Marker
	}
	sort.Strings(items)
	return items, nil
}

func listIAMGroupsForUser(ctx context.Context, client iamAPI, username string) ([]string, error) {
	items := make([]string, 0)
	var marker *string
	for {
		page, err := client.ListGroupsForUser(ctx, &iam.ListGroupsForUserInput{
			UserName: ptr(username),
			Marker:   marker,
		})
		if err != nil {
			return nil, err
		}
		for _, group := range page.Groups {
			name := pointerToString(group.GroupName)
			if name != "" {
				items = append(items, name)
			}
		}
		if !page.IsTruncated || page.Marker == nil || *page.Marker == "" {
			break
		}
		marker = page.Marker
	}
	sort.Strings(items)
	return items, nil
}

func listIAMSigningCertificates(ctx context.Context, client iamAPI, username string) ([]string, error) {
	items := make([]string, 0)
	var marker *string
	for {
		page, err := client.ListSigningCertificates(ctx, &iam.ListSigningCertificatesInput{
			UserName: ptr(username),
			Marker:   marker,
		})
		if err != nil {
			return nil, err
		}
		for _, certificate := range page.Certificates {
			certificateID := pointerToString(certificate.CertificateId)
			if certificateID != "" {
				items = append(items, certificateID)
			}
		}
		if !page.IsTruncated || page.Marker == nil || *page.Marker == "" {
			break
		}
		marker = page.Marker
	}
	sort.Strings(items)
	return items, nil
}

func listIAMSSHPublicKeys(ctx context.Context, client iamAPI, username string) ([]string, error) {
	items := make([]string, 0)
	var marker *string
	for {
		page, err := client.ListSSHPublicKeys(ctx, &iam.ListSSHPublicKeysInput{
			UserName: ptr(username),
			Marker:   marker,
		})
		if err != nil {
			return nil, err
		}
		for _, key := range page.SSHPublicKeys {
			keyID := pointerToString(key.SSHPublicKeyId)
			if keyID != "" {
				items = append(items, keyID)
			}
		}
		if !page.IsTruncated || page.Marker == nil || *page.Marker == "" {
			break
		}
		marker = page.Marker
	}
	sort.Strings(items)
	return items, nil
}

func isIAMNoSuchEntity(err error) bool {
	code := strings.ToLower(awsErrorCode(err))
	return code == "nosuchentity" || code == "nosuchentityexception"
}
