package cli

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/account"
	accounttypes "github.com/aws/aws-sdk-go-v2/service/account/types"
	"github.com/aws/aws-sdk-go-v2/service/identitystore"
	identitystoretypes "github.com/aws/aws-sdk-go-v2/service/identitystore/types"
	"github.com/spf13/cobra"
	awstbxaws "github.com/towardsthecloud/aws-toolbox/internal/aws"
)

type orgImportRow struct {
	FirstName string
	LastName  string
	Email     string
	GroupName string
}

type orgContact struct {
	Name         string `json:"name"`
	Title        string `json:"title"`
	EmailAddress string `json:"emailAddress"`
	PhoneNumber  string `json:"phoneNumber"`
}

type orgContactsPayload struct {
	SecurityContact   orgContact `json:"securityContact"`
	BillingContact    orgContact `json:"billingContact"`
	OperationsContact orgContact `json:"operationsContact"`
	Security          orgContact `json:"security"`
	Billing           orgContact `json:"billing"`
	Operations        orgContact `json:"operations"`
}

func runOrgImportSSOUsers(cmd *cobra.Command, inputFile string) error {
	if strings.TrimSpace(inputFile) == "" {
		return fmt.Errorf("--input-file is required")
	}
	imports, err := readOrgImportCSV(inputFile)
	if err != nil {
		return err
	}

	runtime, _, ssoClient, identityClient, _, err := orgRuntimeClients(cmd)
	if err != nil {
		return err
	}
	ctx := cmd.Context()

	instance, err := resolveOrgSSOInstance(ctx, ssoClient)
	if err != nil {
		return err
	}

	rows := make([][]string, 0, len(imports))
	for _, row := range imports {
		groupID, groupAction, groupErr := ensureOrgGroup(ctx, identityClient, instance.IdentityStoreID, row.GroupName, runtime.Options.DryRun)
		if groupErr != nil {
			rows = append(rows, []string{row.Email, row.GroupName, "failed", "failed", "failed: " + awstbxaws.FormatUserError(groupErr)})
			continue
		}
		userID, userAction, userErr := ensureOrgUser(ctx, identityClient, instance.IdentityStoreID, row, runtime.Options.DryRun)
		if userErr != nil {
			rows = append(rows, []string{row.Email, row.GroupName, userAction, groupAction, "failed: " + awstbxaws.FormatUserError(userErr)})
			continue
		}

		membershipAction := "would-add-to-group"
		if !runtime.Options.DryRun {
			_, membershipErr := identityClient.CreateGroupMembership(ctx, &identitystore.CreateGroupMembershipInput{
				IdentityStoreId: ptr(instance.IdentityStoreID),
				GroupId:         ptr(groupID),
				MemberId:        &identitystoretypes.MemberIdMemberUserId{Value: userID},
			})
			if membershipErr != nil {
				membershipAction = "failed: " + awstbxaws.FormatUserError(membershipErr)
			} else {
				membershipAction = "added-to-group"
			}
		}

		rows = append(rows, []string{row.Email, row.GroupName, userAction, groupAction, membershipAction})
	}

	return writeDataset(cmd, runtime, []string{"email", "group_name", "user_action", "group_action", "membership_action"}, rows)
}

func runOrgSetAlternateContact(cmd *cobra.Command, inputFile string) error {
	if strings.TrimSpace(inputFile) == "" {
		return fmt.Errorf("--input-file is required")
	}
	contactsByType, err := loadOrgContacts(inputFile)
	if err != nil {
		return err
	}

	runtime, orgClient, _, _, accountClient, err := orgRuntimeClients(cmd)
	if err != nil {
		return err
	}

	accounts, err := listOrgAccounts(cmd.Context(), orgClient)
	if err != nil {
		return fmt.Errorf("list accounts: %s", awstbxaws.FormatUserError(err))
	}
	sortAccountsByID(accounts)

	typesInOrder := []accounttypes.AlternateContactType{
		accounttypes.AlternateContactTypeSecurity,
		accounttypes.AlternateContactTypeBilling,
		accounttypes.AlternateContactTypeOperations,
	}

	rows := make([][]string, 0, len(accounts)*len(typesInOrder))
	for _, acct := range accounts {
		id := pointerToString(acct.Id)
		for _, contactType := range typesInOrder {
			contact := contactsByType[contactType]
			action := "would-set"
			if !runtime.Options.DryRun {
				action = "pending"
			}
			rows = append(rows, []string{id, string(contactType), contact.EmailAddress, contact.Name, contact.Title, contact.PhoneNumber, action})
		}
	}

	if !runtime.Options.DryRun && len(rows) > 0 {
		ok, confirmErr := runtime.Prompter.Confirm(fmt.Sprintf("Set alternate contacts for %d account(s)", len(accounts)), runtime.Options.NoConfirm)
		if confirmErr != nil {
			return confirmErr
		}
		if !ok {
			for i := range rows {
				rows[i][6] = "cancelled"
			}
			return writeDataset(cmd, runtime, []string{"account_id", "contact_type", "email", "name", "title", "phone", "action"}, rows)
		}

		for i := range rows {
			contactType := accounttypes.AlternateContactType(rows[i][1])
			contact := contactsByType[contactType]
			_, putErr := accountClient.PutAlternateContact(cmd.Context(), &account.PutAlternateContactInput{
				AccountId:            ptr(rows[i][0]),
				AlternateContactType: contactType,
				EmailAddress:         ptr(contact.EmailAddress),
				Name:                 ptr(contact.Name),
				PhoneNumber:          ptr(contact.PhoneNumber),
				Title:                ptr(contact.Title),
			})
			if putErr != nil {
				rows[i][6] = "failed: " + awstbxaws.FormatUserError(putErr)
				continue
			}
			rows[i][6] = "updated"
		}
	}

	return writeDataset(cmd, runtime, []string{"account_id", "contact_type", "email", "name", "title", "phone", "action"}, rows)
}

func readOrgImportCSV(path string) ([]orgImportRow, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open input file: %w", err)
	}
	defer f.Close()

	recs, err := csv.NewReader(f).ReadAll()
	if err != nil {
		return nil, fmt.Errorf("read CSV file: %w", err)
	}
	rows := make([]orgImportRow, 0, len(recs))
	for i, rec := range recs {
		if len(rec) < 4 {
			return nil, fmt.Errorf("invalid CSV row %d: expected 4 columns", i+1)
		}
		first := strings.TrimSpace(rec[0])
		last := strings.TrimSpace(rec[1])
		email := strings.TrimSpace(rec[2])
		group := strings.TrimSpace(rec[3])
		if i == 0 && strings.EqualFold(first, "first_name") && strings.EqualFold(last, "last_name") && strings.EqualFold(email, "email") {
			continue
		}
		rows = append(rows, orgImportRow{FirstName: first, LastName: last, Email: email, GroupName: group})
	}
	return rows, nil
}

func ensureOrgGroup(ctx context.Context, identityClient identityStoreAPI, storeID, groupName string, dryRun bool) (string, string, error) {
	out, err := identityClient.ListGroups(ctx, &identitystore.ListGroupsInput{IdentityStoreId: ptr(storeID), Filters: []identitystoretypes.Filter{{AttributePath: ptr("DisplayName"), AttributeValue: ptr(groupName)}}})
	if err != nil {
		return "", "failed", err
	}
	if len(out.Groups) > 0 {
		return pointerToString(out.Groups[0].GroupId), "existing-group", nil
	}
	if dryRun {
		return "dryrun-group", "would-create-group", nil
	}
	createOut, err := identityClient.CreateGroup(ctx, &identitystore.CreateGroupInput{IdentityStoreId: ptr(storeID), DisplayName: ptr(groupName)})
	if err != nil {
		return "", "failed", err
	}
	return pointerToString(createOut.GroupId), "created-group", nil
}

func ensureOrgUser(ctx context.Context, identityClient identityStoreAPI, storeID string, row orgImportRow, dryRun bool) (string, string, error) {
	out, err := identityClient.ListUsers(ctx, &identitystore.ListUsersInput{IdentityStoreId: ptr(storeID), Filters: []identitystoretypes.Filter{{AttributePath: ptr("UserName"), AttributeValue: ptr(row.Email)}}})
	if err != nil {
		return "", "failed", err
	}
	if len(out.Users) > 0 {
		return pointerToString(out.Users[0].UserId), "existing-user", nil
	}
	if dryRun {
		return "dryrun-user", "would-create-user", nil
	}
	display := strings.TrimSpace(row.FirstName + " " + row.LastName)
	createOut, err := identityClient.CreateUser(ctx, &identitystore.CreateUserInput{
		IdentityStoreId: ptr(storeID),
		UserName:        ptr(row.Email),
		DisplayName:     ptr(display),
		Name:            &identitystoretypes.Name{GivenName: ptr(row.FirstName), FamilyName: ptr(row.LastName), Formatted: ptr(display)},
		Emails:          []identitystoretypes.Email{{Value: ptr(row.Email), Type: ptr("Work"), Primary: true}},
	})
	if err != nil {
		return "", "failed", err
	}
	return pointerToString(createOut.UserId), "created-user", nil
}

func loadOrgContacts(path string) (map[accounttypes.AlternateContactType]orgContact, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read contacts file: %w", err)
	}
	var payload orgContactsPayload
	if err := json.Unmarshal(b, &payload); err != nil {
		return nil, fmt.Errorf("parse contacts file: %w", err)
	}

	security := payload.Security
	if security == (orgContact{}) {
		security = payload.SecurityContact
	}
	billing := payload.Billing
	if billing == (orgContact{}) {
		billing = payload.BillingContact
	}
	operations := payload.Operations
	if operations == (orgContact{}) {
		operations = payload.OperationsContact
	}

	contacts := map[accounttypes.AlternateContactType]orgContact{
		accounttypes.AlternateContactTypeSecurity:   security,
		accounttypes.AlternateContactTypeBilling:    billing,
		accounttypes.AlternateContactTypeOperations: operations,
	}
	for contactType, contact := range contacts {
		if strings.TrimSpace(contact.Name) == "" || strings.TrimSpace(contact.Title) == "" || strings.TrimSpace(contact.EmailAddress) == "" || strings.TrimSpace(contact.PhoneNumber) == "" {
			return nil, fmt.Errorf("contacts file missing required fields for %s contact", strings.ToLower(string(contactType)))
		}
	}
	return contacts, nil
}
