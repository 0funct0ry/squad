package examples

import (
	"database/sql"
	"strings"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/0funct0ry/squad/internal/db"
)

func TestExampleSchemas(t *testing.T) {
	for _, ex := range All() {
		ex := ex
		t.Run(ex.Slug, func(t *testing.T) {
			sqldb, err := sql.Open("sqlite", ":memory:")
			if err != nil {
				t.Fatalf("open in-memory db: %v", err)
			}
			defer sqldb.Close()

			stmts, err := db.SplitStatements(ex.Schema)
			if err != nil {
				t.Fatalf("splitting schema statements: %v", err)
			}
			for _, stmt := range stmts {
				stmt = strings.TrimSpace(stmt)
				if stmt == "" {
					continue
				}
				if _, err := sqldb.Exec(stmt); err != nil {
					t.Fatalf("executing statement failed: %v\nstatement:\n%s", err, stmt)
				}
			}

			upper := strings.ToUpper(ex.Schema)
			assertObjectCount(t, sqldb, "table", "CREATE TABLE", upper)
			assertObjectCount(t, sqldb, "view", "CREATE VIEW", upper)
			assertObjectCount(t, sqldb, "trigger", "CREATE TRIGGER", upper)
			assertObjectCount(t, sqldb, "index", "CREATE INDEX", upper)

			rows, err := sqldb.Query("PRAGMA foreign_key_check")
			if err != nil {
				t.Fatalf("foreign_key_check: %v", err)
			}
			var violations []string
			cols, _ := rows.Columns()
			for rows.Next() {
				vals := make([]any, len(cols))
				ptrs := make([]any, len(cols))
				for i := range vals {
					ptrs[i] = &vals[i]
				}
				if err := rows.Scan(ptrs...); err != nil {
					rows.Close()
					t.Fatalf("scanning foreign_key_check row: %v", err)
				}
				violations = append(violations, "violation found")
			}
			rows.Close()
			if len(violations) > 0 {
				t.Fatalf("foreign_key_check reported %d violation(s)", len(violations))
			}

			var integrityResult string
			if err := sqldb.QueryRow("PRAGMA integrity_check").Scan(&integrityResult); err != nil {
				t.Fatalf("integrity_check: %v", err)
			}
			if integrityResult != "ok" {
				t.Fatalf("integrity_check reported: %s", integrityResult)
			}
		})
	}
}

func assertObjectCount(t *testing.T, sqldb *sql.DB, sqliteType, ddlKeyword, schemaUpper string) {
	t.Helper()
	if !strings.Contains(schemaUpper, ddlKeyword) {
		return
	}
	var count int
	err := sqldb.QueryRow(
		"SELECT COUNT(*) FROM sqlite_master WHERE type = ? AND name NOT LIKE 'sqlite_%'",
		sqliteType,
	).Scan(&count)
	if err != nil {
		t.Fatalf("counting %s objects: %v", sqliteType, err)
	}
	if count == 0 {
		t.Fatalf("schema contains %q but no %s objects were created", ddlKeyword, sqliteType)
	}
}
