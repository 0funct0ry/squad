package importer

import "testing"

func specType(specs []ColumnSpec, name string) string {
	for _, s := range specs {
		if s.Name == name {
			return s.Type
		}
	}
	return ""
}

func TestInferSchemaJSONTypes(t *testing.T) {
	pf := ParsedFile{
		Columns: []string{"id", "score", "name", "active"},
		Rows: []map[string]any{
			{"id": float64(1), "score": float64(9.5), "name": "Alice", "active": true},
			{"id": float64(2), "score": float64(3), "name": "Bob", "active": false},
		},
	}
	specs := InferSchema(pf)
	if specType(specs, "id") != "INTEGER" {
		t.Errorf("expected id -> INTEGER, got %s", specType(specs, "id"))
	}
	if specType(specs, "score") != "REAL" {
		t.Errorf("expected score -> REAL (mixed int/float widens to REAL), got %s", specType(specs, "score"))
	}
	if specType(specs, "name") != "TEXT" {
		t.Errorf("expected name -> TEXT, got %s", specType(specs, "name"))
	}
	if specType(specs, "active") != "INTEGER" {
		t.Errorf("expected active -> INTEGER (bool), got %s", specType(specs, "active"))
	}
}

func TestInferSchemaConflictWidensToText(t *testing.T) {
	pf := ParsedFile{
		Columns: []string{"mixed"},
		Rows: []map[string]any{
			{"mixed": float64(1)},
			{"mixed": "not-a-number"},
		},
	}
	specs := InferSchema(pf)
	if specType(specs, "mixed") != "TEXT" {
		t.Errorf("expected int/text conflict to widen to TEXT, got %s", specType(specs, "mixed"))
	}
}

func TestInferSchemaCSVValueSampling(t *testing.T) {
	pf := ParsedFile{
		Columns: []string{"id", "price", "label", "created_at"},
		Rows: []map[string]any{
			{"id": "1", "price": "9.99", "label": "widget", "created_at": "2024-01-15"},
			{"id": "2", "price": "19.99", "label": "gadget", "created_at": "2024-02-20"},
		},
	}
	specs := InferSchema(pf)
	if specType(specs, "id") != "INTEGER" {
		t.Errorf("expected id -> INTEGER, got %s", specType(specs, "id"))
	}
	if specType(specs, "price") != "REAL" {
		t.Errorf("expected price -> REAL, got %s", specType(specs, "price"))
	}
	if specType(specs, "label") != "TEXT" {
		t.Errorf("expected label -> TEXT, got %s", specType(specs, "label"))
	}
	if specType(specs, "created_at") != "TEXT" {
		t.Errorf("expected created_at -> TEXT (date-like), got %s", specType(specs, "created_at"))
	}
}

func TestInferSchemaAllNullDefaultsToText(t *testing.T) {
	pf := ParsedFile{
		Columns: []string{"maybe"},
		Rows: []map[string]any{
			{"maybe": nil},
			{"maybe": ""},
		},
	}
	specs := InferSchema(pf)
	if specType(specs, "maybe") != "TEXT" {
		t.Errorf("expected all-null column -> TEXT, got %s", specType(specs, "maybe"))
	}
}
