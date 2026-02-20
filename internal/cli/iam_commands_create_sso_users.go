package cli

import (
	"context"
	"fmt"
	"os"
	"strings"
	"unicode"

	"github.com/aws/aws-sdk-go-v2/service/identitystore"
	identitystoretypes "github.com/aws/aws-sdk-go-v2/service/identitystore/types"
	"github.com/aws/aws-sdk-go-v2/service/ssoadmin"
	ssoadmintypes "github.com/aws/aws-sdk-go-v2/service/ssoadmin/types"
	"github.com/spf13/cobra"
	awstbxaws "github.com/towardsthecloud/aws-toolbox/internal/aws"
)

func runIAMCreateSSOUsers(cmd *cobra.Command, emailFlags []string, inputFile, groupName string) error {
	emails, err := resolveSSOUserEmails(emailFlags, inputFile)
	if err != nil {
		return err
	}

	runtime, cfg, err := newServiceConfigRuntime(cmd, iamLoadAWSConfig)
	if err != nil {
		return err
	}

	ctx := cmd.Context()
	ssoClient := iamNewSSOAdminClient(cfg)
	identityStoreClient := iamNewIdentityStoreClient(cfg)

	identityStoreID, err := resolveIdentityStoreID(ctx, ssoClient)
	if err != nil {
		return fmt.Errorf("resolve IAM Identity Center instance: %s", awstbxaws.FormatUserError(err))
	}

	requestedGroup := strings.TrimSpace(groupName)
	groupID := ""
	if requestedGroup != "" {
		groupID, err = resolveIdentityStoreGroupID(ctx, identityStoreClient, identityStoreID, requestedGroup)
		if err != nil {
			return fmt.Errorf("lookup group %q: %s", requestedGroup, awstbxaws.FormatUserError(err))
		}
	}

	rows := make([][]string, 0, len(emails))
	for _, email := range emails {
		firstName, lastName := parseNameFromEmail(email)
		displayName := strings.TrimSpace(firstName + " " + lastName)
		action := "would-create"
		if !runtime.Options.DryRun {
			action = "pending"
		}
		rows = append(rows, []string{email, displayName, requestedGroup, action})
	}

	if runtime.Options.DryRun {
		return writeDataset(cmd, runtime, []string{"email", "display_name", "group", "action"}, rows)
	}

	ok, confirmErr := runtime.Prompter.Confirm(
		fmt.Sprintf("Create %d IAM Identity Center user(s)", len(rows)),
		runtime.Options.NoConfirm,
	)
	if confirmErr != nil {
		return confirmErr
	}
	if !ok {
		for i := range rows {
			rows[i][3] = "cancelled"
		}
		return writeDataset(cmd, runtime, []string{"email", "display_name", "group", "action"}, rows)
	}

	for i, email := range emails {
		firstName, lastName := parseNameFromEmail(email)
		displayName := strings.TrimSpace(firstName + " " + lastName)
		userOut, createErr := identityStoreClient.CreateUser(ctx, &identitystore.CreateUserInput{
			IdentityStoreId: ptr(identityStoreID),
			UserName:        ptr(email),
			DisplayName:     ptr(displayName),
			Name: &identitystoretypes.Name{
				GivenName:  ptr(firstName),
				FamilyName: ptr(lastName),
			},
			Emails: []identitystoretypes.Email{{
				Value:   ptr(email),
				Type:    ptr("Work"),
				Primary: true,
			}},
		})
		if createErr != nil {
			rows[i][3] = failedActionMessage(awstbxaws.FormatUserError(createErr))
			continue
		}

		if groupID == "" {
			if requestedGroup == "" {
				rows[i][3] = "created"
			} else {
				rows[i][3] = "created-without-group"
			}
			continue
		}

		_, membershipErr := identityStoreClient.CreateGroupMembership(ctx, &identitystore.CreateGroupMembershipInput{
			IdentityStoreId: ptr(identityStoreID),
			GroupId:         ptr(groupID),
			MemberId:        &identitystoretypes.MemberIdMemberUserId{Value: pointerToString(userOut.UserId)},
		})
		if membershipErr != nil {
			rows[i][3] = "created-user-failed-group:" + awstbxaws.FormatUserError(membershipErr)
			continue
		}

		rows[i][3] = "created"
	}

	return writeDataset(cmd, runtime, []string{"email", "display_name", "group", "action"}, rows)
}

func resolveSSOUserEmails(emailFlags []string, inputFile string) ([]string, error) {
	candidates := make([]string, 0)
	for _, value := range emailFlags {
		candidates = append(candidates, splitEmailCandidates(value)...)
	}

	filePath := strings.TrimSpace(inputFile)
	if filePath != "" {
		content, err := os.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("read --input-file: %w", err)
		}
		candidates = append(candidates, splitEmailCandidates(string(content))...)
	}

	if len(candidates) == 0 {
		return nil, fmt.Errorf("set at least one email via --emails or --input-file")
	}

	seen := make(map[string]struct{})
	emails := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		email := strings.TrimSpace(candidate)
		if email == "" {
			continue
		}
		if !strings.Contains(email, "@") || strings.HasPrefix(email, "@") || strings.HasSuffix(email, "@") {
			return nil, fmt.Errorf("invalid email %q", email)
		}

		key := strings.ToLower(email)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		emails = append(emails, email)
	}

	if len(emails) == 0 {
		return nil, fmt.Errorf("no valid emails found")
	}

	return emails, nil
}

func splitEmailCandidates(raw string) []string {
	return strings.FieldsFunc(raw, func(r rune) bool {
		switch r {
		case '\n', '\r', '\t', ',', ';', ' ':
			return true
		default:
			return false
		}
	})
}

func parseNameFromEmail(email string) (string, string) {
	local := email
	if at := strings.Index(email, "@"); at > 0 {
		local = email[:at]
	}

	parts := strings.FieldsFunc(local, func(r rune) bool {
		return r == '.' || r == '_' || r == '-'
	})

	if len(parts) >= 2 {
		return titleCase(parts[0]), titleCase(parts[len(parts)-1])
	}

	return titleCase(local), "User"
}

func titleCase(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "User"
	}

	runes := []rune(strings.ToLower(trimmed))
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}

func resolveIdentityStoreID(ctx context.Context, client iamSSOAdminAPI) (string, error) {
	instances, err := awstbxaws.CollectAllPages(ctx, func(callCtx context.Context, nextToken *string) (awstbxaws.PageResult[ssoadmintypes.InstanceMetadata], error) {
		page, listErr := client.ListInstances(callCtx, &ssoadmin.ListInstancesInput{NextToken: nextToken})
		if listErr != nil {
			return awstbxaws.PageResult[ssoadmintypes.InstanceMetadata]{}, listErr
		}
		return awstbxaws.PageResult[ssoadmintypes.InstanceMetadata]{
			Items:     page.Instances,
			NextToken: page.NextToken,
		}, nil
	})
	if err != nil {
		return "", err
	}

	for _, instance := range instances {
		id := pointerToString(instance.IdentityStoreId)
		if id != "" {
			return id, nil
		}
	}

	return "", fmt.Errorf("no IAM Identity Center instances found")
}

func resolveIdentityStoreGroupID(ctx context.Context, client iamIdentityStoreAPI, identityStoreID, groupName string) (string, error) {
	groups, err := awstbxaws.CollectAllPages(ctx, func(callCtx context.Context, nextToken *string) (awstbxaws.PageResult[identitystoretypes.Group], error) {
		page, listErr := client.ListGroups(callCtx, &identitystore.ListGroupsInput{
			IdentityStoreId: ptr(identityStoreID),
			Filters: []identitystoretypes.Filter{{
				AttributePath:  ptr("DisplayName"),
				AttributeValue: ptr(groupName),
			}},
			NextToken: nextToken,
		})
		if listErr != nil {
			return awstbxaws.PageResult[identitystoretypes.Group]{}, listErr
		}
		return awstbxaws.PageResult[identitystoretypes.Group]{
			Items:     page.Groups,
			NextToken: page.NextToken,
		}, nil
	})
	if err != nil {
		return "", err
	}

	for _, group := range groups {
		if strings.EqualFold(pointerToString(group.DisplayName), groupName) {
			return pointerToString(group.GroupId), nil
		}
	}

	return "", nil
}
