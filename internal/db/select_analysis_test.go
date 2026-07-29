package db

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAnalyzeSelect(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "squad-select-analysis-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "test.db")
	sqldb, err := OpenDB(dbPath, false)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer sqldb.Close()

	_, err = sqldb.Exec(`
		CREATE TABLE users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			email TEXT NOT NULL,
			name TEXT
		);
		CREATE TABLE composite (
			a INTEGER NOT NULL,
			b INTEGER NOT NULL,
			val TEXT,
			PRIMARY KEY (a, b)
		);
		CREATE TABLE norowid (
			id INTEGER,
			val TEXT
		);
		CREATE VIEW user_view AS SELECT id, email FROM users;
	`)
	if err != nil {
		t.Fatalf("failed to create schema: %v", err)
	}

	lookup := func(name string) (*TableSchema, error) {
		return GetTableSchema(sqldb, name)
	}

	positives := []struct {
		name       string
		sql        string
		wantTable  string
		wantPKCols []string
	}{
		{"select star", "SELECT * FROM users", "users", []string{"id"}},
		{"select subset with pk", "SELECT id, email FROM users WHERE email = 'x'", "users", []string{"id"}},
		{"select with alias and order/limit", `SELECT u.id, u.name FROM users AS u ORDER BY u.name LIMIT 10`, "users", []string{"id"}},
		{"composite pk both selected", "SELECT a, b, val FROM composite", "composite", []string{"a", "b"}},
		{"quoted table and column", `SELECT "id", "email" FROM "users"`, "users", []string{"id"}},
		{"trailing semicolon-free plain alias", "SELECT id, name FROM users u WHERE id > 1", "users", []string{"id"}},
	}

	for _, tc := range positives {
		t.Run(tc.name, func(t *testing.T) {
			table, pkCols, ok := AnalyzeSelect(tc.sql, lookup)
			if !ok {
				t.Fatalf("expected AnalyzeSelect to succeed for %q", tc.sql)
			}
			if table != tc.wantTable {
				t.Errorf("table = %q, want %q", table, tc.wantTable)
			}
			if len(pkCols) != len(tc.wantPKCols) {
				t.Fatalf("pkCols = %v, want %v", pkCols, tc.wantPKCols)
			}
			for i, c := range tc.wantPKCols {
				if pkCols[i] != c {
					t.Errorf("pkCols[%d] = %q, want %q", i, pkCols[i], c)
				}
			}
		})
	}

	negatives := []struct {
		name string
		sql  string
	}{
		{"join", "SELECT users.id, composite.val FROM users JOIN composite ON users.id = composite.a"},
		{"aggregate count", "SELECT COUNT(*) FROM users"},
		{"group by", "SELECT name, COUNT(*) FROM users GROUP BY name"},
		{"distinct", "SELECT DISTINCT name FROM users"},
		{"subquery from", "SELECT * FROM (SELECT * FROM users)"},
		{"cte", "WITH x AS (SELECT * FROM users) SELECT * FROM x"},
		{"multi-table from", "SELECT * FROM users, composite"},
		{"union", "SELECT id FROM users UNION SELECT a FROM composite"},
		{"computed column", "SELECT id, email || name FROM users"},
		{"view", "SELECT * FROM user_view"},
		{"missing pk column, explicit list", "SELECT email, name FROM users"},
		{"missing pk composite (only one of two)", "SELECT a, val FROM composite"},
		{"without-rowid-like table missing pk selection", "SELECT val FROM norowid"},
	}

	for _, tc := range negatives {
		t.Run(tc.name, func(t *testing.T) {
			_, _, ok := AnalyzeSelect(tc.sql, lookup)
			if ok {
				t.Fatalf("expected AnalyzeSelect to reject %q", tc.sql)
			}
		})
	}
}
