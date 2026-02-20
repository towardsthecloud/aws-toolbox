package cliutil

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/spf13/cobra"
)

func TestGlobalOptionsFromCommandAndRuntime(t *testing.T) {
	dummy := &cobra.Command{Use: "dummy", RunE: func(*cobra.Command, []string) error { return nil }}
	root := NewTestRootCommand(dummy)
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

	opts, err := GlobalOptionsFromCommand(root)
	if err != nil {
		t.Fatalf("GlobalOptionsFromCommand: %v", err)
	}
	if opts.Profile != "my-profile" || opts.Region != "us-east-1" || !opts.DryRun || opts.OutputFormat != "json" || !opts.NoConfirm {
		t.Fatalf("unexpected options: %+v", opts)
	}

	runtime, err := NewCommandRuntime(root)
	if err != nil {
		t.Fatalf("NewCommandRuntime: %v", err)
	}
	if runtime.Formatter == nil {
		t.Fatal("expected formatter")
	}
}

func TestNewCommandRuntimeInvalidOutputFormat(t *testing.T) {
	dummy := &cobra.Command{Use: "dummy", RunE: func(*cobra.Command, []string) error { return nil }}
	root := NewTestRootCommand(dummy)
	root.SetIn(strings.NewReader(""))
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})

	if err := root.PersistentFlags().Set("output", "yaml"); err != nil {
		t.Fatalf("set output: %v", err)
	}

	_, err := NewCommandRuntime(root)
	if err == nil {
		t.Fatal("expected error for invalid output format")
	}
	if !strings.Contains(err.Error(), "unsupported output format") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNewCommandRuntimeNoFlags(t *testing.T) {
	// Command without persistent flags should fail
	bare := &cobra.Command{Use: "bare"}
	_, err := NewCommandRuntime(bare)
	if err == nil {
		t.Fatal("expected error when flags are missing")
	}
}

func TestWriteDataset(t *testing.T) {
	dummy := &cobra.Command{Use: "dummy", RunE: func(*cobra.Command, []string) error { return nil }}
	root := NewTestRootCommand(dummy)
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(&bytes.Buffer{})
	root.SetIn(strings.NewReader(""))

	if err := root.PersistentFlags().Set("output", "json"); err != nil {
		t.Fatalf("set output: %v", err)
	}

	runtime, err := NewCommandRuntime(root)
	if err != nil {
		t.Fatalf("NewCommandRuntime: %v", err)
	}

	headers := []string{"id", "name", "status"}
	rows := [][]string{
		{"1", "alpha", "active"},
		{"2", "beta", "inactive"},
	}

	if err := WriteDataset(root, runtime, headers, rows); err != nil {
		t.Fatalf("WriteDataset: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "alpha") || !strings.Contains(output, "beta") {
		t.Fatalf("expected data in output: %s", output)
	}
}

func TestWriteDatasetEmptyRows(t *testing.T) {
	dummy := &cobra.Command{Use: "dummy", RunE: func(*cobra.Command, []string) error { return nil }}
	root := NewTestRootCommand(dummy)
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(&bytes.Buffer{})
	root.SetIn(strings.NewReader(""))

	if err := root.PersistentFlags().Set("output", "json"); err != nil {
		t.Fatalf("set output: %v", err)
	}

	runtime, err := NewCommandRuntime(root)
	if err != nil {
		t.Fatalf("NewCommandRuntime: %v", err)
	}

	if err := WriteDataset(root, runtime, []string{"id"}, [][]string{}); err != nil {
		t.Fatalf("WriteDataset empty: %v", err)
	}
}

func TestWriteDatasetTableFormat(t *testing.T) {
	dummy := &cobra.Command{Use: "dummy", RunE: func(*cobra.Command, []string) error { return nil }}
	root := NewTestRootCommand(dummy)
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(&bytes.Buffer{})
	root.SetIn(strings.NewReader(""))

	runtime, err := NewCommandRuntime(root)
	if err != nil {
		t.Fatalf("NewCommandRuntime: %v", err)
	}

	headers := []string{"key", "value"}
	rows := [][]string{{"foo", "bar"}}

	if err := WriteDataset(root, runtime, headers, rows); err != nil {
		t.Fatalf("WriteDataset table: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "foo") || !strings.Contains(output, "bar") {
		t.Fatalf("expected table data: %s", output)
	}
}

func TestWriteDatasetTextFormat(t *testing.T) {
	dummy := &cobra.Command{Use: "dummy", RunE: func(*cobra.Command, []string) error { return nil }}
	root := NewTestRootCommand(dummy)
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(&bytes.Buffer{})
	root.SetIn(strings.NewReader(""))

	if err := root.PersistentFlags().Set("output", "text"); err != nil {
		t.Fatalf("set output: %v", err)
	}

	runtime, err := NewCommandRuntime(root)
	if err != nil {
		t.Fatalf("NewCommandRuntime: %v", err)
	}

	headers := []string{"key", "value"}
	rows := [][]string{{"foo", "bar"}}

	if err := WriteDataset(root, runtime, headers, rows); err != nil {
		t.Fatalf("WriteDataset text: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "foo") {
		t.Fatalf("expected text data: %s", output)
	}
}

func TestNewServiceRuntime(t *testing.T) {
	dummy := &cobra.Command{Use: "dummy", RunE: func(*cobra.Command, []string) error { return nil }}
	root := NewTestRootCommand(dummy)
	root.SetIn(strings.NewReader(""))
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})

	if err := root.PersistentFlags().Set("output", "json"); err != nil {
		t.Fatalf("set output: %v", err)
	}

	type fakeClient struct{ Name string }

	runtime, cfg, client, err := NewServiceRuntime(root,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "us-west-2"}, nil },
		func(cfg awssdk.Config) fakeClient { return fakeClient{Name: cfg.Region} },
	)
	if err != nil {
		t.Fatalf("NewServiceRuntime: %v", err)
	}
	if runtime.Formatter == nil {
		t.Fatal("expected formatter")
	}
	if cfg.Region != "us-west-2" {
		t.Fatalf("expected region us-west-2, got %s", cfg.Region)
	}
	if client.Name != "us-west-2" {
		t.Fatalf("expected client name us-west-2, got %s", client.Name)
	}
}

func TestNewServiceRuntimeConfigLoadError(t *testing.T) {
	dummy := &cobra.Command{Use: "dummy", RunE: func(*cobra.Command, []string) error { return nil }}
	root := NewTestRootCommand(dummy)
	root.SetIn(strings.NewReader(""))
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})

	if err := root.PersistentFlags().Set("output", "json"); err != nil {
		t.Fatalf("set output: %v", err)
	}

	_, _, _, err := NewServiceRuntime(root,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{}, errors.New("config error") },
		func(awssdk.Config) struct{} { return struct{}{} },
	)
	if err == nil {
		t.Fatal("expected error from config load failure")
	}
	if !strings.Contains(err.Error(), "load AWS config") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNewServiceRuntimeBadOutputFormat(t *testing.T) {
	dummy := &cobra.Command{Use: "dummy", RunE: func(*cobra.Command, []string) error { return nil }}
	root := NewTestRootCommand(dummy)
	root.SetIn(strings.NewReader(""))
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})

	if err := root.PersistentFlags().Set("output", "xml"); err != nil {
		t.Fatalf("set output: %v", err)
	}

	_, _, _, err := NewServiceRuntime(root,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{}, nil },
		func(awssdk.Config) struct{} { return struct{}{} },
	)
	if err == nil {
		t.Fatal("expected error for invalid output format")
	}
}

func TestNewServiceConfigRuntime(t *testing.T) {
	dummy := &cobra.Command{Use: "dummy", RunE: func(*cobra.Command, []string) error { return nil }}
	root := NewTestRootCommand(dummy)
	root.SetIn(strings.NewReader(""))
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})

	if err := root.PersistentFlags().Set("output", "json"); err != nil {
		t.Fatalf("set output: %v", err)
	}

	runtime, cfg, err := NewServiceConfigRuntime(root,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{Region: "eu-west-1"}, nil },
	)
	if err != nil {
		t.Fatalf("NewServiceConfigRuntime: %v", err)
	}
	if runtime.Formatter == nil {
		t.Fatal("expected formatter")
	}
	if cfg.Region != "eu-west-1" {
		t.Fatalf("expected region eu-west-1, got %s", cfg.Region)
	}
}

func TestNewServiceConfigRuntimeError(t *testing.T) {
	dummy := &cobra.Command{Use: "dummy", RunE: func(*cobra.Command, []string) error { return nil }}
	root := NewTestRootCommand(dummy)
	root.SetIn(strings.NewReader(""))
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})

	if err := root.PersistentFlags().Set("output", "json"); err != nil {
		t.Fatalf("set output: %v", err)
	}

	_, _, err := NewServiceConfigRuntime(root,
		func(_, _ string) (awssdk.Config, error) { return awssdk.Config{}, errors.New("config error") },
	)
	if err == nil {
		t.Fatal("expected error from config load failure")
	}
}

func TestGlobalOptionsFromCommandVersion(t *testing.T) {
	dummy := &cobra.Command{Use: "dummy", RunE: func(*cobra.Command, []string) error { return nil }}
	root := NewTestRootCommand(dummy)
	root.SetIn(strings.NewReader(""))
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})

	if err := root.PersistentFlags().Set("version", "true"); err != nil {
		t.Fatalf("set version: %v", err)
	}

	opts, err := GlobalOptionsFromCommand(root)
	if err != nil {
		t.Fatalf("GlobalOptionsFromCommand: %v", err)
	}
	if !opts.ShowVersion {
		t.Fatal("expected ShowVersion to be true")
	}
}

func TestGlobalOptionsDefaultValues(t *testing.T) {
	dummy := &cobra.Command{Use: "dummy", RunE: func(*cobra.Command, []string) error { return nil }}
	root := NewTestRootCommand(dummy)
	root.SetIn(strings.NewReader(""))
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})

	opts, err := GlobalOptionsFromCommand(root)
	if err != nil {
		t.Fatalf("GlobalOptionsFromCommand: %v", err)
	}
	if opts.Profile != "" {
		t.Fatalf("expected empty profile default, got %q", opts.Profile)
	}
	if opts.Region != "" {
		t.Fatalf("expected empty region default, got %q", opts.Region)
	}
	if opts.DryRun {
		t.Fatal("expected DryRun to be false by default")
	}
	if opts.OutputFormat != "table" {
		t.Fatalf("expected table default, got %q", opts.OutputFormat)
	}
	if opts.NoConfirm {
		t.Fatal("expected NoConfirm to be false by default")
	}
	if opts.ShowVersion {
		t.Fatal("expected ShowVersion to be false by default")
	}
}
