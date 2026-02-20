package cliutil

import (
	"fmt"
	"strings"
)

// ParseTagFilter parses a "KEY=VALUE" string into its key and value parts.
// Returns empty strings without error when raw is empty.
func ParseTagFilter(raw string) (string, string, error) {
	if raw == "" {
		return "", "", nil
	}

	parts := strings.SplitN(raw, "=", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" {
		return "", "", fmt.Errorf("--filter-tag must use KEY=VALUE format")
	}

	return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]), nil
}
