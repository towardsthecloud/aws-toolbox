package ssm

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
	"github.com/spf13/cobra"
	awstbxaws "github.com/towardsthecloud/aws-toolbox/internal/aws"
	"github.com/towardsthecloud/aws-toolbox/internal/cliutil"
)

// API is the subset of the SSM client used by this package.
type API interface {
	DeleteParameter(context.Context, *ssm.DeleteParameterInput, ...func(*ssm.Options)) (*ssm.DeleteParameterOutput, error)
	PutParameter(context.Context, *ssm.PutParameterInput, ...func(*ssm.Options)) (*ssm.PutParameterOutput, error)
}

type parameterFileRecord struct {
	Name          string `json:"Name"`
	NameLower     string `json:"name"`
	Type          string `json:"Type"`
	TypeLower     string `json:"type"`
	Value         string `json:"Value"`
	ValueLower    string `json:"value"`
	Overwrite     *bool  `json:"Overwrite"`
	OverwriteLow  *bool  `json:"overwrite"`
	Description   string `json:"Description"`
	DescriptionLo string `json:"description"`
}

type parameterFileEnvelope struct {
	Parameters    []parameterFileRecord `json:"parameters"`
	ParametersCap []parameterFileRecord `json:"Parameters"`
}

type importParameter struct {
	Name        string
	Type        ssmtypes.ParameterType
	Value       string
	Overwrite   bool
	Description string
}

var loadAWSConfig = awstbxaws.LoadAWSConfig
var newClient = func(cfg awssdk.Config) API {
	return ssm.NewFromConfig(cfg)
}

// NewCommand returns the ssm service group command.
func NewCommand() *cobra.Command {
	cmd := cliutil.NewServiceGroupCommand("ssm", "Manage SSM resources")

	cmd.AddCommand(newDeleteParametersCommand())
	cmd.AddCommand(newImportParametersCommand())

	return cmd
}

func newDeleteParametersCommand() *cobra.Command {
	var inputFile string

	cmd := &cobra.Command{
		Use:   "delete-parameters",
		Short: "Delete SSM parameters listed in an input JSON file",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runDeleteParameters(cmd, inputFile)
		},
		SilenceUsage: true,
	}
	cmd.Flags().StringVar(&inputFile, "input-file", "", "Path to a JSON file containing parameter names")

	return cmd
}

func newImportParametersCommand() *cobra.Command {
	var inputFile string

	cmd := &cobra.Command{
		Use:   "import-parameters",
		Short: "Import SSM parameters from a JSON file",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runImportParameters(cmd, inputFile)
		},
		SilenceUsage: true,
	}
	cmd.Flags().StringVar(&inputFile, "input-file", "", "Path to a JSON file containing parameter records")

	return cmd
}

func runDeleteParameters(cmd *cobra.Command, inputFile string) error {
	if strings.TrimSpace(inputFile) == "" {
		return fmt.Errorf("--input-file is required")
	}

	names, err := readParameterNamesFile(inputFile)
	if err != nil {
		return err
	}

	runtime, _, client, err := cliutil.NewServiceRuntime(cmd, loadAWSConfig, newClient)
	if err != nil {
		return err
	}

	sort.Strings(names)
	rows := make([][]string, 0, len(names))
	for _, name := range names {
		action := cliutil.ActionWouldDelete
		if !runtime.Options.DryRun {
			action = cliutil.ActionPending
		}
		rows = append(rows, []string{name, action})
	}

	return cliutil.RunDestructiveActionPlan(cmd, runtime, cliutil.DestructiveActionPlan{
		Headers:       []string{"parameter_name", "action"},
		Rows:          rows,
		ActionColumn:  1,
		ConfirmPrompt: fmt.Sprintf("Delete %d SSM parameter(s)", len(rows)),
		Execute: func(rowIndex int) string {
			_, deleteErr := client.DeleteParameter(cmd.Context(), &ssm.DeleteParameterInput{
				Name: cliutil.Ptr(rows[rowIndex][0]),
			})
			if deleteErr != nil {
				return cliutil.FailedActionMessage(awstbxaws.FormatUserError(deleteErr))
			}
			return cliutil.ActionDeleted
		},
	})
}

func runImportParameters(cmd *cobra.Command, inputFile string) error {
	if strings.TrimSpace(inputFile) == "" {
		return fmt.Errorf("--input-file is required")
	}

	parameters, err := readImportParametersFile(inputFile)
	if err != nil {
		return err
	}

	runtime, _, client, err := cliutil.NewServiceRuntime(cmd, loadAWSConfig, newClient)
	if err != nil {
		return err
	}

	sort.Slice(parameters, func(i, j int) bool {
		return parameters[i].Name < parameters[j].Name
	})

	rows := make([][]string, 0, len(parameters))
	for _, parameter := range parameters {
		action := "would-import"
		if runtime.Options.DryRun {
			rows = append(rows, []string{parameter.Name, string(parameter.Type), fmt.Sprintf("%t", parameter.Overwrite), action})
			continue
		}

		_, putErr := client.PutParameter(cmd.Context(), &ssm.PutParameterInput{
			Name:        cliutil.Ptr(parameter.Name),
			Type:        parameter.Type,
			Value:       cliutil.Ptr(parameter.Value),
			Overwrite:   cliutil.Ptr(parameter.Overwrite),
			Description: cliutil.Ptr(parameter.Description),
		})
		if putErr != nil {
			action = cliutil.FailedActionMessage(awstbxaws.FormatUserError(putErr))
		} else {
			action = "imported"
		}

		rows = append(rows, []string{parameter.Name, string(parameter.Type), fmt.Sprintf("%t", parameter.Overwrite), action})
	}

	return cliutil.WriteDataset(cmd, runtime, []string{"parameter_name", "type", "overwrite", "action"}, rows)
}

func readParameterNamesFile(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read input file: %w", err)
	}

	var rawNames []string
	if err := json.Unmarshal(data, &rawNames); err == nil {
		names := make([]string, 0, len(rawNames))
		for i, name := range rawNames {
			trimmed := strings.TrimSpace(name)
			if trimmed == "" {
				return nil, fmt.Errorf("input file entry %d has an empty name", i+1)
			}
			names = append(names, trimmed)
		}
		return uniqueStrings(names), nil
	}

	records, err := parseParameterRecords(data)
	if err != nil {
		return nil, err
	}

	names := make([]string, 0, len(records))
	for i, record := range records {
		name := strings.TrimSpace(firstNonEmpty(record.Name, record.NameLower))
		if name == "" {
			return nil, fmt.Errorf("input file entry %d is missing Name", i+1)
		}
		names = append(names, name)
	}

	return uniqueStrings(names), nil
}

func readImportParametersFile(path string) ([]importParameter, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read input file: %w", err)
	}

	records, err := parseParameterRecords(data)
	if err != nil {
		return nil, err
	}

	parameters := make([]importParameter, 0, len(records))
	for i, record := range records {
		name := strings.TrimSpace(firstNonEmpty(record.Name, record.NameLower))
		if name == "" {
			return nil, fmt.Errorf("input file entry %d is missing Name", i+1)
		}

		typ, err := parseParameterType(firstNonEmpty(record.Type, record.TypeLower))
		if err != nil {
			return nil, fmt.Errorf("input file entry %d: %w", i+1, err)
		}

		overwrite := false
		if record.Overwrite != nil {
			overwrite = *record.Overwrite
		}
		if record.OverwriteLow != nil {
			overwrite = *record.OverwriteLow
		}

		parameters = append(parameters, importParameter{
			Name:        name,
			Type:        typ,
			Value:       firstNonEmpty(record.Value, record.ValueLower),
			Overwrite:   overwrite,
			Description: firstNonEmpty(record.Description, record.DescriptionLo),
		})
	}

	return parameters, nil
}

func parseParameterRecords(data []byte) ([]parameterFileRecord, error) {
	var records []parameterFileRecord
	if err := json.Unmarshal(data, &records); err == nil {
		return records, nil
	}

	var envelope parameterFileEnvelope
	if err := json.Unmarshal(data, &envelope); err == nil {
		if len(envelope.Parameters) > 0 {
			return envelope.Parameters, nil
		}
		if len(envelope.ParametersCap) > 0 {
			return envelope.ParametersCap, nil
		}
	}

	return nil, fmt.Errorf("parse input file: expected JSON array of parameter objects")
}

func parseParameterType(raw string) (ssmtypes.ParameterType, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ssmtypes.ParameterTypeString, nil
	}

	switch strings.ToLower(value) {
	case strings.ToLower(string(ssmtypes.ParameterTypeString)):
		return ssmtypes.ParameterTypeString, nil
	case strings.ToLower(string(ssmtypes.ParameterTypeStringList)):
		return ssmtypes.ParameterTypeStringList, nil
	case strings.ToLower(string(ssmtypes.ParameterTypeSecureString)):
		return ssmtypes.ParameterTypeSecureString, nil
	default:
		return "", fmt.Errorf("unsupported parameter type %q", raw)
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func uniqueStrings(values []string) []string {
	unique := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		unique = append(unique, value)
	}
	return unique
}
