package main

import (
	"testing"
	"time"
)

func TestResolveManHeaderDateDefaultsToUnixEpoch(t *testing.T) {
	t.Setenv("SOURCE_DATE_EPOCH", "")

	got, err := resolveManHeaderDate()
	if err != nil {
		t.Fatalf("resolveManHeaderDate() error = %v", err)
	}

	want := time.Unix(0, 0).UTC()
	if !got.Equal(want) {
		t.Fatalf("resolveManHeaderDate() = %v, want %v", got, want)
	}
}

func TestResolveManHeaderDateUsesSourceDateEpoch(t *testing.T) {
	t.Setenv("SOURCE_DATE_EPOCH", "1700000000")

	got, err := resolveManHeaderDate()
	if err != nil {
		t.Fatalf("resolveManHeaderDate() error = %v", err)
	}

	want := time.Unix(1700000000, 0).UTC()
	if !got.Equal(want) {
		t.Fatalf("resolveManHeaderDate() = %v, want %v", got, want)
	}
}

func TestResolveManHeaderDateRejectsInvalidSourceDateEpoch(t *testing.T) {
	t.Setenv("SOURCE_DATE_EPOCH", "not-a-number")

	if _, err := resolveManHeaderDate(); err == nil {
		t.Fatal("resolveManHeaderDate() expected error for invalid SOURCE_DATE_EPOCH")
	}
}
