package cliutil

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestUtilityHelpers(t *testing.T) {
	if got := FailedAction(nil); !strings.HasPrefix(got, "failed:") {
		t.Fatalf("expected failed action value, got %q", got)
	}
	if got := SkippedActionMessage(""); got != "skipped:skipped" {
		t.Fatalf("unexpected skipped action default: %q", got)
	}
	if got := FailedActionMessage(""); got != "failed:unknown" {
		t.Fatalf("unexpected failed action default: %q", got)
	}

	rows := [][]string{{"x", "pending"}}
	if err := SetActionForRow(rows, 0, 1, ActionDeleted); err != nil {
		t.Fatalf("SetActionForRow success: %v", err)
	}
	if rows[0][1] != ActionDeleted {
		t.Fatalf("unexpected row state: %#v", rows)
	}
	if err := SetActionForRow(rows, 3, 1, ActionDeleted); err == nil {
		t.Fatal("expected out-of-bounds error")
	}
}

func TestFailedActionWithError(t *testing.T) {
	got := FailedAction(errors.New("boom"))
	if got != "failed:boom" {
		t.Fatalf("expected 'failed:boom', got %q", got)
	}
}

func TestFailedActionMessageWithReason(t *testing.T) {
	got := FailedActionMessage("  access denied  ")
	if got != "failed:access denied" {
		t.Fatalf("expected 'failed:access denied', got %q", got)
	}
}

func TestSkippedActionMessageWithReason(t *testing.T) {
	got := SkippedActionMessage("already exists")
	if got != "skipped:already exists" {
		t.Fatalf("expected 'skipped:already exists', got %q", got)
	}
}

func TestSetActionForAllRows(t *testing.T) {
	rows := [][]string{
		{"a", "pending"},
		{"b", "pending"},
		{"c", "pending"},
	}
	SetActionForAllRows(rows, 1, ActionCancelled)
	for i, row := range rows {
		if row[1] != ActionCancelled {
			t.Fatalf("row %d: expected cancelled, got %q", i, row[1])
		}
	}
}

func TestSetActionForAllRowsEmpty(t *testing.T) {
	// Should not panic on empty slice
	SetActionForAllRows([][]string{}, 0, ActionDeleted)
}

func TestSetActionForRowNegativeIndex(t *testing.T) {
	rows := [][]string{{"x", "pending"}}
	if err := SetActionForRow(rows, -1, 1, ActionDeleted); err == nil {
		t.Fatal("expected error for negative index")
	}
}

func newTestRuntimeCmd(t *testing.T, output string) (*cobra.Command, *bytes.Buffer) {
	t.Helper()
	dummy := &cobra.Command{Use: "dummy", RunE: func(*cobra.Command, []string) error { return nil }}
	root := NewTestRootCommand(dummy)
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(&bytes.Buffer{})
	root.SetIn(strings.NewReader(""))
	if output != "" {
		if err := root.PersistentFlags().Set("output", output); err != nil {
			t.Fatalf("set output: %v", err)
		}
	}
	return root, buf
}

func TestRunDestructiveActionPlanDryRun(t *testing.T) {
	root, buf := newTestRuntimeCmd(t, "json")
	if err := root.PersistentFlags().Set("dry-run", "true"); err != nil {
		t.Fatalf("set dry-run: %v", err)
	}

	runtime, err := NewCommandRuntime(root)
	if err != nil {
		t.Fatalf("NewCommandRuntime: %v", err)
	}

	plan := DestructiveActionPlan{
		Headers:       []string{"id", "action"},
		Rows:          [][]string{{"item-1", ActionWouldDelete}},
		ActionColumn:  1,
		ConfirmPrompt: "Delete?",
	}

	if err := RunDestructiveActionPlan(root, runtime, plan); err != nil {
		t.Fatalf("RunDestructiveActionPlan: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "would-delete") {
		t.Fatalf("expected would-delete in output: %s", output)
	}
}

func TestRunDestructiveActionPlanEmptyRows(t *testing.T) {
	root, _ := newTestRuntimeCmd(t, "json")

	runtime, err := NewCommandRuntime(root)
	if err != nil {
		t.Fatalf("NewCommandRuntime: %v", err)
	}

	plan := DestructiveActionPlan{
		Headers:       []string{"id", "action"},
		Rows:          [][]string{},
		ActionColumn:  1,
		ConfirmPrompt: "Delete?",
	}

	if err := RunDestructiveActionPlan(root, runtime, plan); err != nil {
		t.Fatalf("RunDestructiveActionPlan: %v", err)
	}
}

func TestRunDestructiveActionPlanNoConfirm(t *testing.T) {
	root, buf := newTestRuntimeCmd(t, "json")
	if err := root.PersistentFlags().Set("no-confirm", "true"); err != nil {
		t.Fatalf("set no-confirm: %v", err)
	}

	runtime, err := NewCommandRuntime(root)
	if err != nil {
		t.Fatalf("NewCommandRuntime: %v", err)
	}

	executed := false
	plan := DestructiveActionPlan{
		Headers:       []string{"id", "action"},
		Rows:          [][]string{{"item-1", ActionPending}},
		ActionColumn:  1,
		ConfirmPrompt: "Delete?",
		Execute: func(rowIndex int) string {
			executed = true
			return ActionDeleted
		},
	}

	if err := RunDestructiveActionPlan(root, runtime, plan); err != nil {
		t.Fatalf("RunDestructiveActionPlan: %v", err)
	}
	if !executed {
		t.Fatal("expected Execute to be called")
	}
	if !strings.Contains(buf.String(), "deleted") {
		t.Fatalf("expected deleted in output: %s", buf.String())
	}
}

func TestRunDestructiveActionPlanUserDeclines(t *testing.T) {
	dummy := &cobra.Command{Use: "dummy", RunE: func(*cobra.Command, []string) error { return nil }}
	root := NewTestRootCommand(dummy)
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(&bytes.Buffer{})
	root.SetIn(strings.NewReader("n\n"))

	if err := root.PersistentFlags().Set("output", "json"); err != nil {
		t.Fatalf("set output: %v", err)
	}

	runtime, err := NewCommandRuntime(root)
	if err != nil {
		t.Fatalf("NewCommandRuntime: %v", err)
	}

	plan := DestructiveActionPlan{
		Headers:       []string{"id", "action"},
		Rows:          [][]string{{"item-1", ActionPending}},
		ActionColumn:  1,
		ConfirmPrompt: "Delete?",
	}

	if err := RunDestructiveActionPlan(root, runtime, plan); err != nil {
		t.Fatalf("RunDestructiveActionPlan: %v", err)
	}
	if !strings.Contains(buf.String(), "cancelled") {
		t.Fatalf("expected cancelled in output: %s", buf.String())
	}
}

func TestRunDestructiveActionPlanExecuteReturnsEmpty(t *testing.T) {
	root, buf := newTestRuntimeCmd(t, "json")
	if err := root.PersistentFlags().Set("no-confirm", "true"); err != nil {
		t.Fatalf("set no-confirm: %v", err)
	}

	runtime, err := NewCommandRuntime(root)
	if err != nil {
		t.Fatalf("NewCommandRuntime: %v", err)
	}

	plan := DestructiveActionPlan{
		Headers:       []string{"id", "action"},
		Rows:          [][]string{{"item-1", ActionPending}},
		ActionColumn:  1,
		ConfirmPrompt: "Delete?",
		Execute: func(rowIndex int) string {
			return "" // empty means no change
		},
	}

	if err := RunDestructiveActionPlan(root, runtime, plan); err != nil {
		t.Fatalf("RunDestructiveActionPlan: %v", err)
	}
	// The action column should remain ActionPending since Execute returned empty
	if !strings.Contains(buf.String(), ActionPending) {
		t.Fatalf("expected pending in output: %s", buf.String())
	}
}

func TestRunDestructiveActionPlanNilExecute(t *testing.T) {
	root, _ := newTestRuntimeCmd(t, "json")
	if err := root.PersistentFlags().Set("no-confirm", "true"); err != nil {
		t.Fatalf("set no-confirm: %v", err)
	}

	runtime, err := NewCommandRuntime(root)
	if err != nil {
		t.Fatalf("NewCommandRuntime: %v", err)
	}

	plan := DestructiveActionPlan{
		Headers:       []string{"id", "action"},
		Rows:          [][]string{{"item-1", ActionPending}},
		ActionColumn:  1,
		ConfirmPrompt: "Delete?",
		Execute:       nil,
	}

	if err := RunDestructiveActionPlan(root, runtime, plan); err != nil {
		t.Fatalf("RunDestructiveActionPlan: %v", err)
	}
}

func TestNewServiceGroupCommand(t *testing.T) {
	cmd := NewServiceGroupCommand("s3", "Manage S3 resources")
	if cmd.Use != "s3" {
		t.Fatalf("expected 's3' use, got %q", cmd.Use)
	}
	if cmd.Short != "Manage S3 resources" {
		t.Fatalf("unexpected short: %q", cmd.Short)
	}
	if !strings.Contains(cmd.Long, "S3") {
		t.Fatalf("expected Long to contain S3: %q", cmd.Long)
	}

	// Test that RunE shows help (doesn't error)
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("RunE: %v", err)
	}
}

func TestNewTestRootCommand(t *testing.T) {
	dummy := &cobra.Command{Use: "child", RunE: func(*cobra.Command, []string) error { return nil }}
	root := NewTestRootCommand(dummy)

	if root.Use != "awstbx" {
		t.Fatalf("expected 'awstbx' use, got %q", root.Use)
	}
	if !root.HasSubCommands() {
		t.Fatal("expected sub-commands")
	}

	// Verify all persistent flags exist
	flags := []string{"profile", "region", "dry-run", "output", "no-confirm", "version"}
	for _, flag := range flags {
		if root.PersistentFlags().Lookup(flag) == nil {
			t.Fatalf("expected persistent flag %q", flag)
		}
	}
}

func TestRunDestructiveActionPlanMultipleRows(t *testing.T) {
	root, buf := newTestRuntimeCmd(t, "json")
	if err := root.PersistentFlags().Set("no-confirm", "true"); err != nil {
		t.Fatalf("set no-confirm: %v", err)
	}

	runtime, err := NewCommandRuntime(root)
	if err != nil {
		t.Fatalf("NewCommandRuntime: %v", err)
	}

	plan := DestructiveActionPlan{
		Headers: []string{"id", "action"},
		Rows: [][]string{
			{"item-1", ActionPending},
			{"item-2", ActionPending},
			{"item-3", ActionPending},
		},
		ActionColumn:  1,
		ConfirmPrompt: "Delete 3 items?",
		Execute: func(rowIndex int) string {
			if rowIndex == 1 {
				return "failed:error"
			}
			return ActionDeleted
		},
	}

	if err := RunDestructiveActionPlan(root, runtime, plan); err != nil {
		t.Fatalf("RunDestructiveActionPlan: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "deleted") {
		t.Fatalf("expected deleted: %s", output)
	}
	if !strings.Contains(output, "failed:error") {
		t.Fatalf("expected failed:error: %s", output)
	}
}
