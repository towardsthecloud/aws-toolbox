package cli

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
)

var orgAccountIDPattern = regexp.MustCompile(`^\d{12}$`)

func runOrgListAccounts(cmd *cobra.Command, ouNames []string) error {
	runtime, orgClient, _, _, _, err := orgRuntimeClients(cmd)
	if err != nil {
		return err
	}

	ctx := cmd.Context()
	accountRows := make(map[string][]string)

	if len(ouNames) == 0 {
		accounts, listErr := listOrgAccounts(ctx, orgClient)
		if listErr != nil {
			return fmt.Errorf("list accounts: %s", awstbxaws.FormatUserError(listErr))
		}
		for _, account := range accounts {
			id := pointerToString(account.Id)
			accountRows[id] = []string{id, pointerToString(account.Name), pointerToString(account.Email), string(account.Status), ""}
		}
	} else {
		rootID, _, rootErr := getOrgRoot(ctx, orgClient)
		if rootErr != nil {
			return fmt.Errorf("resolve organization root: %s", awstbxaws.FormatUserError(rootErr))
		}
		for _, ouName := range ouNames {
			ou, ouErr := findOUByName(ctx, orgClient, rootID, ouName)
			if ouErr != nil {
				return ouErr
			}
			accounts, listErr := listOrgAccountsForParent(ctx, orgClient, pointerToString(ou.Id))
			if listErr != nil {
				return fmt.Errorf("list accounts for OU %q: %s", ouName, awstbxaws.FormatUserError(listErr))
			}
			for _, account := range accounts {
				id := pointerToString(account.Id)
				accountRows[id] = []string{id, pointerToString(account.Name), pointerToString(account.Email), string(account.Status), "/" + pointerToString(ou.Name)}
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
	return writeDataset(cmd, runtime, []string{"account_id", "account_name", "email", "status", "parent"}, rows)
}

func runOrgGetAccount(cmd *cobra.Command, accountID string) error {
	if err := validateOrgAccountID(accountID); err != nil {
		return err
	}

	runtime, orgClient, _, _, _, err := orgRuntimeClients(cmd)
	if err != nil {
		return err
	}

	ctx := cmd.Context()
	out, err := orgClient.DescribeAccount(ctx, &organizations.DescribeAccountInput{AccountId: ptr(accountID)})
	if err != nil {
		return fmt.Errorf("describe account %s: %s", accountID, awstbxaws.FormatUserError(err))
	}

	tags, err := orgClient.ListTagsForResource(ctx, &organizations.ListTagsForResourceInput{ResourceId: ptr(accountID)})
	if err != nil {
		return fmt.Errorf("list account tags: %s", awstbxaws.FormatUserError(err))
	}

	rows := [][]string{
		{"account_id", pointerToString(out.Account.Id)},
		{"account_name", pointerToString(out.Account.Name)},
		{"email", pointerToString(out.Account.Email)},
		{"status", string(out.Account.Status)},
		{"arn", pointerToString(out.Account.Arn)},
		{"joined_method", string(out.Account.JoinedMethod)},
		{"joined_timestamp", formatOrgTime(out.Account.JoinedTimestamp)},
	}
	sort.Slice(tags.Tags, func(i, j int) bool {
		return pointerToString(tags.Tags[i].Key) < pointerToString(tags.Tags[j].Key)
	})
	for _, tag := range tags.Tags {
		rows = append(rows, []string{"tag:" + pointerToString(tag.Key), pointerToString(tag.Value)})
	}

	return writeDataset(cmd, runtime, []string{"field", "value"}, rows)
}

func runOrgGenerateDiagram(cmd *cobra.Command, maxAccountsPerOU int) error {
	if maxAccountsPerOU < 1 {
		return fmt.Errorf("--max-accounts-per-ou must be >= 1")
	}

	runtime, orgClient, _, _, _, err := orgRuntimeClients(cmd)
	if err != nil {
		return err
	}
	_ = runtime

	ctx := cmd.Context()
	rootID, rootName, err := getOrgRoot(ctx, orgClient)
	if err != nil {
		return fmt.Errorf("resolve organization root: %s", awstbxaws.FormatUserError(err))
	}

	lines := []string{"graph TB"}
	rootNode := orgMermaidID(rootID)
	lines = append(lines, fmt.Sprintf("    %s[\"%s (Root)\"]", rootNode, orgMermaidEscape(rootName)))

	appendLines, err := buildOrgMermaid(ctx, orgClient, rootID, rootNode, maxAccountsPerOU)
	if err != nil {
		return fmt.Errorf("build diagram: %s", awstbxaws.FormatUserError(err))
	}
	lines = append(lines, appendLines...)

	_, err = fmt.Fprintln(cmd.OutOrStdout(), strings.Join(lines, "\n"))
	return err
}

func validateOrgAccountID(accountID string) error {
	if !orgAccountIDPattern.MatchString(strings.TrimSpace(accountID)) {
		return fmt.Errorf("--account-id must be a 12-digit AWS account ID")
	}
	return nil
}

func getOrgRoot(ctx context.Context, orgClient organizationsAPI) (string, string, error) {
	out, err := orgClient.ListRoots(ctx, &organizations.ListRootsInput{})
	if err != nil {
		return "", "", err
	}
	if len(out.Roots) == 0 {
		return "", "", fmt.Errorf("no organization roots found")
	}
	name := pointerToString(out.Roots[0].Name)
	if name == "" {
		name = "Organization"
	}
	return pointerToString(out.Roots[0].Id), name, nil
}

func listOrgAccounts(ctx context.Context, orgClient organizationsAPI) ([]organizationtypes.Account, error) {
	rows := make([]organizationtypes.Account, 0)
	var nextToken *string
	for {
		out, err := orgClient.ListAccounts(ctx, &organizations.ListAccountsInput{NextToken: nextToken})
		if err != nil {
			return nil, err
		}
		rows = append(rows, out.Accounts...)
		if out.NextToken == nil || *out.NextToken == "" {
			break
		}
		nextToken = out.NextToken
	}
	return rows, nil
}

func listOrgAccountsForParent(ctx context.Context, orgClient organizationsAPI, parentID string) ([]organizationtypes.Account, error) {
	rows := make([]organizationtypes.Account, 0)
	var nextToken *string
	for {
		out, err := orgClient.ListAccountsForParent(ctx, &organizations.ListAccountsForParentInput{ParentId: ptr(parentID), NextToken: nextToken})
		if err != nil {
			return nil, err
		}
		rows = append(rows, out.Accounts...)
		if out.NextToken == nil || *out.NextToken == "" {
			break
		}
		nextToken = out.NextToken
	}
	return rows, nil
}

func listOrgOUsForParent(ctx context.Context, orgClient organizationsAPI, parentID string) ([]organizationtypes.OrganizationalUnit, error) {
	rows := make([]organizationtypes.OrganizationalUnit, 0)
	var nextToken *string
	for {
		out, err := orgClient.ListOrganizationalUnitsForParent(ctx, &organizations.ListOrganizationalUnitsForParentInput{ParentId: ptr(parentID), NextToken: nextToken})
		if err != nil {
			return nil, err
		}
		rows = append(rows, out.OrganizationalUnits...)
		if out.NextToken == nil || *out.NextToken == "" {
			break
		}
		nextToken = out.NextToken
	}
	return rows, nil
}

func findOUByName(ctx context.Context, orgClient organizationsAPI, rootID, ouName string) (organizationtypes.OrganizationalUnit, error) {
	targetName := strings.TrimSpace(ouName)
	if targetName == "" {
		return organizationtypes.OrganizationalUnit{}, fmt.Errorf("organizational unit not found: %s", ouName)
	}

	queue := []string{rootID}
	for len(queue) > 0 {
		parentID := queue[0]
		queue = queue[1:]

		ous, err := listOrgOUsForParent(ctx, orgClient, parentID)
		if err != nil {
			return organizationtypes.OrganizationalUnit{}, err
		}
		for _, ou := range ous {
			if strings.EqualFold(pointerToString(ou.Name), targetName) {
				return ou, nil
			}
			if id := pointerToString(ou.Id); id != "" {
				queue = append(queue, id)
			}
		}
	}

	return organizationtypes.OrganizationalUnit{}, fmt.Errorf("organizational unit not found: %s", ouName)
}

func listOrgAccountIDsByOU(ctx context.Context, orgClient organizationsAPI, ouName string) ([]string, error) {
	rootID, _, err := getOrgRoot(ctx, orgClient)
	if err != nil {
		return nil, fmt.Errorf("resolve organization root: %s", awstbxaws.FormatUserError(err))
	}
	ou, err := findOUByName(ctx, orgClient, rootID, ouName)
	if err != nil {
		return nil, err
	}
	accounts, err := listOrgAccountsForParent(ctx, orgClient, pointerToString(ou.Id))
	if err != nil {
		return nil, fmt.Errorf("list accounts for OU %q: %s", ouName, awstbxaws.FormatUserError(err))
	}
	ids := make([]string, 0, len(accounts))
	for _, account := range accounts {
		if id := pointerToString(account.Id); id != "" {
			ids = append(ids, id)
		}
	}
	return ids, nil
}

func buildOrgMermaid(ctx context.Context, orgClient organizationsAPI, parentID, parentNode string, maxAccountsPerOU int) ([]string, error) {
	lines := make([]string, 0)
	accounts, err := listOrgAccountsForParent(ctx, orgClient, parentID)
	if err != nil {
		return nil, err
	}
	sort.Slice(accounts, func(i, j int) bool { return pointerToString(accounts[i].Name) < pointerToString(accounts[j].Name) })
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
		node := orgMermaidID(pointerToString(acct.Id))
		lines = append(lines, fmt.Sprintf("    %s[\"%s\"]", node, orgMermaidEscape(pointerToString(acct.Name))))
		lines = append(lines, fmt.Sprintf("    %s --> %s", parentNode, node))
	}

	ous, err := listOrgOUsForParent(ctx, orgClient, parentID)
	if err != nil {
		return nil, err
	}
	sort.Slice(ous, func(i, j int) bool { return pointerToString(ous[i].Name) < pointerToString(ous[j].Name) })
	for _, ou := range ous {
		ouID := pointerToString(ou.Id)
		node := orgMermaidID(ouID)
		lines = append(lines, fmt.Sprintf("    %s[\"%s\"]", node, orgMermaidEscape(pointerToString(ou.Name))))
		lines = append(lines, fmt.Sprintf("    %s --> %s", parentNode, node))
		children, childErr := buildOrgMermaid(ctx, orgClient, ouID, node, maxAccountsPerOU)
		if childErr != nil {
			return nil, childErr
		}
		lines = append(lines, children...)
	}
	return lines, nil
}

func orgMermaidID(raw string) string {
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

func orgMermaidEscape(raw string) string {
	return strings.ReplaceAll(raw, "\"", `\\"`)
}

func formatOrgTime(value *time.Time) string {
	if value == nil {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}
