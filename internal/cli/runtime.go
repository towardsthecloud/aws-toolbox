package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/towardsthecloud/aws-toolbox/internal/confirm"
	"github.com/towardsthecloud/aws-toolbox/internal/output"
)

type commandRuntime struct {
	Options   GlobalOptions
	Formatter output.Formatter
	Prompter  confirm.Prompter
}

func newCommandRuntime(cmd *cobra.Command) (commandRuntime, error) {
	opts, err := globalOptionsFromCommand(cmd)
	if err != nil {
		return commandRuntime{}, err
	}

	formatter, err := output.NewFormatter(opts.OutputFormat)
	if err != nil {
		return commandRuntime{}, err
	}

	return commandRuntime{
		Options:   opts,
		Formatter: formatter,
		Prompter:  confirm.NewPrompter(cmd.InOrStdin(), cmd.OutOrStdout()),
	}, nil
}

func globalOptionsFromCommand(cmd *cobra.Command) (GlobalOptions, error) {
	root := cmd.Root()

	profile, err := root.Flags().GetString("profile")
	if err != nil {
		return GlobalOptions{}, fmt.Errorf("read --profile: %w", err)
	}

	region, err := root.Flags().GetString("region")
	if err != nil {
		return GlobalOptions{}, fmt.Errorf("read --region: %w", err)
	}

	dryRun, err := root.Flags().GetBool("dry-run")
	if err != nil {
		return GlobalOptions{}, fmt.Errorf("read --dry-run: %w", err)
	}

	outputFormat, err := root.Flags().GetString("output")
	if err != nil {
		return GlobalOptions{}, fmt.Errorf("read --output: %w", err)
	}

	noConfirm, err := root.Flags().GetBool("no-confirm")
	if err != nil {
		return GlobalOptions{}, fmt.Errorf("read --no-confirm: %w", err)
	}

	showVersion, err := root.Flags().GetBool("version")
	if err != nil {
		return GlobalOptions{}, fmt.Errorf("read --version: %w", err)
	}

	return GlobalOptions{
		Profile:      profile,
		Region:       region,
		DryRun:       dryRun,
		OutputFormat: outputFormat,
		NoConfirm:    noConfirm,
		ShowVersion:  showVersion,
	}, nil
}

func writeDataset(cmd *cobra.Command, runtime commandRuntime, headers []string, rows [][]string) error {
	return runtime.Formatter.Format(cmd.OutOrStdout(), output.Dataset{Headers: headers, Rows: rows})
}
