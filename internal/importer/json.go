package importer

import (
	"encoding/json"
	"fmt"
	"io"
)

// ParseJSON reads a top-level JSON array of objects into a ParsedFile.
// Column order is first-seen order across all rows (a row missing a key
// that appeared in an earlier row simply omits it, treated as nil later).
func ParseJSON(r io.Reader) (ParsedFile, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return ParsedFile{}, fmt.Errorf("failed to read JSON file: %w", err)
	}

	var raw []map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return ParsedFile{}, fmt.Errorf("expected a JSON array of objects: %w", err)
	}

	var columns []string
	seen := make(map[string]bool)
	for _, row := range raw {
		for k := range row {
			if !seen[k] {
				seen[k] = true
				columns = append(columns, k)
			}
		}
	}

	rows := make([]map[string]any, len(raw))
	for i, row := range raw {
		out := make(map[string]any, len(columns))
		for _, col := range columns {
			out[col] = row[col] // nil if absent from this row
		}
		rows[i] = out
	}

	return ParsedFile{Columns: columns, Rows: rows}, nil
}
