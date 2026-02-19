package cli

import "github.com/spf13/cobra"

func newS3Command() *cobra.Command {
	return newServiceGroupCommand("s3", "Manage S3 resources")
}
