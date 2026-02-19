package cli

import "github.com/spf13/cobra"

func newCFNCommand() *cobra.Command {
	return newServiceGroupCommand("cfn", "Manage CloudFormation resources")
}
