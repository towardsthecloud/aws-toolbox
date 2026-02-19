package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/towardsthecloud/aws-toolbox/internal/version"
)

type GlobalOptions struct {
	Profile      string
	Region       string
	DryRun       bool
	OutputFormat string
	NoConfirm    bool
	ShowVersion  bool
}

var validOutputFormats = map[string]struct{}{
	"table": {},
	"json":  {},
	"text":  {},
}

func Execute() error {
	return NewRootCommand().Execute()
}

func NewRootCommand() *cobra.Command {
	opts := &GlobalOptions{}

	rootCmd := &cobra.Command{
		Use:   "awstbx",
		Short: "Unified CLI for AWS infrastructure automation",
		Long:  "awstbx unifies AWS automation commands behind a consistent CLI and safety defaults.",
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			if _, ok := validOutputFormats[opts.OutputFormat]; !ok {
				return fmt.Errorf("invalid --output %q (valid: table, json, text)", opts.OutputFormat)
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			if opts.ShowVersion {
				if _, err := fmt.Fprintln(cmd.OutOrStdout(), version.Detailed()); err != nil {
					return err
				}
				return nil
			}
			return cmd.Help()
		},
		SilenceUsage: true,
	}

	rootCmd.PersistentFlags().StringVarP(&opts.Profile, "profile", "p", "", "AWS CLI profile name")
	rootCmd.PersistentFlags().StringVarP(&opts.Region, "region", "r", "", "AWS region override")
	rootCmd.PersistentFlags().BoolVar(&opts.DryRun, "dry-run", false, "Preview changes without executing")
	rootCmd.PersistentFlags().StringVarP(&opts.OutputFormat, "output", "o", "table", "Output format: table, json, text")
	rootCmd.PersistentFlags().BoolVar(&opts.NoConfirm, "no-confirm", false, "Skip confirmation prompts")
	rootCmd.PersistentFlags().BoolVar(&opts.ShowVersion, "version", false, "Print build metadata and exit")

	rootCmd.AddCommand(newCompletionCommand())
	rootCmd.AddCommand(newVersionCommand())

	rootCmd.AddCommand(newAppStreamCommand())
	rootCmd.AddCommand(newCFNCommand())
	rootCmd.AddCommand(newCloudWatchCommand())
	rootCmd.AddCommand(newEC2Command())
	rootCmd.AddCommand(newECSCommand())
	rootCmd.AddCommand(newEFSCommand())
	rootCmd.AddCommand(newIAMCommand())
	rootCmd.AddCommand(newKMSCommand())
	rootCmd.AddCommand(newOrgCommand())
	rootCmd.AddCommand(newR53Command())
	rootCmd.AddCommand(newS3Command())
	rootCmd.AddCommand(newSageMakerCommand())
	rootCmd.AddCommand(newSSMCommand())

	return rootCmd
}

func newServiceGroupCommand(use, short string) *cobra.Command {
	return &cobra.Command{
		Use:   use,
		Short: short,
		Long:  fmt.Sprintf("Commands for %s operations.", strings.ToUpper(use)),
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
		SilenceUsage: true,
	}
}
