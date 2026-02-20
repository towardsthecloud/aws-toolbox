package aws

import (
	"context"
	"fmt"
)

// PageResult holds a single page of items and a continuation token.
type PageResult[T any] struct {
	Items     []T
	NextToken *string
}

// PageFetcher fetches one page of items based on the provided continuation token.
type PageFetcher[T any] func(ctx context.Context, nextToken *string) (PageResult[T], error)

// CollectAllPages walks all pages and returns a flattened list of items.
func CollectAllPages[T any](ctx context.Context, fetch PageFetcher[T]) ([]T, error) {
	items := make([]T, 0)
	seenTokens := make(map[string]struct{})

	var next *string
	for {
		page, err := fetch(ctx, next)
		if err != nil {
			return nil, err
		}

		items = append(items, page.Items...)
		if page.NextToken == nil || *page.NextToken == "" {
			return items, nil
		}

		token := *page.NextToken
		if _, exists := seenTokens[token]; exists {
			return nil, fmt.Errorf("paginator returned duplicate token %q", token)
		}
		seenTokens[token] = struct{}{}

		copiedToken := token
		next = &copiedToken
	}
}
