package output

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
)

// TableFormatter emits aligned tabular output for humans.
type TableFormatter struct{}

func (TableFormatter) Format(w io.Writer, data Dataset) error {
	headers := normalizeHeaders(data.Headers, data.Rows)
	if len(headers) == 0 {
		return nil
	}

	rows := normalizeRows(data.Rows, len(headers))
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)

	if _, err := fmt.Fprintln(tw, strings.Join(headers, "\t")); err != nil {
		return err
	}

	for _, row := range rows {
		if _, err := fmt.Fprintln(tw, strings.Join(row, "\t")); err != nil {
			return err
		}
	}

	return tw.Flush()
}
