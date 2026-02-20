package org

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/identitystore"
	identitystoretypes "github.com/aws/aws-sdk-go-v2/service/identitystore/types"
	"github.com/aws/aws-sdk-go-v2/service/organizations"
	organizationtypes "github.com/aws/aws-sdk-go-v2/service/organizations/types"
	"github.com/aws/aws-sdk-go-v2/service/ssoadmin"
	ssoadmintypes "github.com/aws/aws-sdk-go-v2/service/ssoadmin/types"
	"github.com/spf13/cobra"
	awstbxaws "github.com/towardsthecloud/aws-toolbox/internal/aws"
	"github.com/towardsthecloud/aws-toolbox/internal/cliutil"
)

type ssoInstance struct {
	InstanceARN     string
	IdentityStoreID string
}

const assignmentStatusPollInterval = 2 * time.Second

func runAssignSSOAccess(cmd *cobra.Command, principalName, principalTypeRaw, permissionSetName, ouName string) error {
	return runSSOAccessChange(cmd, principalName, principalTypeRaw, permissionSetName, ouName, true)
}

func runRemoveSSOAccess(cmd *cobra.Command, principalName, principalTypeRaw, permissionSetName, ouName string) error {
	return runSSOAccessChange(cmd, principalName, principalTypeRaw, permissionSetName, ouName, false)
}

func runSSOAccessChange(cmd *cobra.Command, principalName, principalTypeRaw, permissionSetName, ouName string, assign bool) error {
	if strings.TrimSpace(principalName) == "" {
		return fmt.Errorf("--principal-name is required")
	}
	if strings.TrimSpace(permissionSetName) == "" {
		return fmt.Errorf("--permission-set-name is required")
	}
	if strings.TrimSpace(ouName) == "" {
		return fmt.Errorf("--ou-name is required")
	}

	principalType, err := ssoPrincipalTypeFromString(principalTypeRaw)
	if err != nil {
		return err
	}

	runtime, orgClient, ssoClient, identityClient, _, err := runtimeClients(cmd)
	if err != nil {
		return err
	}
	ctx := cmd.Context()

	instance, err := resolveSSOInstance(ctx, ssoClient)
	if err != nil {
		return err
	}
	principalID, err := resolvePrincipalID(ctx, identityClient, instance.IdentityStoreID, principalName, principalType)
	if err != nil {
		return err
	}
	permissionSetARN, err := resolvePermissionSetARN(ctx, ssoClient, instance.InstanceARN, permissionSetName)
	if err != nil {
		return err
	}
	accountIDs, err := listAccountIDsByOU(ctx, orgClient, ouName)
	if err != nil {
		return err
	}
	sort.Strings(accountIDs)

	actionWould := "would-remove"
	actionDone := "removed"
	confirmText := "Remove"
	if assign {
		actionWould = "would-assign"
		actionDone = "assigned"
		confirmText = "Assign"
	}

	rows := make([][]string, 0, len(accountIDs))
	for _, id := range accountIDs {
		action := actionWould
		if !runtime.Options.DryRun {
			action = cliutil.ActionPending
		}
		rows = append(rows, []string{id, string(principalType), principalName, permissionSetName, action})
	}
	if len(rows) == 0 {
		return cliutil.WriteDataset(cmd, runtime, []string{"account_id", "principal_type", "principal_name", "permission_set", "action"}, rows)
	}

	if !runtime.Options.DryRun {
		ok, confirmErr := runtime.Prompter.Confirm(fmt.Sprintf("%s access for %d account(s)", confirmText, len(rows)), runtime.Options.NoConfirm)
		if confirmErr != nil {
			return confirmErr
		}
		if !ok {
			for i := range rows {
				rows[i][4] = cliutil.ActionCancelled
			}
			return cliutil.WriteDataset(cmd, runtime, []string{"account_id", "principal_type", "principal_name", "permission_set", "action"}, rows)
		}
		for i, id := range accountIDs {
			var opErr error
			if assign {
				createOut, createErr := ssoClient.CreateAccountAssignment(ctx, &ssoadmin.CreateAccountAssignmentInput{
					InstanceArn:      cliutil.Ptr(instance.InstanceARN),
					TargetId:         cliutil.Ptr(id),
					TargetType:       ssoadmintypes.TargetTypeAwsAccount,
					PrincipalType:    principalType,
					PrincipalId:      cliutil.Ptr(principalID),
					PermissionSetArn: cliutil.Ptr(permissionSetARN),
				})
				if createErr == nil {
					opErr = waitForAssignmentCreation(ctx, ssoClient, instance.InstanceARN, createOut)
				} else {
					opErr = createErr
				}
			} else {
				deleteOut, deleteErr := ssoClient.DeleteAccountAssignment(ctx, &ssoadmin.DeleteAccountAssignmentInput{
					InstanceArn:      cliutil.Ptr(instance.InstanceARN),
					TargetId:         cliutil.Ptr(id),
					TargetType:       ssoadmintypes.TargetTypeAwsAccount,
					PrincipalType:    principalType,
					PrincipalId:      cliutil.Ptr(principalID),
					PermissionSetArn: cliutil.Ptr(permissionSetARN),
				})
				if deleteErr == nil {
					opErr = waitForAssignmentDeletion(ctx, ssoClient, instance.InstanceARN, deleteOut)
				} else {
					opErr = deleteErr
				}
			}
			if opErr != nil {
				rows[i][4] = cliutil.FailedActionMessage(awstbxaws.FormatUserError(opErr))
				continue
			}
			rows[i][4] = actionDone
		}
	}

	return cliutil.WriteDataset(cmd, runtime, []string{"account_id", "principal_type", "principal_name", "permission_set", "action"}, rows)
}

func runListSSOAssignments(cmd *cobra.Command, accountID string) error {
	if accountID != "" {
		if err := validateAccountID(accountID); err != nil {
			return err
		}
	}

	runtime, orgClient, ssoClient, _, _, err := runtimeClients(cmd)
	if err != nil {
		return err
	}
	ctx := cmd.Context()

	instance, err := resolveSSOInstance(ctx, ssoClient)
	if err != nil {
		return err
	}
	permissionSets, err := listPermissionSets(ctx, ssoClient, instance.InstanceARN)
	if err != nil {
		return fmt.Errorf("list permission sets: %s", awstbxaws.FormatUserError(err))
	}
	sort.Strings(permissionSets)

	accounts := make([]organizationtypes.Account, 0)
	if accountID == "" {
		accounts, err = listAccounts(ctx, orgClient)
		if err != nil {
			return fmt.Errorf("list accounts: %s", awstbxaws.FormatUserError(err))
		}
	} else {
		out, describeErr := orgClient.DescribeAccount(ctx, &organizations.DescribeAccountInput{AccountId: cliutil.Ptr(accountID)})
		if describeErr != nil {
			return fmt.Errorf("describe account %s: %s", accountID, awstbxaws.FormatUserError(describeErr))
		}
		if out.Account != nil {
			accounts = append(accounts, *out.Account)
		}
	}
	sortAccountsByID(accounts)

	rows := make([][]string, 0)
	for _, acct := range accounts {
		id := cliutil.PointerToString(acct.Id)
		for _, psArn := range permissionSets {
			assignments, listErr := listAssignments(ctx, ssoClient, instance.InstanceARN, id, psArn)
			if listErr != nil {
				return fmt.Errorf("list assignments for account %s: %s", id, awstbxaws.FormatUserError(listErr))
			}
			for _, assignment := range assignments {
				rows = append(rows, []string{id, cliutil.PointerToString(acct.Name), string(assignment.PrincipalType), cliutil.PointerToString(assignment.PrincipalId), psArn})
			}
		}
	}
	sort.Slice(rows, func(i, j int) bool { return strings.Join(rows[i], "\x00") < strings.Join(rows[j], "\x00") })

	return cliutil.WriteDataset(cmd, runtime, []string{"account_id", "account_name", "principal_type", "principal_id", "permission_set_arn"}, rows)
}

func resolveSSOInstance(ctx context.Context, ssoClient SSOAdminAPI) (ssoInstance, error) {
	out, err := ssoClient.ListInstances(ctx, &ssoadmin.ListInstancesInput{})
	if err != nil {
		return ssoInstance{}, fmt.Errorf("list SSO instances: %s", awstbxaws.FormatUserError(err))
	}
	if len(out.Instances) == 0 {
		return ssoInstance{}, fmt.Errorf("no IAM Identity Center instances found")
	}
	return ssoInstance{InstanceARN: cliutil.PointerToString(out.Instances[0].InstanceArn), IdentityStoreID: cliutil.PointerToString(out.Instances[0].IdentityStoreId)}, nil
}

func resolvePrincipalID(ctx context.Context, identityClient IdentityStoreAPI, storeID, principalName string, principalType ssoadmintypes.PrincipalType) (string, error) {
	filter := identityStoreFilterForPrincipal(principalName, principalType)
	if principalType == ssoadmintypes.PrincipalTypeUser {
		out, err := identityClient.ListUsers(ctx, &identitystore.ListUsersInput{IdentityStoreId: cliutil.Ptr(storeID), Filters: []identitystoretypes.Filter{filter}})
		if err != nil {
			return "", fmt.Errorf("lookup user %q: %s", principalName, awstbxaws.FormatUserError(err))
		}
		if len(out.Users) == 0 {
			return "", fmt.Errorf("principal not found: %s", principalName)
		}
		return cliutil.PointerToString(out.Users[0].UserId), nil
	}
	out, err := identityClient.ListGroups(ctx, &identitystore.ListGroupsInput{IdentityStoreId: cliutil.Ptr(storeID), Filters: []identitystoretypes.Filter{filter}})
	if err != nil {
		return "", fmt.Errorf("lookup group %q: %s", principalName, awstbxaws.FormatUserError(err))
	}
	if len(out.Groups) == 0 {
		return "", fmt.Errorf("principal not found: %s", principalName)
	}
	return cliutil.PointerToString(out.Groups[0].GroupId), nil
}

func listPermissionSets(ctx context.Context, ssoClient SSOAdminAPI, instanceARN string) ([]string, error) {
	return awstbxaws.CollectAllPages(ctx, func(callCtx context.Context, nextToken *string) (awstbxaws.PageResult[string], error) {
		out, err := ssoClient.ListPermissionSets(callCtx, &ssoadmin.ListPermissionSetsInput{InstanceArn: cliutil.Ptr(instanceARN), NextToken: nextToken})
		if err != nil {
			return awstbxaws.PageResult[string]{}, err
		}
		return awstbxaws.PageResult[string]{
			Items:     out.PermissionSets,
			NextToken: out.NextToken,
		}, nil
	})
}

func resolvePermissionSetARN(ctx context.Context, ssoClient SSOAdminAPI, instanceARN, permissionSetName string) (string, error) {
	arns, err := listPermissionSets(ctx, ssoClient, instanceARN)
	if err != nil {
		return "", fmt.Errorf("list permission sets: %s", awstbxaws.FormatUserError(err))
	}
	for _, arn := range arns {
		out, describeErr := ssoClient.DescribePermissionSet(ctx, &ssoadmin.DescribePermissionSetInput{InstanceArn: cliutil.Ptr(instanceARN), PermissionSetArn: cliutil.Ptr(arn)})
		if describeErr == nil && strings.EqualFold(cliutil.PointerToString(out.PermissionSet.Name), strings.TrimSpace(permissionSetName)) {
			return arn, nil
		}
	}
	return "", fmt.Errorf("permission set not found: %s", permissionSetName)
}

func listAssignments(ctx context.Context, ssoClient SSOAdminAPI, instanceARN, accountID, permissionSetARN string) ([]ssoadmintypes.AccountAssignment, error) {
	return awstbxaws.CollectAllPages(ctx, func(callCtx context.Context, nextToken *string) (awstbxaws.PageResult[ssoadmintypes.AccountAssignment], error) {
		out, err := ssoClient.ListAccountAssignments(callCtx, &ssoadmin.ListAccountAssignmentsInput{
			InstanceArn:      cliutil.Ptr(instanceARN),
			AccountId:        cliutil.Ptr(accountID),
			PermissionSetArn: cliutil.Ptr(permissionSetARN),
			NextToken:        nextToken,
		})
		if err != nil {
			return awstbxaws.PageResult[ssoadmintypes.AccountAssignment]{}, err
		}
		return awstbxaws.PageResult[ssoadmintypes.AccountAssignment]{
			Items:     out.AccountAssignments,
			NextToken: out.NextToken,
		}, nil
	})
}

func waitForAssignmentCreation(ctx context.Context, ssoClient SSOAdminAPI, instanceARN string, out *ssoadmin.CreateAccountAssignmentOutput) error {
	requestID := ""
	if out != nil && out.AccountAssignmentCreationStatus != nil {
		requestID = cliutil.PointerToString(out.AccountAssignmentCreationStatus.RequestId)
	}
	if strings.TrimSpace(requestID) == "" {
		return fmt.Errorf("missing assignment creation request ID")
	}

	ticker := time.NewTicker(assignmentStatusPollInterval)
	defer ticker.Stop()

	for {
		statusOut, err := ssoClient.DescribeAccountAssignmentCreationStatus(ctx, &ssoadmin.DescribeAccountAssignmentCreationStatusInput{
			InstanceArn:                        cliutil.Ptr(instanceARN),
			AccountAssignmentCreationRequestId: cliutil.Ptr(requestID),
		})
		if err != nil {
			return err
		}
		if statusOut == nil || statusOut.AccountAssignmentCreationStatus == nil {
			return fmt.Errorf("empty assignment creation status response")
		}

		status := statusOut.AccountAssignmentCreationStatus
		switch status.Status {
		case ssoadmintypes.StatusValuesSucceeded:
			return nil
		case ssoadmintypes.StatusValuesFailed:
			reason := strings.TrimSpace(cliutil.PointerToString(status.FailureReason))
			if reason == "" {
				reason = "assignment creation failed"
			}
			return fmt.Errorf("%s", reason)
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func waitForAssignmentDeletion(ctx context.Context, ssoClient SSOAdminAPI, instanceARN string, out *ssoadmin.DeleteAccountAssignmentOutput) error {
	requestID := ""
	if out != nil && out.AccountAssignmentDeletionStatus != nil {
		requestID = cliutil.PointerToString(out.AccountAssignmentDeletionStatus.RequestId)
	}
	if strings.TrimSpace(requestID) == "" {
		return fmt.Errorf("missing assignment deletion request ID")
	}

	ticker := time.NewTicker(assignmentStatusPollInterval)
	defer ticker.Stop()

	for {
		statusOut, err := ssoClient.DescribeAccountAssignmentDeletionStatus(ctx, &ssoadmin.DescribeAccountAssignmentDeletionStatusInput{
			InstanceArn:                        cliutil.Ptr(instanceARN),
			AccountAssignmentDeletionRequestId: cliutil.Ptr(requestID),
		})
		if err != nil {
			return err
		}
		if statusOut == nil || statusOut.AccountAssignmentDeletionStatus == nil {
			return fmt.Errorf("empty assignment deletion status response")
		}

		status := statusOut.AccountAssignmentDeletionStatus
		switch status.Status {
		case ssoadmintypes.StatusValuesSucceeded:
			return nil
		case ssoadmintypes.StatusValuesFailed:
			reason := strings.TrimSpace(cliutil.PointerToString(status.FailureReason))
			if reason == "" {
				reason = "assignment deletion failed"
			}
			return fmt.Errorf("%s", reason)
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}
