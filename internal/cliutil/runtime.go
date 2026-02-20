package cliutil

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/towardsthecloud/aws-toolbox/internal/confirm"
	"github.com/towardsthecloud/aws-toolbox/internal/output"
)

// GlobalOptions holds the persistent flags shared by all commands.
type GlobalOptions struct {
	Profile      string
	Region       string
	DryRun       bool
	OutputFormat string
	NoConfirm    bool
	ShowVersion  bool
}

// ValidOutputFormats enumerates the allowed --output values.
var ValidOutputFormats = map[string]struct{}{
	"table": {},
	"json":  {},
	"text":  {},
}

// CommandRuntime bundles the parsed options, formatter, and prompter for a single command invocation.
type CommandRuntime struct {
	Options   GlobalOptions
	Formatter output.Formatter
	Prompter  confirm.Prompter
}

// NewCommandRuntime extracts global options from the cobra command and builds a CommandRuntime.
func NewCommandRuntime(cmd *cobra.Command) (CommandRuntime, error) {
	opts, err := GlobalOptionsFromCommand(cmd)
	if err != nil {
		return CommandRuntime{}, err
	}

	formatter, err := output.NewFormatter(opts.OutputFormat)
	if err != nil {
		return CommandRuntime{}, err
	}

	return CommandRuntime{
		Options:   opts,
		Formatter: formatter,
		Prompter:  confirm.NewPrompter(cmd.InOrStdin(), cmd.OutOrStdout()),
	}, nil
}

// GlobalOptionsFromCommand reads persistent flags from the root command.
func GlobalOptionsFromCommand(cmd *cobra.Command) (GlobalOptions, error) {
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

// WriteDataset formats a tabular dataset to the command's output.
func WriteDataset(cmd *cobra.Command, runtime CommandRuntime, headers []string, rows [][]string) error {
	return runtime.Formatter.Format(cmd.OutOrStdout(), output.Dataset{Headers: headers, Rows: rows})
}
