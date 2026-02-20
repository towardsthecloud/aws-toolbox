package cliutil

import "github.com/spf13/cobra"

// NewTestRootCommand wraps a service command under a minimal root that has all
// persistent flags, suitable for use in service-package tests.
func NewTestRootCommand(serviceCmd *cobra.Command) *cobra.Command {
	root := &cobra.Command{
		Use:          "awstbx",
		SilenceUsage: true,
	}

	root.PersistentFlags().StringP("profile", "p", "", "AWS CLI profile name")
	root.PersistentFlags().StringP("region", "r", "", "AWS region override")
	root.PersistentFlags().Bool("dry-run", false, "Preview changes without executing")
	root.PersistentFlags().StringP("output", "o", "table", "Output format: table, json, text")
	root.PersistentFlags().Bool("no-confirm", false, "Skip confirmation prompts")
	root.PersistentFlags().Bool("version", false, "Print build metadata and exit")

	root.AddCommand(serviceCmd)

	return root
}
