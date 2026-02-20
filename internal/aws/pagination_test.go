package aws

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

func TestCollectAllPagesMergesPages(t *testing.T) {
	pages := map[string]PageResult[int]{
		"":        {Items: []int{1, 2}, NextToken: ptr("token-1")},
		"token-1": {Items: []int{3, 4}, NextToken: ptr("token-2")},
		"token-2": {Items: []int{5}},
	}

	got, err := CollectAllPages(context.Background(), func(_ context.Context, next *string) (PageResult[int], error) {
		token := ""
		if next != nil {
			token = *next
		}
		return pages[token], nil
	})
	if err != nil {
		t.Fatalf("CollectAllPages() error = %v", err)
	}

	want := []int{1, 2, 3, 4, 5}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("CollectAllPages() mismatch\nwant: %v\ngot:  %v", want, got)
	}
}

func TestCollectAllPagesDuplicateTokenReturnsError(t *testing.T) {
	_, err := CollectAllPages(context.Background(), func(_ context.Context, _ *string) (PageResult[int], error) {
		return PageResult[int]{Items: []int{1}, NextToken: ptr("repeat")}, nil
	})
	if err == nil {
		t.Fatal("expected duplicate token error")
	}
}

func TestCollectAllPagesPropagatesFetcherError(t *testing.T) {
	boom := errors.New("boom")
	_, err := CollectAllPages(context.Background(), func(_ context.Context, _ *string) (PageResult[int], error) {
		return PageResult[int]{}, boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("expected propagated error %v, got %v", boom, err)
	}
}

func ptr(value string) *string {
	return &value
}
