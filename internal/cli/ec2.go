package cli

import "github.com/spf13/cobra"

func newEC2Command() *cobra.Command {
	return newServiceGroupCommand("ec2", "Manage EC2 resources")
}
