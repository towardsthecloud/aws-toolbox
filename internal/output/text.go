package output

import (
	"fmt"
	"io"
	"strings"
)

// TextFormatter emits shell-friendly line-oriented output.
type TextFormatter struct{}

func (TextFormatter) Format(w io.Writer, data Dataset) error {
	headers := normalizeHeaders(data.Headers, data.Rows)
	if len(headers) == 0 {
		return nil
	}

	for _, row := range normalizeRows(data.Rows, len(headers)) {
		line := row[0]
		if len(headers) > 1 {
			parts := make([]string, len(headers))
			for i, header := range headers {
				parts[i] = fmt.Sprintf("%s=%s", header, row[i])
			}
			line = strings.Join(parts, " ")
		}

		if _, err := fmt.Fprintln(w, line); err != nil {
			return err
		}
	}

	return nil
}
