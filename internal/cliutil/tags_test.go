package cliutil

import "testing"

func TestParseTagFilter(t *testing.T) {
	key, value, err := ParseTagFilter("")
	if err != nil || key != "" || value != "" {
		t.Fatalf("empty tag filter: key=%q value=%q err=%v", key, value, err)
	}

	key, value, err = ParseTagFilter("env=dev")
	if err != nil || key != "env" || value != "dev" {
		t.Fatalf("valid tag filter: key=%q value=%q err=%v", key, value, err)
	}

	_, _, err = ParseTagFilter("invalid")
	if err == nil {
		t.Fatal("expected error for invalid tag filter")
	}

	_, _, err = ParseTagFilter("=value")
	if err == nil {
		t.Fatal("expected error for empty key")
	}
}
