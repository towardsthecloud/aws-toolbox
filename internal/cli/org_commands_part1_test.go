package cli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/account"
	"github.com/aws/aws-sdk-go-v2/service/identitystore"
	identitystoretypes "github.com/aws/aws-sdk-go-v2/service/identitystore/types"
	"github.com/aws/aws-sdk-go-v2/service/organizations"
	organizationtypes "github.com/aws/aws-sdk-go-v2/service/organizations/types"
	"github.com/aws/aws-sdk-go-v2/service/ssoadmin"
	ssoadmintypes "github.com/aws/aws-sdk-go-v2/service/ssoadmin/types"
)

func TestOrgListAccountsAllOutputFormats(t *testing.T) {
	orgClient := &mockOrgOrganizationsClient{
		listAccountsFn: func(_ context.Context, _ *organizations.ListAccountsInput, _ ...func(*organizations.Options)) (*organizations.ListAccountsOutput, error) {
			return &organizations.ListAccountsOutput{Accounts: []organizationtypes.Account{{Id: ptr("123456789012"), Name: ptr("sandbox"), Email: ptr("sandbox@example.com"), Status: organizationtypes.AccountStatusActive}}}, nil
		},
	}

	withMockOrgDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) organizationsAPI { return orgClient },
		func(awssdk.Config) ssoAdminAPI { return &mockOrgSSOAdminClient{} },
		func(awssdk.Config) identityStoreAPI { return &mockOrgIdentityStoreClient{} },
		func(awssdk.Config) accountAPI { return &mockOrgAccountClient{} },
	)

	for _, format := range []string{"table", "json", "text"} {
		output, err := executeCommand(t, "--output", format, "org", "list-accounts")
		if err != nil {
			t.Fatalf("execute list-accounts (%s): %v", format, err)
		}
		if !strings.Contains(output, "123456789012") {
			t.Fatalf("expected account id in output for format=%s: %s", format, output)
		}
	}
}

func TestOrgListAccountsByOUFilter(t *testing.T) {
	orgClient := &mockOrgOrganizationsClient{
		listRootsFn: func(_ context.Context, _ *organizations.ListRootsInput, _ ...func(*organizations.Options)) (*organizations.ListRootsOutput, error) {
			return &organizations.ListRootsOutput{Roots: []organizationtypes.Root{{Id: ptr("r-root"), Name: ptr("Main")}}}, nil
		},
		listOUsFn: func(_ context.Context, in *organizations.ListOrganizationalUnitsForParentInput, _ ...func(*organizations.Options)) (*organizations.ListOrganizationalUnitsForParentOutput, error) {
			if pointerToString(in.ParentId) != "r-root" {
				return &organizations.ListOrganizationalUnitsForParentOutput{}, nil
			}
			return &organizations.ListOrganizationalUnitsForParentOutput{
				OrganizationalUnits: []organizationtypes.OrganizationalUnit{{Id: ptr("ou-1"), Name: ptr("Sandbox")}},
			}, nil
		},
		listForParentFn: func(_ context.Context, in *organizations.ListAccountsForParentInput, _ ...func(*organizations.Options)) (*organizations.ListAccountsForParentOutput, error) {
			if pointerToString(in.ParentId) != "ou-1" {
				return &organizations.ListAccountsForParentOutput{}, nil
			}
			return &organizations.ListAccountsForParentOutput{
				Accounts: []organizationtypes.Account{{Id: ptr("123456789012"), Name: ptr("sandbox"), Email: ptr("sandbox@example.com"), Status: organizationtypes.AccountStatusActive}},
			}, nil
		},
	}

	withMockOrgDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) organizationsAPI { return orgClient },
		func(awssdk.Config) ssoAdminAPI { return &mockOrgSSOAdminClient{} },
		func(awssdk.Config) identityStoreAPI { return &mockOrgIdentityStoreClient{} },
		func(awssdk.Config) accountAPI { return &mockOrgAccountClient{} },
	)

	output, err := executeCommand(t, "--output", "json", "org", "list-accounts", "--ou-name", "Sandbox")
	if err != nil {
		t.Fatalf("execute list-accounts --ou-name: %v", err)
	}
	if !strings.Contains(output, "\"parent\": \"/Sandbox\"") {
		t.Fatalf("expected OU path in output: %s", output)
	}
}

func TestOrgListAccountsByNestedOUFilter(t *testing.T) {
	orgClient := &mockOrgOrganizationsClient{
		listRootsFn: func(_ context.Context, _ *organizations.ListRootsInput, _ ...func(*organizations.Options)) (*organizations.ListRootsOutput, error) {
			return &organizations.ListRootsOutput{Roots: []organizationtypes.Root{{Id: ptr("r-root"), Name: ptr("Main")}}}, nil
		},
		listOUsFn: func(_ context.Context, in *organizations.ListOrganizationalUnitsForParentInput, _ ...func(*organizations.Options)) (*organizations.ListOrganizationalUnitsForParentOutput, error) {
			switch pointerToString(in.ParentId) {
			case "r-root":
				return &organizations.ListOrganizationalUnitsForParentOutput{
					OrganizationalUnits: []organizationtypes.OrganizationalUnit{{Id: ptr("ou-parent"), Name: ptr("Engineering")}},
				}, nil
			case "ou-parent":
				return &organizations.ListOrganizationalUnitsForParentOutput{
					OrganizationalUnits: []organizationtypes.OrganizationalUnit{{Id: ptr("ou-child"), Name: ptr("Sandbox")}},
				}, nil
			default:
				return &organizations.ListOrganizationalUnitsForParentOutput{}, nil
			}
		},
		listForParentFn: func(_ context.Context, in *organizations.ListAccountsForParentInput, _ ...func(*organizations.Options)) (*organizations.ListAccountsForParentOutput, error) {
			if pointerToString(in.ParentId) != "ou-child" {
				return &organizations.ListAccountsForParentOutput{}, nil
			}
			return &organizations.ListAccountsForParentOutput{
				Accounts: []organizationtypes.Account{{Id: ptr("123456789012"), Name: ptr("sandbox"), Email: ptr("sandbox@example.com"), Status: organizationtypes.AccountStatusActive}},
			}, nil
		},
	}

	withMockOrgDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) organizationsAPI { return orgClient },
		func(awssdk.Config) ssoAdminAPI { return &mockOrgSSOAdminClient{} },
		func(awssdk.Config) identityStoreAPI { return &mockOrgIdentityStoreClient{} },
		func(awssdk.Config) accountAPI { return &mockOrgAccountClient{} },
	)

	output, err := executeCommand(t, "--output", "json", "org", "list-accounts", "--ou-name", "Sandbox")
	if err != nil {
		t.Fatalf("execute list-accounts --ou-name on nested OU: %v", err)
	}
	if !strings.Contains(output, "\"account_id\": \"123456789012\"") || !strings.Contains(output, "\"parent\": \"/Sandbox\"") {
		t.Fatalf("expected nested OU account in output: %s", output)
	}
}

func TestOrgGenerateDiagramEmitsMermaid(t *testing.T) {
	orgClient := &mockOrgOrganizationsClient{
		listRootsFn: func(_ context.Context, _ *organizations.ListRootsInput, _ ...func(*organizations.Options)) (*organizations.ListRootsOutput, error) {
			return &organizations.ListRootsOutput{Roots: []organizationtypes.Root{{Id: ptr("r-root"), Name: ptr("Main")}}}, nil
		},
		listForParentFn: func(_ context.Context, in *organizations.ListAccountsForParentInput, _ ...func(*organizations.Options)) (*organizations.ListAccountsForParentOutput, error) {
			parentID := pointerToString(in.ParentId)
			if parentID == "ou-sandbox" {
				return &organizations.ListAccountsForParentOutput{Accounts: []organizationtypes.Account{{Id: ptr("111111111111"), Name: ptr("dev"), Status: organizationtypes.AccountStatusActive}}}, nil
			}
			return &organizations.ListAccountsForParentOutput{}, nil
		},
		listOUsFn: func(_ context.Context, in *organizations.ListOrganizationalUnitsForParentInput, _ ...func(*organizations.Options)) (*organizations.ListOrganizationalUnitsForParentOutput, error) {
			if pointerToString(in.ParentId) == "r-root" {
				return &organizations.ListOrganizationalUnitsForParentOutput{OrganizationalUnits: []organizationtypes.OrganizationalUnit{{Id: ptr("ou-sandbox"), Name: ptr("Sandbox")}}}, nil
			}
			return &organizations.ListOrganizationalUnitsForParentOutput{}, nil
		},
	}

	withMockOrgDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) organizationsAPI { return orgClient },
		func(awssdk.Config) ssoAdminAPI { return &mockOrgSSOAdminClient{} },
		func(awssdk.Config) identityStoreAPI { return &mockOrgIdentityStoreClient{} },
		func(awssdk.Config) accountAPI { return &mockOrgAccountClient{} },
	)

	output, err := executeCommand(t, "org", "generate-diagram")
	if err != nil {
		t.Fatalf("execute generate-diagram: %v", err)
	}
	if !strings.HasPrefix(strings.TrimSpace(output), "graph TB") {
		t.Fatalf("expected Mermaid graph output: %s", output)
	}
	if !strings.Contains(output, "-->") || !strings.Contains(output, "Sandbox") {
		t.Fatalf("expected Mermaid edges and OU label in output: %s", output)
	}
}

func TestOrgSetAlternateContactRequiresContactsFile(t *testing.T) {
	if _, err := executeCommand(t, "org", "set-alternate-contact"); err == nil {
		t.Fatal("expected contacts-file validation error")
	}
}

func TestOrgSetAlternateContactDryRun(t *testing.T) {
	putCalls := 0
	orgClient := &mockOrgOrganizationsClient{
		listAccountsFn: func(_ context.Context, _ *organizations.ListAccountsInput, _ ...func(*organizations.Options)) (*organizations.ListAccountsOutput, error) {
			return &organizations.ListAccountsOutput{Accounts: []organizationtypes.Account{{Id: ptr("123456789012"), Name: ptr("sandbox")}}}, nil
		},
	}
	accountClient := &mockOrgAccountClient{
		putAlternateContactFn: func(_ context.Context, _ *account.PutAlternateContactInput, _ ...func(*account.Options)) (*account.PutAlternateContactOutput, error) {
			putCalls++
			return &account.PutAlternateContactOutput{}, nil
		},
	}

	withMockOrgDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) organizationsAPI { return orgClient },
		func(awssdk.Config) ssoAdminAPI { return &mockOrgSSOAdminClient{} },
		func(awssdk.Config) identityStoreAPI { return &mockOrgIdentityStoreClient{} },
		func(awssdk.Config) accountAPI { return accountClient },
	)

	contactsFile := filepath.Join(t.TempDir(), "contacts.json")
	content := `{
	  "security": {"name":"Sec","title":"Security Lead","emailAddress":"sec@example.com","phoneNumber":"+10000000000"},
	  "billing": {"name":"Bill","title":"Finance Lead","emailAddress":"bill@example.com","phoneNumber":"+10000000001"},
	  "operations": {"name":"Ops","title":"Ops Lead","emailAddress":"ops@example.com","phoneNumber":"+10000000002"}
	}`
	if err := os.WriteFile(contactsFile, []byte(content), 0o600); err != nil {
		t.Fatalf("write contacts file: %v", err)
	}

	output, err := executeCommand(t, "--output", "json", "--dry-run", "org", "set-alternate-contact", "--contacts-file", contactsFile)
	if err != nil {
		t.Fatalf("execute set-alternate-contact --dry-run: %v", err)
	}

	if putCalls != 0 {
		t.Fatalf("expected no put calls in dry-run, got %d", putCalls)
	}
	if !strings.Contains(output, "would-set") || !strings.Contains(output, "SECURITY") {
		t.Fatalf("unexpected dry-run output: %s", output)
	}
}

func TestOrgAssignSSOAccessDryRun(t *testing.T) {
	createCalls := 0
	orgClient := &mockOrgOrganizationsClient{
		listRootsFn: func(_ context.Context, _ *organizations.ListRootsInput, _ ...func(*organizations.Options)) (*organizations.ListRootsOutput, error) {
			return &organizations.ListRootsOutput{Roots: []organizationtypes.Root{{Id: ptr("r-root")}}}, nil
		},
		listOUsFn: func(_ context.Context, in *organizations.ListOrganizationalUnitsForParentInput, _ ...func(*organizations.Options)) (*organizations.ListOrganizationalUnitsForParentOutput, error) {
			if pointerToString(in.ParentId) != "r-root" {
				return &organizations.ListOrganizationalUnitsForParentOutput{}, nil
			}
			return &organizations.ListOrganizationalUnitsForParentOutput{OrganizationalUnits: []organizationtypes.OrganizationalUnit{{Id: ptr("ou-1"), Name: ptr("Sandbox")}}}, nil
		},
		listForParentFn: func(_ context.Context, in *organizations.ListAccountsForParentInput, _ ...func(*organizations.Options)) (*organizations.ListAccountsForParentOutput, error) {
			if pointerToString(in.ParentId) != "ou-1" {
				return &organizations.ListAccountsForParentOutput{}, nil
			}
			return &organizations.ListAccountsForParentOutput{Accounts: []organizationtypes.Account{{Id: ptr("123456789012")}}}, nil
		},
	}
	ssoClient := &mockOrgSSOAdminClient{
		listInstancesFn: func(_ context.Context, _ *ssoadmin.ListInstancesInput, _ ...func(*ssoadmin.Options)) (*ssoadmin.ListInstancesOutput, error) {
			return &ssoadmin.ListInstancesOutput{Instances: []ssoadmintypes.InstanceMetadata{{InstanceArn: ptr("arn:aws:sso:::instance/ssoins-123"), IdentityStoreId: ptr("d-123")}}}, nil
		},
		listPSFn: func(_ context.Context, _ *ssoadmin.ListPermissionSetsInput, _ ...func(*ssoadmin.Options)) (*ssoadmin.ListPermissionSetsOutput, error) {
			return &ssoadmin.ListPermissionSetsOutput{PermissionSets: []string{"arn:aws:sso:::permissionSet/ps-1"}}, nil
		},
		describePSFn: func(_ context.Context, _ *ssoadmin.DescribePermissionSetInput, _ ...func(*ssoadmin.Options)) (*ssoadmin.DescribePermissionSetOutput, error) {
			return &ssoadmin.DescribePermissionSetOutput{PermissionSet: &ssoadmintypes.PermissionSet{Name: ptr("AdministratorAccess")}}, nil
		},
		createAssignmentFn: func(_ context.Context, _ *ssoadmin.CreateAccountAssignmentInput, _ ...func(*ssoadmin.Options)) (*ssoadmin.CreateAccountAssignmentOutput, error) {
			createCalls++
			return &ssoadmin.CreateAccountAssignmentOutput{}, nil
		},
	}
	identityClient := &mockOrgIdentityStoreClient{
		listGroupsFn: func(_ context.Context, _ *identitystore.ListGroupsInput, _ ...func(*identitystore.Options)) (*identitystore.ListGroupsOutput, error) {
			return &identitystore.ListGroupsOutput{Groups: []identitystoretypes.Group{{GroupId: ptr("group-1")}}}, nil
		},
	}

	withMockOrgDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) organizationsAPI { return orgClient },
		func(awssdk.Config) ssoAdminAPI { return ssoClient },
		func(awssdk.Config) identityStoreAPI { return identityClient },
		func(awssdk.Config) accountAPI { return &mockOrgAccountClient{} },
	)

	output, err := executeCommand(
		t,
		"--dry-run",
		"--output", "json",
		"org", "assign-sso-access",
		"--principal-name", "Administrators",
		"--principal-type", "GROUP",
		"--permission-set-name", "AdministratorAccess",
		"--ou-name", "Sandbox",
	)
	if err != nil {
		t.Fatalf("execute assign-sso-access --dry-run: %v", err)
	}

	if createCalls != 0 {
		t.Fatalf("expected no assignment API calls during dry-run, got %d", createCalls)
	}
	if !strings.Contains(output, "would-assign") || !strings.Contains(output, "123456789012") {
		t.Fatalf("unexpected dry-run output: %s", output)
	}
}

func TestOrgAssignSSOAccessNoConfirmExecutes(t *testing.T) {
	createCalls := 0
	orgClient := &mockOrgOrganizationsClient{
		listRootsFn: func(_ context.Context, _ *organizations.ListRootsInput, _ ...func(*organizations.Options)) (*organizations.ListRootsOutput, error) {
			return &organizations.ListRootsOutput{Roots: []organizationtypes.Root{{Id: ptr("r-root")}}}, nil
		},
		listOUsFn: func(_ context.Context, _ *organizations.ListOrganizationalUnitsForParentInput, _ ...func(*organizations.Options)) (*organizations.ListOrganizationalUnitsForParentOutput, error) {
			return &organizations.ListOrganizationalUnitsForParentOutput{
				OrganizationalUnits: []organizationtypes.OrganizationalUnit{{Id: ptr("ou-1"), Name: ptr("Sandbox")}},
			}, nil
		},
		listForParentFn: func(_ context.Context, _ *organizations.ListAccountsForParentInput, _ ...func(*organizations.Options)) (*organizations.ListAccountsForParentOutput, error) {
			return &organizations.ListAccountsForParentOutput{
				Accounts: []organizationtypes.Account{{Id: ptr("123456789012")}},
			}, nil
		},
	}
	ssoClient := &mockOrgSSOAdminClient{
		listInstancesFn: func(_ context.Context, _ *ssoadmin.ListInstancesInput, _ ...func(*ssoadmin.Options)) (*ssoadmin.ListInstancesOutput, error) {
			return &ssoadmin.ListInstancesOutput{
				Instances: []ssoadmintypes.InstanceMetadata{{InstanceArn: ptr("arn:aws:sso:::instance/ssoins-123"), IdentityStoreId: ptr("d-123")}},
			}, nil
		},
		listPSFn: func(_ context.Context, _ *ssoadmin.ListPermissionSetsInput, _ ...func(*ssoadmin.Options)) (*ssoadmin.ListPermissionSetsOutput, error) {
			return &ssoadmin.ListPermissionSetsOutput{PermissionSets: []string{"arn:aws:sso:::permissionSet/ps-1"}}, nil
		},
		describePSFn: func(_ context.Context, _ *ssoadmin.DescribePermissionSetInput, _ ...func(*ssoadmin.Options)) (*ssoadmin.DescribePermissionSetOutput, error) {
			return &ssoadmin.DescribePermissionSetOutput{PermissionSet: &ssoadmintypes.PermissionSet{Name: ptr("AdministratorAccess")}}, nil
		},
		createAssignmentFn: func(_ context.Context, _ *ssoadmin.CreateAccountAssignmentInput, _ ...func(*ssoadmin.Options)) (*ssoadmin.CreateAccountAssignmentOutput, error) {
			createCalls++
			return &ssoadmin.CreateAccountAssignmentOutput{
				AccountAssignmentCreationStatus: &ssoadmintypes.AccountAssignmentOperationStatus{RequestId: ptr("req-1")},
			}, nil
		},
		describeCreationStatusFn: func(_ context.Context, _ *ssoadmin.DescribeAccountAssignmentCreationStatusInput, _ ...func(*ssoadmin.Options)) (*ssoadmin.DescribeAccountAssignmentCreationStatusOutput, error) {
			return &ssoadmin.DescribeAccountAssignmentCreationStatusOutput{
				AccountAssignmentCreationStatus: &ssoadmintypes.AccountAssignmentOperationStatus{Status: ssoadmintypes.StatusValuesSucceeded},
			}, nil
		},
	}
	identityClient := &mockOrgIdentityStoreClient{
		listGroupsFn: func(_ context.Context, _ *identitystore.ListGroupsInput, _ ...func(*identitystore.Options)) (*identitystore.ListGroupsOutput, error) {
			return &identitystore.ListGroupsOutput{Groups: []identitystoretypes.Group{{GroupId: ptr("group-1")}}}, nil
		},
	}

	withMockOrgDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) organizationsAPI { return orgClient },
		func(awssdk.Config) ssoAdminAPI { return ssoClient },
		func(awssdk.Config) identityStoreAPI { return identityClient },
		func(awssdk.Config) accountAPI { return &mockOrgAccountClient{} },
	)

	output, err := executeCommand(
		t,
		"--output", "json",
		"--no-confirm",
		"org", "assign-sso-access",
		"--principal-name", "Administrators",
		"--principal-type", "GROUP",
		"--permission-set-name", "AdministratorAccess",
		"--ou-name", "Sandbox",
	)
	if err != nil {
		t.Fatalf("execute assign-sso-access --no-confirm: %v", err)
	}

	if createCalls != 1 || !strings.Contains(output, "\"action\": \"assigned\"") {
		t.Fatalf("unexpected output: %s", output)
	}
}

func TestOrgAssignSSOAccessNoConfirmReportsProvisioningFailure(t *testing.T) {
	orgClient := &mockOrgOrganizationsClient{
		listRootsFn: func(_ context.Context, _ *organizations.ListRootsInput, _ ...func(*organizations.Options)) (*organizations.ListRootsOutput, error) {
			return &organizations.ListRootsOutput{Roots: []organizationtypes.Root{{Id: ptr("r-root")}}}, nil
		},
		listOUsFn: func(_ context.Context, _ *organizations.ListOrganizationalUnitsForParentInput, _ ...func(*organizations.Options)) (*organizations.ListOrganizationalUnitsForParentOutput, error) {
			return &organizations.ListOrganizationalUnitsForParentOutput{
				OrganizationalUnits: []organizationtypes.OrganizationalUnit{{Id: ptr("ou-1"), Name: ptr("Sandbox")}},
			}, nil
		},
		listForParentFn: func(_ context.Context, _ *organizations.ListAccountsForParentInput, _ ...func(*organizations.Options)) (*organizations.ListAccountsForParentOutput, error) {
			return &organizations.ListAccountsForParentOutput{
				Accounts: []organizationtypes.Account{{Id: ptr("123456789012")}},
			}, nil
		},
	}
	ssoClient := &mockOrgSSOAdminClient{
		listInstancesFn: func(_ context.Context, _ *ssoadmin.ListInstancesInput, _ ...func(*ssoadmin.Options)) (*ssoadmin.ListInstancesOutput, error) {
			return &ssoadmin.ListInstancesOutput{
				Instances: []ssoadmintypes.InstanceMetadata{{InstanceArn: ptr("arn:aws:sso:::instance/ssoins-123"), IdentityStoreId: ptr("d-123")}},
			}, nil
		},
		listPSFn: func(_ context.Context, _ *ssoadmin.ListPermissionSetsInput, _ ...func(*ssoadmin.Options)) (*ssoadmin.ListPermissionSetsOutput, error) {
			return &ssoadmin.ListPermissionSetsOutput{PermissionSets: []string{"arn:aws:sso:::permissionSet/ps-1"}}, nil
		},
		describePSFn: func(_ context.Context, _ *ssoadmin.DescribePermissionSetInput, _ ...func(*ssoadmin.Options)) (*ssoadmin.DescribePermissionSetOutput, error) {
			return &ssoadmin.DescribePermissionSetOutput{PermissionSet: &ssoadmintypes.PermissionSet{Name: ptr("AdministratorAccess")}}, nil
		},
		createAssignmentFn: func(_ context.Context, _ *ssoadmin.CreateAccountAssignmentInput, _ ...func(*ssoadmin.Options)) (*ssoadmin.CreateAccountAssignmentOutput, error) {
			return &ssoadmin.CreateAccountAssignmentOutput{
				AccountAssignmentCreationStatus: &ssoadmintypes.AccountAssignmentOperationStatus{RequestId: ptr("req-1")},
			}, nil
		},
		describeCreationStatusFn: func(_ context.Context, _ *ssoadmin.DescribeAccountAssignmentCreationStatusInput, _ ...func(*ssoadmin.Options)) (*ssoadmin.DescribeAccountAssignmentCreationStatusOutput, error) {
			return &ssoadmin.DescribeAccountAssignmentCreationStatusOutput{
				AccountAssignmentCreationStatus: &ssoadmintypes.AccountAssignmentOperationStatus{
					Status:        ssoadmintypes.StatusValuesFailed,
					FailureReason: ptr("provisioning failed"),
				},
			}, nil
		},
	}
	identityClient := &mockOrgIdentityStoreClient{
		listGroupsFn: func(_ context.Context, _ *identitystore.ListGroupsInput, _ ...func(*identitystore.Options)) (*identitystore.ListGroupsOutput, error) {
			return &identitystore.ListGroupsOutput{Groups: []identitystoretypes.Group{{GroupId: ptr("group-1")}}}, nil
		},
	}

	withMockOrgDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) organizationsAPI { return orgClient },
		func(awssdk.Config) ssoAdminAPI { return ssoClient },
		func(awssdk.Config) identityStoreAPI { return identityClient },
		func(awssdk.Config) accountAPI { return &mockOrgAccountClient{} },
	)

	output, err := executeCommand(
		t,
		"--output", "json",
		"--no-confirm",
		"org", "assign-sso-access",
		"--principal-name", "Administrators",
		"--principal-type", "GROUP",
		"--permission-set-name", "AdministratorAccess",
		"--ou-name", "Sandbox",
	)
	if err != nil {
		t.Fatalf("execute assign-sso-access --no-confirm with failed status: %v", err)
	}

	if !strings.Contains(output, "failed: provisioning failed") {
		t.Fatalf("unexpected output: %s", output)
	}
}

func TestOrgRemoveSSOAccessDryRun(t *testing.T) {
	deleteCalls := 0
	orgClient := &mockOrgOrganizationsClient{
		listRootsFn: func(_ context.Context, _ *organizations.ListRootsInput, _ ...func(*organizations.Options)) (*organizations.ListRootsOutput, error) {
			return &organizations.ListRootsOutput{Roots: []organizationtypes.Root{{Id: ptr("r-root")}}}, nil
		},
		listOUsFn: func(_ context.Context, in *organizations.ListOrganizationalUnitsForParentInput, _ ...func(*organizations.Options)) (*organizations.ListOrganizationalUnitsForParentOutput, error) {
			if pointerToString(in.ParentId) != "r-root" {
				return &organizations.ListOrganizationalUnitsForParentOutput{}, nil
			}
			return &organizations.ListOrganizationalUnitsForParentOutput{OrganizationalUnits: []organizationtypes.OrganizationalUnit{{Id: ptr("ou-1"), Name: ptr("Sandbox")}}}, nil
		},
		listForParentFn: func(_ context.Context, in *organizations.ListAccountsForParentInput, _ ...func(*organizations.Options)) (*organizations.ListAccountsForParentOutput, error) {
			if pointerToString(in.ParentId) != "ou-1" {
				return &organizations.ListAccountsForParentOutput{}, nil
			}
			return &organizations.ListAccountsForParentOutput{Accounts: []organizationtypes.Account{{Id: ptr("123456789012")}}}, nil
		},
	}
	ssoClient := &mockOrgSSOAdminClient{
		listInstancesFn: func(_ context.Context, _ *ssoadmin.ListInstancesInput, _ ...func(*ssoadmin.Options)) (*ssoadmin.ListInstancesOutput, error) {
			return &ssoadmin.ListInstancesOutput{Instances: []ssoadmintypes.InstanceMetadata{{InstanceArn: ptr("arn:aws:sso:::instance/ssoins-123"), IdentityStoreId: ptr("d-123")}}}, nil
		},
		listPSFn: func(_ context.Context, _ *ssoadmin.ListPermissionSetsInput, _ ...func(*ssoadmin.Options)) (*ssoadmin.ListPermissionSetsOutput, error) {
			return &ssoadmin.ListPermissionSetsOutput{PermissionSets: []string{"arn:aws:sso:::permissionSet/ps-1"}}, nil
		},
		describePSFn: func(_ context.Context, _ *ssoadmin.DescribePermissionSetInput, _ ...func(*ssoadmin.Options)) (*ssoadmin.DescribePermissionSetOutput, error) {
			return &ssoadmin.DescribePermissionSetOutput{PermissionSet: &ssoadmintypes.PermissionSet{Name: ptr("AdministratorAccess")}}, nil
		},
		deleteAssignmentFn: func(_ context.Context, _ *ssoadmin.DeleteAccountAssignmentInput, _ ...func(*ssoadmin.Options)) (*ssoadmin.DeleteAccountAssignmentOutput, error) {
			deleteCalls++
			return &ssoadmin.DeleteAccountAssignmentOutput{}, nil
		},
	}
	identityClient := &mockOrgIdentityStoreClient{
		listGroupsFn: func(_ context.Context, _ *identitystore.ListGroupsInput, _ ...func(*identitystore.Options)) (*identitystore.ListGroupsOutput, error) {
			return &identitystore.ListGroupsOutput{Groups: []identitystoretypes.Group{{GroupId: ptr("group-1")}}}, nil
		},
	}

	withMockOrgDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) organizationsAPI { return orgClient },
		func(awssdk.Config) ssoAdminAPI { return ssoClient },
		func(awssdk.Config) identityStoreAPI { return identityClient },
		func(awssdk.Config) accountAPI { return &mockOrgAccountClient{} },
	)

	output, err := executeCommand(
		t,
		"--dry-run",
		"--output", "json",
		"org", "remove-sso-access",
		"--principal-name", "Administrators",
		"--principal-type", "GROUP",
		"--permission-set-name", "AdministratorAccess",
		"--ou-name", "Sandbox",
	)
	if err != nil {
		t.Fatalf("execute remove-sso-access --dry-run: %v", err)
	}

	if deleteCalls != 0 {
		t.Fatalf("expected no delete assignment API calls during dry-run, got %d", deleteCalls)
	}
	if !strings.Contains(output, "would-remove") || !strings.Contains(output, "123456789012") {
		t.Fatalf("unexpected dry-run output: %s", output)
	}
}

func TestOrgRemoveSSOAccessNoConfirmExecutes(t *testing.T) {
	deleteCalls := 0
	orgClient := &mockOrgOrganizationsClient{
		listRootsFn: func(_ context.Context, _ *organizations.ListRootsInput, _ ...func(*organizations.Options)) (*organizations.ListRootsOutput, error) {
			return &organizations.ListRootsOutput{Roots: []organizationtypes.Root{{Id: ptr("r-root")}}}, nil
		},
		listOUsFn: func(_ context.Context, _ *organizations.ListOrganizationalUnitsForParentInput, _ ...func(*organizations.Options)) (*organizations.ListOrganizationalUnitsForParentOutput, error) {
			return &organizations.ListOrganizationalUnitsForParentOutput{
				OrganizationalUnits: []organizationtypes.OrganizationalUnit{{Id: ptr("ou-1"), Name: ptr("Sandbox")}},
			}, nil
		},
		listForParentFn: func(_ context.Context, _ *organizations.ListAccountsForParentInput, _ ...func(*organizations.Options)) (*organizations.ListAccountsForParentOutput, error) {
			return &organizations.ListAccountsForParentOutput{
				Accounts: []organizationtypes.Account{{Id: ptr("123456789012")}},
			}, nil
		},
	}
	ssoClient := &mockOrgSSOAdminClient{
		listInstancesFn: func(_ context.Context, _ *ssoadmin.ListInstancesInput, _ ...func(*ssoadmin.Options)) (*ssoadmin.ListInstancesOutput, error) {
			return &ssoadmin.ListInstancesOutput{
				Instances: []ssoadmintypes.InstanceMetadata{{InstanceArn: ptr("arn:aws:sso:::instance/ssoins-123"), IdentityStoreId: ptr("d-123")}},
			}, nil
		},
		listPSFn: func(_ context.Context, _ *ssoadmin.ListPermissionSetsInput, _ ...func(*ssoadmin.Options)) (*ssoadmin.ListPermissionSetsOutput, error) {
			return &ssoadmin.ListPermissionSetsOutput{PermissionSets: []string{"arn:aws:sso:::permissionSet/ps-1"}}, nil
		},
		describePSFn: func(_ context.Context, _ *ssoadmin.DescribePermissionSetInput, _ ...func(*ssoadmin.Options)) (*ssoadmin.DescribePermissionSetOutput, error) {
			return &ssoadmin.DescribePermissionSetOutput{PermissionSet: &ssoadmintypes.PermissionSet{Name: ptr("AdministratorAccess")}}, nil
		},
		deleteAssignmentFn: func(_ context.Context, _ *ssoadmin.DeleteAccountAssignmentInput, _ ...func(*ssoadmin.Options)) (*ssoadmin.DeleteAccountAssignmentOutput, error) {
			deleteCalls++
			return &ssoadmin.DeleteAccountAssignmentOutput{
				AccountAssignmentDeletionStatus: &ssoadmintypes.AccountAssignmentOperationStatus{RequestId: ptr("req-1")},
			}, nil
		},
		describeDeletionStatusFn: func(_ context.Context, _ *ssoadmin.DescribeAccountAssignmentDeletionStatusInput, _ ...func(*ssoadmin.Options)) (*ssoadmin.DescribeAccountAssignmentDeletionStatusOutput, error) {
			return &ssoadmin.DescribeAccountAssignmentDeletionStatusOutput{
				AccountAssignmentDeletionStatus: &ssoadmintypes.AccountAssignmentOperationStatus{Status: ssoadmintypes.StatusValuesSucceeded},
			}, nil
		},
	}
	identityClient := &mockOrgIdentityStoreClient{
		listGroupsFn: func(_ context.Context, _ *identitystore.ListGroupsInput, _ ...func(*identitystore.Options)) (*identitystore.ListGroupsOutput, error) {
			return &identitystore.ListGroupsOutput{Groups: []identitystoretypes.Group{{GroupId: ptr("group-1")}}}, nil
		},
	}

	withMockOrgDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) organizationsAPI { return orgClient },
		func(awssdk.Config) ssoAdminAPI { return ssoClient },
		func(awssdk.Config) identityStoreAPI { return identityClient },
		func(awssdk.Config) accountAPI { return &mockOrgAccountClient{} },
	)

	output, err := executeCommand(
		t,
		"--output", "json",
		"--no-confirm",
		"org", "remove-sso-access",
		"--principal-name", "Administrators",
		"--principal-type", "GROUP",
		"--permission-set-name", "AdministratorAccess",
		"--ou-name", "Sandbox",
	)
	if err != nil {
		t.Fatalf("execute remove-sso-access --no-confirm: %v", err)
	}

	if deleteCalls != 1 || !strings.Contains(output, "\"action\": \"removed\"") {
		t.Fatalf("unexpected output: %s", output)
	}
}

func TestOrgGetAccountIncludesTags(t *testing.T) {
	joined := time.Date(2024, time.January, 10, 12, 30, 0, 0, time.UTC)
	orgClient := &mockOrgOrganizationsClient{
		describeAccountFn: func(_ context.Context, in *organizations.DescribeAccountInput, _ ...func(*organizations.Options)) (*organizations.DescribeAccountOutput, error) {
			if pointerToString(in.AccountId) != "123456789012" {
				t.Fatalf("unexpected account id: %s", pointerToString(in.AccountId))
			}
			return &organizations.DescribeAccountOutput{
				Account: &organizationtypes.Account{
					Id:              ptr("123456789012"),
					Name:            ptr("sandbox"),
					Email:           ptr("sandbox@example.com"),
					Status:          organizationtypes.AccountStatusActive,
					Arn:             ptr("arn:aws:organizations::123456789012:account/o-root/123456789012"),
					JoinedMethod:    organizationtypes.AccountJoinedMethodInvited,
					JoinedTimestamp: &joined,
				},
			}, nil
		},
		listTagsFn: func(_ context.Context, _ *organizations.ListTagsForResourceInput, _ ...func(*organizations.Options)) (*organizations.ListTagsForResourceOutput, error) {
			return &organizations.ListTagsForResourceOutput{
				Tags: []organizationtypes.Tag{{Key: ptr("env"), Value: ptr("dev")}},
			}, nil
		},
	}

	withMockOrgDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) organizationsAPI { return orgClient },
		func(awssdk.Config) ssoAdminAPI { return &mockOrgSSOAdminClient{} },
		func(awssdk.Config) identityStoreAPI { return &mockOrgIdentityStoreClient{} },
		func(awssdk.Config) accountAPI { return &mockOrgAccountClient{} },
	)

	output, err := executeCommand(t, "--output", "json", "org", "get-account", "--account-id", "123456789012")
	if err != nil {
		t.Fatalf("execute get-account: %v", err)
	}
	if !strings.Contains(output, "sandbox@example.com") || !strings.Contains(output, "tag:env") || !strings.Contains(output, "2024-01-10T12:30:00Z") {
		t.Fatalf("unexpected output: %s", output)
	}
}

func TestOrgGetAccountRejectsInvalidAccountID(t *testing.T) {
	if _, err := executeCommand(t, "org", "get-account", "--account-id", "1234"); err == nil {
		t.Fatal("expected account-id validation error")
	}
}

func TestOrgListSSOAssignments(t *testing.T) {
	orgClient := &mockOrgOrganizationsClient{
		listAccountsFn: func(_ context.Context, _ *organizations.ListAccountsInput, _ ...func(*organizations.Options)) (*organizations.ListAccountsOutput, error) {
			return &organizations.ListAccountsOutput{
				Accounts: []organizationtypes.Account{{Id: ptr("123456789012"), Name: ptr("sandbox"), Status: organizationtypes.AccountStatusActive}},
			}, nil
		},
	}
	ssoClient := &mockOrgSSOAdminClient{
		listInstancesFn: func(_ context.Context, _ *ssoadmin.ListInstancesInput, _ ...func(*ssoadmin.Options)) (*ssoadmin.ListInstancesOutput, error) {
			return &ssoadmin.ListInstancesOutput{
				Instances: []ssoadmintypes.InstanceMetadata{{InstanceArn: ptr("arn:aws:sso:::instance/ssoins-123"), IdentityStoreId: ptr("d-123")}},
			}, nil
		},
		listPSFn: func(_ context.Context, _ *ssoadmin.ListPermissionSetsInput, _ ...func(*ssoadmin.Options)) (*ssoadmin.ListPermissionSetsOutput, error) {
			return &ssoadmin.ListPermissionSetsOutput{PermissionSets: []string{"arn:aws:sso:::permissionSet/ps-1"}}, nil
		},
		listAssignmentsFn: func(_ context.Context, in *ssoadmin.ListAccountAssignmentsInput, _ ...func(*ssoadmin.Options)) (*ssoadmin.ListAccountAssignmentsOutput, error) {
			if pointerToString(in.AccountId) != "123456789012" {
				t.Fatalf("unexpected account id: %s", pointerToString(in.AccountId))
			}
			return &ssoadmin.ListAccountAssignmentsOutput{
				AccountAssignments: []ssoadmintypes.AccountAssignment{
					{PrincipalType: ssoadmintypes.PrincipalTypeGroup, PrincipalId: ptr("group-1")},
				},
			}, nil
		},
	}

	withMockOrgDeps(
		t,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-east-1"}, nil },
		func(awssdk.Config) organizationsAPI { return orgClient },
		func(awssdk.Config) ssoAdminAPI { return ssoClient },
		func(awssdk.Config) identityStoreAPI { return &mockOrgIdentityStoreClient{} },
		func(awssdk.Config) accountAPI { return &mockOrgAccountClient{} },
	)

	output, err := executeCommand(t, "--output", "json", "org", "list-sso-assignments")
	if err != nil {
		t.Fatalf("execute list-sso-assignments: %v", err)
	}
	if !strings.Contains(output, "group-1") || !strings.Contains(output, "arn:aws:sso:::permissionSet/ps-1") {
		t.Fatalf("unexpected output: %s", output)
	}
}
