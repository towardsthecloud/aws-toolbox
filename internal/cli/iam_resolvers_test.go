package cli

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/identitystore"
	identitystoretypes "github.com/aws/aws-sdk-go-v2/service/identitystore/types"
	"github.com/aws/aws-sdk-go-v2/service/ssoadmin"
	ssoadmintypes "github.com/aws/aws-sdk-go-v2/service/ssoadmin/types"
)

func TestResolveSSOUserEmailsFromInputFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "emails.txt")
	content := "john.doe@example.com, jane.doe@example.com\njohn.doe@example.com"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write emails file: %v", err)
	}

	emails, err := resolveSSOUserEmails(nil, path)
	if err != nil {
		t.Fatalf("resolve emails: %v", err)
	}
	if len(emails) != 2 {
		t.Fatalf("expected deduplicated emails, got %v", emails)
	}
}

func TestResolveIdentityStoreIDAndGroupLookup(t *testing.T) {
	ssoClient := &mockSSOAdminClient{
		listInstancesFn: func(_ context.Context, _ *ssoadmin.ListInstancesInput, _ ...func(*ssoadmin.Options)) (*ssoadmin.ListInstancesOutput, error) {
			return &ssoadmin.ListInstancesOutput{
				Instances: []ssoadmintypes.InstanceMetadata{{IdentityStoreId: ptr("d-1234567890")}},
			}, nil
		},
	}
	identityStoreID, err := resolveIdentityStoreID(context.Background(), ssoClient)
	if err != nil || identityStoreID != "d-1234567890" {
		t.Fatalf("unexpected identity store resolution: id=%s err=%v", identityStoreID, err)
	}

	identityClient := &mockIdentityStoreClient{
		listGroupsFn: func(_ context.Context, _ *identitystore.ListGroupsInput, _ ...func(*identitystore.Options)) (*identitystore.ListGroupsOutput, error) {
			return &identitystore.ListGroupsOutput{
				Groups: []identitystoretypes.Group{{DisplayName: ptr("engineering"), GroupId: ptr("grp-1")}},
			}, nil
		},
	}
	groupID, err := resolveIdentityStoreGroupID(context.Background(), identityClient, "d-1234567890", "engineering")
	if err != nil || groupID != "grp-1" {
		t.Fatalf("unexpected group resolution: id=%s err=%v", groupID, err)
	}
}
