package export

import (
	"bytes"
	"testing"

	"github.com/pelletier/go-toml/v2"
)

type tomlDoc struct {
	Rows []map[string]any `toml:"rows"`
}

func TestExportTOML(t *testing.T) {
	columns := []string{"id", "name", "null_col"}
	rows := [][]any{
		{int64(1), "Alice", nil},
		{int64(2), "Bob", "note"},
	}

	var buf bytes.Buffer
	if err := ExportTOML(columns, newRowSource(rows), &buf); err != nil {
		t.Fatalf("ExportTOML failed: %v", err)
	}

	var doc tomlDoc
	if err := toml.Unmarshal(buf.Bytes(), &doc); err != nil {
		t.Fatalf("failed to decode TOML: %v\n%s", err, buf.String())
	}
	if len(doc.Rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(doc.Rows))
	}
	if doc.Rows[0]["name"] != "Alice" {
		t.Errorf("unexpected row 0 name: %v", doc.Rows[0]["name"])
	}
	if _, ok := doc.Rows[0]["null_col"]; ok {
		t.Errorf("expected null_col to be omitted for nil value, got %v", doc.Rows[0]["null_col"])
	}
	if doc.Rows[1]["null_col"] != "note" {
		t.Errorf("unexpected row 1 null_col: %v", doc.Rows[1]["null_col"])
	}
}

func TestExportTOMLColumnSubset(t *testing.T) {
	columns := []string{"name"}
	rows := [][]any{{"Alice"}, {"Bob"}}

	var buf bytes.Buffer
	if err := ExportTOML(columns, newRowSource(rows), &buf); err != nil {
		t.Fatalf("ExportTOML failed: %v", err)
	}
	var doc tomlDoc
	if err := toml.Unmarshal(buf.Bytes(), &doc); err != nil {
		t.Fatalf("failed to decode TOML: %v", err)
	}
	if len(doc.Rows) != 2 || len(doc.Rows[0]) != 1 {
		t.Fatalf("unexpected structure: %+v", doc.Rows)
	}
}
