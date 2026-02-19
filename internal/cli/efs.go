package cli

import "github.com/spf13/cobra"

func newEFSCommand() *cobra.Command {
	return newServiceGroupCommand("efs", "Manage EFS resources")
}
