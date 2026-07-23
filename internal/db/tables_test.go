package db

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTablesIntrospection(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "squad-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := OpenDB(dbPath, false)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	// Create test table
	_, err = db.Exec(`
		CREATE TABLE users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			email TEXT NOT NULL UNIQUE,
			full_name TEXT,
			is_active INTEGER NOT NULL DEFAULT 1,
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
		CREATE INDEX idx_users_email ON users(email);
		CREATE TRIGGER trigger_update_active AFTER UPDATE OF is_active ON users
		BEGIN
			UPDATE users SET is_active = 0 WHERE id = OLD.id;
		END;
		INSERT INTO users (email, full_name) VALUES ('ada@example.com', 'Ada Lovelace');
		INSERT INTO users (email, full_name) VALUES ('linus@example.com', 'Linus Torvalds');
	`)
	if err != nil {
		t.Fatalf("failed to seed db: %v", err)
	}

	// Test GetTables
	tables, err := GetTables(db)
	if err != nil {
		t.Fatalf("failed to get tables: %v", err)
	}
	if len(tables) != 1 {
		t.Errorf("expected 1 table, got %d", len(tables))
	}
	if tables[0].Name != "users" || tables[0].Type != "table" || tables[0].RowCount != 2 {
		t.Errorf("unexpected table details: %+v", tables[0])
	}

	// Test GetTableSchema
	schema, err := GetTableSchema(db, "users")
	if err != nil {
		t.Fatalf("failed to get schema: %v", err)
	}
	if len(schema.Columns) != 5 {
		t.Errorf("expected 5 columns, got %d", len(schema.Columns))
	}
	if schema.Columns[0].Name != "id" || schema.Columns[0].PK != 1 {
		t.Errorf("expected id as PK, got %+v", schema.Columns[0])
	}
	foundIndex := false
	for _, idx := range schema.Indexes {
		if idx.Name == "idx_users_email" {
			foundIndex = true
			if len(idx.Columns) != 1 || idx.Columns[0] != "email" {
				t.Errorf("expected idx_users_email columns to be ['email'], got %+v", idx.Columns)
			}
		}
	}
	if !foundIndex {
		t.Errorf("expected idx_users_email in indexes, got %+v", schema.Indexes)
	}
	if len(schema.Triggers) != 1 || schema.Triggers[0].Name != "trigger_update_active" {
		t.Errorf("expected trigger, got %+v", schema.Triggers)
	}

	// Test GetTableRows
	params := RowQueryParams{
		Limit:  1,
		Offset: 1,
	}
	res, err := GetTableRows(db, "users", params)
	if err != nil {
		t.Fatalf("failed to get rows: %v", err)
	}
	if res.Total != 2 {
		t.Errorf("expected total 2, got %d", res.Total)
	}
	if len(res.Rows) != 1 {
		t.Errorf("expected 1 row due to limit, got %d", len(res.Rows))
	}
	if res.Rows[0][1] != "linus@example.com" {
		t.Errorf("expected second row to be linus, got %+v", res.Rows[0])
	}

	// Test Filtering
	filterParams := RowQueryParams{
		Limit:   10,
		Offset:  0,
		Filters: map[string]string{"email": "ada"},
	}
	resFiltered, err := GetTableRows(db, "users", filterParams)
	if err != nil {
		t.Fatalf("failed to get filtered rows: %v", err)
	}
	if resFiltered.Total != 1 {
		t.Errorf("expected total filtered 1, got %d", resFiltered.Total)
	}
	if resFiltered.Rows[0][1] != "ada@example.com" {
		t.Errorf("expected filtered row to be ada, got %+v", resFiltered.Rows[0])
	}
}
