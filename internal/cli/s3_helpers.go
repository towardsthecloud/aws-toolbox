package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	awstbxaws "github.com/towardsthecloud/aws-toolbox/internal/aws"
)

func deleteS3ObjectBatch(ctx context.Context, client s3API, bucket string, objects []s3types.ObjectIdentifier) error {
	if len(objects) == 0 {
		return nil
	}

	_, err := client.DeleteObjects(ctx, &s3.DeleteObjectsInput{
		Bucket: ptr(bucket),
		Delete: &s3types.Delete{
			Objects: objects,
			Quiet:   ptr(true),
		},
	})
	if err != nil {
		return fmt.Errorf("delete objects from bucket %s: %s", bucket, awstbxaws.FormatUserError(err))
	}

	return nil
}

func downloadS3Object(ctx context.Context, client s3API, bucket, key, targetPath string) error {
	out, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: ptr(bucket),
		Key:    ptr(key),
	})
	if err != nil {
		return fmt.Errorf("download %s: %s", key, awstbxaws.FormatUserError(err))
	}
	defer out.Body.Close()

	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return fmt.Errorf("create directory for %s: %w", targetPath, err)
	}

	f, err := os.Create(targetPath)
	if err != nil {
		return fmt.Errorf("create file %s: %w", targetPath, err)
	}
	defer f.Close()

	if _, err := io.Copy(f, out.Body); err != nil {
		return fmt.Errorf("write file %s: %w", targetPath, err)
	}

	return nil
}

func normalizeS3KeyQueries(raw []string) []string {
	queries := make([]string, 0, len(raw))
	seen := make(map[string]struct{}, len(raw))

	for _, item := range raw {
		for _, part := range strings.Split(item, ",") {
			key := strings.TrimSpace(part)
			if key == "" {
				continue
			}
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			queries = append(queries, key)
		}
	}

	return queries
}

func sortS3ObjectsByKey(objects []s3types.Object) {
	sort.Slice(objects, func(i, j int) bool {
		return s3ObjectKey(objects[i]) < s3ObjectKey(objects[j])
	})
}

func s3ObjectKey(object s3types.Object) string {
	return pointerToString(object.Key)
}

func s3ObjectLastModified(object s3types.Object) time.Time {
	if object.LastModified == nil {
		return time.Time{}
	}
	return object.LastModified.UTC()
}

func s3ObjectSize(object s3types.Object) int64 {
	if object.Size == nil {
		return 0
	}
	return *object.Size
}
