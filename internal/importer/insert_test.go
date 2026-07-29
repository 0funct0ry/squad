package importer

import (
	"path/filepath"
	"testing"

	"github.com/0funct0ry/squad/internal/db"
)

func TestBulkInsertRows(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	sqldb, err := db.OpenDB(dbPath, false)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer sqldb.Close()

	if _, err := sqldb.Exec("CREATE TABLE items (id INTEGER PRIMARY KEY, name TEXT, price REAL)"); err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	tx, err := sqldb.Begin()
	if err != nil {
		t.Fatalf("failed to begin tx: %v", err)
	}

	rows := []map[string]any{
		{"id": int64(1), "name": "widget", "price": 9.99},
		{"id": int64(2), "name": "gadget", "price": 19.99},
	}
	n, err := BulkInsertRows(tx, "items", []string{"id", "name", "price"}, rows)
	if err != nil {
		t.Fatalf("BulkInsertRows failed: %v", err)
	}
	if n != 2 {
		t.Errorf("expected 2 rows inserted, got %d", n)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit failed: %v", err)
	}

	var count int
	sqldb.QueryRow("SELECT COUNT(*) FROM items").Scan(&count)
	if count != 2 {
		t.Errorf("expected 2 rows in table, got %d", count)
	}
}

func TestBulkInsertRowsRollsBackOnFailure(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	sqldb, err := db.OpenDB(dbPath, false)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer sqldb.Close()

	if _, err := sqldb.Exec("CREATE TABLE items (id INTEGER PRIMARY KEY, name TEXT NOT NULL)"); err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	tx, err := sqldb.Begin()
	if err != nil {
		t.Fatalf("failed to begin tx: %v", err)
	}

	rows := []map[string]any{
		{"id": int64(1), "name": "ok"},
		{"id": int64(2), "name": nil}, // violates NOT NULL
	}
	_, err = BulkInsertRows(tx, "items", []string{"id", "name"}, rows)
	if err == nil {
		t.Fatal("expected an error from the NOT NULL violation")
	}
	tx.Rollback()

	var count int
	sqldb.QueryRow("SELECT COUNT(*) FROM items").Scan(&count)
	if count != 0 {
		t.Errorf("expected 0 rows after rollback, got %d", count)
	}
}
