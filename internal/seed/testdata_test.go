package seed

import (
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/0funct0ry/squad/internal/db"
	"github.com/0funct0ry/squad/internal/testfixtures"
)

// openScratchExample builds the named fixture database (blog/library/types_zoo)
// fresh into a temp file and opens it read-write.
func openScratchExample(t *testing.T, name string) *sql.DB {
	t.Helper()

	dst := filepath.Join(t.TempDir(), name+".db")
	if err := buildFixture(name, dst); err != nil {
		t.Fatalf("failed to build fixture db %s: %v", name, err)
	}

	database, err := db.OpenDB(dst, false)
	if err != nil {
		t.Fatalf("failed to open scratch db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	return database
}

func buildFixture(name, path string) error {
	switch name {
	case "blog":
		return testfixtures.BuildBlog(path)
	case "library":
		return testfixtures.BuildLibrary(path)
	case "types_zoo":
		return testfixtures.BuildTypesZoo(path)
	default:
		panic("testfixtures: unknown fixture " + name)
	}
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
