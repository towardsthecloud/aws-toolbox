package cliutil

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

const (
	ActionWouldDelete = "would-delete"
	ActionPending     = "pending"
	ActionDeleted     = "deleted"
	ActionCancelled   = "cancelled"
)

// DestructiveActionPlan describes a set of rows that may be mutated, with a
// confirmation prompt and an Execute callback per row.
type DestructiveActionPlan struct {
	Headers       []string
	Rows          [][]string
	ActionColumn  int
	ConfirmPrompt string
	Execute       func(rowIndex int) string
}

// RunDestructiveActionPlan implements the 3-phase safety pattern:
// empty/dry-run/confirm+execute.
func RunDestructiveActionPlan(cmd *cobra.Command, runtime CommandRuntime, plan DestructiveActionPlan) error {
	if len(plan.Rows) == 0 {
		return WriteDataset(cmd, runtime, plan.Headers, plan.Rows)
	}

	if runtime.Options.DryRun {
		return WriteDataset(cmd, runtime, plan.Headers, plan.Rows)
	}

	ok, err := runtime.Prompter.Confirm(plan.ConfirmPrompt, runtime.Options.NoConfirm)
	if err != nil {
		return err
	}
	if !ok {
		SetActionForAllRows(plan.Rows, plan.ActionColumn, ActionCancelled)
		return WriteDataset(cmd, runtime, plan.Headers, plan.Rows)
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

	return WriteDataset(cmd, runtime, plan.Headers, plan.Rows)
}

// SetActionForAllRows sets the action column to the given value for every row.
func SetActionForAllRows(rows [][]string, actionColumn int, action string) {
	for i := range rows {
		rows[i][actionColumn] = action
	}
}

// SetActionForRow sets the action column for a single row, with bounds checking.
func SetActionForRow(rows [][]string, rowIndex, actionColumn int, action string) error {
	if rowIndex < 0 || rowIndex >= len(rows) {
		return fmt.Errorf("row index out of bounds: %d", rowIndex)
	}
	rows[rowIndex][actionColumn] = action
	return nil
}

// FailedAction formats a failure string from an error.
func FailedAction(err error) string {
	if err == nil {
		return "failed:unknown"
	}
	return FailedActionMessage(err.Error())
}

// FailedActionMessage formats a failure string from a reason.
func FailedActionMessage(reason string) string {
	clean := strings.TrimSpace(reason)
	if clean == "" {
		clean = "unknown"
	}
	return "failed:" + clean
}

// SkippedActionMessage formats a skip string from a reason.
func SkippedActionMessage(reason string) string {
	clean := strings.TrimSpace(reason)
	if clean == "" {
		clean = "skipped"
	}
	return "skipped:" + clean
}
