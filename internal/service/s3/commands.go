package s3

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/spf13/cobra"
	awstbxaws "github.com/towardsthecloud/aws-toolbox/internal/aws"
	"github.com/towardsthecloud/aws-toolbox/internal/cliutil"
)

func runDeleteBuckets(cmd *cobra.Command, emptyOnly bool, filterNameContains string) error {
	filterNameContains = strings.TrimSpace(filterNameContains)
	if !emptyOnly && filterNameContains == "" {
		return fmt.Errorf("set --empty or --filter-name-contains")
	}

	runtime, _, client, err := cliutil.NewServiceRuntime(cmd, loadAWSConfig, newClient)
	if err != nil {
		return err
	}

	buckets, err := listBuckets(cmd.Context(), client)
	if err != nil {
		return fmt.Errorf("list buckets: %s", awstbxaws.FormatUserError(err))
	}

	targets := make([]string, 0, len(buckets))
	for _, bucket := range buckets {
		name := cliutil.PointerToString(bucket.Name)
		if name == "" {
			continue
		}
		if filterNameContains != "" && !strings.Contains(name, filterNameContains) {
			continue
		}
		if emptyOnly {
			ok, checkErr := isBucketEmptyAndUnversioned(cmd.Context(), client, name)
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
		action := cliutil.ActionWouldDelete
		if !runtime.Options.DryRun {
			action = cliutil.ActionPending
		}
		rows = append(rows, []string{name, action})
	}

	return cliutil.RunDestructiveActionPlan(cmd, runtime, cliutil.DestructiveActionPlan{
		Headers:       []string{"bucket", "action"},
		Rows:          rows,
		ActionColumn:  1,
		ConfirmPrompt: fmt.Sprintf("Delete %d S3 bucket(s)", len(rows)),
		Execute: func(rowIndex int) string {
			bucket := rows[rowIndex][0]
			if clearErr := deleteAllObjectsFromBucket(cmd.Context(), client, bucket); clearErr != nil {
				return cliutil.FailedAction(clearErr)
			}
			_, deleteErr := client.DeleteBucket(cmd.Context(), &s3.DeleteBucketInput{Bucket: cliutil.Ptr(bucket)})
			if deleteErr != nil {
				return cliutil.FailedActionMessage(awstbxaws.FormatUserError(deleteErr))
			}
			return cliutil.ActionDeleted
		},
	})
}

func runDownloadBucket(cmd *cobra.Command, bucket, prefix, outputDir string) error {
	if strings.TrimSpace(bucket) == "" {
		return fmt.Errorf("--bucket-name is required")
	}
	if strings.TrimSpace(prefix) == "" {
		return fmt.Errorf("--prefix is required")
	}

	runtime, _, client, err := cliutil.NewServiceRuntime(cmd, loadAWSConfig, newClient)
	if err != nil {
		return err
	}

	objects, err := listObjects(cmd.Context(), client, bucket, prefix)
	if err != nil {
		return fmt.Errorf("list objects: %s", awstbxaws.FormatUserError(err))
	}
	sortObjectsByKey(objects)

	rows := make([][]string, 0, len(objects))
	for _, object := range objects {
		key := objectKey(object)
		relativeKey := strings.TrimPrefix(key, prefix)
		relativeKey = strings.TrimPrefix(relativeKey, "/")
		if relativeKey == "" {
			relativeKey = key
		}
		targetPath, pathErr := resolveDownloadTargetPath(outputDir, relativeKey)
		if pathErr != nil {
			rows = append(rows, []string{bucket, key, "", cliutil.FailedAction(pathErr)})
			continue
		}

		action := "would-download"
		if runtime.Options.DryRun {
			rows = append(rows, []string{bucket, key, targetPath, action})
			continue
		}

		if err := downloadObject(cmd.Context(), client, bucket, key, targetPath); err != nil {
			action = cliutil.FailedAction(err)
		} else {
			action = "downloaded"
		}
		rows = append(rows, []string{bucket, key, targetPath, action})
	}

	return cliutil.WriteDataset(cmd, runtime, []string{"bucket", "key", "target_path", "action"}, rows)
}

func runListOldFiles(cmd *cobra.Command, bucket, prefix string, olderThanDays int) error {
	if strings.TrimSpace(bucket) == "" {
		return fmt.Errorf("--bucket-name is required")
	}
	if olderThanDays < 0 {
		return fmt.Errorf("--older-than-days must be >= 0")
	}

	runtime, _, client, err := cliutil.NewServiceRuntime(cmd, loadAWSConfig, newClient)
	if err != nil {
		return err
	}

	objects, err := listObjects(cmd.Context(), client, bucket, prefix)
	if err != nil {
		return fmt.Errorf("list objects: %s", awstbxaws.FormatUserError(err))
	}
	sortObjectsByKey(objects)

	now := time.Now().UTC()
	rows := make([][]string, 0, len(objects))
	for _, object := range objects {
		lastModified := objectLastModified(object)
		if lastModified.IsZero() {
			continue
		}
		ageDays := int(now.Sub(lastModified).Hours() / 24)
		if ageDays < olderThanDays {
			continue
		}
		rows = append(rows, []string{
			bucket,
			objectKey(object),
			lastModified.Format(time.RFC3339),
			fmt.Sprintf("%d", ageDays),
			fmt.Sprintf("%d", objectSize(object)),
		})
	}

	return cliutil.WriteDataset(cmd, runtime, []string{"bucket", "key", "last_modified", "age_days", "size_bytes"}, rows)
}

func runSearchObjects(cmd *cobra.Command, bucket, prefix string, keys []string) error {
	if strings.TrimSpace(bucket) == "" {
		return fmt.Errorf("--bucket-name is required")
	}

	queries := normalizeKeyQueries(keys)
	if strings.TrimSpace(prefix) == "" && len(queries) == 0 {
		return fmt.Errorf("set --prefix and/or --keys")
	}

	runtime, _, client, err := cliutil.NewServiceRuntime(cmd, loadAWSConfig, newClient)
	if err != nil {
		return err
	}

	objects, err := listObjects(cmd.Context(), client, bucket, prefix)
	if err != nil {
		return fmt.Errorf("list objects: %s", awstbxaws.FormatUserError(err))
	}
	sortObjectsByKey(objects)

	if len(queries) == 0 {
		rows := make([][]string, 0, len(objects))
		for _, object := range objects {
			rows = append(rows, []string{
				bucket,
				objectKey(object),
				"true",
				objectLastModified(object).Format(time.RFC3339),
				fmt.Sprintf("%d", objectSize(object)),
			})
		}
		return cliutil.WriteDataset(cmd, runtime, []string{"bucket", "key", "exists", "last_modified", "size_bytes"}, rows)
	}

	objectByKey := make(map[string]s3types.Object, len(objects))
	for _, object := range objects {
		objectByKey[objectKey(object)] = object
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
			if ts := objectLastModified(object); !ts.IsZero() {
				lastModified = ts.Format(time.RFC3339)
			}
			size = fmt.Sprintf("%d", objectSize(object))
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

	return cliutil.WriteDataset(cmd, runtime, []string{"bucket", "query_key", "matched_key", "exists", "last_modified", "size_bytes"}, rows)
}

func listBuckets(ctx context.Context, client API) ([]s3types.Bucket, error) {
	out, err := client.ListBuckets(ctx, &s3.ListBucketsInput{})
	if err != nil {
		return nil, err
	}
	return out.Buckets, nil
}

func listObjects(ctx context.Context, client API, bucket, prefix string) ([]s3types.Object, error) {
	objects := make([]s3types.Object, 0)
	var continuationToken *string

	for {
		input := &s3.ListObjectsV2Input{
			Bucket:            cliutil.Ptr(bucket),
			ContinuationToken: continuationToken,
		}
		if strings.TrimSpace(prefix) != "" {
			input.Prefix = cliutil.Ptr(prefix)
		}

		out, err := client.ListObjectsV2(ctx, input)
		if err != nil {
			return nil, err
		}

		objects = append(objects, out.Contents...)
		if out.NextContinuationToken == nil || cliutil.PointerToString(out.NextContinuationToken) == "" {
			break
		}
		continuationToken = out.NextContinuationToken
	}

	return objects, nil
}

func isBucketEmptyAndUnversioned(ctx context.Context, client API, bucket string) (bool, error) {
	objects, err := client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket:  cliutil.Ptr(bucket),
		MaxKeys: cliutil.Ptr(int32(1)),
	})
	if err != nil {
		return false, fmt.Errorf("list objects for bucket %s: %s", bucket, awstbxaws.FormatUserError(err))
	}
	if len(objects.Contents) > 0 {
		return false, nil
	}

	versioning, err := client.GetBucketVersioning(ctx, &s3.GetBucketVersioningInput{Bucket: cliutil.Ptr(bucket)})
	if err != nil {
		return false, fmt.Errorf("get versioning for bucket %s: %s", bucket, awstbxaws.FormatUserError(err))
	}

	return versioning.Status != s3types.BucketVersioningStatusEnabled, nil
}

func deleteAllObjectsFromBucket(ctx context.Context, client API, bucket string) error {
	// Delete regular objects first.
	var continuationToken *string
	for {
		page, err := client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
			Bucket:            cliutil.Ptr(bucket),
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
		if err := deleteObjectBatch(ctx, client, bucket, batch); err != nil {
			return err
		}

		if page.NextContinuationToken == nil || cliutil.PointerToString(page.NextContinuationToken) == "" {
			break
		}
		continuationToken = page.NextContinuationToken
	}

	// Keep this loop local instead of CollectAllPages: ListObjectVersions needs two continuation markers
	// (`KeyMarker` and `VersionIdMarker`), while the shared paginator supports one token.
	// Delete versioned objects + delete markers.
	var keyMarker *string
	var versionIDMarker *string
	for {
		page, err := client.ListObjectVersions(ctx, &s3.ListObjectVersionsInput{
			Bucket:          cliutil.Ptr(bucket),
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
		if err := deleteObjectBatch(ctx, client, bucket, batch); err != nil {
			return err
		}

		if (page.NextKeyMarker == nil || cliutil.PointerToString(page.NextKeyMarker) == "") &&
			(page.NextVersionIdMarker == nil || cliutil.PointerToString(page.NextVersionIdMarker) == "") {
			break
		}

		keyMarker = page.NextKeyMarker
		versionIDMarker = page.NextVersionIdMarker
	}

	return nil
}
