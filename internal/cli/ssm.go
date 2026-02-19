package cli

import "github.com/spf13/cobra"

func newSSMCommand() *cobra.Command {
	return newServiceGroupCommand("ssm", "Manage SSM resources")
}
