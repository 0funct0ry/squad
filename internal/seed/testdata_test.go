package seed

import (
	"database/sql"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/0funct0ry/squad/internal/db"
)

// openScratchExample copies examples/<name>.db into a temp file and opens it
// read-write, so tests never mutate the checked-in fixtures.
func openScratchExample(t *testing.T, name string) *sql.DB {
	t.Helper()

	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("failed to resolve caller for examples path")
	}
	src := filepath.Join(filepath.Dir(thisFile), "..", "..", "examples", name+".db")

	srcBytes, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("failed to read example db %s: %v", src, err)
	}

	dst := filepath.Join(t.TempDir(), name+".db")
	if err := os.WriteFile(dst, srcBytes, 0o644); err != nil {
		t.Fatalf("failed to write scratch db: %v", err)
	}

	database, err := db.OpenDB(dst, false)
	if err != nil {
		t.Fatalf("failed to open scratch db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	return database
}

func openScratchDB(t *testing.T) *sql.DB {
	t.Helper()
	dst := filepath.Join(t.TempDir(), "scratch.db")
	database, err := db.OpenDB(dst, false)
	if err != nil {
		t.Fatalf("failed to open scratch db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	return database
}
