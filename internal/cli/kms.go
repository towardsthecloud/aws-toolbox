package cli

import "github.com/spf13/cobra"

func newKMSCommand() *cobra.Command {
	return newServiceGroupCommand("kms", "Manage KMS resources")
}
