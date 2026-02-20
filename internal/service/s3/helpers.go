package s3

import (
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	awstbxaws "github.com/towardsthecloud/aws-toolbox/internal/aws"
	"github.com/towardsthecloud/aws-toolbox/internal/cliutil"
)

func deleteObjectBatch(ctx context.Context, client API, bucket string, objects []s3types.ObjectIdentifier) error {
	if len(objects) == 0 {
		return nil
	}

	_, err := client.DeleteObjects(ctx, &s3.DeleteObjectsInput{
		Bucket: cliutil.Ptr(bucket),
		Delete: &s3types.Delete{
			Objects: objects,
			Quiet:   cliutil.Ptr(true),
		},
	})
	if err != nil {
		return fmt.Errorf("delete objects from bucket %s: %s", bucket, awstbxaws.FormatUserError(err))
	}

	return nil
}

func downloadObject(ctx context.Context, client API, bucket, key, targetPath string) error {
	out, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: cliutil.Ptr(bucket),
		Key:    cliutil.Ptr(key),
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

func resolveDownloadTargetPath(outputDir, relativeKey string) (string, error) {
	normalizedKey := strings.ReplaceAll(relativeKey, "\\", "/")
	normalizedKey = strings.TrimPrefix(normalizedKey, "/")
	normalizedKey = path.Clean(normalizedKey)
	if normalizedKey == "." || normalizedKey == ".." || strings.HasPrefix(normalizedKey, "../") {
		return "", fmt.Errorf("invalid object key path %q", relativeKey)
	}

	basePath := filepath.Clean(outputDir)
	targetPath := filepath.Join(basePath, filepath.FromSlash(normalizedKey))
	relPath, err := filepath.Rel(basePath, targetPath)
	if err != nil {
		return "", fmt.Errorf("resolve target path for key %q: %w", relativeKey, err)
	}
	if relPath == ".." || strings.HasPrefix(relPath, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("invalid object key path %q", relativeKey)
	}

	return targetPath, nil
}

func normalizeKeyQueries(raw []string) []string {
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

func sortObjectsByKey(objects []s3types.Object) {
	sort.Slice(objects, func(i, j int) bool {
		return objectKey(objects[i]) < objectKey(objects[j])
	})
}

func objectKey(object s3types.Object) string {
	return cliutil.PointerToString(object.Key)
}

func objectLastModified(object s3types.Object) time.Time {
	if object.LastModified == nil {
		return time.Time{}
	}
	return object.LastModified.UTC()
}

func objectSize(object s3types.Object) int64 {
	if object.Size == nil {
		return 0
	}
	return *object.Size
}
