package cli

import (
	"context"
	"strings"
	"testing"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/aws/aws-sdk-go-v2/service/identitystore"
	identitystoretypes "github.com/aws/aws-sdk-go-v2/service/identitystore/types"
	"github.com/aws/aws-sdk-go-v2/service/ssoadmin"
	ssoadmintypes "github.com/aws/aws-sdk-go-v2/service/ssoadmin/types"
)

func TestIAMDeleteUserCascadeExecutesInOrder(t *testing.T) {
	calls := make([]string, 0)
	client := &mockIAMClient{
		listAccessKeysFn: func(_ context.Context, _ *iam.ListAccessKeysInput, _ ...func(*iam.Options)) (*iam.ListAccessKeysOutput, error) {
			calls = append(calls, "list-access-keys")
			return &iam.ListAccessKeysOutput{AccessKeyMetadata: []iamtypes.AccessKeyMetadata{{AccessKeyId: ptr("AKIA123")}}}, nil
		},
		deleteAccessKeyFn: func(_ context.Context, in *iam.DeleteAccessKeyInput, _ ...func(*iam.Options)) (*iam.DeleteAccessKeyOutput, error) {
			calls = append(calls, "delete-access-key:"+pointerToString(in.AccessKeyId))
			return &iam.DeleteAccessKeyOutput{}, nil
		},
		listMFADevicesFn: func(_ context.Context, _ *iam.ListMFADevicesInput, _ ...func(*iam.Options)) (*iam.ListMFADevicesOutput, error) {
			calls = append(calls, "list-mfa-devices")
			return &iam.ListMFADevicesOutput{MFADevices: []iamtypes.MFADevice{{SerialNumber: ptr("mfa-1")}}}, nil
		},
		deactivateMFADeviceFn: func(_ context.Context, in *iam.DeactivateMFADeviceInput, _ ...func(*iam.Options)) (*iam.DeactivateMFADeviceOutput, error) {
			calls = append(calls, "deactivate-mfa:"+pointerToString(in.SerialNumber))
			return &iam.DeactivateMFADeviceOutput{}, nil
		},
		listAttachedUserPoliciesFn: func(_ context.Context, _ *iam.ListAttachedUserPoliciesInput, _ ...func(*iam.Options)) (*iam.ListAttachedUserPoliciesOutput, error) {
			calls = append(calls, "list-attached-policies")
			return &iam.ListAttachedUserPoliciesOutput{AttachedPolicies: []iamtypes.AttachedPolicy{{PolicyArn: ptr("arn:aws:iam::aws:policy/ReadOnlyAccess")}}}, nil
		},
		detachUserPolicyFn: func(_ context.Context, in *iam.DetachUserPolicyInput, _ ...func(*iam.Options)) (*iam.DetachUserPolicyOutput, error) {
			calls = append(calls, "detach-policy:"+pointerToString(in.PolicyArn))
			return &iam.DetachUserPolicyOutput{}, nil
		},
		listUserPoliciesFn: func(_ context.Context, _ *iam.ListUserPoliciesInput, _ ...func(*iam.Options)) (*iam.ListUserPoliciesOutput, error) {
			calls = append(calls, "list-inline-policies")
			return &iam.ListUserPoliciesOutput{PolicyNames: []string{"inline-pol"}}, nil
		},
		deleteUserPolicyFn: func(_ context.Context, in *iam.DeleteUserPolicyInput, _ ...func(*iam.Options)) (*iam.DeleteUserPolicyOutput, error) {
			calls = append(calls, "delete-inline-policy:"+pointerToString(in.PolicyName))
			return &iam.DeleteUserPolicyOutput{}, nil
		},
		listGroupsForUserFn: func(_ context.Context, _ *iam.ListGroupsForUserInput, _ ...func(*iam.Options)) (*iam.ListGroupsForUserOutput, error) {
			calls = append(calls, "list-groups")
			return &iam.ListGroupsForUserOutput{Groups: []iamtypes.Group{{GroupName: ptr("admins")}}}, nil
		},
		removeUserFromGroupFn: func(_ context.Context, in *iam.RemoveUserFromGroupInput, _ ...func(*iam.Options)) (*iam.RemoveUserFromGroupOutput, error) {
			calls = append(calls, "remove-group:"+pointerToString(in.GroupName))
			return &iam.RemoveUserFromGroupOutput{}, nil
		},
		deleteUserPermissionsBoundaryFn: func(_ context.Context, _ *iam.DeleteUserPermissionsBoundaryInput, _ ...func(*iam.Options)) (*iam.DeleteUserPermissionsBoundaryOutput, error) {
			calls = append(calls, "delete-permissions-boundary")
			return &iam.DeleteUserPermissionsBoundaryOutput{}, nil
		},
		deleteLoginProfileFn: func(_ context.Context, _ *iam.DeleteLoginProfileInput, _ ...func(*iam.Options)) (*iam.DeleteLoginProfileOutput, error) {
			calls = append(calls, "delete-login-profile")
			return &iam.DeleteLoginProfileOutput{}, nil
		},
		listSigningCertificatesFn: func(_ context.Context, _ *iam.ListSigningCertificatesInput, _ ...func(*iam.Options)) (*iam.ListSigningCertificatesOutput, error) {
			calls = append(calls, "list-signing-certificates")
			return &iam.ListSigningCertificatesOutput{Certificates: []iamtypes.SigningCertificate{{CertificateId: ptr("cert-1")}}}, nil
		},
		deleteSigningCertificateFn: func(_ context.Context, in *iam.DeleteSigningCertificateInput, _ ...func(*iam.Options)) (*iam.DeleteSigningCertificateOutput, error) {
			calls = append(calls, "delete-signing-certificate:"+pointerToString(in.CertificateId))
			return &iam.DeleteSigningCertificateOutput{}, nil
		},
		listSSHPublicKeysFn: func(_ context.Context, _ *iam.ListSSHPublicKeysInput, _ ...func(*iam.Options)) (*iam.ListSSHPublicKeysOutput, error) {
			calls = append(calls, "list-ssh-public-keys")
			return &iam.ListSSHPublicKeysOutput{SSHPublicKeys: []iamtypes.SSHPublicKeyMetadata{{SSHPublicKeyId: ptr("ssh-1")}}}, nil
		},
		deleteSSHPublicKeyFn: func(_ context.Context, in *iam.DeleteSSHPublicKeyInput, _ ...func(*iam.Options)) (*iam.DeleteSSHPublicKeyOutput, error) {
			calls = append(calls, "delete-ssh-public-key:"+pointerToString(in.SSHPublicKeyId))
			return &iam.DeleteSSHPublicKeyOutput{}, nil
		},
		deleteUserFn: func(_ context.Context, in *iam.DeleteUserInput, _ ...func(*iam.Options)) (*iam.DeleteUserOutput, error) {
			calls = append(calls, "delete-user:"+pointerToString(in.UserName))
			return &iam.DeleteUserOutput{}, nil
		},
	}

	withMockIAMDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) iamAPI { return client },
		func(awssdk.Config) iamIdentityStoreAPI { return &mockIdentityStoreClient{} },
		func(awssdk.Config) iamSSOAdminAPI { return &mockSSOAdminClient{} },
	)

	output, err := executeCommand(t, "--output", "json", "--no-confirm", "iam", "delete-user", "--username", "alice")
	if err != nil {
		t.Fatalf("execute iam delete-user: %v", err)
	}

	expected := []string{
		"list-access-keys",
		"list-mfa-devices",
		"list-attached-policies",
		"list-inline-policies",
		"list-groups",
		"list-signing-certificates",
		"list-ssh-public-keys",
		"delete-access-key:AKIA123",
		"deactivate-mfa:mfa-1",
		"detach-policy:arn:aws:iam::aws:policy/ReadOnlyAccess",
		"delete-inline-policy:inline-pol",
		"remove-group:admins",
		"delete-permissions-boundary",
		"delete-login-profile",
		"delete-signing-certificate:cert-1",
		"delete-ssh-public-key:ssh-1",
		"delete-user:alice",
	}

	if len(calls) != len(expected) {
		t.Fatalf("unexpected call count: got=%d want=%d calls=%v", len(calls), len(expected), calls)
	}
	for i := range expected {
		if calls[i] != expected[i] {
			t.Fatalf("unexpected call order at %d: got=%q want=%q calls=%v", i, calls[i], expected[i], calls)
		}
	}
	if !strings.Contains(output, "\"step\": \"user\"") || !strings.Contains(output, "\"action\": \"deleted\"") {
		t.Fatalf("unexpected output: %s", output)
	}
}

func TestIAMCreateSSOUsersAllOutputFormats(t *testing.T) {
	createUserCalls := 0
	createMembershipCalls := 0

	identityClient := &mockIdentityStoreClient{
		listGroupsFn: func(_ context.Context, in *identitystore.ListGroupsInput, _ ...func(*identitystore.Options)) (*identitystore.ListGroupsOutput, error) {
			if pointerToString(in.IdentityStoreId) != "d-1234567890" {
				t.Fatalf("unexpected identity store ID: %s", pointerToString(in.IdentityStoreId))
			}
			return &identitystore.ListGroupsOutput{
				Groups: []identitystoretypes.Group{{DisplayName: ptr("engineering"), GroupId: ptr("grp-1")}},
			}, nil
		},
		createUserFn: func(_ context.Context, in *identitystore.CreateUserInput, _ ...func(*identitystore.Options)) (*identitystore.CreateUserOutput, error) {
			createUserCalls++
			if pointerToString(in.UserName) != "john.doe@example.com" {
				t.Fatalf("unexpected username: %s", pointerToString(in.UserName))
			}
			return &identitystore.CreateUserOutput{UserId: ptr("user-1"), IdentityStoreId: in.IdentityStoreId}, nil
		},
		createGroupMembershipFn: func(_ context.Context, in *identitystore.CreateGroupMembershipInput, _ ...func(*identitystore.Options)) (*identitystore.CreateGroupMembershipOutput, error) {
			createMembershipCalls++
			if pointerToString(in.GroupId) != "grp-1" {
				t.Fatalf("unexpected group id: %s", pointerToString(in.GroupId))
			}
			return &identitystore.CreateGroupMembershipOutput{}, nil
		},
	}

	ssoClient := &mockSSOAdminClient{
		listInstancesFn: func(_ context.Context, _ *ssoadmin.ListInstancesInput, _ ...func(*ssoadmin.Options)) (*ssoadmin.ListInstancesOutput, error) {
			return &ssoadmin.ListInstancesOutput{
				Instances: []ssoadmintypes.InstanceMetadata{{IdentityStoreId: ptr("d-1234567890")}},
			}, nil
		},
	}

	withMockIAMDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) iamAPI { return &mockIAMClient{} },
		func(awssdk.Config) iamIdentityStoreAPI { return identityClient },
		func(awssdk.Config) iamSSOAdminAPI { return ssoClient },
	)

	for _, format := range []string{"table", "json", "text"} {
		output, err := executeCommand(t, "--output", format, "--no-confirm", "iam", "create-sso-users", "--emails", "john.doe@example.com", "--group", "engineering")
		if err != nil {
			t.Fatalf("execute create-sso-users (%s): %v", format, err)
		}
		if !strings.Contains(output, "john.doe@example.com") || !strings.Contains(output, "created") {
			t.Fatalf("unexpected output for format=%s: %s", format, output)
		}
	}

	if createUserCalls != 3 {
		t.Fatalf("expected create user call per format, got %d", createUserCalls)
	}
	if createMembershipCalls != 3 {
		t.Fatalf("expected group membership call per format, got %d", createMembershipCalls)
	}
}

func TestIAMCreateSSOUsersDryRunNoMutations(t *testing.T) {
	createUserCalls := 0
	createMembershipCalls := 0

	identityClient := &mockIdentityStoreClient{
		listGroupsFn: func(_ context.Context, _ *identitystore.ListGroupsInput, _ ...func(*identitystore.Options)) (*identitystore.ListGroupsOutput, error) {
			return &identitystore.ListGroupsOutput{
				Groups: []identitystoretypes.Group{{DisplayName: ptr("engineering"), GroupId: ptr("grp-1")}},
			}, nil
		},
		createUserFn: func(_ context.Context, _ *identitystore.CreateUserInput, _ ...func(*identitystore.Options)) (*identitystore.CreateUserOutput, error) {
			createUserCalls++
			return &identitystore.CreateUserOutput{}, nil
		},
		createGroupMembershipFn: func(_ context.Context, _ *identitystore.CreateGroupMembershipInput, _ ...func(*identitystore.Options)) (*identitystore.CreateGroupMembershipOutput, error) {
			createMembershipCalls++
			return &identitystore.CreateGroupMembershipOutput{}, nil
		},
	}

	ssoClient := &mockSSOAdminClient{
		listInstancesFn: func(_ context.Context, _ *ssoadmin.ListInstancesInput, _ ...func(*ssoadmin.Options)) (*ssoadmin.ListInstancesOutput, error) {
			return &ssoadmin.ListInstancesOutput{
				Instances: []ssoadmintypes.InstanceMetadata{{IdentityStoreId: ptr("d-1234567890")}},
			}, nil
		},
	}

	withMockIAMDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) iamAPI { return &mockIAMClient{} },
		func(awssdk.Config) iamIdentityStoreAPI { return identityClient },
		func(awssdk.Config) iamSSOAdminAPI { return ssoClient },
	)

	output, err := executeCommand(t, "--output", "json", "--dry-run", "iam", "create-sso-users", "--emails", "john.doe@example.com", "--group", "engineering")
	if err != nil {
		t.Fatalf("execute create-sso-users --dry-run: %v", err)
	}
	if createUserCalls != 0 || createMembershipCalls != 0 {
		t.Fatalf("expected no create calls in dry-run, got user=%d membership=%d", createUserCalls, createMembershipCalls)
	}
	if !strings.Contains(output, "would-create") {
		t.Fatalf("unexpected output: %s", output)
	}
}

func TestIAMRotateKeysCreateWhenNoConfirm(t *testing.T) {
	client := &mockIAMClient{
		listAccessKeysFn: func(_ context.Context, _ *iam.ListAccessKeysInput, _ ...func(*iam.Options)) (*iam.ListAccessKeysOutput, error) {
			return &iam.ListAccessKeysOutput{
				AccessKeyMetadata: []iamtypes.AccessKeyMetadata{{AccessKeyId: ptr("AKIAOLD"), Status: iamtypes.StatusTypeActive}},
			}, nil
		},
		createAccessKeyFn: func(_ context.Context, _ *iam.CreateAccessKeyInput, _ ...func(*iam.Options)) (*iam.CreateAccessKeyOutput, error) {
			return &iam.CreateAccessKeyOutput{
				AccessKey: &iamtypes.AccessKey{
					AccessKeyId:     ptr("AKIANEW"),
					SecretAccessKey: ptr("secret-value"),
				},
			}, nil
		},
	}

	withMockIAMDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) iamAPI { return client },
		func(awssdk.Config) iamIdentityStoreAPI { return &mockIdentityStoreClient{} },
		func(awssdk.Config) iamSSOAdminAPI { return &mockSSOAdminClient{} },
	)

	output, err := executeCommand(t, "--output", "json", "--no-confirm", "iam", "rotate-keys", "--username", "alice")
	if err != nil {
		t.Fatalf("execute rotate-keys: %v", err)
	}

	if !strings.Contains(output, "AKIANEW") || !strings.Contains(output, "secret-value") || !strings.Contains(output, "created") {
		t.Fatalf("unexpected output: %s", output)
	}
}

func TestIAMRotateKeysDisableWhenNoConfirm(t *testing.T) {
	updated := 0
	client := &mockIAMClient{
		listAccessKeysFn: func(_ context.Context, _ *iam.ListAccessKeysInput, _ ...func(*iam.Options)) (*iam.ListAccessKeysOutput, error) {
			return &iam.ListAccessKeysOutput{
				AccessKeyMetadata: []iamtypes.AccessKeyMetadata{{AccessKeyId: ptr("AKIAOLD"), Status: iamtypes.StatusTypeActive}},
			}, nil
		},
		updateAccessKeyFn: func(_ context.Context, in *iam.UpdateAccessKeyInput, _ ...func(*iam.Options)) (*iam.UpdateAccessKeyOutput, error) {
			updated++
			if pointerToString(in.AccessKeyId) != "AKIAOLD" || in.Status != iamtypes.StatusTypeInactive {
				t.Fatalf("unexpected update input: key=%s status=%s", pointerToString(in.AccessKeyId), in.Status)
			}
			return &iam.UpdateAccessKeyOutput{}, nil
		},
	}

	withMockIAMDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) iamAPI { return client },
		func(awssdk.Config) iamIdentityStoreAPI { return &mockIdentityStoreClient{} },
		func(awssdk.Config) iamSSOAdminAPI { return &mockSSOAdminClient{} },
	)

	output, err := executeCommand(
		t,
		"--output", "json",
		"--no-confirm",
		"iam", "rotate-keys",
		"--username", "alice",
		"--disable",
		"--key", "AKIAOLD",
	)
	if err != nil {
		t.Fatalf("execute rotate-keys --disable: %v", err)
	}

	if updated != 1 || !strings.Contains(output, "disabled") {
		t.Fatalf("unexpected output: %s", output)
	}
}

func TestIAMRotateKeysDeleteWhenNoConfirm(t *testing.T) {
	deleted := 0
	client := &mockIAMClient{
		listAccessKeysFn: func(_ context.Context, _ *iam.ListAccessKeysInput, _ ...func(*iam.Options)) (*iam.ListAccessKeysOutput, error) {
			return &iam.ListAccessKeysOutput{
				AccessKeyMetadata: []iamtypes.AccessKeyMetadata{{AccessKeyId: ptr("AKIAOLD"), Status: iamtypes.StatusTypeInactive}},
			}, nil
		},
		deleteAccessKeyFn: func(_ context.Context, in *iam.DeleteAccessKeyInput, _ ...func(*iam.Options)) (*iam.DeleteAccessKeyOutput, error) {
			deleted++
			if pointerToString(in.AccessKeyId) != "AKIAOLD" {
				t.Fatalf("unexpected delete input: key=%s", pointerToString(in.AccessKeyId))
			}
			return &iam.DeleteAccessKeyOutput{}, nil
		},
	}

	withMockIAMDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) iamAPI { return client },
		func(awssdk.Config) iamIdentityStoreAPI { return &mockIdentityStoreClient{} },
		func(awssdk.Config) iamSSOAdminAPI { return &mockSSOAdminClient{} },
	)

	output, err := executeCommand(
		t,
		"--output", "json",
		"--no-confirm",
		"iam", "rotate-keys",
		"--username", "alice",
		"--delete",
		"--key", "AKIAOLD",
	)
	if err != nil {
		t.Fatalf("execute rotate-keys --delete: %v", err)
	}

	if deleted != 1 || !strings.Contains(output, "deleted") {
		t.Fatalf("unexpected output: %s", output)
	}
}

func TestIAMDeleteUserDryRunDoesNotMutate(t *testing.T) {
	client := &mockIAMClient{
		listAccessKeysFn: func(_ context.Context, _ *iam.ListAccessKeysInput, _ ...func(*iam.Options)) (*iam.ListAccessKeysOutput, error) {
			return &iam.ListAccessKeysOutput{AccessKeyMetadata: []iamtypes.AccessKeyMetadata{{AccessKeyId: ptr("AKIA123")}}}, nil
		},
		listMFADevicesFn: func(_ context.Context, _ *iam.ListMFADevicesInput, _ ...func(*iam.Options)) (*iam.ListMFADevicesOutput, error) {
			return &iam.ListMFADevicesOutput{MFADevices: []iamtypes.MFADevice{{SerialNumber: ptr("mfa-1")}}}, nil
		},
		listAttachedUserPoliciesFn: func(_ context.Context, _ *iam.ListAttachedUserPoliciesInput, _ ...func(*iam.Options)) (*iam.ListAttachedUserPoliciesOutput, error) {
			return &iam.ListAttachedUserPoliciesOutput{AttachedPolicies: []iamtypes.AttachedPolicy{{PolicyArn: ptr("arn:aws:iam::aws:policy/ReadOnlyAccess")}}}, nil
		},
		listUserPoliciesFn: func(_ context.Context, _ *iam.ListUserPoliciesInput, _ ...func(*iam.Options)) (*iam.ListUserPoliciesOutput, error) {
			return &iam.ListUserPoliciesOutput{PolicyNames: []string{"inline-pol"}}, nil
		},
		listGroupsForUserFn: func(_ context.Context, _ *iam.ListGroupsForUserInput, _ ...func(*iam.Options)) (*iam.ListGroupsForUserOutput, error) {
			return &iam.ListGroupsForUserOutput{Groups: []iamtypes.Group{{GroupName: ptr("admins")}}}, nil
		},
		listSigningCertificatesFn: func(_ context.Context, _ *iam.ListSigningCertificatesInput, _ ...func(*iam.Options)) (*iam.ListSigningCertificatesOutput, error) {
			return &iam.ListSigningCertificatesOutput{Certificates: []iamtypes.SigningCertificate{{CertificateId: ptr("cert-1")}}}, nil
		},
		listSSHPublicKeysFn: func(_ context.Context, _ *iam.ListSSHPublicKeysInput, _ ...func(*iam.Options)) (*iam.ListSSHPublicKeysOutput, error) {
			return &iam.ListSSHPublicKeysOutput{SSHPublicKeys: []iamtypes.SSHPublicKeyMetadata{{SSHPublicKeyId: ptr("ssh-1")}}}, nil
		},
		deleteAccessKeyFn: func(_ context.Context, _ *iam.DeleteAccessKeyInput, _ ...func(*iam.Options)) (*iam.DeleteAccessKeyOutput, error) {
			t.Fatal("delete access key should not be called in dry-run")
			return nil, nil
		},
		deleteUserFn: func(_ context.Context, _ *iam.DeleteUserInput, _ ...func(*iam.Options)) (*iam.DeleteUserOutput, error) {
			t.Fatal("delete user should not be called in dry-run")
			return nil, nil
		},
	}

	withMockIAMDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) iamAPI { return client },
		func(awssdk.Config) iamIdentityStoreAPI { return &mockIdentityStoreClient{} },
		func(awssdk.Config) iamSSOAdminAPI { return &mockSSOAdminClient{} },
	)

	output, err := executeCommand(t, "--output", "json", "--dry-run", "iam", "delete-user", "--username", "alice")
	if err != nil {
		t.Fatalf("execute delete-user --dry-run: %v", err)
	}

	if !strings.Contains(output, "would-delete") || !strings.Contains(output, "AKIA123") {
		t.Fatalf("unexpected output: %s", output)
	}
}

func TestIAMRotateKeysDryRunDoesNotMutate(t *testing.T) {
	client := &mockIAMClient{
		listAccessKeysFn: func(_ context.Context, _ *iam.ListAccessKeysInput, _ ...func(*iam.Options)) (*iam.ListAccessKeysOutput, error) {
			return &iam.ListAccessKeysOutput{
				AccessKeyMetadata: []iamtypes.AccessKeyMetadata{{AccessKeyId: ptr("AKIAOLD"), Status: iamtypes.StatusTypeActive}},
			}, nil
		},
		createAccessKeyFn: func(_ context.Context, _ *iam.CreateAccessKeyInput, _ ...func(*iam.Options)) (*iam.CreateAccessKeyOutput, error) {
			t.Fatal("create access key should not be called in dry-run")
			return nil, nil
		},
	}

	withMockIAMDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) iamAPI { return client },
		func(awssdk.Config) iamIdentityStoreAPI { return &mockIdentityStoreClient{} },
		func(awssdk.Config) iamSSOAdminAPI { return &mockSSOAdminClient{} },
	)

	output, err := executeCommand(t, "--output", "json", "--dry-run", "iam", "rotate-keys", "--username", "alice")
	if err != nil {
		t.Fatalf("execute rotate-keys --dry-run: %v", err)
	}
	if !strings.Contains(output, "\"action\": \"would-create\"") {
		t.Fatalf("unexpected output: %s", output)
	}
}

func TestIAMDeleteUserNoSuchEntityMarkedNotFound(t *testing.T) {
	client := &mockIAMClient{
		listAccessKeysFn: func(_ context.Context, _ *iam.ListAccessKeysInput, _ ...func(*iam.Options)) (*iam.ListAccessKeysOutput, error) {
			return &iam.ListAccessKeysOutput{}, nil
		},
		listMFADevicesFn: func(_ context.Context, _ *iam.ListMFADevicesInput, _ ...func(*iam.Options)) (*iam.ListMFADevicesOutput, error) {
			return &iam.ListMFADevicesOutput{}, nil
		},
		listAttachedUserPoliciesFn: func(_ context.Context, _ *iam.ListAttachedUserPoliciesInput, _ ...func(*iam.Options)) (*iam.ListAttachedUserPoliciesOutput, error) {
			return &iam.ListAttachedUserPoliciesOutput{}, nil
		},
		listUserPoliciesFn: func(_ context.Context, _ *iam.ListUserPoliciesInput, _ ...func(*iam.Options)) (*iam.ListUserPoliciesOutput, error) {
			return &iam.ListUserPoliciesOutput{}, nil
		},
		listGroupsForUserFn: func(_ context.Context, _ *iam.ListGroupsForUserInput, _ ...func(*iam.Options)) (*iam.ListGroupsForUserOutput, error) {
			return &iam.ListGroupsForUserOutput{}, nil
		},
		listSigningCertificatesFn: func(_ context.Context, _ *iam.ListSigningCertificatesInput, _ ...func(*iam.Options)) (*iam.ListSigningCertificatesOutput, error) {
			return &iam.ListSigningCertificatesOutput{}, nil
		},
		listSSHPublicKeysFn: func(_ context.Context, _ *iam.ListSSHPublicKeysInput, _ ...func(*iam.Options)) (*iam.ListSSHPublicKeysOutput, error) {
			return &iam.ListSSHPublicKeysOutput{}, nil
		},
		deleteUserPermissionsBoundaryFn: func(_ context.Context, _ *iam.DeleteUserPermissionsBoundaryInput, _ ...func(*iam.Options)) (*iam.DeleteUserPermissionsBoundaryOutput, error) {
			return nil, noSuchEntityAPIError{}
		},
		deleteLoginProfileFn: func(_ context.Context, _ *iam.DeleteLoginProfileInput, _ ...func(*iam.Options)) (*iam.DeleteLoginProfileOutput, error) {
			return nil, noSuchEntityAPIError{}
		},
		deleteUserFn: func(_ context.Context, _ *iam.DeleteUserInput, _ ...func(*iam.Options)) (*iam.DeleteUserOutput, error) {
			return nil, noSuchEntityAPIError{}
		},
	}

	withMockIAMDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) iamAPI { return client },
		func(awssdk.Config) iamIdentityStoreAPI { return &mockIdentityStoreClient{} },
		func(awssdk.Config) iamSSOAdminAPI { return &mockSSOAdminClient{} },
	)

	output, err := executeCommand(t, "--output", "json", "--no-confirm", "iam", "delete-user", "--username", "alice")
	if err != nil {
		t.Fatalf("execute delete-user --no-confirm: %v", err)
	}
	if !strings.Contains(output, "\"action\": \"not-found\"") {
		t.Fatalf("expected not-found action in output: %s", output)
	}
}
