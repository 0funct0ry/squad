package db

import (
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
)

func newFilterTestDB(t *testing.T) *sql.DB {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "filters.db")
	conn, err := OpenDB(dbPath, false)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	_, err = conn.Exec(`
		CREATE TABLE people (
			id INTEGER PRIMARY KEY,
			name TEXT,
			age INTEGER,
			nickname TEXT
		);
		INSERT INTO people (id, name, age, nickname) VALUES (1, 'Ada Lovelace', 36, NULL);
		INSERT INTO people (id, name, age, nickname) VALUES (2, 'Linus Torvalds', 54, 'linus');
		INSERT INTO people (id, name, age, nickname) VALUES (3, 'Grace Hopper', 85, 'amazing grace');
	`)
	if err != nil {
		t.Fatalf("failed to seed table: %v", err)
	}
	return conn
}

func runFilter(t *testing.T, conn *sql.DB, filters []Filter) []int {
	t.Helper()
	schema, err := GetTableSchema(conn, "people")
	if err != nil {
		t.Fatalf("failed to get schema: %v", err)
	}
	whereClause, args, err := BuildFilterClause(schema, filters)
	if err != nil {
		t.Fatalf("BuildFilterClause failed: %v", err)
	}
	query := "SELECT id FROM people"
	if whereClause != "" {
		query += " WHERE " + whereClause
	}
	query += " ORDER BY id"
	rows, err := conn.Query(query, args...)
	if err != nil {
		t.Fatalf("query failed: %v (sql: %s)", err, query)
	}
	defer rows.Close()
	var ids []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			t.Fatalf("scan failed: %v", err)
		}
		ids = append(ids, id)
	}
	return ids
}

func assertIDs(t *testing.T, got []int, want ...int) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("expected ids %v, got %v", want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("expected ids %v, got %v", want, got)
		}
	}
}

func TestBuildFilterClauseOperators(t *testing.T) {
	conn := newFilterTestDB(t)

	t.Run("eq", func(t *testing.T) {
		got := runFilter(t, conn, []Filter{{Column: "name", Operator: "eq", Value: "Ada Lovelace"}})
		assertIDs(t, got, 1)
	})

	t.Run("neq", func(t *testing.T) {
		got := runFilter(t, conn, []Filter{{Column: "id", Operator: "neq", Value: 1}})
		assertIDs(t, got, 2, 3)
	})

	t.Run("contains", func(t *testing.T) {
		got := runFilter(t, conn, []Filter{{Column: "name", Operator: "contains", Value: "ov"}})
		assertIDs(t, got, 1)
	})

	t.Run("starts_with", func(t *testing.T) {
		got := runFilter(t, conn, []Filter{{Column: "name", Operator: "starts_with", Value: "Grace"}})
		assertIDs(t, got, 3)
	})

	t.Run("ends_with", func(t *testing.T) {
		got := runFilter(t, conn, []Filter{{Column: "name", Operator: "ends_with", Value: "Hopper"}})
		assertIDs(t, got, 3)
	})

	t.Run("gt", func(t *testing.T) {
		got := runFilter(t, conn, []Filter{{Column: "age", Operator: "gt", Value: 54}})
		assertIDs(t, got, 3)
	})

	t.Run("lt", func(t *testing.T) {
		got := runFilter(t, conn, []Filter{{Column: "age", Operator: "lt", Value: 54}})
		assertIDs(t, got, 1)
	})

	t.Run("between", func(t *testing.T) {
		got := runFilter(t, conn, []Filter{{Column: "age", Operator: "between", Value: float64(30), Value2: float64(60)}})
		assertIDs(t, got, 1, 2)
	})

	t.Run("between reversed bounds", func(t *testing.T) {
		got := runFilter(t, conn, []Filter{{Column: "age", Operator: "between", Value: float64(60), Value2: float64(30)}})
		assertIDs(t, got, 1, 2)
	})

	t.Run("between non-numeric bound is rejected", func(t *testing.T) {
		schema, err := GetTableSchema(conn, "people")
		if err != nil {
			t.Fatalf("failed to get schema: %v", err)
		}
		_, _, err = BuildFilterClause(schema, []Filter{{Column: "age", Operator: "between", Value: "not-a-number", Value2: float64(30)}})
		if !errors.Is(err, ErrFilterUnsupported) {
			t.Fatalf("expected ErrFilterUnsupported, got %v", err)
		}
	})

	t.Run("is_null", func(t *testing.T) {
		got := runFilter(t, conn, []Filter{{Column: "nickname", Operator: "is_null"}})
		assertIDs(t, got, 1)
	})

	t.Run("is_not_null", func(t *testing.T) {
		got := runFilter(t, conn, []Filter{{Column: "nickname", Operator: "is_not_null"}})
		assertIDs(t, got, 2, 3)
	})

	t.Run("in", func(t *testing.T) {
		got := runFilter(t, conn, []Filter{{Column: "id", Operator: "in", Values: []interface{}{1, 3}}})
		assertIDs(t, got, 1, 3)
	})

	t.Run("not_in", func(t *testing.T) {
		got := runFilter(t, conn, []Filter{{Column: "id", Operator: "not_in", Values: []interface{}{1, 3}}})
		assertIDs(t, got, 2)
	})

	t.Run("in with empty list is rejected", func(t *testing.T) {
		schema, err := GetTableSchema(conn, "people")
		if err != nil {
			t.Fatalf("failed to get schema: %v", err)
		}
		_, _, err = BuildFilterClause(schema, []Filter{{Column: "id", Operator: "in", Values: nil}})
		if !errors.Is(err, ErrFilterUnsupported) {
			t.Fatalf("expected ErrFilterUnsupported, got %v", err)
		}
	})

	t.Run("regexp is rejected", func(t *testing.T) {
		schema, err := GetTableSchema(conn, "people")
		if err != nil {
			t.Fatalf("failed to get schema: %v", err)
		}
		_, _, err = BuildFilterClause(schema, []Filter{{Column: "name", Operator: "regexp", Value: "^A"}})
		if !errors.Is(err, ErrFilterUnsupported) {
			t.Fatalf("expected ErrFilterUnsupported, got %v", err)
		}
	})

	t.Run("unknown operator is rejected", func(t *testing.T) {
		schema, err := GetTableSchema(conn, "people")
		if err != nil {
			t.Fatalf("failed to get schema: %v", err)
		}
		_, _, err = BuildFilterClause(schema, []Filter{{Column: "name", Operator: "bogus", Value: "x"}})
		if !errors.Is(err, ErrFilterUnsupported) {
			t.Fatalf("expected ErrFilterUnsupported, got %v", err)
		}
	})

	t.Run("unknown column is rejected", func(t *testing.T) {
		schema, err := GetTableSchema(conn, "people")
		if err != nil {
			t.Fatalf("failed to get schema: %v", err)
		}
		_, _, err = BuildFilterClause(schema, []Filter{{Column: "nope", Operator: "eq", Value: "x"}})
		if !errors.Is(err, ErrFilterUnsupported) {
			t.Fatalf("expected ErrFilterUnsupported, got %v", err)
		}
	})

	t.Run("multiple filters AND-combined", func(t *testing.T) {
		got := runFilter(t, conn, []Filter{
			{Column: "age", Operator: "gt", Value: 30},
			{Column: "nickname", Operator: "is_not_null"},
		})
		assertIDs(t, got, 2, 3)
	})

	t.Run("no filters", func(t *testing.T) {
		got := runFilter(t, conn, nil)
		assertIDs(t, got, 1, 2, 3)
	})
}
