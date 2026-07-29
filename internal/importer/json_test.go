package importer

import (
	"strings"
	"testing"
)

func TestParseJSON(t *testing.T) {
	input := `[{"id":1,"name":"Alice"},{"id":2,"name":"Bob","extra":"x"}]`
	pf, err := ParseJSON(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseJSON failed: %v", err)
	}
	if len(pf.Columns) != 3 {
		t.Fatalf("expected 3 columns (union of keys), got %v", pf.Columns)
	}
	if len(pf.Rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(pf.Rows))
	}
	if pf.Rows[0]["extra"] != nil {
		t.Errorf("expected row 0 missing 'extra' key to be nil, got %v", pf.Rows[0]["extra"])
	}
	if pf.Rows[1]["extra"] != "x" {
		t.Errorf("expected row 1 'extra' to be 'x', got %v", pf.Rows[1]["extra"])
	}
}

func TestParseJSONNotArray(t *testing.T) {
	_, err := ParseJSON(strings.NewReader(`{"id":1}`))
	if err == nil {
		t.Fatal("expected an error for a non-array top-level JSON value")
	}
}
