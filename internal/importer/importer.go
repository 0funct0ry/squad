// Package importer parses uploaded CSV/JSON/YAML files into generic rows,
// maps their fields onto a target table's columns (or infers a schema for a
// brand-new table), and bulk-inserts the result inside a caller-owned
// transaction. It has no HTTP awareness — internal/server wires it to the
// import endpoints.
package importer

import "fmt"

// ParsedFile is the generic result of parsing an uploaded file: the column
// names in file order, and each row as a {column: value} map. Values are
// left as their natural decoded type (string for CSV, bool/float64/string/
// nil/map/slice for JSON/YAML) so InferSchema can inspect them.
type ParsedFile struct {
	Columns []string
	Rows    []map[string]any
}

// SkipMapping is the sentinel mapping target meaning "don't import this
// file column".
const SkipMapping = "__skip__"

// ValidationError is returned by ApplyFieldMapping when required target
// columns aren't covered by the supplied mapping.
type ValidationError struct {
	MissingColumns []string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("missing required column mapping for: %v", e.MissingColumns)
}
