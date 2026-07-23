package db

import (
	"errors"
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

func TestGetTableSchemaDetailed(t *testing.T) {
	db, err := OpenDB(":memory:", false)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	_, err = db.Exec(`
		CREATE TABLE order_items (
			order_id INTEGER NOT NULL,
			line_no INTEGER NOT NULL,
			qty INTEGER NOT NULL,
			unit_price REAL NOT NULL,
			total REAL GENERATED ALWAYS AS (qty * unit_price) STORED,
			PRIMARY KEY (order_id, line_no)
		);

		CREATE TABLE customers (
			id INTEGER PRIMARY KEY,
			active INTEGER NOT NULL DEFAULT 1
		);

		CREATE INDEX idx_customers_active ON customers(id) WHERE active = 1;

		CREATE TABLE parents (
			id INTEGER PRIMARY KEY
		);
		CREATE TABLE children (
			id INTEGER PRIMARY KEY,
			parent_id INTEGER,
			FOREIGN KEY (parent_id) REFERENCES parents(id) ON DELETE CASCADE
		);

		CREATE VIEW customer_view AS SELECT id, active FROM customers;

		CREATE TABLE "weird names" (
			"select" INTEGER PRIMARY KEY,
			"space col" TEXT
		);
	`)
	if err != nil {
		t.Fatalf("failed to seed db: %v", err)
	}

	t.Run("composite primary key in order", func(t *testing.T) {
		schema, err := GetTableSchema(db, "order_items")
		if err != nil {
			t.Fatalf("failed to get schema: %v", err)
		}
		if len(schema.PrimaryKey) != 2 || schema.PrimaryKey[0] != "order_id" || schema.PrimaryKey[1] != "line_no" {
			t.Errorf("expected composite PK [order_id, line_no], got %+v", schema.PrimaryKey)
		}
	})

	t.Run("generated column reports stored", func(t *testing.T) {
		schema, err := GetTableSchema(db, "order_items")
		if err != nil {
			t.Fatalf("failed to get schema: %v", err)
		}
		var found bool
		for _, col := range schema.Columns {
			if col.Name == "total" {
				found = true
				if col.Generated == nil || *col.Generated != "stored" {
					t.Errorf("expected total to be generated:stored, got %+v", col)
				}
			}
		}
		if !found {
			t.Errorf("expected to find generated column 'total', columns: %+v", schema.Columns)
		}
	})

	t.Run("partial index reports partial true", func(t *testing.T) {
		schema, err := GetTableSchema(db, "customers")
		if err != nil {
			t.Fatalf("failed to get schema: %v", err)
		}
		var found bool
		for _, idx := range schema.Indexes {
			if idx.Name == "idx_customers_active" {
				found = true
				if !idx.Partial {
					t.Errorf("expected idx_customers_active to be partial, got %+v", idx)
				}
			}
		}
		if !found {
			t.Errorf("expected to find idx_customers_active, indexes: %+v", schema.Indexes)
		}
	})

	t.Run("foreign key reports on delete cascade", func(t *testing.T) {
		schema, err := GetTableSchema(db, "children")
		if err != nil {
			t.Fatalf("failed to get schema: %v", err)
		}
		if len(schema.ForeignKeys) != 1 || schema.ForeignKeys[0].OnDelete != "CASCADE" {
			t.Errorf("expected FK with ON DELETE CASCADE, got %+v", schema.ForeignKeys)
		}
	})

	t.Run("view returns type view with empty indexes and fks", func(t *testing.T) {
		schema, err := GetTableSchema(db, "customer_view")
		if err != nil {
			t.Fatalf("failed to get schema: %v", err)
		}
		if schema.Type != "view" {
			t.Errorf("expected type view, got %s", schema.Type)
		}
		if len(schema.Indexes) != 0 || len(schema.ForeignKeys) != 0 {
			t.Errorf("expected no indexes/FKs for view, got indexes=%+v fks=%+v", schema.Indexes, schema.ForeignKeys)
		}
		if len(schema.Columns) != 2 {
			t.Errorf("expected 2 view columns, got %+v", schema.Columns)
		}
	})

	t.Run("missing table returns ErrNotFound", func(t *testing.T) {
		_, err := GetTableSchema(db, "does_not_exist")
		if !errors.Is(err, ErrNotFound) {
			t.Errorf("expected ErrNotFound, got %v", err)
		}
	})

	t.Run("quoted identifier table resolves correctly", func(t *testing.T) {
		schema, err := GetTableSchema(db, "weird names")
		if err != nil {
			t.Fatalf("failed to get schema for quoted table: %v", err)
		}
		if len(schema.Columns) != 2 {
			t.Errorf("expected 2 columns, got %+v", schema.Columns)
		}
		if schema.PrimaryKey[0] != "select" {
			t.Errorf("expected pk 'select', got %+v", schema.PrimaryKey)
		}
	})
}
