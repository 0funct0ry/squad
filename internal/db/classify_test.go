package db

import (
	"errors"
	"testing"
)

func TestClassify(t *testing.T) {
	tests := []struct {
		name      string
		sql       string
		wantClass string
		wantErr   error
		wantKey   string // Expected keyword for checking
	}{
		{
			name:      "bare SELECT",
			sql:       "SELECT * FROM users",
			wantClass: ClassRead,
		},
		{
			name:      "SELECT with leading comment and whitespace",
			sql:       "  -- this is a comment\n /* block comment */ \n SELECT * FROM users;",
			wantClass: ClassRead,
		},
		{
			name:      "EXPLAIN",
			sql:       "EXPLAIN SELECT * FROM users",
			wantClass: ClassRead,
		},
		{
			name:      "EXPLAIN QUERY PLAN",
			sql:       "EXPLAIN QUERY PLAN SELECT * FROM users",
			wantClass: ClassRead,
		},
		{
			name:      "WITH ... SELECT",
			sql:       "WITH cte AS (SELECT 1) SELECT * FROM cte",
			wantClass: ClassRead,
		},
		{
			name:      "WITH RECURSIVE ... SELECT",
			sql:       "WITH RECURSIVE cte AS (SELECT 1) SELECT * FROM cte",
			wantClass: ClassRead,
		},
		{
			name:      "WITH ... INSERT",
			sql:       "WITH cte AS (SELECT 1) INSERT INTO logs SELECT * FROM cte",
			wantClass: ClassWrite,
		},
		{
			name:      "PRAGMA table_info",
			sql:       "PRAGMA table_info(users)",
			wantClass: ClassRead,
		},
		{
			name:      "PRAGMA user_version read",
			sql:       "PRAGMA user_version",
			wantClass: ClassRead,
		},
		{
			name:      "PRAGMA user_version write",
			sql:       "PRAGMA user_version = 1",
			wantClass: ClassWrite,
		},
		{
			name:      "PRAGMA user_version write paren",
			sql:       "PRAGMA user_version(1)",
			wantClass: ClassWrite,
		},
		{
			name:      "INSERT",
			sql:       "INSERT INTO users (email) VALUES ('a@b.com')",
			wantClass: ClassWrite,
		},
		{
			name:      "UPDATE",
			sql:       "UPDATE users SET email = 'a@b.com' WHERE id = 1",
			wantClass: ClassWrite,
		},
		{
			name:      "DELETE",
			sql:       "DELETE FROM users",
			wantClass: ClassWrite,
		},
		{
			name:      "CREATE",
			sql:       "CREATE TABLE users (id INT)",
			wantClass: ClassWrite,
		},
		{
			name:      "ALTER",
			sql:       "ALTER TABLE users ADD COLUMN age INT",
			wantClass: ClassWrite,
		},
		{
			name:      "DROP",
			sql:       "DROP TABLE users",
			wantClass: ClassWrite,
		},
		{
			name:    "empty string",
			sql:     "   \n  \t  ",
			wantErr: ErrEmptyQuery,
		},
		{
			name:    "comment only",
			sql:     "-- just a comment\n/* block only */",
			wantErr: ErrEmptyQuery,
		},
		{
			name:      "nonsense keyword (fail closed)",
			sql:       "FROBNICATE x",
			wantClass: ClassWrite,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Classify(tt.sql)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("Classify() error = %v, wantErr %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Errorf("Classify() unexpected error: %v", err)
				return
			}
			if got != tt.wantClass {
				t.Errorf("Classify() = %v, want %v", got, tt.wantClass)
			}
		})
	}
}

func TestSplitStatements(t *testing.T) {
	sql := `SELECT 1; -- comment;
SELECT 2; /* comment ; */ SELECT 3;`
	got, err := SplitStatements(sql)
	if err != nil {
		t.Fatalf("SplitStatements error: %v", err)
	}
	// We expect 3 statements since the first ends at SELECT 1;, second at SELECT 2;, third at SELECT 3;
	// Semicolons inside comments should be ignored.
	if len(got) != 3 {
		t.Errorf("expected 3 statements, got %d: %q", len(got), got)
	}
}
