package org

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/account"
	accounttypes "github.com/aws/aws-sdk-go-v2/service/account/types"
	"github.com/aws/aws-sdk-go-v2/service/identitystore"
	identitystoretypes "github.com/aws/aws-sdk-go-v2/service/identitystore/types"
	"github.com/aws/aws-sdk-go-v2/service/organizations"
	organizationtypes "github.com/aws/aws-sdk-go-v2/service/organizations/types"
	"github.com/aws/aws-sdk-go-v2/service/ssoadmin"
	ssoadmintypes "github.com/aws/aws-sdk-go-v2/service/ssoadmin/types"
	"github.com/towardsthecloud/aws-toolbox/internal/cliutil"
)

func TestOrgHelpListsMilestone4Subcommands(t *testing.T) {
	output, err := executeCommand(t, "org", "--help")
	if err != nil {
		t.Fatalf("execute org --help: %v", err)
	}

	for _, subcommand := range []string{
		"assign-sso-access",
		"generate-diagram",
		"get-account",
		"import-sso-users",
		"list-accounts",
		"list-sso-assignments",
		"remove-sso-access",
		"set-alternate-contact",
	} {
		if !strings.Contains(output, subcommand) {
			t.Fatalf("missing subcommand %q in help output\n%s", subcommand, output)
		}
	}
}

func TestOrgImportSSOUsersDryRun(t *testing.T) {
	csvPath := filepath.Join(t.TempDir(), "users.csv")
	csvContent := "first_name,last_name,email,group_name\nJohn,Doe,john.doe@example.com,engineering\n"
	if err := os.WriteFile(csvPath, []byte(csvContent), 0o600); err != nil {
		t.Fatalf("write csv file: %v", err)
	}

	identityClient := &mockIdentityStoreClient{
		listGroupsFn: func(_ context.Context, _ *identitystore.ListGroupsInput, _ ...func(*identitystore.Options)) (*identitystore.ListGroupsOutput, error) {
			return &identitystore.ListGroupsOutput{}, nil
		},
		listUsersFn: func(_ context.Context, _ *identitystore.ListUsersInput, _ ...func(*identitystore.Options)) (*identitystore.ListUsersOutput, error) {
			return &identitystore.ListUsersOutput{}, nil
		},
		createGroupFn: func(_ context.Context, _ *identitystore.CreateGroupInput, _ ...func(*identitystore.Options)) (*identitystore.CreateGroupOutput, error) {
			t.Fatal("create group should not be called in dry-run")
			return nil, nil
		},
		createUserFn: func(_ context.Context, _ *identitystore.CreateUserInput, _ ...func(*identitystore.Options)) (*identitystore.CreateUserOutput, error) {
			t.Fatal("create user should not be called in dry-run")
			return nil, nil
		},
		createMemberFn: func(_ context.Context, _ *identitystore.CreateGroupMembershipInput, _ ...func(*identitystore.Options)) (*identitystore.CreateGroupMembershipOutput, error) {
			t.Fatal("create membership should not be called in dry-run")
			return nil, nil
		},
	}
	ssoClient := &mockSSOAdminClient{
		listInstancesFn: func(_ context.Context, _ *ssoadmin.ListInstancesInput, _ ...func(*ssoadmin.Options)) (*ssoadmin.ListInstancesOutput, error) {
			return &ssoadmin.ListInstancesOutput{
				Instances: []ssoadmintypes.InstanceMetadata{{InstanceArn: cliutil.Ptr("arn:aws:sso:::instance/ssoins-123"), IdentityStoreId: cliutil.Ptr("d-123")}},
			}, nil
		},
	}

	withMockDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) OrganizationsAPI { return &mockOrganizationsClient{} },
		func(awssdk.Config) SSOAdminAPI { return ssoClient },
		func(awssdk.Config) IdentityStoreAPI { return identityClient },
		func(awssdk.Config) AccountAPI { return &mockAccountClient{} },
	)

	output, err := executeCommand(t, "--output", "json", "--dry-run", "org", "import-sso-users", "--input-file", csvPath)
	if err != nil {
		t.Fatalf("execute import-sso-users --dry-run: %v", err)
	}
	if !strings.Contains(output, "would-create-user") || !strings.Contains(output, "would-create-group") || !strings.Contains(output, "would-add-to-group") {
		t.Fatalf("unexpected output: %s", output)
	}
}

func TestOrgImportSSOUsersNoDryRunCreatesResources(t *testing.T) {
	csvPath := filepath.Join(t.TempDir(), "users.csv")
	csvContent := "first_name,last_name,email,group_name\nJohn,Doe,john.doe@example.com,engineering\n"
	if err := os.WriteFile(csvPath, []byte(csvContent), 0o600); err != nil {
		t.Fatalf("write csv file: %v", err)
	}

	createGroupCalls := 0
	createUserCalls := 0
	createMembershipCalls := 0

	identityClient := &mockIdentityStoreClient{
		listGroupsFn: func(_ context.Context, _ *identitystore.ListGroupsInput, _ ...func(*identitystore.Options)) (*identitystore.ListGroupsOutput, error) {
			return &identitystore.ListGroupsOutput{}, nil
		},
		createGroupFn: func(_ context.Context, in *identitystore.CreateGroupInput, _ ...func(*identitystore.Options)) (*identitystore.CreateGroupOutput, error) {
			createGroupCalls++
			if cliutil.PointerToString(in.DisplayName) != "engineering" {
				t.Fatalf("unexpected group name: %s", cliutil.PointerToString(in.DisplayName))
			}
			return &identitystore.CreateGroupOutput{GroupId: cliutil.Ptr("group-1"), IdentityStoreId: in.IdentityStoreId}, nil
		},
		listUsersFn: func(_ context.Context, _ *identitystore.ListUsersInput, _ ...func(*identitystore.Options)) (*identitystore.ListUsersOutput, error) {
			return &identitystore.ListUsersOutput{}, nil
		},
		createUserFn: func(_ context.Context, in *identitystore.CreateUserInput, _ ...func(*identitystore.Options)) (*identitystore.CreateUserOutput, error) {
			createUserCalls++
			if cliutil.PointerToString(in.UserName) != "john.doe@example.com" {
				t.Fatalf("unexpected user name: %s", cliutil.PointerToString(in.UserName))
			}
			return &identitystore.CreateUserOutput{UserId: cliutil.Ptr("user-1"), IdentityStoreId: in.IdentityStoreId}, nil
		},
		createMemberFn: func(_ context.Context, in *identitystore.CreateGroupMembershipInput, _ ...func(*identitystore.Options)) (*identitystore.CreateGroupMembershipOutput, error) {
			createMembershipCalls++
			if cliutil.PointerToString(in.GroupId) != "group-1" {
				t.Fatalf("unexpected group id: %s", cliutil.PointerToString(in.GroupId))
			}
			return &identitystore.CreateGroupMembershipOutput{}, nil
		},
	}
	ssoClient := &mockSSOAdminClient{
		listInstancesFn: func(_ context.Context, _ *ssoadmin.ListInstancesInput, _ ...func(*ssoadmin.Options)) (*ssoadmin.ListInstancesOutput, error) {
			return &ssoadmin.ListInstancesOutput{
				Instances: []ssoadmintypes.InstanceMetadata{{InstanceArn: cliutil.Ptr("arn:aws:sso:::instance/ssoins-123"), IdentityStoreId: cliutil.Ptr("d-123")}},
			}, nil
		},
	}

	withMockDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) OrganizationsAPI { return &mockOrganizationsClient{} },
		func(awssdk.Config) SSOAdminAPI { return ssoClient },
		func(awssdk.Config) IdentityStoreAPI { return identityClient },
		func(awssdk.Config) AccountAPI { return &mockAccountClient{} },
	)

	output, err := executeCommand(t, "--output", "json", "org", "import-sso-users", "--input-file", csvPath)
	if err != nil {
		t.Fatalf("execute import-sso-users: %v", err)
	}
	if createGroupCalls != 1 || createUserCalls != 1 || createMembershipCalls != 1 {
		t.Fatalf("unexpected create call counts group=%d user=%d membership=%d", createGroupCalls, createUserCalls, createMembershipCalls)
	}
	if !strings.Contains(output, "created-group") || !strings.Contains(output, "created-user") || !strings.Contains(output, "added-to-group") {
		t.Fatalf("unexpected output: %s", output)
	}
}

func TestOrgSetAlternateContactNoConfirmExecutes(t *testing.T) {
	putCalls := 0
	orgClient := &mockOrganizationsClient{
		listAccountsFn: func(_ context.Context, _ *organizations.ListAccountsInput, _ ...func(*organizations.Options)) (*organizations.ListAccountsOutput, error) {
			return &organizations.ListAccountsOutput{
				Accounts: []organizationtypes.Account{{Id: cliutil.Ptr("123456789012"), Name: cliutil.Ptr("sandbox")}},
			}, nil
		},
	}
	accountClient := &mockAccountClient{
		putAlternateContactFn: func(_ context.Context, _ *account.PutAlternateContactInput, _ ...func(*account.Options)) (*account.PutAlternateContactOutput, error) {
			putCalls++
			return &account.PutAlternateContactOutput{}, nil
		},
	}
	contactsFile := filepath.Join(t.TempDir(), "contacts.json")
	content := `{
	  "security": {"name":"Sec","title":"Security Lead","emailAddress":"sec@example.com","phoneNumber":"+10000000000"},
	  "billing": {"name":"Bill","title":"Finance Lead","emailAddress":"bill@example.com","phoneNumber":"+10000000001"},
	  "operations": {"name":"Ops","title":"Ops Lead","emailAddress":"ops@example.com","phoneNumber":"+10000000002"}
	}`
	if err := os.WriteFile(contactsFile, []byte(content), 0o600); err != nil {
		t.Fatalf("write contacts file: %v", err)
	}

	withMockDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) OrganizationsAPI { return orgClient },
		func(awssdk.Config) SSOAdminAPI { return &mockSSOAdminClient{} },
		func(awssdk.Config) IdentityStoreAPI { return &mockIdentityStoreClient{} },
		func(awssdk.Config) AccountAPI { return accountClient },
	)

	output, err := executeCommand(t, "--output", "json", "--no-confirm", "org", "set-alternate-contact", "--input-file", contactsFile)
	if err != nil {
		t.Fatalf("execute set-alternate-contact --no-confirm: %v", err)
	}
	if putCalls != 3 {
		t.Fatalf("expected 3 alternate contact updates, got %d", putCalls)
	}
	if !strings.Contains(output, "\"action\": \"updated\"") {
		t.Fatalf("unexpected output: %s", output)
	}
}

func TestResolvePrincipalIDUserAndGroup(t *testing.T) {
	identityClient := &mockIdentityStoreClient{
		listUsersFn: func(_ context.Context, in *identitystore.ListUsersInput, _ ...func(*identitystore.Options)) (*identitystore.ListUsersOutput, error) {
			if cliutil.PointerToString(in.IdentityStoreId) != "d-123" {
				t.Fatalf("unexpected identity store id: %s", cliutil.PointerToString(in.IdentityStoreId))
			}
			return &identitystore.ListUsersOutput{Users: []identitystoretypes.User{{UserId: cliutil.Ptr("user-1"), UserName: cliutil.Ptr("alice@example.com")}}}, nil
		},
		listGroupsFn: func(_ context.Context, _ *identitystore.ListGroupsInput, _ ...func(*identitystore.Options)) (*identitystore.ListGroupsOutput, error) {
			return &identitystore.ListGroupsOutput{Groups: []identitystoretypes.Group{{GroupId: cliutil.Ptr("group-1"), DisplayName: cliutil.Ptr("Admins")}}}, nil
		},
	}

	userID, err := resolvePrincipalID(context.Background(), identityClient, "d-123", "alice@example.com", ssoadmintypes.PrincipalTypeUser)
	if err != nil || userID != "user-1" {
		t.Fatalf("unexpected user resolution: id=%s err=%v", userID, err)
	}

	groupID, err := resolvePrincipalID(context.Background(), identityClient, "d-123", "Admins", ssoadmintypes.PrincipalTypeGroup)
	if err != nil || groupID != "group-1" {
		t.Fatalf("unexpected group resolution: id=%s err=%v", groupID, err)
	}
}

func TestEnsureGroupAndUserExistingPaths(t *testing.T) {
	identityClient := &mockIdentityStoreClient{
		listGroupsFn: func(_ context.Context, _ *identitystore.ListGroupsInput, _ ...func(*identitystore.Options)) (*identitystore.ListGroupsOutput, error) {
			return &identitystore.ListGroupsOutput{Groups: []identitystoretypes.Group{{GroupId: cliutil.Ptr("group-existing"), DisplayName: cliutil.Ptr("engineering")}}}, nil
		},
		listUsersFn: func(_ context.Context, _ *identitystore.ListUsersInput, _ ...func(*identitystore.Options)) (*identitystore.ListUsersOutput, error) {
			return &identitystore.ListUsersOutput{Users: []identitystoretypes.User{{UserId: cliutil.Ptr("user-existing"), UserName: cliutil.Ptr("john.doe@example.com")}}}, nil
		},
	}

	groupID, groupAction, err := ensureGroup(context.Background(), identityClient, "d-123", "engineering", false)
	if err != nil || groupID != "group-existing" || groupAction != "existing-group" {
		t.Fatalf("unexpected group ensure result: id=%s action=%s err=%v", groupID, groupAction, err)
	}

	userID, userAction, err := ensureUser(context.Background(), identityClient, "d-123", importRow{
		FirstName: "John",
		LastName:  "Doe",
		Email:     "john.doe@example.com",
		GroupName: "engineering",
	}, false)
	if err != nil || userID != "user-existing" || userAction != "existing-user" {
		t.Fatalf("unexpected user ensure result: id=%s action=%s err=%v", userID, userAction, err)
	}
}

func TestLoadContactsLegacyKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), "contacts.json")
	content := `{
	  "securityContact": {"name":"Sec","title":"Security Lead","emailAddress":"sec@example.com","phoneNumber":"+10000000000"},
	  "billingContact": {"name":"Bill","title":"Finance Lead","emailAddress":"bill@example.com","phoneNumber":"+10000000001"},
	  "operationsContact": {"name":"Ops","title":"Ops Lead","emailAddress":"ops@example.com","phoneNumber":"+10000000002"}
	}`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write contacts file: %v", err)
	}

	contacts, err := loadContacts(path)
	if err != nil {
		t.Fatalf("load contacts: %v", err)
	}
	if contacts[accounttypes.AlternateContactTypeSecurity].EmailAddress != "sec@example.com" {
		t.Fatalf("unexpected security contact: %+v", contacts[accounttypes.AlternateContactTypeSecurity])
	}
}

func TestSortAccountsByID(t *testing.T) {
	accounts := []organizationtypes.Account{
		{Id: cliutil.Ptr("333333333333")},
		{Id: cliutil.Ptr("111111111111")},
		{Id: cliutil.Ptr("222222222222")},
	}
	sortAccountsByID(accounts)
	if *accounts[0].Id != "111111111111" || *accounts[1].Id != "222222222222" || *accounts[2].Id != "333333333333" {
		t.Fatalf("unexpected sort order: %v, %v, %v", *accounts[0].Id, *accounts[1].Id, *accounts[2].Id)
	}
}

func TestSSoPrincipalTypeFromString(t *testing.T) {
	typ, err := ssoPrincipalTypeFromString("USER")
	if err != nil || typ != ssoadmintypes.PrincipalTypeUser {
		t.Fatalf("USER: type=%v err=%v", typ, err)
	}

	typ, err = ssoPrincipalTypeFromString("group")
	if err != nil || typ != ssoadmintypes.PrincipalTypeGroup {
		t.Fatalf("group: type=%v err=%v", typ, err)
	}

	_, err = ssoPrincipalTypeFromString("invalid")
	if err == nil {
		t.Fatal("expected error for invalid principal type")
	}
}

func TestFormatTime(t *testing.T) {
	if got := formatTime(nil); got != "" {
		t.Fatalf("expected empty for nil, got %q", got)
	}
	ts := time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC)
	if got := formatTime(&ts); got != "2025-01-15T12:00:00Z" {
		t.Fatalf("unexpected format: %q", got)
	}
}
