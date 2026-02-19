package cli

import (
	"context"
	"errors"
	"testing"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/identitystore"
	"github.com/aws/aws-sdk-go-v2/service/ssoadmin"
	"github.com/aws/smithy-go"
)

type mockIAMClient struct {
	createAccessKeyFn               func(context.Context, *iam.CreateAccessKeyInput, ...func(*iam.Options)) (*iam.CreateAccessKeyOutput, error)
	deactivateMFADeviceFn           func(context.Context, *iam.DeactivateMFADeviceInput, ...func(*iam.Options)) (*iam.DeactivateMFADeviceOutput, error)
	deleteAccessKeyFn               func(context.Context, *iam.DeleteAccessKeyInput, ...func(*iam.Options)) (*iam.DeleteAccessKeyOutput, error)
	deleteLoginProfileFn            func(context.Context, *iam.DeleteLoginProfileInput, ...func(*iam.Options)) (*iam.DeleteLoginProfileOutput, error)
	deleteSSHPublicKeyFn            func(context.Context, *iam.DeleteSSHPublicKeyInput, ...func(*iam.Options)) (*iam.DeleteSSHPublicKeyOutput, error)
	deleteSigningCertificateFn      func(context.Context, *iam.DeleteSigningCertificateInput, ...func(*iam.Options)) (*iam.DeleteSigningCertificateOutput, error)
	deleteUserFn                    func(context.Context, *iam.DeleteUserInput, ...func(*iam.Options)) (*iam.DeleteUserOutput, error)
	deleteUserPermissionsBoundaryFn func(context.Context, *iam.DeleteUserPermissionsBoundaryInput, ...func(*iam.Options)) (*iam.DeleteUserPermissionsBoundaryOutput, error)
	deleteUserPolicyFn              func(context.Context, *iam.DeleteUserPolicyInput, ...func(*iam.Options)) (*iam.DeleteUserPolicyOutput, error)
	detachUserPolicyFn              func(context.Context, *iam.DetachUserPolicyInput, ...func(*iam.Options)) (*iam.DetachUserPolicyOutput, error)
	listAccessKeysFn                func(context.Context, *iam.ListAccessKeysInput, ...func(*iam.Options)) (*iam.ListAccessKeysOutput, error)
	listAttachedUserPoliciesFn      func(context.Context, *iam.ListAttachedUserPoliciesInput, ...func(*iam.Options)) (*iam.ListAttachedUserPoliciesOutput, error)
	listGroupsForUserFn             func(context.Context, *iam.ListGroupsForUserInput, ...func(*iam.Options)) (*iam.ListGroupsForUserOutput, error)
	listMFADevicesFn                func(context.Context, *iam.ListMFADevicesInput, ...func(*iam.Options)) (*iam.ListMFADevicesOutput, error)
	listSigningCertificatesFn       func(context.Context, *iam.ListSigningCertificatesInput, ...func(*iam.Options)) (*iam.ListSigningCertificatesOutput, error)
	listSSHPublicKeysFn             func(context.Context, *iam.ListSSHPublicKeysInput, ...func(*iam.Options)) (*iam.ListSSHPublicKeysOutput, error)
	listUserPoliciesFn              func(context.Context, *iam.ListUserPoliciesInput, ...func(*iam.Options)) (*iam.ListUserPoliciesOutput, error)
	removeUserFromGroupFn           func(context.Context, *iam.RemoveUserFromGroupInput, ...func(*iam.Options)) (*iam.RemoveUserFromGroupOutput, error)
	updateAccessKeyFn               func(context.Context, *iam.UpdateAccessKeyInput, ...func(*iam.Options)) (*iam.UpdateAccessKeyOutput, error)
}

func (m *mockIAMClient) CreateAccessKey(ctx context.Context, in *iam.CreateAccessKeyInput, optFns ...func(*iam.Options)) (*iam.CreateAccessKeyOutput, error) {
	if m.createAccessKeyFn == nil {
		return nil, errors.New("CreateAccessKey not mocked")
	}
	return m.createAccessKeyFn(ctx, in, optFns...)
}

func (m *mockIAMClient) DeactivateMFADevice(ctx context.Context, in *iam.DeactivateMFADeviceInput, optFns ...func(*iam.Options)) (*iam.DeactivateMFADeviceOutput, error) {
	if m.deactivateMFADeviceFn == nil {
		return nil, errors.New("DeactivateMFADevice not mocked")
	}
	return m.deactivateMFADeviceFn(ctx, in, optFns...)
}

func (m *mockIAMClient) DeleteAccessKey(ctx context.Context, in *iam.DeleteAccessKeyInput, optFns ...func(*iam.Options)) (*iam.DeleteAccessKeyOutput, error) {
	if m.deleteAccessKeyFn == nil {
		return nil, errors.New("DeleteAccessKey not mocked")
	}
	return m.deleteAccessKeyFn(ctx, in, optFns...)
}

func (m *mockIAMClient) DeleteLoginProfile(ctx context.Context, in *iam.DeleteLoginProfileInput, optFns ...func(*iam.Options)) (*iam.DeleteLoginProfileOutput, error) {
	if m.deleteLoginProfileFn == nil {
		return nil, errors.New("DeleteLoginProfile not mocked")
	}
	return m.deleteLoginProfileFn(ctx, in, optFns...)
}

func (m *mockIAMClient) DeleteSSHPublicKey(ctx context.Context, in *iam.DeleteSSHPublicKeyInput, optFns ...func(*iam.Options)) (*iam.DeleteSSHPublicKeyOutput, error) {
	if m.deleteSSHPublicKeyFn == nil {
		return nil, errors.New("DeleteSSHPublicKey not mocked")
	}
	return m.deleteSSHPublicKeyFn(ctx, in, optFns...)
}

func (m *mockIAMClient) DeleteSigningCertificate(ctx context.Context, in *iam.DeleteSigningCertificateInput, optFns ...func(*iam.Options)) (*iam.DeleteSigningCertificateOutput, error) {
	if m.deleteSigningCertificateFn == nil {
		return nil, errors.New("DeleteSigningCertificate not mocked")
	}
	return m.deleteSigningCertificateFn(ctx, in, optFns...)
}

func (m *mockIAMClient) DeleteUser(ctx context.Context, in *iam.DeleteUserInput, optFns ...func(*iam.Options)) (*iam.DeleteUserOutput, error) {
	if m.deleteUserFn == nil {
		return nil, errors.New("DeleteUser not mocked")
	}
	return m.deleteUserFn(ctx, in, optFns...)
}

func (m *mockIAMClient) DeleteUserPermissionsBoundary(ctx context.Context, in *iam.DeleteUserPermissionsBoundaryInput, optFns ...func(*iam.Options)) (*iam.DeleteUserPermissionsBoundaryOutput, error) {
	if m.deleteUserPermissionsBoundaryFn == nil {
		return nil, errors.New("DeleteUserPermissionsBoundary not mocked")
	}
	return m.deleteUserPermissionsBoundaryFn(ctx, in, optFns...)
}

func (m *mockIAMClient) DeleteUserPolicy(ctx context.Context, in *iam.DeleteUserPolicyInput, optFns ...func(*iam.Options)) (*iam.DeleteUserPolicyOutput, error) {
	if m.deleteUserPolicyFn == nil {
		return nil, errors.New("DeleteUserPolicy not mocked")
	}
	return m.deleteUserPolicyFn(ctx, in, optFns...)
}

func (m *mockIAMClient) DetachUserPolicy(ctx context.Context, in *iam.DetachUserPolicyInput, optFns ...func(*iam.Options)) (*iam.DetachUserPolicyOutput, error) {
	if m.detachUserPolicyFn == nil {
		return nil, errors.New("DetachUserPolicy not mocked")
	}
	return m.detachUserPolicyFn(ctx, in, optFns...)
}

func (m *mockIAMClient) ListAccessKeys(ctx context.Context, in *iam.ListAccessKeysInput, optFns ...func(*iam.Options)) (*iam.ListAccessKeysOutput, error) {
	if m.listAccessKeysFn == nil {
		return nil, errors.New("ListAccessKeys not mocked")
	}
	return m.listAccessKeysFn(ctx, in, optFns...)
}

func (m *mockIAMClient) ListAttachedUserPolicies(ctx context.Context, in *iam.ListAttachedUserPoliciesInput, optFns ...func(*iam.Options)) (*iam.ListAttachedUserPoliciesOutput, error) {
	if m.listAttachedUserPoliciesFn == nil {
		return nil, errors.New("ListAttachedUserPolicies not mocked")
	}
	return m.listAttachedUserPoliciesFn(ctx, in, optFns...)
}

func (m *mockIAMClient) ListGroupsForUser(ctx context.Context, in *iam.ListGroupsForUserInput, optFns ...func(*iam.Options)) (*iam.ListGroupsForUserOutput, error) {
	if m.listGroupsForUserFn == nil {
		return nil, errors.New("ListGroupsForUser not mocked")
	}
	return m.listGroupsForUserFn(ctx, in, optFns...)
}

func (m *mockIAMClient) ListMFADevices(ctx context.Context, in *iam.ListMFADevicesInput, optFns ...func(*iam.Options)) (*iam.ListMFADevicesOutput, error) {
	if m.listMFADevicesFn == nil {
		return nil, errors.New("ListMFADevices not mocked")
	}
	return m.listMFADevicesFn(ctx, in, optFns...)
}

func (m *mockIAMClient) ListSigningCertificates(ctx context.Context, in *iam.ListSigningCertificatesInput, optFns ...func(*iam.Options)) (*iam.ListSigningCertificatesOutput, error) {
	if m.listSigningCertificatesFn == nil {
		return nil, errors.New("ListSigningCertificates not mocked")
	}
	return m.listSigningCertificatesFn(ctx, in, optFns...)
}

func (m *mockIAMClient) ListSSHPublicKeys(ctx context.Context, in *iam.ListSSHPublicKeysInput, optFns ...func(*iam.Options)) (*iam.ListSSHPublicKeysOutput, error) {
	if m.listSSHPublicKeysFn == nil {
		return nil, errors.New("ListSSHPublicKeys not mocked")
	}
	return m.listSSHPublicKeysFn(ctx, in, optFns...)
}

func (m *mockIAMClient) ListUserPolicies(ctx context.Context, in *iam.ListUserPoliciesInput, optFns ...func(*iam.Options)) (*iam.ListUserPoliciesOutput, error) {
	if m.listUserPoliciesFn == nil {
		return nil, errors.New("ListUserPolicies not mocked")
	}
	return m.listUserPoliciesFn(ctx, in, optFns...)
}

func (m *mockIAMClient) RemoveUserFromGroup(ctx context.Context, in *iam.RemoveUserFromGroupInput, optFns ...func(*iam.Options)) (*iam.RemoveUserFromGroupOutput, error) {
	if m.removeUserFromGroupFn == nil {
		return nil, errors.New("RemoveUserFromGroup not mocked")
	}
	return m.removeUserFromGroupFn(ctx, in, optFns...)
}

func (m *mockIAMClient) UpdateAccessKey(ctx context.Context, in *iam.UpdateAccessKeyInput, optFns ...func(*iam.Options)) (*iam.UpdateAccessKeyOutput, error) {
	if m.updateAccessKeyFn == nil {
		return nil, errors.New("UpdateAccessKey not mocked")
	}
	return m.updateAccessKeyFn(ctx, in, optFns...)
}

type mockIdentityStoreClient struct {
	createGroupMembershipFn func(context.Context, *identitystore.CreateGroupMembershipInput, ...func(*identitystore.Options)) (*identitystore.CreateGroupMembershipOutput, error)
	createUserFn            func(context.Context, *identitystore.CreateUserInput, ...func(*identitystore.Options)) (*identitystore.CreateUserOutput, error)
	listGroupsFn            func(context.Context, *identitystore.ListGroupsInput, ...func(*identitystore.Options)) (*identitystore.ListGroupsOutput, error)
}

func (m *mockIdentityStoreClient) CreateGroupMembership(ctx context.Context, in *identitystore.CreateGroupMembershipInput, optFns ...func(*identitystore.Options)) (*identitystore.CreateGroupMembershipOutput, error) {
	if m.createGroupMembershipFn == nil {
		return nil, errors.New("CreateGroupMembership not mocked")
	}
	return m.createGroupMembershipFn(ctx, in, optFns...)
}

func (m *mockIdentityStoreClient) CreateUser(ctx context.Context, in *identitystore.CreateUserInput, optFns ...func(*identitystore.Options)) (*identitystore.CreateUserOutput, error) {
	if m.createUserFn == nil {
		return nil, errors.New("CreateUser not mocked")
	}
	return m.createUserFn(ctx, in, optFns...)
}

func (m *mockIdentityStoreClient) ListGroups(ctx context.Context, in *identitystore.ListGroupsInput, optFns ...func(*identitystore.Options)) (*identitystore.ListGroupsOutput, error) {
	if m.listGroupsFn == nil {
		return nil, errors.New("ListGroups not mocked")
	}
	return m.listGroupsFn(ctx, in, optFns...)
}

type mockSSOAdminClient struct {
	listInstancesFn func(context.Context, *ssoadmin.ListInstancesInput, ...func(*ssoadmin.Options)) (*ssoadmin.ListInstancesOutput, error)
}

func (m *mockSSOAdminClient) ListInstances(ctx context.Context, in *ssoadmin.ListInstancesInput, optFns ...func(*ssoadmin.Options)) (*ssoadmin.ListInstancesOutput, error) {
	if m.listInstancesFn == nil {
		return nil, errors.New("ListInstances not mocked")
	}
	return m.listInstancesFn(ctx, in, optFns...)
}

func withMockIAMDeps(
	t *testing.T,
	loader func(string, string) (awssdk.Config, error),
	newIAMClientFn func(awssdk.Config) iamAPI,
	newIdentityStoreClientFn func(awssdk.Config) iamIdentityStoreAPI,
	newSSOAdminClientFn func(awssdk.Config) iamSSOAdminAPI,
) {
	t.Helper()

	oldLoader := iamLoadAWSConfig
	oldIAMClient := iamNewClient
	oldIdentityStoreClient := iamNewIdentityStoreClient
	oldSSOAdminClient := iamNewSSOAdminClient

	iamLoadAWSConfig = loader
	iamNewClient = newIAMClientFn
	iamNewIdentityStoreClient = newIdentityStoreClientFn
	iamNewSSOAdminClient = newSSOAdminClientFn

	t.Cleanup(func() {
		iamLoadAWSConfig = oldLoader
		iamNewClient = oldIAMClient
		iamNewIdentityStoreClient = oldIdentityStoreClient
		iamNewSSOAdminClient = oldSSOAdminClient
	})
}

type noSuchEntityAPIError struct{}

func (noSuchEntityAPIError) Error() string                 { return "not found" }
func (noSuchEntityAPIError) ErrorCode() string             { return "NoSuchEntity" }
func (noSuchEntityAPIError) ErrorMessage() string          { return "not found" }
func (noSuchEntityAPIError) ErrorFault() smithy.ErrorFault { return smithy.FaultClient }
