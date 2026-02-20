package cli

import (
	"context"
	"fmt"
	"sort"
	"strings"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/account"
	"github.com/aws/aws-sdk-go-v2/service/identitystore"
	identitystoretypes "github.com/aws/aws-sdk-go-v2/service/identitystore/types"
	"github.com/aws/aws-sdk-go-v2/service/organizations"
	organizationtypes "github.com/aws/aws-sdk-go-v2/service/organizations/types"
	"github.com/aws/aws-sdk-go-v2/service/ssoadmin"
	ssoadmintypes "github.com/aws/aws-sdk-go-v2/service/ssoadmin/types"
	"github.com/spf13/cobra"
	awstbxaws "github.com/towardsthecloud/aws-toolbox/internal/aws"
)

type organizationsAPI interface {
	DescribeAccount(context.Context, *organizations.DescribeAccountInput, ...func(*organizations.Options)) (*organizations.DescribeAccountOutput, error)
	DescribeOrganizationalUnit(context.Context, *organizations.DescribeOrganizationalUnitInput, ...func(*organizations.Options)) (*organizations.DescribeOrganizationalUnitOutput, error)
	ListAccounts(context.Context, *organizations.ListAccountsInput, ...func(*organizations.Options)) (*organizations.ListAccountsOutput, error)
	ListAccountsForParent(context.Context, *organizations.ListAccountsForParentInput, ...func(*organizations.Options)) (*organizations.ListAccountsForParentOutput, error)
	ListOrganizationalUnitsForParent(context.Context, *organizations.ListOrganizationalUnitsForParentInput, ...func(*organizations.Options)) (*organizations.ListOrganizationalUnitsForParentOutput, error)
	ListParents(context.Context, *organizations.ListParentsInput, ...func(*organizations.Options)) (*organizations.ListParentsOutput, error)
	ListRoots(context.Context, *organizations.ListRootsInput, ...func(*organizations.Options)) (*organizations.ListRootsOutput, error)
	ListTagsForResource(context.Context, *organizations.ListTagsForResourceInput, ...func(*organizations.Options)) (*organizations.ListTagsForResourceOutput, error)
}

type ssoAdminAPI interface {
	CreateAccountAssignment(context.Context, *ssoadmin.CreateAccountAssignmentInput, ...func(*ssoadmin.Options)) (*ssoadmin.CreateAccountAssignmentOutput, error)
	DescribeAccountAssignmentCreationStatus(context.Context, *ssoadmin.DescribeAccountAssignmentCreationStatusInput, ...func(*ssoadmin.Options)) (*ssoadmin.DescribeAccountAssignmentCreationStatusOutput, error)
	DescribeAccountAssignmentDeletionStatus(context.Context, *ssoadmin.DescribeAccountAssignmentDeletionStatusInput, ...func(*ssoadmin.Options)) (*ssoadmin.DescribeAccountAssignmentDeletionStatusOutput, error)
	DeleteAccountAssignment(context.Context, *ssoadmin.DeleteAccountAssignmentInput, ...func(*ssoadmin.Options)) (*ssoadmin.DeleteAccountAssignmentOutput, error)
	DescribePermissionSet(context.Context, *ssoadmin.DescribePermissionSetInput, ...func(*ssoadmin.Options)) (*ssoadmin.DescribePermissionSetOutput, error)
	ListAccountAssignments(context.Context, *ssoadmin.ListAccountAssignmentsInput, ...func(*ssoadmin.Options)) (*ssoadmin.ListAccountAssignmentsOutput, error)
	ListInstances(context.Context, *ssoadmin.ListInstancesInput, ...func(*ssoadmin.Options)) (*ssoadmin.ListInstancesOutput, error)
	ListPermissionSets(context.Context, *ssoadmin.ListPermissionSetsInput, ...func(*ssoadmin.Options)) (*ssoadmin.ListPermissionSetsOutput, error)
}

type identityStoreAPI interface {
	CreateGroup(context.Context, *identitystore.CreateGroupInput, ...func(*identitystore.Options)) (*identitystore.CreateGroupOutput, error)
	CreateGroupMembership(context.Context, *identitystore.CreateGroupMembershipInput, ...func(*identitystore.Options)) (*identitystore.CreateGroupMembershipOutput, error)
	CreateUser(context.Context, *identitystore.CreateUserInput, ...func(*identitystore.Options)) (*identitystore.CreateUserOutput, error)
	DescribeGroup(context.Context, *identitystore.DescribeGroupInput, ...func(*identitystore.Options)) (*identitystore.DescribeGroupOutput, error)
	DescribeUser(context.Context, *identitystore.DescribeUserInput, ...func(*identitystore.Options)) (*identitystore.DescribeUserOutput, error)
	ListGroups(context.Context, *identitystore.ListGroupsInput, ...func(*identitystore.Options)) (*identitystore.ListGroupsOutput, error)
	ListUsers(context.Context, *identitystore.ListUsersInput, ...func(*identitystore.Options)) (*identitystore.ListUsersOutput, error)
}

type accountAPI interface {
	PutAlternateContact(context.Context, *account.PutAlternateContactInput, ...func(*account.Options)) (*account.PutAlternateContactOutput, error)
}

var orgLoadAWSConfig = awstbxaws.LoadAWSConfig
var orgNewOrganizationsClient = func(cfg awssdk.Config) organizationsAPI {
	return organizations.NewFromConfig(cfg)
}
var orgNewSSOAdminClient = func(cfg awssdk.Config) ssoAdminAPI {
	return ssoadmin.NewFromConfig(cfg)
}
var orgNewIdentityStoreClient = func(cfg awssdk.Config) identityStoreAPI {
	return identitystore.NewFromConfig(cfg)
}
var orgNewAccountClient = func(cfg awssdk.Config) accountAPI {
	return account.NewFromConfig(cfg)
}

func newOrgCommand() *cobra.Command {
	cmd := newServiceGroupCommand("org", "Manage Organizations resources")

	cmd.AddCommand(newOrgAssignSSOAccessCommand())
	cmd.AddCommand(newOrgGenerateDiagramCommand())
	cmd.AddCommand(newOrgGetAccountCommand())
	cmd.AddCommand(newOrgImportSSOUsersCommand())
	cmd.AddCommand(newOrgListAccountsCommand())
	cmd.AddCommand(newOrgListSSOAssignmentsCommand())
	cmd.AddCommand(newOrgRemoveSSOAccessCommand())
	cmd.AddCommand(newOrgSetAlternateContactCommand())

	return cmd
}

func newOrgAssignSSOAccessCommand() *cobra.Command {
	var principalName string
	var principalType string
	var permissionSetName string
	var ouName string

	cmd := &cobra.Command{
		Use:   "assign-sso-access",
		Short: "Assign an SSO permission set to accounts in an OU",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runOrgAssignSSOAccess(cmd, principalName, principalType, permissionSetName, ouName)
		},
		SilenceUsage: true,
	}

	cmd.Flags().StringVar(&principalName, "principal-name", "", "Identity Center principal name")
	cmd.Flags().StringVar(&principalType, "principal-type", "GROUP", "Principal type: USER or GROUP")
	cmd.Flags().StringVar(&permissionSetName, "permission-set-name", "", "Identity Center permission set name")
	cmd.Flags().StringVar(&ouName, "ou-name", "", "Organizational unit name")

	return cmd
}

func newOrgGenerateDiagramCommand() *cobra.Command {
	var maxAccountsPerOU int

	cmd := &cobra.Command{
		Use:   "generate-diagram",
		Short: "Generate a Mermaid diagram for organization structure",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runOrgGenerateDiagram(cmd, maxAccountsPerOU)
		},
		SilenceUsage: true,
	}
	cmd.Flags().IntVar(&maxAccountsPerOU, "max-accounts-per-ou", 6, "Maximum accounts to render under each OU")

	return cmd
}

func newOrgGetAccountCommand() *cobra.Command {
	var accountID string

	cmd := &cobra.Command{
		Use:   "get-account",
		Short: "Get account details by account ID",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runOrgGetAccount(cmd, accountID)
		},
		SilenceUsage: true,
	}
	cmd.Flags().StringVar(&accountID, "account-id", "", "12-digit AWS account ID")

	return cmd
}

func newOrgImportSSOUsersCommand() *cobra.Command {
	var inputFile string

	cmd := &cobra.Command{
		Use:   "import-sso-users",
		Short: "Import users and groups into IAM Identity Center from CSV",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runOrgImportSSOUsers(cmd, inputFile)
		},
		SilenceUsage: true,
	}
	cmd.Flags().StringVar(&inputFile, "input-file", "", "CSV file with first_name,last_name,email,group_name")

	return cmd
}

func newOrgListAccountsCommand() *cobra.Command {
	var ouNames []string

	cmd := &cobra.Command{
		Use:   "list-accounts",
		Short: "List organization accounts",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runOrgListAccounts(cmd, ouNames)
		},
		SilenceUsage: true,
	}
	cmd.Flags().StringSliceVar(&ouNames, "ou-name", nil, "Filter by one or more OU names")

	return cmd
}

func newOrgListSSOAssignmentsCommand() *cobra.Command {
	var accountID string

	cmd := &cobra.Command{
		Use:   "list-sso-assignments",
		Short: "List Identity Center assignments for accounts",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runOrgListSSOAssignments(cmd, accountID)
		},
		SilenceUsage: true,
	}
	cmd.Flags().StringVar(&accountID, "account-id", "", "Optional 12-digit account ID filter")

	return cmd
}

func newOrgRemoveSSOAccessCommand() *cobra.Command {
	var principalName string
	var principalType string
	var permissionSetName string
	var ouName string

	cmd := &cobra.Command{
		Use:   "remove-sso-access",
		Short: "Remove an SSO permission set from accounts in an OU",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runOrgRemoveSSOAccess(cmd, principalName, principalType, permissionSetName, ouName)
		},
		SilenceUsage: true,
	}
	cmd.Flags().StringVar(&principalName, "principal-name", "", "Identity Center principal name")
	cmd.Flags().StringVar(&principalType, "principal-type", "GROUP", "Principal type: USER or GROUP")
	cmd.Flags().StringVar(&permissionSetName, "permission-set-name", "", "Identity Center permission set name")
	cmd.Flags().StringVar(&ouName, "ou-name", "", "Organizational unit name")

	return cmd
}

func newOrgSetAlternateContactCommand() *cobra.Command {
	var inputFile string

	cmd := &cobra.Command{
		Use:   "set-alternate-contact",
		Short: "Set alternate contacts for organization accounts",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runOrgSetAlternateContact(cmd, inputFile)
		},
		SilenceUsage: true,
	}
	cmd.Flags().StringVar(&inputFile, "input-file", "", "JSON file with security/billing/operations contact details")

	return cmd
}

func sortAccountsByID(accounts []organizationtypes.Account) {
	sort.Slice(accounts, func(i, j int) bool {
		return pointerToString(accounts[i].Id) < pointerToString(accounts[j].Id)
	})
}

func ssoPrincipalTypeFromString(raw string) (ssoadmintypes.PrincipalType, error) {
	switch strings.ToUpper(strings.TrimSpace(raw)) {
	case "USER":
		return ssoadmintypes.PrincipalTypeUser, nil
	case "GROUP":
		return ssoadmintypes.PrincipalTypeGroup, nil
	default:
		return "", fmt.Errorf("--principal-type must be USER or GROUP")
	}
}

func identityStoreFilterForPrincipal(principalName string, principalType ssoadmintypes.PrincipalType) identitystoretypes.Filter {
	attributePath := "DisplayName"
	if principalType == ssoadmintypes.PrincipalTypeUser {
		attributePath = "UserName"
	}
	return identitystoretypes.Filter{
		AttributePath:  ptr(attributePath),
		AttributeValue: ptr(principalName),
	}
}
