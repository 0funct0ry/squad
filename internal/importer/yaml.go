package importer

import (
	"fmt"
	"io"

	"github.com/goccy/go-yaml"
)

// ParseYAML reads a top-level YAML sequence of mappings into a ParsedFile,
// mirroring ParseJSON's column-ordering and missing-key behavior.
func ParseYAML(r io.Reader) (ParsedFile, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return ParsedFile{}, fmt.Errorf("failed to read YAML file: %w", err)
	}

	var raw []map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return ParsedFile{}, fmt.Errorf("expected a YAML array of mappings: %w", err)
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
			out[col] = row[col]
		}
		rows[i] = out
	}

	return ParsedFile{Columns: columns, Rows: rows}, nil
}
