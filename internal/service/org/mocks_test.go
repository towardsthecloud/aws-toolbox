package org

import (
	"context"
	"errors"
	"testing"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/account"
	"github.com/aws/aws-sdk-go-v2/service/identitystore"
	"github.com/aws/aws-sdk-go-v2/service/organizations"
	"github.com/aws/aws-sdk-go-v2/service/ssoadmin"
)

type mockOrganizationsClient struct {
	describeAccountFn func(context.Context, *organizations.DescribeAccountInput, ...func(*organizations.Options)) (*organizations.DescribeAccountOutput, error)
	describeOUFn      func(context.Context, *organizations.DescribeOrganizationalUnitInput, ...func(*organizations.Options)) (*organizations.DescribeOrganizationalUnitOutput, error)
	listAccountsFn    func(context.Context, *organizations.ListAccountsInput, ...func(*organizations.Options)) (*organizations.ListAccountsOutput, error)
	listForParentFn   func(context.Context, *organizations.ListAccountsForParentInput, ...func(*organizations.Options)) (*organizations.ListAccountsForParentOutput, error)
	listOUsFn         func(context.Context, *organizations.ListOrganizationalUnitsForParentInput, ...func(*organizations.Options)) (*organizations.ListOrganizationalUnitsForParentOutput, error)
	listParentsFn     func(context.Context, *organizations.ListParentsInput, ...func(*organizations.Options)) (*organizations.ListParentsOutput, error)
	listRootsFn       func(context.Context, *organizations.ListRootsInput, ...func(*organizations.Options)) (*organizations.ListRootsOutput, error)
	listTagsFn        func(context.Context, *organizations.ListTagsForResourceInput, ...func(*organizations.Options)) (*organizations.ListTagsForResourceOutput, error)
}

func (m *mockOrganizationsClient) DescribeAccount(ctx context.Context, in *organizations.DescribeAccountInput, optFns ...func(*organizations.Options)) (*organizations.DescribeAccountOutput, error) {
	if m.describeAccountFn == nil {
		return nil, errors.New("DescribeAccount not mocked")
	}
	return m.describeAccountFn(ctx, in, optFns...)
}

func (m *mockOrganizationsClient) DescribeOrganizationalUnit(ctx context.Context, in *organizations.DescribeOrganizationalUnitInput, optFns ...func(*organizations.Options)) (*organizations.DescribeOrganizationalUnitOutput, error) {
	if m.describeOUFn == nil {
		return nil, errors.New("DescribeOrganizationalUnit not mocked")
	}
	return m.describeOUFn(ctx, in, optFns...)
}

func (m *mockOrganizationsClient) ListAccounts(ctx context.Context, in *organizations.ListAccountsInput, optFns ...func(*organizations.Options)) (*organizations.ListAccountsOutput, error) {
	if m.listAccountsFn == nil {
		return nil, errors.New("ListAccounts not mocked")
	}
	return m.listAccountsFn(ctx, in, optFns...)
}

func (m *mockOrganizationsClient) ListAccountsForParent(ctx context.Context, in *organizations.ListAccountsForParentInput, optFns ...func(*organizations.Options)) (*organizations.ListAccountsForParentOutput, error) {
	if m.listForParentFn == nil {
		return nil, errors.New("ListAccountsForParent not mocked")
	}
	return m.listForParentFn(ctx, in, optFns...)
}

func (m *mockOrganizationsClient) ListOrganizationalUnitsForParent(ctx context.Context, in *organizations.ListOrganizationalUnitsForParentInput, optFns ...func(*organizations.Options)) (*organizations.ListOrganizationalUnitsForParentOutput, error) {
	if m.listOUsFn == nil {
		return nil, errors.New("ListOrganizationalUnitsForParent not mocked")
	}
	return m.listOUsFn(ctx, in, optFns...)
}

func (m *mockOrganizationsClient) ListParents(ctx context.Context, in *organizations.ListParentsInput, optFns ...func(*organizations.Options)) (*organizations.ListParentsOutput, error) {
	if m.listParentsFn == nil {
		return nil, errors.New("ListParents not mocked")
	}
	return m.listParentsFn(ctx, in, optFns...)
}

func (m *mockOrganizationsClient) ListRoots(ctx context.Context, in *organizations.ListRootsInput, optFns ...func(*organizations.Options)) (*organizations.ListRootsOutput, error) {
	if m.listRootsFn == nil {
		return nil, errors.New("ListRoots not mocked")
	}
	return m.listRootsFn(ctx, in, optFns...)
}

func (m *mockOrganizationsClient) ListTagsForResource(ctx context.Context, in *organizations.ListTagsForResourceInput, optFns ...func(*organizations.Options)) (*organizations.ListTagsForResourceOutput, error) {
	if m.listTagsFn == nil {
		return nil, errors.New("ListTagsForResource not mocked")
	}
	return m.listTagsFn(ctx, in, optFns...)
}

type mockSSOAdminClient struct {
	createAssignmentFn       func(context.Context, *ssoadmin.CreateAccountAssignmentInput, ...func(*ssoadmin.Options)) (*ssoadmin.CreateAccountAssignmentOutput, error)
	deleteAssignmentFn       func(context.Context, *ssoadmin.DeleteAccountAssignmentInput, ...func(*ssoadmin.Options)) (*ssoadmin.DeleteAccountAssignmentOutput, error)
	describeCreationStatusFn func(context.Context, *ssoadmin.DescribeAccountAssignmentCreationStatusInput, ...func(*ssoadmin.Options)) (*ssoadmin.DescribeAccountAssignmentCreationStatusOutput, error)
	describeDeletionStatusFn func(context.Context, *ssoadmin.DescribeAccountAssignmentDeletionStatusInput, ...func(*ssoadmin.Options)) (*ssoadmin.DescribeAccountAssignmentDeletionStatusOutput, error)
	describePSFn             func(context.Context, *ssoadmin.DescribePermissionSetInput, ...func(*ssoadmin.Options)) (*ssoadmin.DescribePermissionSetOutput, error)
	listAssignmentsFn        func(context.Context, *ssoadmin.ListAccountAssignmentsInput, ...func(*ssoadmin.Options)) (*ssoadmin.ListAccountAssignmentsOutput, error)
	listInstancesFn          func(context.Context, *ssoadmin.ListInstancesInput, ...func(*ssoadmin.Options)) (*ssoadmin.ListInstancesOutput, error)
	listPSFn                 func(context.Context, *ssoadmin.ListPermissionSetsInput, ...func(*ssoadmin.Options)) (*ssoadmin.ListPermissionSetsOutput, error)
}

func (m *mockSSOAdminClient) CreateAccountAssignment(ctx context.Context, in *ssoadmin.CreateAccountAssignmentInput, optFns ...func(*ssoadmin.Options)) (*ssoadmin.CreateAccountAssignmentOutput, error) {
	if m.createAssignmentFn == nil {
		return nil, errors.New("CreateAccountAssignment not mocked")
	}
	return m.createAssignmentFn(ctx, in, optFns...)
}

func (m *mockSSOAdminClient) DeleteAccountAssignment(ctx context.Context, in *ssoadmin.DeleteAccountAssignmentInput, optFns ...func(*ssoadmin.Options)) (*ssoadmin.DeleteAccountAssignmentOutput, error) {
	if m.deleteAssignmentFn == nil {
		return nil, errors.New("DeleteAccountAssignment not mocked")
	}
	return m.deleteAssignmentFn(ctx, in, optFns...)
}

func (m *mockSSOAdminClient) DescribeAccountAssignmentCreationStatus(ctx context.Context, in *ssoadmin.DescribeAccountAssignmentCreationStatusInput, optFns ...func(*ssoadmin.Options)) (*ssoadmin.DescribeAccountAssignmentCreationStatusOutput, error) {
	if m.describeCreationStatusFn == nil {
		return nil, errors.New("DescribeAccountAssignmentCreationStatus not mocked")
	}
	return m.describeCreationStatusFn(ctx, in, optFns...)
}

func (m *mockSSOAdminClient) DescribeAccountAssignmentDeletionStatus(ctx context.Context, in *ssoadmin.DescribeAccountAssignmentDeletionStatusInput, optFns ...func(*ssoadmin.Options)) (*ssoadmin.DescribeAccountAssignmentDeletionStatusOutput, error) {
	if m.describeDeletionStatusFn == nil {
		return nil, errors.New("DescribeAccountAssignmentDeletionStatus not mocked")
	}
	return m.describeDeletionStatusFn(ctx, in, optFns...)
}

func (m *mockSSOAdminClient) DescribePermissionSet(ctx context.Context, in *ssoadmin.DescribePermissionSetInput, optFns ...func(*ssoadmin.Options)) (*ssoadmin.DescribePermissionSetOutput, error) {
	if m.describePSFn == nil {
		return nil, errors.New("DescribePermissionSet not mocked")
	}
	return m.describePSFn(ctx, in, optFns...)
}

func (m *mockSSOAdminClient) ListAccountAssignments(ctx context.Context, in *ssoadmin.ListAccountAssignmentsInput, optFns ...func(*ssoadmin.Options)) (*ssoadmin.ListAccountAssignmentsOutput, error) {
	if m.listAssignmentsFn == nil {
		return nil, errors.New("ListAccountAssignments not mocked")
	}
	return m.listAssignmentsFn(ctx, in, optFns...)
}

func (m *mockSSOAdminClient) ListInstances(ctx context.Context, in *ssoadmin.ListInstancesInput, optFns ...func(*ssoadmin.Options)) (*ssoadmin.ListInstancesOutput, error) {
	if m.listInstancesFn == nil {
		return nil, errors.New("ListInstances not mocked")
	}
	return m.listInstancesFn(ctx, in, optFns...)
}

func (m *mockSSOAdminClient) ListPermissionSets(ctx context.Context, in *ssoadmin.ListPermissionSetsInput, optFns ...func(*ssoadmin.Options)) (*ssoadmin.ListPermissionSetsOutput, error) {
	if m.listPSFn == nil {
		return nil, errors.New("ListPermissionSets not mocked")
	}
	return m.listPSFn(ctx, in, optFns...)
}

type mockIdentityStoreClient struct {
	createGroupFn   func(context.Context, *identitystore.CreateGroupInput, ...func(*identitystore.Options)) (*identitystore.CreateGroupOutput, error)
	createMemberFn  func(context.Context, *identitystore.CreateGroupMembershipInput, ...func(*identitystore.Options)) (*identitystore.CreateGroupMembershipOutput, error)
	createUserFn    func(context.Context, *identitystore.CreateUserInput, ...func(*identitystore.Options)) (*identitystore.CreateUserOutput, error)
	describeGroupFn func(context.Context, *identitystore.DescribeGroupInput, ...func(*identitystore.Options)) (*identitystore.DescribeGroupOutput, error)
	describeUserFn  func(context.Context, *identitystore.DescribeUserInput, ...func(*identitystore.Options)) (*identitystore.DescribeUserOutput, error)
	listGroupsFn    func(context.Context, *identitystore.ListGroupsInput, ...func(*identitystore.Options)) (*identitystore.ListGroupsOutput, error)
	listUsersFn     func(context.Context, *identitystore.ListUsersInput, ...func(*identitystore.Options)) (*identitystore.ListUsersOutput, error)
}

func (m *mockIdentityStoreClient) CreateGroup(ctx context.Context, in *identitystore.CreateGroupInput, optFns ...func(*identitystore.Options)) (*identitystore.CreateGroupOutput, error) {
	if m.createGroupFn == nil {
		return nil, errors.New("CreateGroup not mocked")
	}
	return m.createGroupFn(ctx, in, optFns...)
}

func (m *mockIdentityStoreClient) CreateGroupMembership(ctx context.Context, in *identitystore.CreateGroupMembershipInput, optFns ...func(*identitystore.Options)) (*identitystore.CreateGroupMembershipOutput, error) {
	if m.createMemberFn == nil {
		return nil, errors.New("CreateGroupMembership not mocked")
	}
	return m.createMemberFn(ctx, in, optFns...)
}

func (m *mockIdentityStoreClient) CreateUser(ctx context.Context, in *identitystore.CreateUserInput, optFns ...func(*identitystore.Options)) (*identitystore.CreateUserOutput, error) {
	if m.createUserFn == nil {
		return nil, errors.New("CreateUser not mocked")
	}
	return m.createUserFn(ctx, in, optFns...)
}

func (m *mockIdentityStoreClient) DescribeGroup(ctx context.Context, in *identitystore.DescribeGroupInput, optFns ...func(*identitystore.Options)) (*identitystore.DescribeGroupOutput, error) {
	if m.describeGroupFn == nil {
		return nil, errors.New("DescribeGroup not mocked")
	}
	return m.describeGroupFn(ctx, in, optFns...)
}

func (m *mockIdentityStoreClient) DescribeUser(ctx context.Context, in *identitystore.DescribeUserInput, optFns ...func(*identitystore.Options)) (*identitystore.DescribeUserOutput, error) {
	if m.describeUserFn == nil {
		return nil, errors.New("DescribeUser not mocked")
	}
	return m.describeUserFn(ctx, in, optFns...)
}

func (m *mockIdentityStoreClient) ListGroups(ctx context.Context, in *identitystore.ListGroupsInput, optFns ...func(*identitystore.Options)) (*identitystore.ListGroupsOutput, error) {
	if m.listGroupsFn == nil {
		return nil, errors.New("ListGroups not mocked")
	}
	return m.listGroupsFn(ctx, in, optFns...)
}

func (m *mockIdentityStoreClient) ListUsers(ctx context.Context, in *identitystore.ListUsersInput, optFns ...func(*identitystore.Options)) (*identitystore.ListUsersOutput, error) {
	if m.listUsersFn == nil {
		return nil, errors.New("ListUsers not mocked")
	}
	return m.listUsersFn(ctx, in, optFns...)
}

type mockAccountClient struct {
	putAlternateContactFn func(context.Context, *account.PutAlternateContactInput, ...func(*account.Options)) (*account.PutAlternateContactOutput, error)
}

func (m *mockAccountClient) PutAlternateContact(ctx context.Context, in *account.PutAlternateContactInput, optFns ...func(*account.Options)) (*account.PutAlternateContactOutput, error) {
	if m.putAlternateContactFn == nil {
		return nil, errors.New("PutAlternateContact not mocked")
	}
	return m.putAlternateContactFn(ctx, in, optFns...)
}

func withMockDeps(t *testing.T, loader func(string, string) (awssdk.Config, error), orgFactory func(awssdk.Config) OrganizationsAPI, ssoFactory func(awssdk.Config) SSOAdminAPI, identityFactory func(awssdk.Config) IdentityStoreAPI, accountFactory func(awssdk.Config) AccountAPI) {
	t.Helper()

	oldLoader := loadAWSConfig
	oldOrg := newOrganizationsClient
	oldSSO := newSSOAdminClient
	oldIdentity := newIdentityStoreClient
	oldAccount := newAccountClient

	loadAWSConfig = loader
	newOrganizationsClient = orgFactory
	newSSOAdminClient = ssoFactory
	newIdentityStoreClient = identityFactory
	newAccountClient = accountFactory

	t.Cleanup(func() {
		loadAWSConfig = oldLoader
		newOrganizationsClient = oldOrg
		newSSOAdminClient = oldSSO
		newIdentityStoreClient = oldIdentity
		newAccountClient = oldAccount
	})
}
