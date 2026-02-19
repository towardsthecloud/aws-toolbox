package cli

import "github.com/spf13/cobra"

func newCloudWatchCommand() *cobra.Command {
	return newServiceGroupCommand("cloudwatch", "Manage CloudWatch resources")
}
