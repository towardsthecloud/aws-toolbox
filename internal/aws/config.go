package aws

import (
	"context"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
)

// LoadAWSConfig loads AWS SDK configuration using optional profile and region overrides.
func LoadAWSConfig(profile, region string) (awssdk.Config, error) {
	return LoadAWSConfigWithContext(context.Background(), profile, region)
}

// LoadAWSConfigWithContext loads AWS SDK configuration with the provided context.
func LoadAWSConfigWithContext(
	ctx context.Context,
	profile, region string,
	extraOpts ...func(*config.LoadOptions) error,
) (awssdk.Config, error) {
	opts := make([]func(*config.LoadOptions) error, 0, 2+len(extraOpts))
	if profile != "" {
		opts = append(opts, config.WithSharedConfigProfile(profile))
	}
	if region != "" {
		opts = append(opts, config.WithRegion(region))
	}
	opts = append(opts, extraOpts...)

	return config.LoadDefaultConfig(ctx, opts...)
}
