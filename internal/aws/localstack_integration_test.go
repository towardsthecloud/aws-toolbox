//go:build integration

package aws

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

func TestLocalStackServiceIntegrations(t *testing.T) {
	ctx := context.Background()
	endpoint := strings.TrimSpace(os.Getenv("AWS_ENDPOINT_URL"))
	if endpoint == "" {
		t.Skip("AWS_ENDPOINT_URL not set; skipping LocalStack integration tests")
	}

	t.Setenv("AWS_EC2_METADATA_DISABLED", "true")
	t.Setenv("AWS_ACCESS_KEY_ID", "test")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test")
	t.Setenv("AWS_REGION", "us-east-1")
	t.Setenv("AWS_DEFAULT_REGION", "us-east-1")

	resolver := awssdk.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (awssdk.Endpoint, error) {
		return awssdk.Endpoint{
			URL:               endpoint,
			HostnameImmutable: true,
		}, nil
	})

	cfg, err := LoadAWSConfigWithContext(
		ctx,
		"",
		"us-east-1",
		config.WithEndpointResolverWithOptions(resolver),
	)
	if err != nil {
		t.Fatalf("LoadAWSConfigWithContext() error = %v", err)
	}

	t.Run("ec2 describe regions", func(t *testing.T) {
		client := ec2.NewFromConfig(cfg)
		out, err := client.DescribeRegions(ctx, &ec2.DescribeRegionsInput{})
		if err != nil {
			t.Fatalf("DescribeRegions() error = %v", err)
		}
		if len(out.Regions) == 0 {
			t.Fatal("expected at least one region in LocalStack response")
		}
	})

	t.Run("s3 create and list object", func(t *testing.T) {
		client := s3.NewFromConfig(cfg, func(options *s3.Options) {
			options.UsePathStyle = true
		})

		bucket := fmt.Sprintf("awstbx-int-%d", time.Now().UnixNano())
		key := "integration-test.txt"

		if _, err := client.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: awssdk.String(bucket)}); err != nil {
			t.Fatalf("CreateBucket() error = %v", err)
		}
		if _, err := client.PutObject(ctx, &s3.PutObjectInput{
			Bucket: awssdk.String(bucket),
			Key:    awssdk.String(key),
			Body:   strings.NewReader("ok"),
		}); err != nil {
			t.Fatalf("PutObject() error = %v", err)
		}

		out, err := client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{Bucket: awssdk.String(bucket)})
		if err != nil {
			t.Fatalf("ListObjectsV2() error = %v", err)
		}
		if len(out.Contents) == 0 {
			t.Fatal("expected uploaded object in bucket listing")
		}
	})

	t.Run("cloudwatch logs create and query group", func(t *testing.T) {
		client := cloudwatchlogs.NewFromConfig(cfg)
		logGroupName := fmt.Sprintf("/awstbx/integration/%d", time.Now().UnixNano())

		if _, err := client.CreateLogGroup(ctx, &cloudwatchlogs.CreateLogGroupInput{
			LogGroupName: awssdk.String(logGroupName),
		}); err != nil {
			t.Fatalf("CreateLogGroup() error = %v", err)
		}

		out, err := client.DescribeLogGroups(ctx, &cloudwatchlogs.DescribeLogGroupsInput{
			LogGroupNamePrefix: awssdk.String(logGroupName),
		})
		if err != nil {
			t.Fatalf("DescribeLogGroups() error = %v", err)
		}
		if len(out.LogGroups) == 0 {
			t.Fatal("expected created log group in DescribeLogGroups() output")
		}
	})

	t.Run("iam create and get user", func(t *testing.T) {
		client := iam.NewFromConfig(cfg)
		username := fmt.Sprintf("awstbx-int-%d", time.Now().UnixNano())

		if _, err := client.CreateUser(ctx, &iam.CreateUserInput{UserName: awssdk.String(username)}); err != nil {
			t.Fatalf("CreateUser() error = %v", err)
		}

		out, err := client.GetUser(ctx, &iam.GetUserInput{UserName: awssdk.String(username)})
		if err != nil {
			t.Fatalf("GetUser() error = %v", err)
		}
		if awssdk.ToString(out.User.UserName) != username {
			t.Fatalf("expected username %q, got %q", username, awssdk.ToString(out.User.UserName))
		}
	})

	t.Run("ssm put and get parameter", func(t *testing.T) {
		client := ssm.NewFromConfig(cfg)
		name := fmt.Sprintf("tbx-integration-%d", time.Now().UnixNano())
		value := "example-value"

		if _, err := client.PutParameter(ctx, &ssm.PutParameterInput{
			Name:      awssdk.String(name),
			Type:      "String",
			Value:     awssdk.String(value),
			Overwrite: awssdk.Bool(true),
		}); err != nil {
			t.Fatalf("PutParameter() error = %v", err)
		}

		out, err := client.GetParameter(ctx, &ssm.GetParameterInput{Name: awssdk.String(name)})
		if err != nil {
			t.Fatalf("GetParameter() error = %v", err)
		}
		if awssdk.ToString(out.Parameter.Value) != value {
			t.Fatalf("expected parameter value %q, got %q", value, awssdk.ToString(out.Parameter.Value))
		}
	})
}
