package cli

import (
	"fmt"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/spf13/cobra"
)

func newServiceRuntime[T any](
	cmd *cobra.Command,
	loadConfig func(profile, region string) (awssdk.Config, error),
	newClient func(awssdk.Config) T,
) (commandRuntime, awssdk.Config, T, error) {
	runtime, err := newCommandRuntime(cmd)
	if err != nil {
		var zeroClient T
		return commandRuntime{}, awssdk.Config{}, zeroClient, err
	}

	cfg, err := loadConfig(runtime.Options.Profile, runtime.Options.Region)
	if err != nil {
		var zeroClient T
		return commandRuntime{}, awssdk.Config{}, zeroClient, fmt.Errorf("load AWS config: %w", err)
	}

	return runtime, cfg, newClient(cfg), nil
}

func newServiceConfigRuntime(
	cmd *cobra.Command,
	loadConfig func(profile, region string) (awssdk.Config, error),
) (commandRuntime, awssdk.Config, error) {
	runtime, cfg, _, err := newServiceRuntime(cmd, loadConfig, func(awssdk.Config) struct{} { return struct{}{} })
	if err != nil {
		return commandRuntime{}, awssdk.Config{}, err
	}
	return runtime, cfg, nil
}
