package export

import (
	"bytes"
	"testing"

	"github.com/goccy/go-yaml"
)

func TestExportYAML(t *testing.T) {
	columns := []string{"id", "name", "note", "null_col"}
	rows := [][]any{
		{int64(1), "Alice", "hello: world", nil},
		{int64(2), "Bob \"The King\"", "plain", "not null"},
	}

	var buf bytes.Buffer
	if err := ExportYAML(columns, newRowSource(rows), &buf); err != nil {
		t.Fatalf("ExportYAML failed: %v", err)
	}

	var decoded []map[string]any
	if err := yaml.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("failed to decode YAML: %v\n%s", err, buf.String())
	}
	if len(decoded) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(decoded))
	}
	if decoded[0]["name"] != "Alice" {
		t.Errorf("unexpected row 0 name: %v", decoded[0]["name"])
	}
	if decoded[0]["note"] != "hello: world" {
		t.Errorf("unexpected row 0 note: %v", decoded[0]["note"])
	}
	if decoded[0]["null_col"] != nil {
		t.Errorf("expected row 0 null_col to decode as nil, got %v", decoded[0]["null_col"])
	}
	if decoded[1]["name"] != `Bob "The King"` {
		t.Errorf("unexpected row 1 name: %v", decoded[1]["name"])
	}
	if decoded[1]["null_col"] != "not null" {
		t.Errorf("unexpected row 1 null_col: %v", decoded[1]["null_col"])
	}
}

func TestExportYAMLColumnSubset(t *testing.T) {
	columns := []string{"name"}
	rows := [][]any{{"Alice"}, {"Bob"}}

	var buf bytes.Buffer
	if err := ExportYAML(columns, newRowSource(rows), &buf); err != nil {
		t.Fatalf("ExportYAML failed: %v", err)
	}
	var decoded []map[string]any
	if err := yaml.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("failed to decode YAML: %v", err)
	}
	if len(decoded) != 2 || len(decoded[0]) != 1 {
		t.Fatalf("unexpected structure: %+v", decoded)
	}
	if decoded[0]["name"] != "Alice" {
		t.Errorf("unexpected value: %v", decoded[0]["name"])
	}
}
