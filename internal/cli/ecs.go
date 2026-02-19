package cli

import "github.com/spf13/cobra"

func newECSCommand() *cobra.Command {
	return newServiceGroupCommand("ecs", "Manage ECS resources")
}
