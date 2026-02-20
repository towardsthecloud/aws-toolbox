package org

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/organizations"
	organizationtypes "github.com/aws/aws-sdk-go-v2/service/organizations/types"
	"github.com/spf13/cobra"
	awstbxaws "github.com/towardsthecloud/aws-toolbox/internal/aws"
	"github.com/towardsthecloud/aws-toolbox/internal/cliutil"
)

var orgAccountIDPattern = regexp.MustCompile(`^\d{12}$`)

func runListAccounts(cmd *cobra.Command, ouNames []string) error {
	runtime, orgClient, _, _, _, err := runtimeClients(cmd)
	if err != nil {
		return err
	}

	ctx := cmd.Context()
	accountRows := make(map[string][]string)

	if len(ouNames) == 0 {
		accounts, listErr := listAccounts(ctx, orgClient)
		if listErr != nil {
			return fmt.Errorf("list accounts: %s", awstbxaws.FormatUserError(listErr))
		}
		for _, account := range accounts {
			id := cliutil.PointerToString(account.Id)
			accountRows[id] = []string{id, cliutil.PointerToString(account.Name), cliutil.PointerToString(account.Email), string(account.Status), ""}
		}
	} else {
		rootID, _, rootErr := getRoot(ctx, orgClient)
		if rootErr != nil {
			return fmt.Errorf("resolve organization root: %s", awstbxaws.FormatUserError(rootErr))
		}
		for _, ouName := range ouNames {
			ou, ouErr := findOUByName(ctx, orgClient, rootID, ouName)
			if ouErr != nil {
				return ouErr
			}
			accounts, listErr := listAccountsForParent(ctx, orgClient, cliutil.PointerToString(ou.Id))
			if listErr != nil {
				return fmt.Errorf("list accounts for OU %q: %s", ouName, awstbxaws.FormatUserError(listErr))
			}
			for _, account := range accounts {
				id := cliutil.PointerToString(account.Id)
				accountRows[id] = []string{id, cliutil.PointerToString(account.Name), cliutil.PointerToString(account.Email), string(account.Status), "/" + cliutil.PointerToString(ou.Name)}
			}
		}
	}

	ids := make([]string, 0, len(accountRows))
	for id := range accountRows {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	rows := make([][]string, 0, len(ids))
	for _, id := range ids {
		rows = append(rows, accountRows[id])
	}
	return cliutil.WriteDataset(cmd, runtime, []string{"account_id", "account_name", "email", "status", "parent"}, rows)
}

func runGetAccount(cmd *cobra.Command, accountID string) error {
	if err := validateAccountID(accountID); err != nil {
		return err
	}

	runtime, orgClient, _, _, _, err := runtimeClients(cmd)
	if err != nil {
		return err
	}

	ctx := cmd.Context()
	out, err := orgClient.DescribeAccount(ctx, &organizations.DescribeAccountInput{AccountId: cliutil.Ptr(accountID)})
	if err != nil {
		return fmt.Errorf("describe account %s: %s", accountID, awstbxaws.FormatUserError(err))
	}

	tags, err := orgClient.ListTagsForResource(ctx, &organizations.ListTagsForResourceInput{ResourceId: cliutil.Ptr(accountID)})
	if err != nil {
		return fmt.Errorf("list account tags: %s", awstbxaws.FormatUserError(err))
	}

	rows := [][]string{
		{"account_id", cliutil.PointerToString(out.Account.Id)},
		{"account_name", cliutil.PointerToString(out.Account.Name)},
		{"email", cliutil.PointerToString(out.Account.Email)},
		{"status", string(out.Account.Status)},
		{"arn", cliutil.PointerToString(out.Account.Arn)},
		{"joined_method", string(out.Account.JoinedMethod)},
		{"joined_timestamp", formatTime(out.Account.JoinedTimestamp)},
	}
	sort.Slice(tags.Tags, func(i, j int) bool {
		return cliutil.PointerToString(tags.Tags[i].Key) < cliutil.PointerToString(tags.Tags[j].Key)
	})
	for _, tag := range tags.Tags {
		rows = append(rows, []string{"tag:" + cliutil.PointerToString(tag.Key), cliutil.PointerToString(tag.Value)})
	}

	return cliutil.WriteDataset(cmd, runtime, []string{"field", "value"}, rows)
}

func runGenerateDiagram(cmd *cobra.Command, maxAccountsPerOU int) error {
	if maxAccountsPerOU < 1 {
		return fmt.Errorf("--max-accounts-per-ou must be >= 1")
	}

	runtime, orgClient, _, _, _, err := runtimeClients(cmd)
	if err != nil {
		return err
	}
	_ = runtime

	ctx := cmd.Context()
	rootID, rootName, err := getRoot(ctx, orgClient)
	if err != nil {
		return fmt.Errorf("resolve organization root: %s", awstbxaws.FormatUserError(err))
	}

	lines := []string{"graph TB"}
	rootNode := mermaidID(rootID)
	lines = append(lines, fmt.Sprintf("    %s[\"%s (Root)\"]", rootNode, mermaidEscape(rootName)))

	appendLines, err := buildMermaid(ctx, orgClient, rootID, rootNode, maxAccountsPerOU)
	if err != nil {
		return fmt.Errorf("build diagram: %s", awstbxaws.FormatUserError(err))
	}
	lines = append(lines, appendLines...)

	_, err = fmt.Fprintln(cmd.OutOrStdout(), strings.Join(lines, "\n"))
	return err
}

func validateAccountID(accountID string) error {
	if !orgAccountIDPattern.MatchString(strings.TrimSpace(accountID)) {
		return fmt.Errorf("--account-id must be a 12-digit AWS account ID")
	}
	return nil
}

func getRoot(ctx context.Context, orgClient OrganizationsAPI) (string, string, error) {
	out, err := orgClient.ListRoots(ctx, &organizations.ListRootsInput{})
	if err != nil {
		return "", "", err
	}
	if len(out.Roots) == 0 {
		return "", "", fmt.Errorf("no organization roots found")
	}
	name := cliutil.PointerToString(out.Roots[0].Name)
	if name == "" {
		name = "Organization"
	}
	return cliutil.PointerToString(out.Roots[0].Id), name, nil
}

func listAccounts(ctx context.Context, orgClient OrganizationsAPI) ([]organizationtypes.Account, error) {
	return awstbxaws.CollectAllPages(ctx, func(callCtx context.Context, nextToken *string) (awstbxaws.PageResult[organizationtypes.Account], error) {
		out, err := orgClient.ListAccounts(callCtx, &organizations.ListAccountsInput{NextToken: nextToken})
		if err != nil {
			return awstbxaws.PageResult[organizationtypes.Account]{}, err
		}
		return awstbxaws.PageResult[organizationtypes.Account]{
			Items:     out.Accounts,
			NextToken: out.NextToken,
		}, nil
	})
}

func listAccountsForParent(ctx context.Context, orgClient OrganizationsAPI, parentID string) ([]organizationtypes.Account, error) {
	return awstbxaws.CollectAllPages(ctx, func(callCtx context.Context, nextToken *string) (awstbxaws.PageResult[organizationtypes.Account], error) {
		out, err := orgClient.ListAccountsForParent(callCtx, &organizations.ListAccountsForParentInput{ParentId: cliutil.Ptr(parentID), NextToken: nextToken})
		if err != nil {
			return awstbxaws.PageResult[organizationtypes.Account]{}, err
		}
		return awstbxaws.PageResult[organizationtypes.Account]{
			Items:     out.Accounts,
			NextToken: out.NextToken,
		}, nil
	})
}

func listOUsForParent(ctx context.Context, orgClient OrganizationsAPI, parentID string) ([]organizationtypes.OrganizationalUnit, error) {
	return awstbxaws.CollectAllPages(ctx, func(callCtx context.Context, nextToken *string) (awstbxaws.PageResult[organizationtypes.OrganizationalUnit], error) {
		out, err := orgClient.ListOrganizationalUnitsForParent(callCtx, &organizations.ListOrganizationalUnitsForParentInput{ParentId: cliutil.Ptr(parentID), NextToken: nextToken})
		if err != nil {
			return awstbxaws.PageResult[organizationtypes.OrganizationalUnit]{}, err
		}
		return awstbxaws.PageResult[organizationtypes.OrganizationalUnit]{
			Items:     out.OrganizationalUnits,
			NextToken: out.NextToken,
		}, nil
	})
}

func findOUByName(ctx context.Context, orgClient OrganizationsAPI, rootID, ouName string) (organizationtypes.OrganizationalUnit, error) {
	targetName := strings.TrimSpace(ouName)
	if targetName == "" {
		return organizationtypes.OrganizationalUnit{}, fmt.Errorf("organizational unit not found: %s", ouName)
	}

	queue := []string{rootID}
	for len(queue) > 0 {
		parentID := queue[0]
		queue = queue[1:]

		ous, err := listOUsForParent(ctx, orgClient, parentID)
		if err != nil {
			return organizationtypes.OrganizationalUnit{}, err
		}
		for _, ou := range ous {
			if strings.EqualFold(cliutil.PointerToString(ou.Name), targetName) {
				return ou, nil
			}
			if id := cliutil.PointerToString(ou.Id); id != "" {
				queue = append(queue, id)
			}
		}
	}

	return organizationtypes.OrganizationalUnit{}, fmt.Errorf("organizational unit not found: %s", ouName)
}

func listAccountIDsByOU(ctx context.Context, orgClient OrganizationsAPI, ouName string) ([]string, error) {
	rootID, _, err := getRoot(ctx, orgClient)
	if err != nil {
		return nil, fmt.Errorf("resolve organization root: %s", awstbxaws.FormatUserError(err))
	}
	ou, err := findOUByName(ctx, orgClient, rootID, ouName)
	if err != nil {
		return nil, err
	}
	accounts, err := listAccountsForParent(ctx, orgClient, cliutil.PointerToString(ou.Id))
	if err != nil {
		return nil, fmt.Errorf("list accounts for OU %q: %s", ouName, awstbxaws.FormatUserError(err))
	}
	ids := make([]string, 0, len(accounts))
	for _, account := range accounts {
		if id := cliutil.PointerToString(account.Id); id != "" {
			ids = append(ids, id)
		}
	}
	return ids, nil
}

func buildMermaid(ctx context.Context, orgClient OrganizationsAPI, parentID, parentNode string, maxAccountsPerOU int) ([]string, error) {
	lines := make([]string, 0)
	accounts, err := listAccountsForParent(ctx, orgClient, parentID)
	if err != nil {
		return nil, err
	}
	sort.Slice(accounts, func(i, j int) bool {
		return cliutil.PointerToString(accounts[i].Name) < cliutil.PointerToString(accounts[j].Name)
	})
	active := make([]organizationtypes.Account, 0)
	for _, acct := range accounts {
		if acct.Status == organizationtypes.AccountStatusActive {
			active = append(active, acct)
		}
	}
	if len(active) > maxAccountsPerOU {
		active = active[:maxAccountsPerOU]
	}
	for _, acct := range active {
		node := mermaidID(cliutil.PointerToString(acct.Id))
		lines = append(lines, fmt.Sprintf("    %s[\"%s\"]", node, mermaidEscape(cliutil.PointerToString(acct.Name))))
		lines = append(lines, fmt.Sprintf("    %s --> %s", parentNode, node))
	}

	ous, err := listOUsForParent(ctx, orgClient, parentID)
	if err != nil {
		return nil, err
	}
	sort.Slice(ous, func(i, j int) bool {
		return cliutil.PointerToString(ous[i].Name) < cliutil.PointerToString(ous[j].Name)
	})
	for _, ou := range ous {
		ouID := cliutil.PointerToString(ou.Id)
		node := mermaidID(ouID)
		lines = append(lines, fmt.Sprintf("    %s[\"%s\"]", node, mermaidEscape(cliutil.PointerToString(ou.Name))))
		lines = append(lines, fmt.Sprintf("    %s --> %s", parentNode, node))
		children, childErr := buildMermaid(ctx, orgClient, ouID, node, maxAccountsPerOU)
		if childErr != nil {
			return nil, childErr
		}
		lines = append(lines, children...)
	}
	return lines, nil
}

func mermaidID(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "node"
	}
	var b strings.Builder
	for _, r := range raw {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	out := b.String()
	if out == "" {
		out = "node"
	}
	if out[0] >= '0' && out[0] <= '9' {
		out = "n_" + out
	}
	return out
}

func mermaidEscape(raw string) string {
	return strings.ReplaceAll(raw, "\"", `\\"`)
}

func formatTime(value *time.Time) string {
	if value == nil {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}
