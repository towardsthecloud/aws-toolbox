package cli

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
)

type orgSSOInstance struct {
	InstanceARN     string
	IdentityStoreID string
}

const orgAssignmentStatusPollInterval = 2 * time.Second

func runOrgAssignSSOAccess(cmd *cobra.Command, principalName, principalTypeRaw, permissionSetName, ouName string) error {
	return runOrgSSOAccessChange(cmd, principalName, principalTypeRaw, permissionSetName, ouName, true)
}

func runOrgRemoveSSOAccess(cmd *cobra.Command, principalName, principalTypeRaw, permissionSetName, ouName string) error {
	return runOrgSSOAccessChange(cmd, principalName, principalTypeRaw, permissionSetName, ouName, false)
}

func runOrgSSOAccessChange(cmd *cobra.Command, principalName, principalTypeRaw, permissionSetName, ouName string, assign bool) error {
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

	runtime, orgClient, ssoClient, identityClient, _, err := orgRuntimeClients(cmd)
	if err != nil {
		return err
	}
	ctx := cmd.Context()

	instance, err := resolveOrgSSOInstance(ctx, ssoClient)
	if err != nil {
		return err
	}
	principalID, err := resolveOrgPrincipalID(ctx, identityClient, instance.IdentityStoreID, principalName, principalType)
	if err != nil {
		return err
	}
	permissionSetARN, err := resolveOrgPermissionSetARN(ctx, ssoClient, instance.InstanceARN, permissionSetName)
	if err != nil {
		return err
	}
	accountIDs, err := listOrgAccountIDsByOU(ctx, orgClient, ouName)
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
			action = "pending"
		}
		rows = append(rows, []string{id, string(principalType), principalName, permissionSetName, action})
	}
	if len(rows) == 0 {
		return writeDataset(cmd, runtime, []string{"account_id", "principal_type", "principal_name", "permission_set", "action"}, rows)
	}

	if !runtime.Options.DryRun {
		ok, confirmErr := runtime.Prompter.Confirm(fmt.Sprintf("%s access for %d account(s)", confirmText, len(rows)), runtime.Options.NoConfirm)
		if confirmErr != nil {
			return confirmErr
		}
		if !ok {
			for i := range rows {
				rows[i][4] = "cancelled"
			}
			return writeDataset(cmd, runtime, []string{"account_id", "principal_type", "principal_name", "permission_set", "action"}, rows)
		}
		for i, id := range accountIDs {
			var opErr error
			if assign {
				createOut, createErr := ssoClient.CreateAccountAssignment(ctx, &ssoadmin.CreateAccountAssignmentInput{
					InstanceArn:      ptr(instance.InstanceARN),
					TargetId:         ptr(id),
					TargetType:       ssoadmintypes.TargetTypeAwsAccount,
					PrincipalType:    principalType,
					PrincipalId:      ptr(principalID),
					PermissionSetArn: ptr(permissionSetARN),
				})
				if createErr == nil {
					opErr = waitForOrgAssignmentCreation(ctx, ssoClient, instance.InstanceARN, createOut)
				} else {
					opErr = createErr
				}
			} else {
				deleteOut, deleteErr := ssoClient.DeleteAccountAssignment(ctx, &ssoadmin.DeleteAccountAssignmentInput{
					InstanceArn:      ptr(instance.InstanceARN),
					TargetId:         ptr(id),
					TargetType:       ssoadmintypes.TargetTypeAwsAccount,
					PrincipalType:    principalType,
					PrincipalId:      ptr(principalID),
					PermissionSetArn: ptr(permissionSetARN),
				})
				if deleteErr == nil {
					opErr = waitForOrgAssignmentDeletion(ctx, ssoClient, instance.InstanceARN, deleteOut)
				} else {
					opErr = deleteErr
				}
			}
			if opErr != nil {
				rows[i][4] = "failed: " + awstbxaws.FormatUserError(opErr)
				continue
			}
			rows[i][4] = actionDone
		}
	}

	return writeDataset(cmd, runtime, []string{"account_id", "principal_type", "principal_name", "permission_set", "action"}, rows)
}

func runOrgListSSOAssignments(cmd *cobra.Command, accountID string) error {
	if accountID != "" {
		if err := validateOrgAccountID(accountID); err != nil {
			return err
		}
	}

	runtime, orgClient, ssoClient, _, _, err := orgRuntimeClients(cmd)
	if err != nil {
		return err
	}
	ctx := cmd.Context()

	instance, err := resolveOrgSSOInstance(ctx, ssoClient)
	if err != nil {
		return err
	}
	permissionSets, err := listOrgPermissionSets(ctx, ssoClient, instance.InstanceARN)
	if err != nil {
		return fmt.Errorf("list permission sets: %s", awstbxaws.FormatUserError(err))
	}
	sort.Strings(permissionSets)

	accounts := make([]organizationtypes.Account, 0)
	if accountID == "" {
		accounts, err = listOrgAccounts(ctx, orgClient)
		if err != nil {
			return fmt.Errorf("list accounts: %s", awstbxaws.FormatUserError(err))
		}
	} else {
		out, describeErr := orgClient.DescribeAccount(ctx, &organizations.DescribeAccountInput{AccountId: ptr(accountID)})
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
		id := pointerToString(acct.Id)
		for _, psArn := range permissionSets {
			assignments, listErr := listOrgAssignments(ctx, ssoClient, instance.InstanceARN, id, psArn)
			if listErr != nil {
				return fmt.Errorf("list assignments for account %s: %s", id, awstbxaws.FormatUserError(listErr))
			}
			for _, assignment := range assignments {
				rows = append(rows, []string{id, pointerToString(acct.Name), string(assignment.PrincipalType), pointerToString(assignment.PrincipalId), psArn})
			}
		}
	}
	sort.Slice(rows, func(i, j int) bool { return strings.Join(rows[i], "\x00") < strings.Join(rows[j], "\x00") })

	return writeDataset(cmd, runtime, []string{"account_id", "account_name", "principal_type", "principal_id", "permission_set_arn"}, rows)
}

func resolveOrgSSOInstance(ctx context.Context, ssoClient ssoAdminAPI) (orgSSOInstance, error) {
	out, err := ssoClient.ListInstances(ctx, &ssoadmin.ListInstancesInput{})
	if err != nil {
		return orgSSOInstance{}, fmt.Errorf("list SSO instances: %s", awstbxaws.FormatUserError(err))
	}
	if len(out.Instances) == 0 {
		return orgSSOInstance{}, fmt.Errorf("no IAM Identity Center instances found")
	}
	return orgSSOInstance{InstanceARN: pointerToString(out.Instances[0].InstanceArn), IdentityStoreID: pointerToString(out.Instances[0].IdentityStoreId)}, nil
}

func resolveOrgPrincipalID(ctx context.Context, identityClient identityStoreAPI, storeID, principalName string, principalType ssoadmintypes.PrincipalType) (string, error) {
	filter := identityStoreFilterForPrincipal(principalName, principalType)
	if principalType == ssoadmintypes.PrincipalTypeUser {
		out, err := identityClient.ListUsers(ctx, &identitystore.ListUsersInput{IdentityStoreId: ptr(storeID), Filters: []identitystoretypes.Filter{filter}})
		if err != nil {
			return "", fmt.Errorf("lookup user %q: %s", principalName, awstbxaws.FormatUserError(err))
		}
		if len(out.Users) == 0 {
			return "", fmt.Errorf("principal not found: %s", principalName)
		}
		return pointerToString(out.Users[0].UserId), nil
	}
	out, err := identityClient.ListGroups(ctx, &identitystore.ListGroupsInput{IdentityStoreId: ptr(storeID), Filters: []identitystoretypes.Filter{filter}})
	if err != nil {
		return "", fmt.Errorf("lookup group %q: %s", principalName, awstbxaws.FormatUserError(err))
	}
	if len(out.Groups) == 0 {
		return "", fmt.Errorf("principal not found: %s", principalName)
	}
	return pointerToString(out.Groups[0].GroupId), nil
}

func listOrgPermissionSets(ctx context.Context, ssoClient ssoAdminAPI, instanceARN string) ([]string, error) {
	items := make([]string, 0)
	var nextToken *string
	for {
		out, err := ssoClient.ListPermissionSets(ctx, &ssoadmin.ListPermissionSetsInput{InstanceArn: ptr(instanceARN), NextToken: nextToken})
		if err != nil {
			return nil, err
		}
		items = append(items, out.PermissionSets...)
		if out.NextToken == nil || *out.NextToken == "" {
			break
		}
		nextToken = out.NextToken
	}
	return items, nil
}

func resolveOrgPermissionSetARN(ctx context.Context, ssoClient ssoAdminAPI, instanceARN, permissionSetName string) (string, error) {
	arns, err := listOrgPermissionSets(ctx, ssoClient, instanceARN)
	if err != nil {
		return "", fmt.Errorf("list permission sets: %s", awstbxaws.FormatUserError(err))
	}
	for _, arn := range arns {
		out, describeErr := ssoClient.DescribePermissionSet(ctx, &ssoadmin.DescribePermissionSetInput{InstanceArn: ptr(instanceARN), PermissionSetArn: ptr(arn)})
		if describeErr == nil && strings.EqualFold(pointerToString(out.PermissionSet.Name), strings.TrimSpace(permissionSetName)) {
			return arn, nil
		}
	}
	return "", fmt.Errorf("permission set not found: %s", permissionSetName)
}

func listOrgAssignments(ctx context.Context, ssoClient ssoAdminAPI, instanceARN, accountID, permissionSetARN string) ([]ssoadmintypes.AccountAssignment, error) {
	items := make([]ssoadmintypes.AccountAssignment, 0)
	var nextToken *string
	for {
		out, err := ssoClient.ListAccountAssignments(ctx, &ssoadmin.ListAccountAssignmentsInput{InstanceArn: ptr(instanceARN), AccountId: ptr(accountID), PermissionSetArn: ptr(permissionSetARN), NextToken: nextToken})
		if err != nil {
			return nil, err
		}
		items = append(items, out.AccountAssignments...)
		if out.NextToken == nil || *out.NextToken == "" {
			break
		}
		nextToken = out.NextToken
	}
	return items, nil
}

func waitForOrgAssignmentCreation(ctx context.Context, ssoClient ssoAdminAPI, instanceARN string, out *ssoadmin.CreateAccountAssignmentOutput) error {
	requestID := ""
	if out != nil && out.AccountAssignmentCreationStatus != nil {
		requestID = pointerToString(out.AccountAssignmentCreationStatus.RequestId)
	}
	if strings.TrimSpace(requestID) == "" {
		return fmt.Errorf("missing assignment creation request ID")
	}

	ticker := time.NewTicker(orgAssignmentStatusPollInterval)
	defer ticker.Stop()

	for {
		statusOut, err := ssoClient.DescribeAccountAssignmentCreationStatus(ctx, &ssoadmin.DescribeAccountAssignmentCreationStatusInput{
			InstanceArn:                        ptr(instanceARN),
			AccountAssignmentCreationRequestId: ptr(requestID),
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
			reason := strings.TrimSpace(pointerToString(status.FailureReason))
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

func waitForOrgAssignmentDeletion(ctx context.Context, ssoClient ssoAdminAPI, instanceARN string, out *ssoadmin.DeleteAccountAssignmentOutput) error {
	requestID := ""
	if out != nil && out.AccountAssignmentDeletionStatus != nil {
		requestID = pointerToString(out.AccountAssignmentDeletionStatus.RequestId)
	}
	if strings.TrimSpace(requestID) == "" {
		return fmt.Errorf("missing assignment deletion request ID")
	}

	ticker := time.NewTicker(orgAssignmentStatusPollInterval)
	defer ticker.Stop()

	for {
		statusOut, err := ssoClient.DescribeAccountAssignmentDeletionStatus(ctx, &ssoadmin.DescribeAccountAssignmentDeletionStatusInput{
			InstanceArn:                        ptr(instanceARN),
			AccountAssignmentDeletionRequestId: ptr(requestID),
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
			reason := strings.TrimSpace(pointerToString(status.FailureReason))
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
