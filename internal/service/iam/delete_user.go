package iam

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/aws/smithy-go"
	"github.com/spf13/cobra"
	awstbxaws "github.com/towardsthecloud/aws-toolbox/internal/aws"
	"github.com/towardsthecloud/aws-toolbox/internal/cliutil"
)

type deleteOperation struct {
	step          string
	resource      string
	dryRunAction  string
	successAction string
	execute       func(context.Context) error
	rowIndex      int
}

func runDeleteUser(cmd *cobra.Command, username string) error {
	user := strings.TrimSpace(username)
	if user == "" {
		return fmt.Errorf("--username is required")
	}

	runtime, _, client, err := cliutil.NewServiceRuntime(cmd, loadAWSConfig, newClient)
	if err != nil {
		return err
	}
	ctx := cmd.Context()

	operations := make([]deleteOperation, 0)
	rows := make([][]string, 0)
	addOperation := func(op deleteOperation) {
		op.rowIndex = len(rows)
		operations = append(operations, op)

		action := op.dryRunAction
		if !runtime.Options.DryRun {
			action = cliutil.ActionPending
		}
		rows = append(rows, []string{user, op.step, op.resource, action})
	}
	addListFailure := func(step string, listErr error) {
		rows = append(rows, []string{user, step, "-", cliutil.FailedActionMessage(awstbxaws.FormatUserError(listErr))})
	}

	accessKeyIDs, listErr := listAccessKeyIDs(ctx, client, user)
	if listErr != nil {
		addListFailure("access-key-list", listErr)
	} else {
		for _, keyID := range accessKeyIDs {
			currentKeyID := keyID
			addOperation(deleteOperation{
				step:          "access-key",
				resource:      currentKeyID,
				dryRunAction:  cliutil.ActionWouldDelete,
				successAction: cliutil.ActionDeleted,
				execute: func(callCtx context.Context) error {
					_, err := client.DeleteAccessKey(callCtx, &iam.DeleteAccessKeyInput{
						UserName:    cliutil.Ptr(user),
						AccessKeyId: cliutil.Ptr(currentKeyID),
					})
					return err
				},
			})
		}
	}

	mfaDevices, listErr := listMFADevices(ctx, client, user)
	if listErr != nil {
		addListFailure("mfa-device-list", listErr)
	} else {
		for _, serial := range mfaDevices {
			currentSerial := serial
			addOperation(deleteOperation{
				step:          "mfa-device",
				resource:      currentSerial,
				dryRunAction:  cliutil.ActionWouldDelete,
				successAction: cliutil.ActionDeleted,
				execute: func(callCtx context.Context) error {
					_, err := client.DeactivateMFADevice(callCtx, &iam.DeactivateMFADeviceInput{
						UserName:     cliutil.Ptr(user),
						SerialNumber: cliutil.Ptr(currentSerial),
					})
					return err
				},
			})
		}
	}

	attachedPolicies, listErr := listAttachedPolicies(ctx, client, user)
	if listErr != nil {
		addListFailure("attached-policy-list", listErr)
	} else {
		for _, policyARN := range attachedPolicies {
			currentPolicyARN := policyARN
			addOperation(deleteOperation{
				step:          "attached-policy",
				resource:      currentPolicyARN,
				dryRunAction:  cliutil.ActionWouldDelete,
				successAction: cliutil.ActionDeleted,
				execute: func(callCtx context.Context) error {
					_, err := client.DetachUserPolicy(callCtx, &iam.DetachUserPolicyInput{
						UserName:  cliutil.Ptr(user),
						PolicyArn: cliutil.Ptr(currentPolicyARN),
					})
					return err
				},
			})
		}
	}

	inlinePolicies, listErr := listInlinePolicies(ctx, client, user)
	if listErr != nil {
		addListFailure("inline-policy-list", listErr)
	} else {
		for _, policyName := range inlinePolicies {
			currentPolicyName := policyName
			addOperation(deleteOperation{
				step:          "inline-policy",
				resource:      currentPolicyName,
				dryRunAction:  cliutil.ActionWouldDelete,
				successAction: cliutil.ActionDeleted,
				execute: func(callCtx context.Context) error {
					_, err := client.DeleteUserPolicy(callCtx, &iam.DeleteUserPolicyInput{
						UserName:   cliutil.Ptr(user),
						PolicyName: cliutil.Ptr(currentPolicyName),
					})
					return err
				},
			})
		}
	}

	groups, listErr := listGroupsForUser(ctx, client, user)
	if listErr != nil {
		addListFailure("group-membership-list", listErr)
	} else {
		for _, groupName := range groups {
			currentGroupName := groupName
			addOperation(deleteOperation{
				step:          "group-membership",
				resource:      currentGroupName,
				dryRunAction:  cliutil.ActionWouldDelete,
				successAction: cliutil.ActionDeleted,
				execute: func(callCtx context.Context) error {
					_, err := client.RemoveUserFromGroup(callCtx, &iam.RemoveUserFromGroupInput{
						UserName:  cliutil.Ptr(user),
						GroupName: cliutil.Ptr(currentGroupName),
					})
					return err
				},
			})
		}
	}

	addOperation(deleteOperation{
		step:          "permissions-boundary",
		resource:      "-",
		dryRunAction:  cliutil.ActionWouldDelete,
		successAction: cliutil.ActionDeleted,
		execute: func(callCtx context.Context) error {
			_, err := client.DeleteUserPermissionsBoundary(callCtx, &iam.DeleteUserPermissionsBoundaryInput{UserName: cliutil.Ptr(user)})
			return err
		},
	})

	addOperation(deleteOperation{
		step:          "login-profile",
		resource:      "-",
		dryRunAction:  cliutil.ActionWouldDelete,
		successAction: cliutil.ActionDeleted,
		execute: func(callCtx context.Context) error {
			_, err := client.DeleteLoginProfile(callCtx, &iam.DeleteLoginProfileInput{UserName: cliutil.Ptr(user)})
			return err
		},
	})

	signingCertIDs, listErr := listSigningCertificates(ctx, client, user)
	if listErr != nil {
		addListFailure("signing-certificate-list", listErr)
	} else {
		for _, certificateID := range signingCertIDs {
			currentCertificateID := certificateID
			addOperation(deleteOperation{
				step:          "signing-certificate",
				resource:      currentCertificateID,
				dryRunAction:  cliutil.ActionWouldDelete,
				successAction: cliutil.ActionDeleted,
				execute: func(callCtx context.Context) error {
					_, err := client.DeleteSigningCertificate(callCtx, &iam.DeleteSigningCertificateInput{
						UserName:      cliutil.Ptr(user),
						CertificateId: cliutil.Ptr(currentCertificateID),
					})
					return err
				},
			})
		}
	}

	sshKeyIDs, listErr := listSSHPublicKeys(ctx, client, user)
	if listErr != nil {
		addListFailure("ssh-public-key-list", listErr)
	} else {
		for _, keyID := range sshKeyIDs {
			currentKeyID := keyID
			addOperation(deleteOperation{
				step:          "ssh-public-key",
				resource:      currentKeyID,
				dryRunAction:  cliutil.ActionWouldDelete,
				successAction: cliutil.ActionDeleted,
				execute: func(callCtx context.Context) error {
					_, err := client.DeleteSSHPublicKey(callCtx, &iam.DeleteSSHPublicKeyInput{
						UserName:       cliutil.Ptr(user),
						SSHPublicKeyId: cliutil.Ptr(currentKeyID),
					})
					return err
				},
			})
		}
	}

	addOperation(deleteOperation{
		step:          "user",
		resource:      user,
		dryRunAction:  cliutil.ActionWouldDelete,
		successAction: cliutil.ActionDeleted,
		execute: func(callCtx context.Context) error {
			_, err := client.DeleteUser(callCtx, &iam.DeleteUserInput{UserName: cliutil.Ptr(user)})
			return err
		},
	})

	if runtime.Options.DryRun {
		return cliutil.WriteDataset(cmd, runtime, []string{"username", "step", "resource", "action"}, rows)
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
			if rows[i][3] == cliutil.ActionPending {
				rows[i][3] = cliutil.ActionCancelled
			}
		}
		return cliutil.WriteDataset(cmd, runtime, []string{"username", "step", "resource", "action"}, rows)
	}

	for _, op := range operations {
		execErr := op.execute(ctx)
		if execErr == nil {
			rows[op.rowIndex][3] = op.successAction
			continue
		}
		if isNoSuchEntity(execErr) {
			rows[op.rowIndex][3] = cliutil.SkippedActionMessage("not-found")
			continue
		}
		rows[op.rowIndex][3] = cliutil.FailedActionMessage(awstbxaws.FormatUserError(execErr))
	}

	return cliutil.WriteDataset(cmd, runtime, []string{"username", "step", "resource", "action"}, rows)
}

func listAccessKeys(ctx context.Context, client API, username string) ([]iamtypes.AccessKeyMetadata, error) {
	keys := make([]iamtypes.AccessKeyMetadata, 0)
	var marker *string
	for {
		page, err := client.ListAccessKeys(ctx, &iam.ListAccessKeysInput{
			UserName: cliutil.Ptr(username),
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

func listAccessKeyIDs(ctx context.Context, client API, username string) ([]string, error) {
	keys, err := listAccessKeys(ctx, client, username)
	if err != nil {
		return nil, err
	}

	ids := make([]string, 0, len(keys))
	for _, key := range keys {
		id := cliutil.PointerToString(key.AccessKeyId)
		if id != "" {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)

	return ids, nil
}

func listMFADevices(ctx context.Context, client API, username string) ([]string, error) {
	items := make([]string, 0)
	var marker *string
	for {
		page, err := client.ListMFADevices(ctx, &iam.ListMFADevicesInput{
			UserName: cliutil.Ptr(username),
			Marker:   marker,
		})
		if err != nil {
			return nil, err
		}
		for _, device := range page.MFADevices {
			serial := cliutil.PointerToString(device.SerialNumber)
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

func listAttachedPolicies(ctx context.Context, client API, username string) ([]string, error) {
	items := make([]string, 0)
	var marker *string
	for {
		page, err := client.ListAttachedUserPolicies(ctx, &iam.ListAttachedUserPoliciesInput{
			UserName: cliutil.Ptr(username),
			Marker:   marker,
		})
		if err != nil {
			return nil, err
		}
		for _, policy := range page.AttachedPolicies {
			arn := cliutil.PointerToString(policy.PolicyArn)
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

func listInlinePolicies(ctx context.Context, client API, username string) ([]string, error) {
	items := make([]string, 0)
	var marker *string
	for {
		page, err := client.ListUserPolicies(ctx, &iam.ListUserPoliciesInput{
			UserName: cliutil.Ptr(username),
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

func listGroupsForUser(ctx context.Context, client API, username string) ([]string, error) {
	items := make([]string, 0)
	var marker *string
	for {
		page, err := client.ListGroupsForUser(ctx, &iam.ListGroupsForUserInput{
			UserName: cliutil.Ptr(username),
			Marker:   marker,
		})
		if err != nil {
			return nil, err
		}
		for _, group := range page.Groups {
			name := cliutil.PointerToString(group.GroupName)
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

func listSigningCertificates(ctx context.Context, client API, username string) ([]string, error) {
	items := make([]string, 0)
	var marker *string
	for {
		page, err := client.ListSigningCertificates(ctx, &iam.ListSigningCertificatesInput{
			UserName: cliutil.Ptr(username),
			Marker:   marker,
		})
		if err != nil {
			return nil, err
		}
		for _, certificate := range page.Certificates {
			certificateID := cliutil.PointerToString(certificate.CertificateId)
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

func listSSHPublicKeys(ctx context.Context, client API, username string) ([]string, error) {
	items := make([]string, 0)
	var marker *string
	for {
		page, err := client.ListSSHPublicKeys(ctx, &iam.ListSSHPublicKeysInput{
			UserName: cliutil.Ptr(username),
			Marker:   marker,
		})
		if err != nil {
			return nil, err
		}
		for _, key := range page.SSHPublicKeys {
			keyID := cliutil.PointerToString(key.SSHPublicKeyId)
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

func isNoSuchEntity(err error) bool {
	var apiErr smithy.APIError
	if !errors.As(err, &apiErr) {
		return false
	}
	code := strings.ToLower(apiErr.ErrorCode())
	return code == "nosuchentity" || code == "nosuchentityexception"
}
