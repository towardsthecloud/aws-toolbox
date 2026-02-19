package cli

import (
	"context"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/identitystore"
	"github.com/aws/aws-sdk-go-v2/service/ssoadmin"
	"github.com/spf13/cobra"
	awstbxaws "github.com/towardsthecloud/aws-toolbox/internal/aws"
)

type iamAPI interface {
	CreateAccessKey(context.Context, *iam.CreateAccessKeyInput, ...func(*iam.Options)) (*iam.CreateAccessKeyOutput, error)
	DeactivateMFADevice(context.Context, *iam.DeactivateMFADeviceInput, ...func(*iam.Options)) (*iam.DeactivateMFADeviceOutput, error)
	DeleteAccessKey(context.Context, *iam.DeleteAccessKeyInput, ...func(*iam.Options)) (*iam.DeleteAccessKeyOutput, error)
	DeleteLoginProfile(context.Context, *iam.DeleteLoginProfileInput, ...func(*iam.Options)) (*iam.DeleteLoginProfileOutput, error)
	DeleteSSHPublicKey(context.Context, *iam.DeleteSSHPublicKeyInput, ...func(*iam.Options)) (*iam.DeleteSSHPublicKeyOutput, error)
	DeleteSigningCertificate(context.Context, *iam.DeleteSigningCertificateInput, ...func(*iam.Options)) (*iam.DeleteSigningCertificateOutput, error)
	DeleteUser(context.Context, *iam.DeleteUserInput, ...func(*iam.Options)) (*iam.DeleteUserOutput, error)
	DeleteUserPermissionsBoundary(context.Context, *iam.DeleteUserPermissionsBoundaryInput, ...func(*iam.Options)) (*iam.DeleteUserPermissionsBoundaryOutput, error)
	DeleteUserPolicy(context.Context, *iam.DeleteUserPolicyInput, ...func(*iam.Options)) (*iam.DeleteUserPolicyOutput, error)
	DetachUserPolicy(context.Context, *iam.DetachUserPolicyInput, ...func(*iam.Options)) (*iam.DetachUserPolicyOutput, error)
	ListAccessKeys(context.Context, *iam.ListAccessKeysInput, ...func(*iam.Options)) (*iam.ListAccessKeysOutput, error)
	ListAttachedUserPolicies(context.Context, *iam.ListAttachedUserPoliciesInput, ...func(*iam.Options)) (*iam.ListAttachedUserPoliciesOutput, error)
	ListGroupsForUser(context.Context, *iam.ListGroupsForUserInput, ...func(*iam.Options)) (*iam.ListGroupsForUserOutput, error)
	ListMFADevices(context.Context, *iam.ListMFADevicesInput, ...func(*iam.Options)) (*iam.ListMFADevicesOutput, error)
	ListSigningCertificates(context.Context, *iam.ListSigningCertificatesInput, ...func(*iam.Options)) (*iam.ListSigningCertificatesOutput, error)
	ListSSHPublicKeys(context.Context, *iam.ListSSHPublicKeysInput, ...func(*iam.Options)) (*iam.ListSSHPublicKeysOutput, error)
	ListUserPolicies(context.Context, *iam.ListUserPoliciesInput, ...func(*iam.Options)) (*iam.ListUserPoliciesOutput, error)
	RemoveUserFromGroup(context.Context, *iam.RemoveUserFromGroupInput, ...func(*iam.Options)) (*iam.RemoveUserFromGroupOutput, error)
	UpdateAccessKey(context.Context, *iam.UpdateAccessKeyInput, ...func(*iam.Options)) (*iam.UpdateAccessKeyOutput, error)
}

type iamIdentityStoreAPI interface {
	CreateGroupMembership(context.Context, *identitystore.CreateGroupMembershipInput, ...func(*identitystore.Options)) (*identitystore.CreateGroupMembershipOutput, error)
	CreateUser(context.Context, *identitystore.CreateUserInput, ...func(*identitystore.Options)) (*identitystore.CreateUserOutput, error)
	ListGroups(context.Context, *identitystore.ListGroupsInput, ...func(*identitystore.Options)) (*identitystore.ListGroupsOutput, error)
}

type iamSSOAdminAPI interface {
	ListInstances(context.Context, *ssoadmin.ListInstancesInput, ...func(*ssoadmin.Options)) (*ssoadmin.ListInstancesOutput, error)
}

var iamLoadAWSConfig = awstbxaws.LoadAWSConfig
var iamNewClient = func(cfg awssdk.Config) iamAPI {
	return iam.NewFromConfig(cfg)
}
var iamNewIdentityStoreClient = func(cfg awssdk.Config) iamIdentityStoreAPI {
	return identitystore.NewFromConfig(cfg)
}
var iamNewSSOAdminClient = func(cfg awssdk.Config) iamSSOAdminAPI {
	return ssoadmin.NewFromConfig(cfg)
}

func newIAMCommand() *cobra.Command {
	cmd := newServiceGroupCommand("iam", "Manage IAM resources")

	cmd.AddCommand(newIAMCreateSSOUsersCommand())
	cmd.AddCommand(newIAMDeleteUserCommand())
	cmd.AddCommand(newIAMRotateKeysCommand())

	return cmd
}

func newIAMCreateSSOUsersCommand() *cobra.Command {
	var emails []string
	var inputFile string
	var groupName string

	cmd := &cobra.Command{
		Use:   "create-sso-users",
		Short: "Create IAM Identity Center users",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runIAMCreateSSOUsers(cmd, emails, inputFile, groupName)
		},
		SilenceUsage: true,
	}
	cmd.Flags().StringSliceVar(&emails, "emails", nil, "Comma-separated email addresses to create")
	cmd.Flags().StringVar(&inputFile, "input-file", "", "Path to a file containing email addresses")
	cmd.Flags().StringVar(&groupName, "group", "", "Optional Identity Center group display name")

	return cmd
}

func newIAMDeleteUserCommand() *cobra.Command {
	var username string

	cmd := &cobra.Command{
		Use:   "delete-user",
		Short: "Cascade-delete an IAM user and dependencies",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runIAMDeleteUser(cmd, username)
		},
		SilenceUsage: true,
	}
	cmd.Flags().StringVar(&username, "username", "", "IAM username to delete")

	return cmd
}

func newIAMRotateKeysCommand() *cobra.Command {
	var username string
	var keyID string
	var disable bool
	var deleteKey bool

	cmd := &cobra.Command{
		Use:   "rotate-keys",
		Short: "Create, disable, or delete IAM access keys",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runIAMRotateKeys(cmd, username, keyID, disable, deleteKey)
		},
		SilenceUsage: true,
	}
	cmd.Flags().StringVar(&username, "username", "", "IAM username")
	cmd.Flags().StringVar(&keyID, "key", "", "Access key ID for --disable or --delete")
	cmd.Flags().BoolVar(&disable, "disable", false, "Disable the access key specified by --key")
	cmd.Flags().BoolVar(&deleteKey, "delete", false, "Delete the access key specified by --key")

	return cmd
}
