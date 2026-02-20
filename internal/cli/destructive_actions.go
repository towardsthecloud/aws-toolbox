package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

const (
	actionWouldDelete = "would-delete"
	actionPending     = "pending"
	actionDeleted     = "deleted"
	actionCancelled   = "cancelled"
)

type destructiveActionPlan struct {
	Headers       []string
	Rows          [][]string
	ActionColumn  int
	ConfirmPrompt string
	Execute       func(rowIndex int) string
}

func runDestructiveActionPlan(cmd *cobra.Command, runtime commandRuntime, plan destructiveActionPlan) error {
	if len(plan.Rows) == 0 {
		return writeDataset(cmd, runtime, plan.Headers, plan.Rows)
	}

	if runtime.Options.DryRun {
		return writeDataset(cmd, runtime, plan.Headers, plan.Rows)
	}

	ok, err := runtime.Prompter.Confirm(plan.ConfirmPrompt, runtime.Options.NoConfirm)
	if err != nil {
		return err
	}
	if !ok {
		setActionForAllRows(plan.Rows, plan.ActionColumn, actionCancelled)
		return writeDataset(cmd, runtime, plan.Headers, plan.Rows)
	}

	if plan.Execute != nil {
		for i := range plan.Rows {
			next := strings.TrimSpace(plan.Execute(i))
			if next == "" {
				continue
			}
			plan.Rows[i][plan.ActionColumn] = next
		}
	}

	return writeDataset(cmd, runtime, plan.Headers, plan.Rows)
}

func setActionForAllRows(rows [][]string, actionColumn int, action string) {
	for i := range rows {
		rows[i][actionColumn] = action
	}
}

func failedAction(err error) string {
	if err == nil {
		return "failed:unknown"
	}
	return failedActionMessage(err.Error())
}

func failedActionMessage(reason string) string {
	clean := strings.TrimSpace(reason)
	if clean == "" {
		clean = "unknown"
	}
	return "failed:" + clean
}

func skippedActionMessage(reason string) string {
	clean := strings.TrimSpace(reason)
	if clean == "" {
		clean = "skipped"
	}
	return "skipped:" + clean
}

func setActionForRow(rows [][]string, rowIndex, actionColumn int, action string) error {
	if rowIndex < 0 || rowIndex >= len(rows) {
		return fmt.Errorf("row index out of bounds: %d", rowIndex)
	}
	rows[rowIndex][actionColumn] = action
	return nil
}
