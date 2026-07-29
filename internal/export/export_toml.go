package export

import (
	"encoding/base64"
	"io"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

// ExportTOML buffers the full result set into memory and encodes it as a
// single top-level "rows" array-of-tables. Unlike the other writers, TOML
// has no natural incremental-array API to stream row-by-row, so this is the
// one writer that isn't bounded-memory streaming; acceptable for a UI
// export action rather than a bulk-data pipeline.
func ExportTOML(columns []string, source RowSource, w io.Writer) error {
	var rows []map[string]any

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
			if b, ok := val.([]byte); ok {
				obj[col] = base64.StdEncoding.EncodeToString(b)
			} else if val == nil {
				// TOML has no null; omit the key entirely for nil values.
				continue
			} else {
				obj[col] = val
			}
		}
		rows = append(rows, obj)
	}

	enc := toml.NewEncoder(w)
	return enc.Encode(map[string]any{"rows": rows})
}
