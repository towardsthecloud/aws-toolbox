package output

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestNewFormatter(t *testing.T) {
	tests := []struct {
		name   string
		format string
		ok     bool
	}{
		{name: "default", format: "", ok: true},
		{name: "table", format: "table", ok: true},
		{name: "json", format: "json", ok: true},
		{name: "text", format: "text", ok: true},
		{name: "invalid", format: "xml", ok: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			formatter, err := NewFormatter(tc.format)
			if tc.ok && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !tc.ok && err == nil {
				t.Fatal("expected error for invalid format")
			}
			if tc.ok && formatter == nil {
				t.Fatal("expected formatter instance")
			}
		})
	}
}

func TestTableFormatterIncludesHeadersAndRows(t *testing.T) {
	var buf bytes.Buffer
	formatter := TableFormatter{}
	data := Dataset{
		Headers: []string{"name", "state"},
		Rows: [][]string{
			{"bucket-a", "active"},
			{"bucket-b", "deleted"},
		},
	}

	if err := formatter.Format(&buf, data); err != nil {
		t.Fatalf("Format() error = %v", err)
	}

	output := buf.String()
	for _, expected := range []string{"name", "state", "bucket-a", "deleted"} {
		if !strings.Contains(output, expected) {
			t.Fatalf("table output missing %q\n%s", expected, output)
		}
	}
}

func TestJSONFormatterProducesParseableJSON(t *testing.T) {
	var buf bytes.Buffer
	formatter := JSONFormatter{}
	data := Dataset{
		Headers: []string{"id", "status"},
		Rows:    [][]string{{"1", "ok"}},
	}

	if err := formatter.Format(&buf, data); err != nil {
		t.Fatalf("Format() error = %v", err)
	}

	var records []map[string]string
	if err := json.Unmarshal(buf.Bytes(), &records); err != nil {
		t.Fatalf("JSON unmarshal error = %v\n%s", err, buf.String())
	}
	if len(records) != 1 || records[0]["status"] != "ok" {
		t.Fatalf("unexpected records: %#v", records)
	}
}

func TestTextFormatterSingleAndMultiColumn(t *testing.T) {
	var single bytes.Buffer
	if err := (TextFormatter{}).Format(&single, Dataset{Headers: []string{"id"}, Rows: [][]string{{"a"}}}); err != nil {
		t.Fatalf("single column format error: %v", err)
	}
	if single.String() != "a\n" {
		t.Fatalf("unexpected single-column output: %q", single.String())
	}

	var multi bytes.Buffer
	if err := (TextFormatter{}).Format(&multi, Dataset{Headers: []string{"id", "status"}, Rows: [][]string{{"1", "ok"}}}); err != nil {
		t.Fatalf("multi column format error: %v", err)
	}
	if multi.String() != "id=1 status=ok\n" {
		t.Fatalf("unexpected multi-column output: %q", multi.String())
	}
}

func TestFormattersHandleMissingHeadersWithGeneratedColumns(t *testing.T) {
	data := Dataset{Rows: [][]string{{"a", "b"}}}

	var table bytes.Buffer
	if err := (TableFormatter{}).Format(&table, data); err != nil {
		t.Fatalf("table format error: %v", err)
	}
	if !strings.Contains(table.String(), "column_1") {
		t.Fatalf("expected generated headers, got %q", table.String())
	}

	var text bytes.Buffer
	if err := (TextFormatter{}).Format(&text, data); err != nil {
		t.Fatalf("text format error: %v", err)
	}
	if !strings.Contains(text.String(), "column_1=a") {
		t.Fatalf("expected generated headers in text format, got %q", text.String())
	}

	var js bytes.Buffer
	if err := (JSONFormatter{}).Format(&js, data); err != nil {
		t.Fatalf("json format error: %v", err)
	}
	if !strings.Contains(js.String(), "column_2") {
		t.Fatalf("expected generated headers in json format, got %q", js.String())
	}
}

func TestNormalizeRowsPadsAndTruncates(t *testing.T) {
	rows := [][]string{
		{"a"},
		{"b", "c", "d"},
	}
	normalized := normalizeRows(rows, 2)
	if len(normalized[0]) != 2 || normalized[0][1] != "" {
		t.Fatalf("expected first row to be padded: %#v", normalized[0])
	}
	if len(normalized[1]) != 2 || normalized[1][1] != "c" {
		t.Fatalf("expected second row to be truncated: %#v", normalized[1])
	}
}

func TestTableAndTextFormatterWithEmptyDataset(t *testing.T) {
	var table bytes.Buffer
	if err := (TableFormatter{}).Format(&table, Dataset{}); err != nil {
		t.Fatalf("table format error: %v", err)
	}
	if table.String() != "" {
		t.Fatalf("expected empty table output, got %q", table.String())
	}

	var text bytes.Buffer
	if err := (TextFormatter{}).Format(&text, Dataset{}); err != nil {
		t.Fatalf("text format error: %v", err)
	}
	if text.String() != "" {
		t.Fatalf("expected empty text output, got %q", text.String())
	}
}

func TestTableFormatterFlushError(t *testing.T) {
	errWriter := &writeErrorWriter{}
	err := (TableFormatter{}).Format(errWriter, Dataset{
		Headers: []string{"a"},
		Rows:    [][]string{{"1"}},
	})
	if err == nil {
		t.Fatal("expected flush error")
	}
}

type writeErrorWriter struct{}

func (*writeErrorWriter) Write(_ []byte) (int, error) {
	return 0, errors.New("write failed")
}
