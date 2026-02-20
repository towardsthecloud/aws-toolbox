package cli

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/towardsthecloud/aws-toolbox/internal/cliutil"
)

func TestServiceRuntimeBranches(t *testing.T) {
	root := NewRootCommand()
	root.SetIn(strings.NewReader(""))
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})

	cmd, _, findErr := root.Find([]string{"ec2", "list-eips"})
	if findErr != nil {
		t.Fatalf("find subcommand: %v", findErr)
	}

	_, _, _, err := cliutil.NewServiceRuntime(cmd, func(_, _ string) (awssdk.Config, error) {
		return awssdk.Config{}, errors.New("load failed")
	}, func(awssdk.Config) struct{} { return struct{}{} })
	if err == nil {
		t.Fatal("expected NewServiceRuntime loader error")
	}

	runtime, _, client, err := cliutil.NewServiceRuntime(cmd, func(_, _ string) (awssdk.Config, error) {
		return awssdk.Config{Region: "us-east-1"}, nil
	}, func(cfg awssdk.Config) string { return cfg.Region })
	if err != nil || client != "us-east-1" || runtime.Options.OutputFormat == "" {
		t.Fatalf("unexpected NewServiceRuntime success result: runtime=%+v client=%q err=%v", runtime, client, err)
	}
}
