package cli

import "github.com/spf13/cobra"

func newIAMCommand() *cobra.Command {
	return newServiceGroupCommand("iam", "Manage IAM resources")
}
