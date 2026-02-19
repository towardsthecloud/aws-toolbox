package cli

import "github.com/spf13/cobra"

func newR53Command() *cobra.Command {
	return newServiceGroupCommand("r53", "Manage Route 53 resources")
}
