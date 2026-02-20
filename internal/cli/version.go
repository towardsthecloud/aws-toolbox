package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/towardsthecloud/aws-toolbox/internal/version"
)

func newVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print build metadata",
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, err := fmt.Fprintln(cmd.OutOrStdout(), version.Detailed())
			return err
		},
		SilenceUsage: true,
	}
}
