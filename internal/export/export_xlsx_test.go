package export

import (
	"bytes"
	"testing"

	"github.com/xuri/excelize/v2"
)

func TestExportXLSX(t *testing.T) {
	columns := []string{"id", "name", "null_col"}
	rows := [][]any{
		{int64(1), "Alice", nil},
		{int64(2), "Bob", "note"},
	}

	var buf bytes.Buffer
	if err := ExportXLSX(columns, newRowSource(rows), &buf); err != nil {
		t.Fatalf("ExportXLSX failed: %v", err)
	}

	f, err := excelize.OpenReader(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("failed to re-read xlsx: %v", err)
	}
	defer f.Close()

	rowsRead, err := f.GetRows("Sheet1")
	if err != nil {
		t.Fatalf("GetRows failed: %v", err)
	}
	if len(rowsRead) != 3 {
		t.Fatalf("expected 3 rows (1 header + 2 data), got %d", len(rowsRead))
	}
	if rowsRead[0][0] != "id" || rowsRead[0][1] != "name" || rowsRead[0][2] != "null_col" {
		t.Errorf("unexpected header: %v", rowsRead[0])
	}
	if rowsRead[1][0] != "1" || rowsRead[1][1] != "Alice" {
		t.Errorf("unexpected row 1: %v", rowsRead[1])
	}
	if rowsRead[2][2] != "note" {
		t.Errorf("unexpected row 2 null_col: %v", rowsRead[2])
	}
}

func TestExportXLSXColumnSubset(t *testing.T) {
	columns := []string{"name"}
	rows := [][]any{{"Alice"}, {"Bob"}}

	var buf bytes.Buffer
	if err := ExportXLSX(columns, newRowSource(rows), &buf); err != nil {
		t.Fatalf("ExportXLSX failed: %v", err)
	}
	f, err := excelize.OpenReader(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("failed to re-read xlsx: %v", err)
	}
	defer f.Close()

	rowsRead, err := f.GetRows("Sheet1")
	if err != nil {
		t.Fatalf("GetRows failed: %v", err)
	}
	if len(rowsRead) != 3 || len(rowsRead[0]) != 1 {
		t.Fatalf("unexpected structure: %v", rowsRead)
	}
	if rowsRead[0][0] != "name" || rowsRead[1][0] != "Alice" {
		t.Errorf("unexpected content: %v", rowsRead)
	}
}
