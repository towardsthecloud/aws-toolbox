package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/towardsthecloud/aws-toolbox/internal/version"
)

func executeCommand(t *testing.T, args ...string) (string, error) {
	t.Helper()
	return executeCommandWithInput(t, "", args...)
}

func executeCommandWithInput(t *testing.T, input string, args ...string) (string, error) {
	t.Helper()

	cmd := NewRootCommand()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetIn(strings.NewReader(input))
	cmd.SetArgs(args)

	err := cmd.Execute()
	return buf.String(), err
}

func TestHelpListsServiceGroups(t *testing.T) {
	output, err := executeCommand(t, "--help")
	if err != nil {
		t.Fatalf("execute --help: %v", err)
	}

	for _, item := range []string{"appstream", "ec2", "org", "s3", "completion", "version"} {
		if !strings.Contains(output, item) {
			t.Fatalf("help output missing %q\n%s", item, output)
		}
	}
}

func TestVersionFlagPrintsBuildMetadata(t *testing.T) {
	oldVersion, oldCommit, oldDate := version.Version, version.Commit, version.Date
	defer func() {
		version.Version, version.Commit, version.Date = oldVersion, oldCommit, oldDate
	}()

	version.Version = "9.9.9"
	version.Commit = "deadbee"
	version.Date = "2026-02-19T00:00:00Z"

	output, err := executeCommand(t, "--version")
	if err != nil {
		t.Fatalf("execute --version: %v", err)
	}

	for _, field := range []string{"version: 9.9.9", "commit: deadbee", "build date: 2026-02-19T00:00:00Z"} {
		if !strings.Contains(output, field) {
			t.Fatalf("version output missing %q\n%s", field, output)
		}
	}
}

func TestCompletionZshGeneratesScript(t *testing.T) {
	output, err := executeCommand(t, "completion", "zsh")
	if err != nil {
		t.Fatalf("execute completion zsh: %v", err)
	}

	if !strings.Contains(output, "#compdef awstbx") {
		t.Fatalf("unexpected zsh completion output\n%s", output)
	}
}

func TestCompletionBashGeneratesScript(t *testing.T) {
	output, err := executeCommand(t, "completion", "bash")
	if err != nil {
		t.Fatalf("execute completion bash: %v", err)
	}

	if !strings.Contains(output, "complete -o default -F __start_awstbx awstbx") {
		t.Fatalf("unexpected bash completion output\n%s", output)
	}
}

func TestVersionCommandPrintsBuildMetadata(t *testing.T) {
	output, err := executeCommand(t, "version")
	if err != nil {
		t.Fatalf("execute version command: %v", err)
	}
	if !strings.Contains(output, "version:") || !strings.Contains(output, "commit:") || !strings.Contains(output, "build date:") {
		t.Fatalf("unexpected version command output\n%s", output)
	}
}
