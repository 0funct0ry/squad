package importer

import (
	"strings"
	"testing"
)

func TestParseYAML(t *testing.T) {
	input := "- id: 1\n  name: Alice\n- id: 2\n  name: Bob\n  extra: x\n"
	pf, err := ParseYAML(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseYAML failed: %v", err)
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

func TestParseYAMLNotSequence(t *testing.T) {
	_, err := ParseYAML(strings.NewReader("id: 1\n"))
	if err == nil {
		t.Fatal("expected an error for a non-sequence top-level YAML value")
	}
}
