package cliutil

import (
	"fmt"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/spf13/cobra"
)

// NewServiceRuntime creates a CommandRuntime, loads an AWS config, and instantiates
// a typed service client in a single call.
func NewServiceRuntime[T any](
	cmd *cobra.Command,
	loadConfig func(profile, region string) (awssdk.Config, error),
	newClient func(awssdk.Config) T,
) (CommandRuntime, awssdk.Config, T, error) {
	runtime, err := NewCommandRuntime(cmd)
	if err != nil {
		var zeroClient T
		return CommandRuntime{}, awssdk.Config{}, zeroClient, err
	}

	cfg, err := loadConfig(runtime.Options.Profile, runtime.Options.Region)
	if err != nil {
		var zeroClient T
		return CommandRuntime{}, awssdk.Config{}, zeroClient, fmt.Errorf("load AWS config: %w", err)
	}

	return runtime, cfg, newClient(cfg), nil
}

// NewServiceConfigRuntime creates a CommandRuntime and loads an AWS config
// without instantiating a service client.
func NewServiceConfigRuntime(
	cmd *cobra.Command,
	loadConfig func(profile, region string) (awssdk.Config, error),
) (CommandRuntime, awssdk.Config, error) {
	runtime, cfg, _, err := NewServiceRuntime(cmd, loadConfig, func(awssdk.Config) struct{} { return struct{}{} })
	if err != nil {
		return CommandRuntime{}, awssdk.Config{}, err
	}
	return runtime, cfg, nil
}
