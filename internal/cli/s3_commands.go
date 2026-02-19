package cli

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/spf13/cobra"
	awstbxaws "github.com/towardsthecloud/aws-toolbox/internal/aws"
)

func runS3DeleteBuckets(cmd *cobra.Command, emptyOnly bool, nameFilter string) error {
	nameFilter = strings.TrimSpace(nameFilter)
	if !emptyOnly && nameFilter == "" {
		return fmt.Errorf("set --empty or --name")
	}

	runtime, err := newCommandRuntime(cmd)
	if err != nil {
		return err
	}

	cfg, err := s3LoadAWSConfig(runtime.Options.Profile, runtime.Options.Region)
	if err != nil {
		return fmt.Errorf("load AWS config: %w", err)
	}
	client := s3NewClient(cfg)

	buckets, err := listS3Buckets(cmd.Context(), client)
	if err != nil {
		return fmt.Errorf("list buckets: %s", awstbxaws.FormatUserError(err))
	}

	targets := make([]string, 0, len(buckets))
	for _, bucket := range buckets {
		name := pointerToString(bucket.Name)
		if name == "" {
			continue
		}
		if nameFilter != "" && !strings.Contains(name, nameFilter) {
			continue
		}
		if emptyOnly {
			ok, checkErr := isS3BucketEmptyAndUnversioned(cmd.Context(), client, name)
			if checkErr != nil {
				return checkErr
			}
			if !ok {
				continue
			}
		}

		targets = append(targets, name)
	}

	sort.Strings(targets)

	rows := make([][]string, 0, len(targets))
	for _, name := range targets {
		action := "would-delete"
		if !runtime.Options.DryRun {
			action = "pending"
		}
		rows = append(rows, []string{name, action})
	}

	if len(rows) == 0 {
		return writeDataset(cmd, runtime, []string{"bucket", "action"}, rows)
	}

	if !runtime.Options.DryRun {
		ok, confirmErr := runtime.Prompter.Confirm(
			fmt.Sprintf("Delete %d S3 bucket(s)", len(rows)),
			runtime.Options.NoConfirm,
		)
		if confirmErr != nil {
			return confirmErr
		}
		if !ok {
			for i := range rows {
				rows[i][1] = "cancelled"
			}
			return writeDataset(cmd, runtime, []string{"bucket", "action"}, rows)
		}

		for i := range rows {
			bucket := rows[i][0]
			if clearErr := deleteAllObjectsFromS3Bucket(cmd.Context(), client, bucket); clearErr != nil {
				rows[i][1] = "failed: " + clearErr.Error()
				continue
			}
			_, deleteErr := client.DeleteBucket(cmd.Context(), &s3.DeleteBucketInput{Bucket: ptr(bucket)})
			if deleteErr != nil {
				rows[i][1] = "failed: " + awstbxaws.FormatUserError(deleteErr)
				continue
			}
			rows[i][1] = "deleted"
		}
	}

	return writeDataset(cmd, runtime, []string{"bucket", "action"}, rows)
}

func runS3DownloadBucket(cmd *cobra.Command, bucket, prefix, outputDir string) error {
	if strings.TrimSpace(bucket) == "" {
		return fmt.Errorf("--bucket is required")
	}
	if strings.TrimSpace(prefix) == "" {
		return fmt.Errorf("--prefix is required")
	}

	runtime, err := newCommandRuntime(cmd)
	if err != nil {
		return err
	}

	cfg, err := s3LoadAWSConfig(runtime.Options.Profile, runtime.Options.Region)
	if err != nil {
		return fmt.Errorf("load AWS config: %w", err)
	}
	client := s3NewClient(cfg)

	objects, err := listS3Objects(cmd.Context(), client, bucket, prefix)
	if err != nil {
		return fmt.Errorf("list objects: %s", awstbxaws.FormatUserError(err))
	}
	sortS3ObjectsByKey(objects)

	rows := make([][]string, 0, len(objects))
	for _, object := range objects {
		key := s3ObjectKey(object)
		relativeKey := strings.TrimPrefix(key, prefix)
		relativeKey = strings.TrimPrefix(relativeKey, "/")
		if relativeKey == "" {
			relativeKey = key
		}
		targetPath := filepath.Join(outputDir, relativeKey)

		action := "would-download"
		if runtime.Options.DryRun {
			rows = append(rows, []string{bucket, key, targetPath, action})
			continue
		}

		if err := downloadS3Object(cmd.Context(), client, bucket, key, targetPath); err != nil {
			action = "failed: " + err.Error()
		} else {
			action = "downloaded"
		}
		rows = append(rows, []string{bucket, key, targetPath, action})
	}

	return writeDataset(cmd, runtime, []string{"bucket", "key", "target_path", "action"}, rows)
}

func runS3ListOldFiles(cmd *cobra.Command, bucket, prefix string, olderThanDays int) error {
	if strings.TrimSpace(bucket) == "" {
		return fmt.Errorf("--bucket is required")
	}
	if olderThanDays < 0 {
		return fmt.Errorf("--older-than-days must be >= 0")
	}

	runtime, err := newCommandRuntime(cmd)
	if err != nil {
		return err
	}

	cfg, err := s3LoadAWSConfig(runtime.Options.Profile, runtime.Options.Region)
	if err != nil {
		return fmt.Errorf("load AWS config: %w", err)
	}
	client := s3NewClient(cfg)

	objects, err := listS3Objects(cmd.Context(), client, bucket, prefix)
	if err != nil {
		return fmt.Errorf("list objects: %s", awstbxaws.FormatUserError(err))
	}
	sortS3ObjectsByKey(objects)

	now := time.Now().UTC()
	rows := make([][]string, 0, len(objects))
	for _, object := range objects {
		lastModified := s3ObjectLastModified(object)
		if lastModified.IsZero() {
			continue
		}
		ageDays := int(now.Sub(lastModified).Hours() / 24)
		if ageDays < olderThanDays {
			continue
		}
		rows = append(rows, []string{
			bucket,
			s3ObjectKey(object),
			lastModified.Format(time.RFC3339),
			fmt.Sprintf("%d", ageDays),
			fmt.Sprintf("%d", s3ObjectSize(object)),
		})
	}

	return writeDataset(cmd, runtime, []string{"bucket", "key", "last_modified", "age_days", "size_bytes"}, rows)
}

func runS3SearchObjects(cmd *cobra.Command, bucket, prefix string, keys []string) error {
	if strings.TrimSpace(bucket) == "" {
		return fmt.Errorf("--bucket is required")
	}

	queries := normalizeS3KeyQueries(keys)
	if strings.TrimSpace(prefix) == "" && len(queries) == 0 {
		return fmt.Errorf("set --prefix and/or --keys")
	}

	runtime, err := newCommandRuntime(cmd)
	if err != nil {
		return err
	}

	cfg, err := s3LoadAWSConfig(runtime.Options.Profile, runtime.Options.Region)
	if err != nil {
		return fmt.Errorf("load AWS config: %w", err)
	}
	client := s3NewClient(cfg)

	objects, err := listS3Objects(cmd.Context(), client, bucket, prefix)
	if err != nil {
		return fmt.Errorf("list objects: %s", awstbxaws.FormatUserError(err))
	}
	sortS3ObjectsByKey(objects)

	if len(queries) == 0 {
		rows := make([][]string, 0, len(objects))
		for _, object := range objects {
			rows = append(rows, []string{
				bucket,
				s3ObjectKey(object),
				"true",
				s3ObjectLastModified(object).Format(time.RFC3339),
				fmt.Sprintf("%d", s3ObjectSize(object)),
			})
		}
		return writeDataset(cmd, runtime, []string{"bucket", "key", "exists", "last_modified", "size_bytes"}, rows)
	}

	objectByKey := make(map[string]s3types.Object, len(objects))
	for _, object := range objects {
		objectByKey[s3ObjectKey(object)] = object
	}

	rows := make([][]string, 0, len(queries))
	for _, query := range queries {
		matchedKey := query
		object, ok := objectByKey[query]
		if !ok && strings.TrimSpace(prefix) != "" {
			candidate := strings.TrimSuffix(prefix, "/") + "/" + strings.TrimPrefix(query, "/")
			if matched, exists := objectByKey[candidate]; exists {
				matchedKey = candidate
				object = matched
				ok = true
			}
		}

		lastModified := ""
		size := "0"
		exists := "false"
		if ok {
			exists = "true"
			if ts := s3ObjectLastModified(object); !ts.IsZero() {
				lastModified = ts.Format(time.RFC3339)
			}
			size = fmt.Sprintf("%d", s3ObjectSize(object))
		}

		rows = append(rows, []string{
			bucket,
			query,
			matchedKey,
			exists,
			lastModified,
			size,
		})
	}

	return writeDataset(cmd, runtime, []string{"bucket", "query_key", "matched_key", "exists", "last_modified", "size_bytes"}, rows)
}

func listS3Buckets(ctx context.Context, client s3API) ([]s3types.Bucket, error) {
	out, err := client.ListBuckets(ctx, &s3.ListBucketsInput{})
	if err != nil {
		return nil, err
	}
	return out.Buckets, nil
}

func listS3Objects(ctx context.Context, client s3API, bucket, prefix string) ([]s3types.Object, error) {
	objects := make([]s3types.Object, 0)
	var continuationToken *string

	for {
		input := &s3.ListObjectsV2Input{
			Bucket:            ptr(bucket),
			ContinuationToken: continuationToken,
		}
		if strings.TrimSpace(prefix) != "" {
			input.Prefix = ptr(prefix)
		}

		out, err := client.ListObjectsV2(ctx, input)
		if err != nil {
			return nil, err
		}

		objects = append(objects, out.Contents...)
		if out.NextContinuationToken == nil || pointerToString(out.NextContinuationToken) == "" {
			break
		}
		continuationToken = out.NextContinuationToken
	}

	return objects, nil
}

func isS3BucketEmptyAndUnversioned(ctx context.Context, client s3API, bucket string) (bool, error) {
	objects, err := client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket:  ptr(bucket),
		MaxKeys: ptr(int32(1)),
	})
	if err != nil {
		return false, fmt.Errorf("list objects for bucket %s: %s", bucket, awstbxaws.FormatUserError(err))
	}
	if len(objects.Contents) > 0 {
		return false, nil
	}

	versioning, err := client.GetBucketVersioning(ctx, &s3.GetBucketVersioningInput{Bucket: ptr(bucket)})
	if err != nil {
		return false, fmt.Errorf("get versioning for bucket %s: %s", bucket, awstbxaws.FormatUserError(err))
	}

	return versioning.Status != s3types.BucketVersioningStatusEnabled, nil
}

func deleteAllObjectsFromS3Bucket(ctx context.Context, client s3API, bucket string) error {
	// Delete regular objects first.
	var continuationToken *string
	for {
		page, err := client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
			Bucket:            ptr(bucket),
			ContinuationToken: continuationToken,
		})
		if err != nil {
			return fmt.Errorf("list objects for bucket %s: %s", bucket, awstbxaws.FormatUserError(err))
		}

		batch := make([]s3types.ObjectIdentifier, 0, len(page.Contents))
		for _, object := range page.Contents {
			if object.Key == nil {
				continue
			}
			batch = append(batch, s3types.ObjectIdentifier{Key: object.Key})
		}
		if err := deleteS3ObjectBatch(ctx, client, bucket, batch); err != nil {
			return err
		}

		if page.NextContinuationToken == nil || pointerToString(page.NextContinuationToken) == "" {
			break
		}
		continuationToken = page.NextContinuationToken
	}

	// Delete versioned objects + delete markers.
	var keyMarker *string
	var versionIDMarker *string
	for {
		page, err := client.ListObjectVersions(ctx, &s3.ListObjectVersionsInput{
			Bucket:          ptr(bucket),
			KeyMarker:       keyMarker,
			VersionIdMarker: versionIDMarker,
		})
		if err != nil {
			return fmt.Errorf("list object versions for bucket %s: %s", bucket, awstbxaws.FormatUserError(err))
		}

		batch := make([]s3types.ObjectIdentifier, 0, len(page.Versions)+len(page.DeleteMarkers))
		for _, version := range page.Versions {
			if version.Key == nil {
				continue
			}
			batch = append(batch, s3types.ObjectIdentifier{Key: version.Key, VersionId: version.VersionId})
		}
		for _, marker := range page.DeleteMarkers {
			if marker.Key == nil {
				continue
			}
			batch = append(batch, s3types.ObjectIdentifier{Key: marker.Key, VersionId: marker.VersionId})
		}
		if err := deleteS3ObjectBatch(ctx, client, bucket, batch); err != nil {
			return err
		}

		if (page.NextKeyMarker == nil || pointerToString(page.NextKeyMarker) == "") &&
			(page.NextVersionIdMarker == nil || pointerToString(page.NextVersionIdMarker) == "") {
			break
		}

		keyMarker = page.NextKeyMarker
		versionIDMarker = page.NextVersionIdMarker
	}

	return nil
}
