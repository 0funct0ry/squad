package importer

import (
	"errors"
	"testing"

	"github.com/0funct0ry/squad/internal/db"
)

func strPtr(s string) *string { return &s }

func TestApplyFieldMapping(t *testing.T) {
	schema := &db.TableSchema{
		Columns: []db.ColumnInfo{
			{Name: "id", NotNull: false},
			{Name: "email", NotNull: true},
			{Name: "bio", NotNull: false},
			{Name: "status", NotNull: true, DefaultVal: strPtr("'active'")},
		},
	}

	pf := ParsedFile{
		Columns: []string{"user_id", "user_email", "notes"},
		Rows: []map[string]any{
			{"user_id": "1", "user_email": "a@example.com", "notes": "hi"},
			{"user_id": "2", "user_email": "b@example.com", "notes": "yo"},
		},
	}

	t.Run("valid mapping with skip", func(t *testing.T) {
		mapping := map[string]string{
			"user_id":    "id",
			"user_email": "email",
			"notes":      SkipMapping,
		}
		rows, err := ApplyFieldMapping(pf, mapping, schema)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(rows) != 2 {
			t.Fatalf("expected 2 rows, got %d", len(rows))
		}
		if rows[0]["id"] != "1" || rows[0]["email"] != "a@example.com" {
			t.Errorf("unexpected mapped row: %v", rows[0])
		}
		if _, ok := rows[0]["notes"]; ok {
			t.Errorf("skipped column should not appear in mapped row: %v", rows[0])
		}
	})

	t.Run("missing required column blocks import", func(t *testing.T) {
		mapping := map[string]string{
			"user_id": "id",
			"notes":   "bio",
			// user_email intentionally not mapped to required "email"
		}
		_, err := ApplyFieldMapping(pf, mapping, schema)
		if err == nil {
			t.Fatal("expected a validation error for missing required column")
		}
		var verr *ValidationError
		if !errors.As(err, &verr) {
			t.Fatalf("expected *ValidationError, got %T: %v", err, err)
		}
		if len(verr.MissingColumns) != 1 || verr.MissingColumns[0] != "email" {
			t.Errorf("expected missing columns [email], got %v", verr.MissingColumns)
		}
	})

	t.Run("required column with default is not required", func(t *testing.T) {
		mapping := map[string]string{
			"user_id":    "id",
			"user_email": "email",
			// status has a default and is not mapped - should be fine
		}
		_, err := ApplyFieldMapping(pf, mapping, schema)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("extra unmapped file column is simply ignored", func(t *testing.T) {
		mapping := map[string]string{
			"user_id":    "id",
			"user_email": "email",
			// "notes" omitted entirely from mapping (not even skip)
		}
		rows, err := ApplyFieldMapping(pf, mapping, schema)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if _, ok := rows[0]["notes"]; ok {
			t.Errorf("unmapped file column should not leak into output: %v", rows[0])
		}
	})
}
