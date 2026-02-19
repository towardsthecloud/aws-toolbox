package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestGlobalOptionsFromCommandAndRuntime(t *testing.T) {
	root := NewRootCommand()
	root.SetIn(strings.NewReader(""))
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})

	if err := root.PersistentFlags().Set("profile", "my-profile"); err != nil {
		t.Fatalf("set profile: %v", err)
	}
	if err := root.PersistentFlags().Set("region", "us-east-1"); err != nil {
		t.Fatalf("set region: %v", err)
	}
	if err := root.PersistentFlags().Set("dry-run", "true"); err != nil {
		t.Fatalf("set dry-run: %v", err)
	}
	if err := root.PersistentFlags().Set("output", "json"); err != nil {
		t.Fatalf("set output: %v", err)
	}
	if err := root.PersistentFlags().Set("no-confirm", "true"); err != nil {
		t.Fatalf("set no-confirm: %v", err)
	}

	cmd, _, err := root.Find([]string{"ec2", "list-eips"})
	if err != nil {
		t.Fatalf("find subcommand: %v", err)
	}

	opts, err := globalOptionsFromCommand(cmd)
	if err != nil {
		t.Fatalf("globalOptionsFromCommand: %v", err)
	}
	if opts.Profile != "my-profile" || opts.Region != "us-east-1" || !opts.DryRun || opts.OutputFormat != "json" || !opts.NoConfirm {
		t.Fatalf("unexpected options: %+v", opts)
	}

	runtime, err := newCommandRuntime(cmd)
	if err != nil {
		t.Fatalf("newCommandRuntime: %v", err)
	}
	if runtime.Formatter == nil {
		t.Fatal("expected formatter")
	}
}

func TestPointerHelpers(t *testing.T) {
	if pointerToString(nil) != "" {
		t.Fatal("expected empty string for nil pointer")
	}
	if pointerToInt32(nil) != 0 {
		t.Fatal("expected zero for nil int pointer")
	}

	s := "abc"
	n := int32(9)
	if pointerToString(&s) != "abc" {
		t.Fatal("unexpected pointerToString value")
	}
	if pointerToInt32(&n) != 9 {
		t.Fatal("unexpected pointerToInt32 value")
	}
	if *ptr("x") != "x" {
		t.Fatal("unexpected ptr helper value")
	}
}
