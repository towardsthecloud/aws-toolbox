package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/towardsthecloud/aws-toolbox/internal/cliutil"
	"github.com/towardsthecloud/aws-toolbox/internal/service/appstream"
	"github.com/towardsthecloud/aws-toolbox/internal/service/cloudformation"
	"github.com/towardsthecloud/aws-toolbox/internal/service/cloudwatch"
	"github.com/towardsthecloud/aws-toolbox/internal/service/ec2"
	"github.com/towardsthecloud/aws-toolbox/internal/service/ecs"
	"github.com/towardsthecloud/aws-toolbox/internal/service/efs"
	"github.com/towardsthecloud/aws-toolbox/internal/service/iam"
	"github.com/towardsthecloud/aws-toolbox/internal/service/kms"
	"github.com/towardsthecloud/aws-toolbox/internal/service/org"
	"github.com/towardsthecloud/aws-toolbox/internal/service/r53"
	"github.com/towardsthecloud/aws-toolbox/internal/service/s3"
	"github.com/towardsthecloud/aws-toolbox/internal/service/sagemaker"
	"github.com/towardsthecloud/aws-toolbox/internal/service/ssm"
	"github.com/towardsthecloud/aws-toolbox/internal/version"
)

func Execute() error {
	return NewRootCommand().Execute()
}

func NewRootCommand() *cobra.Command {
	opts := &cliutil.GlobalOptions{}

	rootCmd := &cobra.Command{
		Use:   "awstbx",
		Short: "Unified CLI for AWS infrastructure automation",
		Long:  "awstbx unifies AWS automation commands behind a consistent CLI and safety defaults.",
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			if _, ok := cliutil.ValidOutputFormats[opts.OutputFormat]; !ok {
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

	rootCmd.AddCommand(appstream.NewCommand())
	rootCmd.AddCommand(cloudformation.NewCommand())
	rootCmd.AddCommand(cloudwatch.NewCommand())
	rootCmd.AddCommand(ec2.NewCommand())
	rootCmd.AddCommand(ecs.NewCommand())
	rootCmd.AddCommand(efs.NewCommand())
	rootCmd.AddCommand(iam.NewCommand())
	rootCmd.AddCommand(kms.NewCommand())
	rootCmd.AddCommand(org.NewCommand())
	rootCmd.AddCommand(r53.NewCommand())
	rootCmd.AddCommand(s3.NewCommand())
	rootCmd.AddCommand(sagemaker.NewCommand())
	rootCmd.AddCommand(ssm.NewCommand())

	applyCommandHelpDefaults(rootCmd)

	return rootCmd
}
