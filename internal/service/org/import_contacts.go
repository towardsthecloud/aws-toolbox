package org

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
	"github.com/towardsthecloud/aws-toolbox/internal/cliutil"
)

type importRow struct {
	FirstName string
	LastName  string
	Email     string
	GroupName string
}

type contact struct {
	Name         string `json:"name"`
	Title        string `json:"title"`
	EmailAddress string `json:"emailAddress"`
	PhoneNumber  string `json:"phoneNumber"`
}

type contactsPayload struct {
	SecurityContact   contact `json:"securityContact"`
	BillingContact    contact `json:"billingContact"`
	OperationsContact contact `json:"operationsContact"`
	Security          contact `json:"security"`
	Billing           contact `json:"billing"`
	Operations        contact `json:"operations"`
}

func runImportSSOUsers(cmd *cobra.Command, inputFile string) error {
	if strings.TrimSpace(inputFile) == "" {
		return fmt.Errorf("--input-file is required")
	}
	imports, err := readImportCSV(inputFile)
	if err != nil {
		return err
	}

	runtime, _, ssoClient, identityClient, _, err := runtimeClients(cmd)
	if err != nil {
		return err
	}
	ctx := cmd.Context()

	instance, err := resolveSSOInstance(ctx, ssoClient)
	if err != nil {
		return err
	}

	rows := make([][]string, 0, len(imports))
	for _, row := range imports {
		groupID, groupAction, groupErr := ensureGroup(ctx, identityClient, instance.IdentityStoreID, row.GroupName, runtime.Options.DryRun)
		if groupErr != nil {
			rows = append(rows, []string{row.Email, row.GroupName, "failed", "failed", "failed: " + awstbxaws.FormatUserError(groupErr)})
			continue
		}
		userID, userAction, userErr := ensureUser(ctx, identityClient, instance.IdentityStoreID, row, runtime.Options.DryRun)
		if userErr != nil {
			rows = append(rows, []string{row.Email, row.GroupName, userAction, groupAction, "failed: " + awstbxaws.FormatUserError(userErr)})
			continue
		}

		membershipAction := "would-add-to-group"
		if !runtime.Options.DryRun {
			_, membershipErr := identityClient.CreateGroupMembership(ctx, &identitystore.CreateGroupMembershipInput{
				IdentityStoreId: cliutil.Ptr(instance.IdentityStoreID),
				GroupId:         cliutil.Ptr(groupID),
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

	return cliutil.WriteDataset(cmd, runtime, []string{"email", "group_name", "user_action", "group_action", "membership_action"}, rows)
}

func runSetAlternateContact(cmd *cobra.Command, inputFile string) error {
	if strings.TrimSpace(inputFile) == "" {
		return fmt.Errorf("--input-file is required")
	}
	contactsByType, err := loadContacts(inputFile)
	if err != nil {
		return err
	}

	runtime, orgClient, _, _, accountClient, err := runtimeClients(cmd)
	if err != nil {
		return err
	}

	accounts, err := listAccounts(cmd.Context(), orgClient)
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
		id := cliutil.PointerToString(acct.Id)
		for _, contactType := range typesInOrder {
			c := contactsByType[contactType]
			action := "would-set"
			if !runtime.Options.DryRun {
				action = "pending"
			}
			rows = append(rows, []string{id, string(contactType), c.EmailAddress, c.Name, c.Title, c.PhoneNumber, action})
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
			return cliutil.WriteDataset(cmd, runtime, []string{"account_id", "contact_type", "email", "name", "title", "phone", "action"}, rows)
		}

		for i := range rows {
			contactType := accounttypes.AlternateContactType(rows[i][1])
			c := contactsByType[contactType]
			_, putErr := accountClient.PutAlternateContact(cmd.Context(), &account.PutAlternateContactInput{
				AccountId:            cliutil.Ptr(rows[i][0]),
				AlternateContactType: contactType,
				EmailAddress:         cliutil.Ptr(c.EmailAddress),
				Name:                 cliutil.Ptr(c.Name),
				PhoneNumber:          cliutil.Ptr(c.PhoneNumber),
				Title:                cliutil.Ptr(c.Title),
			})
			if putErr != nil {
				rows[i][6] = "failed: " + awstbxaws.FormatUserError(putErr)
				continue
			}
			rows[i][6] = "updated"
		}
	}

	return cliutil.WriteDataset(cmd, runtime, []string{"account_id", "contact_type", "email", "name", "title", "phone", "action"}, rows)
}

func readImportCSV(path string) ([]importRow, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open input file: %w", err)
	}
	defer f.Close()

	recs, err := csv.NewReader(f).ReadAll()
	if err != nil {
		return nil, fmt.Errorf("read CSV file: %w", err)
	}
	rows := make([]importRow, 0, len(recs))
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
		rows = append(rows, importRow{FirstName: first, LastName: last, Email: email, GroupName: group})
	}
	return rows, nil
}

func ensureGroup(ctx context.Context, identityClient IdentityStoreAPI, storeID, groupName string, dryRun bool) (string, string, error) {
	out, err := identityClient.ListGroups(ctx, &identitystore.ListGroupsInput{IdentityStoreId: cliutil.Ptr(storeID), Filters: []identitystoretypes.Filter{{AttributePath: cliutil.Ptr("DisplayName"), AttributeValue: cliutil.Ptr(groupName)}}})
	if err != nil {
		return "", "failed", err
	}
	if len(out.Groups) > 0 {
		return cliutil.PointerToString(out.Groups[0].GroupId), "existing-group", nil
	}
	if dryRun {
		return "dryrun-group", "would-create-group", nil
	}
	createOut, err := identityClient.CreateGroup(ctx, &identitystore.CreateGroupInput{IdentityStoreId: cliutil.Ptr(storeID), DisplayName: cliutil.Ptr(groupName)})
	if err != nil {
		return "", "failed", err
	}
	return cliutil.PointerToString(createOut.GroupId), "created-group", nil
}

func ensureUser(ctx context.Context, identityClient IdentityStoreAPI, storeID string, row importRow, dryRun bool) (string, string, error) {
	out, err := identityClient.ListUsers(ctx, &identitystore.ListUsersInput{IdentityStoreId: cliutil.Ptr(storeID), Filters: []identitystoretypes.Filter{{AttributePath: cliutil.Ptr("UserName"), AttributeValue: cliutil.Ptr(row.Email)}}})
	if err != nil {
		return "", "failed", err
	}
	if len(out.Users) > 0 {
		return cliutil.PointerToString(out.Users[0].UserId), "existing-user", nil
	}
	if dryRun {
		return "dryrun-user", "would-create-user", nil
	}
	display := strings.TrimSpace(row.FirstName + " " + row.LastName)
	createOut, err := identityClient.CreateUser(ctx, &identitystore.CreateUserInput{
		IdentityStoreId: cliutil.Ptr(storeID),
		UserName:        cliutil.Ptr(row.Email),
		DisplayName:     cliutil.Ptr(display),
		Name:            &identitystoretypes.Name{GivenName: cliutil.Ptr(row.FirstName), FamilyName: cliutil.Ptr(row.LastName), Formatted: cliutil.Ptr(display)},
		Emails:          []identitystoretypes.Email{{Value: cliutil.Ptr(row.Email), Type: cliutil.Ptr("Work"), Primary: true}},
	})
	if err != nil {
		return "", "failed", err
	}
	return cliutil.PointerToString(createOut.UserId), "created-user", nil
}

func loadContacts(path string) (map[accounttypes.AlternateContactType]contact, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read contacts file: %w", err)
	}
	var payload contactsPayload
	if err := json.Unmarshal(b, &payload); err != nil {
		return nil, fmt.Errorf("parse contacts file: %w", err)
	}

	security := payload.Security
	if security == (contact{}) {
		security = payload.SecurityContact
	}
	billing := payload.Billing
	if billing == (contact{}) {
		billing = payload.BillingContact
	}
	operations := payload.Operations
	if operations == (contact{}) {
		operations = payload.OperationsContact
	}

	contacts := map[accounttypes.AlternateContactType]contact{
		accounttypes.AlternateContactTypeSecurity:   security,
		accounttypes.AlternateContactTypeBilling:    billing,
		accounttypes.AlternateContactTypeOperations: operations,
	}
	for contactType, c := range contacts {
		if strings.TrimSpace(c.Name) == "" || strings.TrimSpace(c.Title) == "" || strings.TrimSpace(c.EmailAddress) == "" || strings.TrimSpace(c.PhoneNumber) == "" {
			return nil, fmt.Errorf("contacts file missing required fields for %s contact", strings.ToLower(string(contactType)))
		}
	}
	return contacts, nil
}
