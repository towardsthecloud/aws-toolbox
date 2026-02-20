package cliutil

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// NewServiceGroupCommand creates a parent command for a service that just shows help.
func NewServiceGroupCommand(use, short string) *cobra.Command {
	return &cobra.Command{
		Use:   use,
		Short: short,
		Long:  fmt.Sprintf("Commands for %s operations.", strings.ToUpper(use)),
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
		SilenceUsage: true,
	}
}
