package cli

import (
	"context"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/spf13/cobra"
	awstbxaws "github.com/towardsthecloud/aws-toolbox/internal/aws"
)

type s3API interface {
	DeleteBucket(context.Context, *s3.DeleteBucketInput, ...func(*s3.Options)) (*s3.DeleteBucketOutput, error)
	DeleteObjects(context.Context, *s3.DeleteObjectsInput, ...func(*s3.Options)) (*s3.DeleteObjectsOutput, error)
	GetBucketVersioning(context.Context, *s3.GetBucketVersioningInput, ...func(*s3.Options)) (*s3.GetBucketVersioningOutput, error)
	GetObject(context.Context, *s3.GetObjectInput, ...func(*s3.Options)) (*s3.GetObjectOutput, error)
	ListBuckets(context.Context, *s3.ListBucketsInput, ...func(*s3.Options)) (*s3.ListBucketsOutput, error)
	ListObjectVersions(context.Context, *s3.ListObjectVersionsInput, ...func(*s3.Options)) (*s3.ListObjectVersionsOutput, error)
	ListObjectsV2(context.Context, *s3.ListObjectsV2Input, ...func(*s3.Options)) (*s3.ListObjectsV2Output, error)
}

var s3LoadAWSConfig = awstbxaws.LoadAWSConfig
var s3NewClient = func(cfg awssdk.Config) s3API {
	return s3.NewFromConfig(cfg)
}

func newS3Command() *cobra.Command {
	cmd := newServiceGroupCommand("s3", "Manage S3 resources")

	cmd.AddCommand(newS3DeleteBucketsCommand())
	cmd.AddCommand(newS3DownloadBucketCommand())
	cmd.AddCommand(newS3ListOldFilesCommand())
	cmd.AddCommand(newS3SearchObjectsCommand())

	return cmd
}

func newS3DeleteBucketsCommand() *cobra.Command {
	var emptyOnly bool
	var nameFilter string

	cmd := &cobra.Command{
		Use:   "delete-buckets",
		Short: "Delete S3 buckets by emptiness and/or name match",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runS3DeleteBuckets(cmd, emptyOnly, nameFilter)
		},
		SilenceUsage: true,
	}
	cmd.Flags().BoolVar(&emptyOnly, "empty", false, "Only target empty buckets with versioning disabled")
	cmd.Flags().StringVar(&nameFilter, "name", "", "Only target buckets containing this name")

	return cmd
}

func newS3DownloadBucketCommand() *cobra.Command {
	var bucket string
	var prefix string
	var outputDir string

	cmd := &cobra.Command{
		Use:   "download-bucket",
		Short: "Download S3 objects from a bucket prefix",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runS3DownloadBucket(cmd, bucket, prefix, outputDir)
		},
		SilenceUsage: true,
	}
	cmd.Flags().StringVar(&bucket, "bucket", "", "Bucket name")
	cmd.Flags().StringVar(&prefix, "prefix", "", "Object key prefix to download")
	cmd.Flags().StringVar(&outputDir, "output-dir", ".", "Local directory for downloaded files")

	return cmd
}

func newS3ListOldFilesCommand() *cobra.Command {
	var bucket string
	var prefix string
	var olderThanDays int

	cmd := &cobra.Command{
		Use:   "list-old-files",
		Short: "List objects older than a threshold",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runS3ListOldFiles(cmd, bucket, prefix, olderThanDays)
		},
		SilenceUsage: true,
	}
	cmd.Flags().StringVar(&bucket, "bucket", "", "Bucket name")
	cmd.Flags().StringVar(&prefix, "prefix", "", "Optional key prefix")
	cmd.Flags().IntVar(&olderThanDays, "older-than-days", 60, "Only show files older than this many days")

	return cmd
}

func newS3SearchObjectsCommand() *cobra.Command {
	var bucket string
	var prefix string
	var keys []string

	cmd := &cobra.Command{
		Use:   "search-objects",
		Short: "Search S3 objects by prefix and/or key list",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runS3SearchObjects(cmd, bucket, prefix, keys)
		},
		SilenceUsage: true,
	}
	cmd.Flags().StringVar(&bucket, "bucket", "", "Bucket name")
	cmd.Flags().StringVar(&prefix, "prefix", "", "Optional key prefix filter")
	cmd.Flags().StringSliceVar(&keys, "keys", nil, "Comma-separated keys to search for")

	return cmd
}
