package importer

import (
	"strings"
	"testing"
)

func TestParseCSV(t *testing.T) {
	input := "id,name,age\n1,Alice,30\n2,Bob,25\n"
	pf, err := ParseCSV(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseCSV failed: %v", err)
	}
	if len(pf.Columns) != 3 || pf.Columns[0] != "id" || pf.Columns[1] != "name" || pf.Columns[2] != "age" {
		t.Fatalf("unexpected columns: %v", pf.Columns)
	}
	if len(pf.Rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(pf.Rows))
	}
	if pf.Rows[0]["name"] != "Alice" || pf.Rows[0]["age"] != "30" {
		t.Errorf("unexpected row 0: %v", pf.Rows[0])
	}
	if pf.Rows[1]["name"] != "Bob" {
		t.Errorf("unexpected row 1: %v", pf.Rows[1])
	}
}

func TestParseCSVRaggedRow(t *testing.T) {
	input := "id,name,age\n1,Alice\n"
	pf, err := ParseCSV(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseCSV failed: %v", err)
	}
	if pf.Rows[0]["age"] != nil {
		t.Errorf("expected missing trailing column to be nil, got %v", pf.Rows[0]["age"])
	}
}

func TestParseCSVNoHeader(t *testing.T) {
	_, err := ParseCSV(strings.NewReader(""))
	if err == nil {
		t.Fatal("expected an error for an empty CSV file")
	}
}
