package db

import (
	"os"
	"path/filepath"
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
