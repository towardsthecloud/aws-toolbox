package cli

import (
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/towardsthecloud/aws-toolbox/internal/cliutil"
)

func TestCoverageBoostRootAndHelpDefaults(t *testing.T) {
	// Exercise root.Execute wrapper.
	oldArgs := os.Args
	os.Args = []string{"awstbx", "--version"}
	t.Cleanup(func() { os.Args = oldArgs })
	if err := Execute(); err != nil {
		t.Fatalf("Execute --version: %v", err)
	}

	// Exercise default example fallback paths.
	fallbackLeaf := &cobra.Command{Use: "leaf", RunE: func(*cobra.Command, []string) error { return nil }}
	fallbackParent := &cobra.Command{Use: "parent"}
	fallbackParent.AddCommand(fallbackLeaf)
	example := defaultCommandExample(fallbackParent)
	if !strings.Contains(example, "parent --help") {
		t.Fatalf("unexpected fallback example: %s", example)
	}

	// Exercise NewServiceGroupCommand RunE/help path.
	groupCmd := cliutil.NewServiceGroupCommand("demo", "Demo service")
	groupCmd.SetArgs([]string{})
	if err := groupCmd.Execute(); err != nil {
		t.Fatalf("execute service group help: %v", err)
	}
}

func TestCoverageBoostDefaultExamples(t *testing.T) {
	// defaultCommandExample map, subcommand, and fallback cases.
	rootExample := defaultCommandExample(NewRootCommand())
	if !strings.Contains(rootExample, "awstbx ec2 list-eips") {
		t.Fatalf("unexpected root example: %s", rootExample)
	}

	parent := &cobra.Command{Use: "custom"}
	child := &cobra.Command{Use: "child", RunE: func(*cobra.Command, []string) error { return nil }}
	parent.AddCommand(child)
	if got := defaultCommandExample(parent); !strings.Contains(got, "custom --help") {
		t.Fatalf("unexpected subcommand default example: %s", got)
	}

	leaf := &cobra.Command{Use: "leaf", RunE: func(*cobra.Command, []string) error { return nil }}
	if got := defaultCommandExample(leaf); !strings.Contains(got, "leaf --output table") {
		t.Fatalf("unexpected leaf default example: %s", got)
	}
}

func TestCoverageBoostCompletionShellVariants(t *testing.T) {
	for _, shell := range []string{"fish", "powershell"} {
		if _, err := executeCommand(t, "completion", shell); err != nil {
			t.Fatalf("completion %s failed: %v", shell, err)
		}
	}

	// Directly invoke RunE to cover the unsupported shell fallback branch.
	cmd := newCompletionCommand()
	cmd.SetOut(io.Discard)
	root := &cobra.Command{Use: "awstbx"}
	root.AddCommand(cmd)
	cmd.SetArgs([]string{"unsupported"})
	if err := cmd.RunE(cmd, []string{"unsupported"}); err == nil {
		t.Fatal("expected unsupported shell error")
	}
}

func TestCoverageBoostRootExecutionBranches(t *testing.T) {
	if _, err := executeCommand(t, "--output", "xml"); err == nil {
		t.Fatal("expected invalid output format error")
	}

	out, err := executeCommand(t)
	if err != nil {
		t.Fatalf("root help execution failed: %v", err)
	}
	if !strings.Contains(out, "Available Commands:") {
		t.Fatalf("unexpected root help output: %s", out)
	}
}

func TestCoverageBoostLegacyFlagsRejected(t *testing.T) {
	cases := [][]string{
		{"appstream", "delete-image", "--name", "img-a"},
		{"cloudformation", "delete-stackset", "--name", "stackset-a"},
		{"cloudwatch", "delete-log-groups", "--keep", "30d"},
		{"ec2", "delete-security-groups", "--tag", "env=dev"},
		{"kms", "delete-keys", "--tag", "env=dev"},
		{"org", "set-alternate-contact", "--contacts-file", "contacts.json"},
		{"s3", "search-objects", "--bucket", "my-bucket", "--keys", "foo"},
	}

	for _, args := range cases {
		_, err := executeCommand(t, args...)
		if err == nil {
			t.Fatalf("expected legacy flag rejection for args: %v", args)
		}
		if !strings.Contains(err.Error(), "unknown flag") {
			t.Fatalf("expected unknown flag error for args=%v, got %v", args, err)
		}
	}
}

// Suppress unused import lint for fmt (used in defaultCommandExample via help_defaults.go).
var _ = fmt.Sprintf
