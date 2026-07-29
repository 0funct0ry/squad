package export

import (
	"encoding/base64"
	"fmt"
	"io"
	"strings"

	"github.com/parquet-go/parquet-go"
)

// ExportParquet writes rows as a parquet file with a single row group built
// from the RowSource. Every column is declared as an optional UTF8 (string)
// leaf - a straightforward tabular serialization matching the plain-text
// convention the other generic writers (CSV/SQL/YAML) already use, rather
// than attempting to preserve SQLite's dynamic per-value typing as
// per-column parquet physical types. The parquet-go writer manages its own
// row-group buffering/flushing internally, so this still only holds one row
// group's worth of data in memory at a time, not the whole result set.
func ExportParquet(columns []string, source RowSource, w io.Writer) error {
	group := make(parquet.Group, len(columns))
	for _, col := range columns {
		group[col] = parquet.Optional(parquet.String())
	}
	schema := parquet.NewSchema("row", group)

	pw := parquet.NewGenericWriter[map[string]any](w, schema)

	for {
		row, err := source()
		if err != nil {
			if err == io.EOF || strings.Contains(err.Error(), "EOF") {
				break
			}
			return err
		}

		obj := make(map[string]any, len(columns))
		for i, col := range columns {
			val := row[i]
			if val == nil {
				continue // optional column, definition level 0 (null)
			}
			if b, ok := val.([]byte); ok {
				obj[col] = base64.StdEncoding.EncodeToString(b)
			} else {
				obj[col] = fmt.Sprintf("%v", val)
			}
		}

		if _, err := pw.Write([]map[string]any{obj}); err != nil {
			return fmt.Errorf("failed to write parquet row: %w", err)
		}
	}

	return pw.Close()
}
