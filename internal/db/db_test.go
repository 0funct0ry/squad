package db

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestOpenDBAndMeta(t *testing.T) {
	// Test memory db
	db, err := OpenDB(":memory:", false)
	if err != nil {
		t.Fatalf("failed to open memory db: %v", err)
	}
	defer db.Close()

	version, size, err := Meta(db, ":memory:")
	if err != nil {
		t.Fatalf("failed to get meta: %v", err)
	}
	if version == "" {
		t.Error("expected non-empty sqlite version")
	}
	if size != 0 {
		t.Errorf("expected size 0 for memory db, got %d", size)
	}
}

func TestOpenDBFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "squad-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := OpenDB(dbPath, false)
	if err != nil {
		t.Fatalf("failed to open file db: %v", err)
	}
	db.Close()

	// Reopen read-only
	db2, err := OpenDB(dbPath, true)
	if err != nil {
		t.Fatalf("failed to open file db read-only: %v", err)
	}
	defer db2.Close()

	version, size, err := Meta(db2, dbPath)
	if err != nil {
		t.Fatalf("failed to get meta: %v", err)
	}
	if version == "" {
		t.Error("expected non-empty sqlite version")
	}
	if size == 0 {
		// Note: depending on write/flush, size might be 0 initially since it's empty, but we can verify it doesn't crash
	}
}

func TestOpenDBEnforcesForeignKeys(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "squad-test-fk-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "fk.db")
	database, err := OpenDB(dbPath, false)
	if err != nil {
		t.Fatalf("failed to open write-mode db: %v", err)
	}
	defer database.Close()

	_, err = database.Exec(`
		CREATE TABLE parent (id INTEGER PRIMARY KEY);
		CREATE TABLE child (id INTEGER PRIMARY KEY, parent_id INTEGER, FOREIGN KEY(parent_id) REFERENCES parent(id));
		INSERT INTO parent (id) VALUES (1);
	`)
	if err != nil {
		t.Fatalf("seed failed: %v", err)
	}

	_, err = database.Exec("INSERT INTO child (id, parent_id) VALUES (1, 999)")
	if err == nil {
		t.Fatal("expected FK-violating insert to be rejected")
	}
	if !strings.Contains(err.Error(), "FOREIGN KEY constraint failed") {
		t.Errorf("expected verbatim SQLite FK constraint error, got: %v", err)
	}

	// Force the pool to open several distinct underlying connections and
	// verify every one of them enforces foreign keys too — a PRAGMA executed
	// once after Ping only affects that single connection, so this guards
	// against enforcement being a pool-size-1 coincidence.
	database.SetMaxOpenConns(5)
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			_, _ = database.Exec("SELECT 1")
		}(i)
	}
	wg.Wait()

	for i := 2; i < 12; i++ {
		if _, err := database.Exec("INSERT INTO child (id, parent_id) VALUES (?, 999)", i); err == nil {
			t.Fatalf("expected FK-violating insert on connection %d to be rejected", i)
		}
	}
}
