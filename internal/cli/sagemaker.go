package cli

import "github.com/spf13/cobra"

func newSageMakerCommand() *cobra.Command {
	return newServiceGroupCommand("sagemaker", "Manage SageMaker resources")
}
