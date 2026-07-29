package export

import (
	"bytes"
	"testing"

	"github.com/parquet-go/parquet-go"
)

func TestExportParquet(t *testing.T) {
	columns := []string{"id", "name", "null_col"}
	rows := [][]any{
		{int64(1), "Alice", nil},
		{int64(2), "Bob", "note"},
	}

	var buf bytes.Buffer
	if err := ExportParquet(columns, newRowSource(rows), &buf); err != nil {
		t.Fatalf("ExportParquet failed: %v", err)
	}

	decoded := readParquetRows(t, buf.Bytes())
	if len(decoded) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(decoded))
	}
	if decoded[0]["name"] != "Alice" {
		t.Errorf("unexpected row 0 name: %v", decoded[0]["name"])
	}
	if decoded[0]["id"] != "1" {
		t.Errorf("unexpected row 0 id: %v", decoded[0]["id"])
	}
	if v, ok := decoded[0]["null_col"]; ok && v != nil {
		t.Errorf("expected row 0 null_col to be nil/absent, got %v", v)
	}
	if decoded[1]["null_col"] != "note" {
		t.Errorf("unexpected row 1 null_col: %v", decoded[1]["null_col"])
	}
}

// readParquetRows opens a parquet file written with a dynamic (map-based)
// schema and reads it back generically. parquet.Read[T] can't infer a
// schema from a map type via reflection alone, so the file's own embedded
// schema is read first and passed explicitly as a ReaderOption.
func readParquetRows(t *testing.T, data []byte) []map[string]any {
	t.Helper()
	f, err := parquet.OpenFile(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("failed to open parquet file: %v", err)
	}

	reader := parquet.NewGenericReader[map[string]any](bytes.NewReader(data), f.Schema())
	defer reader.Close()

	rows := make([]map[string]any, f.NumRows())
	for i := range rows {
		rows[i] = map[string]any{}
	}
	n, err := reader.Read(rows)
	if err != nil && n < len(rows) {
		t.Fatalf("failed to read parquet rows: %v", err)
	}
	return rows[:n]
}

func TestExportParquetColumnSubset(t *testing.T) {
	columns := []string{"name"}
	rows := [][]any{{"Alice"}, {"Bob"}}

	var buf bytes.Buffer
	if err := ExportParquet(columns, newRowSource(rows), &buf); err != nil {
		t.Fatalf("ExportParquet failed: %v", err)
	}
	decoded := readParquetRows(t, buf.Bytes())
	if len(decoded) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(decoded))
	}
	if decoded[0]["name"] != "Alice" {
		t.Errorf("unexpected value: %v", decoded[0]["name"])
	}
}
