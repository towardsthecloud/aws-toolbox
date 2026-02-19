package output

import (
	"encoding/json"
	"io"
)

// JSONFormatter emits machine-readable JSON records.
type JSONFormatter struct{}

func (JSONFormatter) Format(w io.Writer, data Dataset) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(rowsAsRecords(data))
}
