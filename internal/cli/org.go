package cli

import "github.com/spf13/cobra"

func newOrgCommand() *cobra.Command {
	return newServiceGroupCommand("org", "Manage Organizations resources")
}
