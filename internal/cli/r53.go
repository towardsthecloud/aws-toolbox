package cli

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	route53types "github.com/aws/aws-sdk-go-v2/service/route53/types"
	"github.com/spf13/cobra"
	awstbxaws "github.com/towardsthecloud/aws-toolbox/internal/aws"
)

type r53API interface {
	ChangeTagsForResource(context.Context, *route53.ChangeTagsForResourceInput, ...func(*route53.Options)) (*route53.ChangeTagsForResourceOutput, error)
	CreateHealthCheck(context.Context, *route53.CreateHealthCheckInput, ...func(*route53.Options)) (*route53.CreateHealthCheckOutput, error)
}

var r53LoadAWSConfig = awstbxaws.LoadAWSConfig
var r53NewClient = func(cfg awssdk.Config) r53API {
	return route53.NewFromConfig(cfg)
}

func newR53Command() *cobra.Command {
	cmd := newServiceGroupCommand("r53", "Manage Route 53 resources")
	cmd.AddCommand(newR53CreateHealthChecksCommand())

	return cmd
}

func newR53CreateHealthChecksCommand() *cobra.Command {
	var domains []string

	cmd := &cobra.Command{
		Use:   "create-health-checks",
		Short: "Create and tag Route 53 health checks for domains",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runR53CreateHealthChecks(cmd, domains)
		},
		SilenceUsage: true,
	}
	cmd.Flags().StringSliceVar(&domains, "domains", nil, "Comma-separated domain list to create health checks for")

	return cmd
}

func runR53CreateHealthChecks(cmd *cobra.Command, rawDomains []string) error {
	domains := normalizeDomains(rawDomains)
	if len(domains) == 0 {
		return fmt.Errorf("--domains is required")
	}

	runtime, _, client, err := newServiceRuntime(cmd, r53LoadAWSConfig, r53NewClient)
	if err != nil {
		return err
	}

	rows := make([][]string, 0, len(domains))
	for _, domain := range domains {
		action := "would-create"
		if !runtime.Options.DryRun {
			action = "pending"
		}
		rows = append(rows, []string{domain, "", action})
	}

	if runtime.Options.DryRun {
		return writeDataset(cmd, runtime, []string{"domain", "health_check_id", "action"}, rows)
	}

	ok, err := runtime.Prompter.Confirm(
		fmt.Sprintf("Create Route 53 health checks for %d domain(s)", len(domains)),
		runtime.Options.NoConfirm,
	)
	if err != nil {
		return err
	}
	if !ok {
		for i := range rows {
			rows[i][2] = "cancelled"
		}
		return writeDataset(cmd, runtime, []string{"domain", "health_check_id", "action"}, rows)
	}

	for i, domain := range domains {
		callerReference := fmt.Sprintf("awstbx-%d-%s", time.Now().UTC().UnixNano(), strings.ReplaceAll(domain, ".", "-"))
		createResp, createErr := client.CreateHealthCheck(cmd.Context(), &route53.CreateHealthCheckInput{
			CallerReference: ptr(callerReference),
			HealthCheckConfig: &route53types.HealthCheckConfig{
				Type:                     route53types.HealthCheckTypeHttps,
				FullyQualifiedDomainName: ptr(domain),
				Port:                     ptr(int32(443)),
				ResourcePath:             ptr("/"),
				RequestInterval:          ptr(int32(30)),
				FailureThreshold:         ptr(int32(3)),
				EnableSNI:                ptr(true),
			},
		})
		if createErr != nil {
			rows[i][2] = failedActionMessage(awstbxaws.FormatUserError(createErr))
			continue
		}

		healthCheckID := strings.TrimSpace(pointerToString(createResp.HealthCheck.Id))
		rows[i][1] = healthCheckID

		_, tagErr := client.ChangeTagsForResource(cmd.Context(), &route53.ChangeTagsForResourceInput{
			ResourceType: route53types.TagResourceTypeHealthcheck,
			ResourceId:   ptr(healthCheckID),
			AddTags: []route53types.Tag{
				{Key: ptr("Name"), Value: ptr(domain)},
				{Key: ptr("ManagedBy"), Value: ptr("awstbx")},
			},
		})
		if tagErr != nil {
			rows[i][2] = failedActionMessage("tagging health check: " + awstbxaws.FormatUserError(tagErr))
			continue
		}

		rows[i][2] = "created"
	}

	return writeDataset(cmd, runtime, []string{"domain", "health_check_id", "action"}, rows)
}

func normalizeDomains(raw []string) []string {
	seen := make(map[string]struct{})
	normalized := make([]string, 0, len(raw))

	for _, entry := range raw {
		for _, part := range strings.Split(entry, ",") {
			domain := strings.ToLower(strings.TrimSpace(part))
			if domain == "" {
				continue
			}
			if _, exists := seen[domain]; exists {
				continue
			}
			seen[domain] = struct{}{}
			normalized = append(normalized, domain)
		}
	}

	sort.Strings(normalized)
	return normalized
}
