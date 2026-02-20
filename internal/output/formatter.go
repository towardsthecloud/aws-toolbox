package output

import (
	"fmt"
	"io"
	"strings"
)

// Dataset is a normalized tabular structure emitted by commands.
type Dataset struct {
	Headers []string
	Rows    [][]string
}

// Formatter writes a dataset in a specific output format.
type Formatter interface {
	Format(w io.Writer, data Dataset) error
}

// NewFormatter returns a formatter for table, json, or text output.
func NewFormatter(format string) (Formatter, error) {
	switch strings.ToLower(format) {
	case "", "table":
		return TableFormatter{}, nil
	case "json":
		return JSONFormatter{}, nil
	case "text":
		return TextFormatter{}, nil
	default:
		return nil, fmt.Errorf("unsupported output format %q", format)
	}
}

func normalizeHeaders(headers []string, rows [][]string) []string {
	if len(headers) > 0 {
		return headers
	}

	width := 0
	for _, row := range rows {
		if len(row) > width {
			width = len(row)
		}
	}

	resolved := make([]string, width)
	for i := range resolved {
		resolved[i] = fmt.Sprintf("column_%d", i+1)
	}

	return resolved
}

func normalizeRows(rows [][]string, width int) [][]string {
	normalized := make([][]string, 0, len(rows))
	for _, row := range rows {
		if len(row) >= width {
			normalized = append(normalized, row[:width])
			continue
		}

		padded := make([]string, width)
		copy(padded, row)
		normalized = append(normalized, padded)
	}

	return normalized
}

func rowsAsRecords(data Dataset) []map[string]string {
	headers := normalizeHeaders(data.Headers, data.Rows)
	rows := normalizeRows(data.Rows, len(headers))

	records := make([]map[string]string, 0, len(rows))
	for _, row := range rows {
		record := make(map[string]string, len(headers))
		for i, header := range headers {
			record[header] = row[i]
		}
		records = append(records, record)
	}

	return records
}
