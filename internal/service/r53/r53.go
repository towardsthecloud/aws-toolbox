package r53

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
	"github.com/towardsthecloud/aws-toolbox/internal/cliutil"
)

// API is the subset of the Route 53 client used by this package.
type API interface {
	ChangeTagsForResource(context.Context, *route53.ChangeTagsForResourceInput, ...func(*route53.Options)) (*route53.ChangeTagsForResourceOutput, error)
	CreateHealthCheck(context.Context, *route53.CreateHealthCheckInput, ...func(*route53.Options)) (*route53.CreateHealthCheckOutput, error)
}

var loadAWSConfig = awstbxaws.LoadAWSConfig
var newClient = func(cfg awssdk.Config) API {
	return route53.NewFromConfig(cfg)
}

// NewCommand returns the r53 service group command.
func NewCommand() *cobra.Command {
	cmd := cliutil.NewServiceGroupCommand("r53", "Manage Route 53 resources")
	cmd.AddCommand(newCreateHealthChecksCommand())

	return cmd
}

func newCreateHealthChecksCommand() *cobra.Command {
	var domains []string

	cmd := &cobra.Command{
		Use:   "create-health-checks",
		Short: "Create and tag Route 53 health checks for domains",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runCreateHealthChecks(cmd, domains)
		},
		SilenceUsage: true,
	}
	cmd.Flags().StringSliceVar(&domains, "domains", nil, "Comma-separated domain list to create health checks for")

	return cmd
}

func runCreateHealthChecks(cmd *cobra.Command, rawDomains []string) error {
	domains := normalizeDomains(rawDomains)
	if len(domains) == 0 {
		return fmt.Errorf("--domains is required")
	}

	runtime, _, client, err := cliutil.NewServiceRuntime(cmd, loadAWSConfig, newClient)
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
		return cliutil.WriteDataset(cmd, runtime, []string{"domain", "health_check_id", "action"}, rows)
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
		return cliutil.WriteDataset(cmd, runtime, []string{"domain", "health_check_id", "action"}, rows)
	}

	for i, domain := range domains {
		callerReference := fmt.Sprintf("awstbx-%d-%s", time.Now().UTC().UnixNano(), strings.ReplaceAll(domain, ".", "-"))
		createResp, createErr := client.CreateHealthCheck(cmd.Context(), &route53.CreateHealthCheckInput{
			CallerReference: cliutil.Ptr(callerReference),
			HealthCheckConfig: &route53types.HealthCheckConfig{
				Type:                     route53types.HealthCheckTypeHttps,
				FullyQualifiedDomainName: cliutil.Ptr(domain),
				Port:                     cliutil.Ptr(int32(443)),
				ResourcePath:             cliutil.Ptr("/"),
				RequestInterval:          cliutil.Ptr(int32(30)),
				FailureThreshold:         cliutil.Ptr(int32(3)),
				EnableSNI:                cliutil.Ptr(true),
			},
		})
		if createErr != nil {
			rows[i][2] = cliutil.FailedActionMessage(awstbxaws.FormatUserError(createErr))
			continue
		}

		healthCheckID := strings.TrimSpace(cliutil.PointerToString(createResp.HealthCheck.Id))
		rows[i][1] = healthCheckID

		_, tagErr := client.ChangeTagsForResource(cmd.Context(), &route53.ChangeTagsForResourceInput{
			ResourceType: route53types.TagResourceTypeHealthcheck,
			ResourceId:   cliutil.Ptr(healthCheckID),
			AddTags: []route53types.Tag{
				{Key: cliutil.Ptr("Name"), Value: cliutil.Ptr(domain)},
				{Key: cliutil.Ptr("ManagedBy"), Value: cliutil.Ptr("awstbx")},
			},
		})
		if tagErr != nil {
			rows[i][2] = cliutil.FailedActionMessage("tagging health check: " + awstbxaws.FormatUserError(tagErr))
			continue
		}

		rows[i][2] = "created"
	}

	return cliutil.WriteDataset(cmd, runtime, []string{"domain", "health_check_id", "action"}, rows)
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
