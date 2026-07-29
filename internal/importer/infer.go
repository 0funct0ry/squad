package importer

import (
	"math"
	"strconv"
	"strings"
	"time"
)

// ColumnSpec is a package-neutral column name/SQLite-type-affinity pair
// produced by InferSchema. The caller (internal/server) converts this into
// whatever create-table request shape the create-table endpoint uses.
type ColumnSpec struct {
	Name string
	Type string
}

type valueKind int

const (
	kindUnknown valueKind = iota
	kindInteger
	kindReal
	kindText
	kindBlob
)

var dateLayouts = []string{
	time.RFC3339,
	"2006-01-02 15:04:05",
	"2006-01-02",
}

// inferValueKind classifies a single decoded value (native Go type for
// JSON/YAML, always string for CSV) into a coarse SQLite-affinity bucket.
// Date-like strings are classified as TEXT, matching SQLite's usual
// convention of storing dates as ISO-8601 text.
func inferValueKind(v any) valueKind {
	switch val := v.(type) {
	case nil:
		return kindUnknown
	case bool:
		return kindInteger
	case int, int32, int64:
		return kindInteger
	case float64:
		if val == math.Trunc(val) && !math.IsInf(val, 0) {
			return kindInteger
		}
		return kindReal
	case []byte:
		return kindBlob
	case string:
		s := strings.TrimSpace(val)
		if s == "" {
			return kindUnknown
		}
		if _, err := strconv.ParseInt(s, 10, 64); err == nil {
			return kindInteger
		}
		if _, err := strconv.ParseFloat(s, 64); err == nil {
			return kindReal
		}
		for _, layout := range dateLayouts {
			if _, err := time.Parse(layout, s); err == nil {
				return kindText
			}
		}
		return kindText
	default:
		// Nested map/slice (JSON/YAML object or array value) - no SQLite
		// scalar type fits; widen to TEXT (e.g. JSON-encoded on insert).
		return kindText
	}
}

// widen combines two kinds seen for the same column across different rows,
// following the spec's "widen to TEXT on conflict" rule, except that an
// integer/real mix stays numeric (widens to REAL) since that's still a
// single SQLite storage class family, not a genuine type conflict.
func widen(a, b valueKind) valueKind {
	if a == kindUnknown {
		return b
	}
	if b == kindUnknown {
		return a
	}
	if a == b {
		return a
	}
	if (a == kindInteger && b == kindReal) || (a == kindReal && b == kindInteger) {
		return kindReal
	}
	return kindText
}

func (k valueKind) affinity() string {
	switch k {
	case kindInteger:
		return "INTEGER"
	case kindReal:
		return "REAL"
	case kindBlob:
		return "BLOB"
	case kindText:
		return "TEXT"
	default:
		// All-null column: default to TEXT, the most permissive affinity.
		return "TEXT"
	}
}

// InferSchema infers a SQLite column type for every column in pf by
// widening the observed value kind across all rows, per column.
func InferSchema(pf ParsedFile) []ColumnSpec {
	kinds := make(map[string]valueKind, len(pf.Columns))
	for _, col := range pf.Columns {
		kinds[col] = kindUnknown
	}
	for _, row := range pf.Rows {
		for _, col := range pf.Columns {
			kinds[col] = widen(kinds[col], inferValueKind(row[col]))
		}
	}

	specs := make([]ColumnSpec, len(pf.Columns))
	for i, col := range pf.Columns {
		specs[i] = ColumnSpec{Name: col, Type: kinds[col].affinity()}
	}
	return specs
}
