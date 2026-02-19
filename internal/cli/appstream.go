package cli

import "github.com/spf13/cobra"

func newAppStreamCommand() *cobra.Command {
	return newServiceGroupCommand("appstream", "Manage AppStream resources")
}
