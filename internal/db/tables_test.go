package db

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
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
		Filters: []Filter{{Column: "email", Operator: "contains", Value: "ada"}},
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

// TestVirtualTableIntrospection exercises the M10e introspection fixes
// against a temp-schema virtual table (fts5, a driver built-in — no need to
// pull in internal/vtab and risk an import cycle back to this package) to
// prove the same properties a --modules mount needs: GetTableSchema resolves
// it via temp.sqlite_master, marks it IsVirtual, and BuildTableQuery does
// not prepend a bogus rowid column for it. GetTables must NOT surface it —
// mounted/temp tables are listed from the MountStore, not sqlite_master.
func TestVirtualTableIntrospection(t *testing.T) {
	sqlDB, err := OpenDB(":memory:", false)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer sqlDB.Close()

	if _, err := sqlDB.Exec(`CREATE VIRTUAL TABLE temp.notes USING fts5(body)`); err != nil {
		t.Skipf("fts5 not available in this build: %v", err)
	}

	schema, err := GetTableSchema(sqlDB, "notes")
	if err != nil {
		t.Fatalf("GetTableSchema on a temp-schema virtual table: %v", err)
	}
	if !schema.IsVirtual {
		t.Error("expected IsVirtual=true for a CREATE VIRTUAL TABLE")
	}
	if len(schema.PrimaryKey) != 0 {
		t.Errorf("fts5 declares no primary key, got %v", schema.PrimaryKey)
	}

	selectQuery, _, _, _, err := BuildTableQuery(sqlDB, "notes", RowQueryParams{})
	if err != nil {
		t.Fatalf("BuildTableQuery: %v", err)
	}
	if strings.Contains(selectQuery, "rowid, *") {
		t.Errorf("expected no rowid prefix for a virtual table, got query %q", selectQuery)
	}

	tables, err := GetTables(sqlDB)
	if err != nil {
		t.Fatalf("GetTables: %v", err)
	}
	for _, tbl := range tables {
		if tbl.Name == "notes" {
			t.Error("GetTables must not surface a temp-schema virtual table; it's listed from the MountStore instead")
		}
	}
}

// TestViewRowsQuery is the sibling of TestVirtualTableIntrospection for views.
// A view reports no primary key from PRAGMA table_info and its DDL contains
// neither "without rowid" nor "virtual table", so it used to clear every guard
// in BuildTableQuery and get a "rowid, *" prefix — making the browser fail with
// "no such column: rowid" for *every* view (and for exports, which share
// BuildTableQuery). The multi-table join with a LEFT JOIN mirrors the shape of
// internal/examples/transit's vw_completed_trip_summary, where this surfaced.
func TestViewRowsQuery(t *testing.T) {
	sqlDB, err := OpenDB(":memory:", false)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer sqlDB.Close()

	if _, err := sqlDB.Exec(`
		CREATE TABLE trips (
			trip_id TEXT PRIMARY KEY,
			status  TEXT NOT NULL
		);
		CREATE TABLE trip_fares (
			trip_id    TEXT PRIMARY KEY REFERENCES trips(trip_id),
			total_fare REAL NOT NULL
		);
		CREATE TABLE ratings (
			trip_id TEXT NOT NULL REFERENCES trips(trip_id),
			score   INTEGER NOT NULL
		);
		CREATE VIEW trip_summary AS
		SELECT t.trip_id, f.total_fare, r.score AS rating
		FROM trips t
		JOIN trip_fares f ON f.trip_id = t.trip_id
		LEFT JOIN ratings r ON r.trip_id = t.trip_id
		WHERE t.status = 'completed';

		INSERT INTO trips VALUES ('t1', 'completed'), ('t2', 'completed'), ('t3', 'cancelled');
		INSERT INTO trip_fares VALUES ('t1', 12.5), ('t2', 30.0), ('t3', 7.25);
		INSERT INTO ratings VALUES ('t1', 5);
	`); err != nil {
		t.Fatalf("failed to seed db: %v", err)
	}

	schema, err := GetTableSchema(sqlDB, "trip_summary")
	if err != nil {
		t.Fatalf("GetTableSchema on a view: %v", err)
	}
	if schema.Type != "view" {
		t.Fatalf("expected type view, got %q", schema.Type)
	}
	// The preconditions that made the rowid prefix fire: nothing else about a
	// view distinguishes it from a PK-less rowid table.
	if len(schema.PrimaryKey) != 0 || schema.WithoutRowid || schema.IsVirtual {
		t.Errorf("expected pk=[] withoutRowid=false isVirtual=false for a view, got pk=%v withoutRowid=%v isVirtual=%v",
			schema.PrimaryKey, schema.WithoutRowid, schema.IsVirtual)
	}

	selectQuery, _, _, _, err := BuildTableQuery(sqlDB, "trip_summary", RowQueryParams{})
	if err != nil {
		t.Fatalf("BuildTableQuery: %v", err)
	}
	if strings.Contains(selectQuery, "rowid, *") {
		t.Errorf("expected no rowid prefix for a view, got query %q", selectQuery)
	}

	// The regression itself: this is what the sidebar does when a view is clicked.
	res, err := GetTableRows(sqlDB, "trip_summary", RowQueryParams{Limit: 10, OrderBy: "total_fare", Dir: "desc"})
	if err != nil {
		t.Fatalf("GetTableRows on a view: %v", err)
	}
	if res.Total != 2 {
		t.Errorf("expected 2 rows matching the view's WHERE, got total=%d", res.Total)
	}
	if len(res.Columns) != 3 || res.Columns[0] != "trip_id" {
		t.Errorf("expected the view's own 3 columns starting at trip_id, got %v", res.Columns)
	}
	if len(res.Rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(res.Rows))
	}
	// ORDER BY total_fare DESC: t2 (30.0) before t1 (12.5).
	if got := res.Rows[0][0]; got != "t2" {
		t.Errorf("expected t2 first under ORDER BY total_fare DESC, got %v", got)
	}
}

// TestGetTablesMarksMainSchemaVirtualTables covers the case a mount doesn't:
// a virtual table declared directly in main (e.g. a user typing `CREATE
// VIRTUAL TABLE ... USING csv(...)` into the SQL editor, rather than
// `.mount`/the Modules tab). Unlike a temp-schema mount, this one *is* a
// real, permanent object in the user's own schema, so GetTables must list it
// like any other table — but flagged IsVirtual so the sidebar can render it
// distinctly from a real, storage-backed table.
func TestGetTablesMarksMainSchemaVirtualTables(t *testing.T) {
	sqlDB, err := OpenDB(":memory:", false)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer sqlDB.Close()

	if _, err := sqlDB.Exec(`CREATE VIRTUAL TABLE notes USING fts5(body)`); err != nil {
		t.Skipf("fts5 not available in this build: %v", err)
	}
	if _, err := sqlDB.Exec(`CREATE TABLE real_table (id INTEGER PRIMARY KEY)`); err != nil {
		t.Fatalf("failed to create real_table: %v", err)
	}

	tables, err := GetTables(sqlDB)
	if err != nil {
		t.Fatalf("GetTables: %v", err)
	}

	byName := make(map[string]TableInfo, len(tables))
	for _, tbl := range tables {
		byName[tbl.Name] = tbl
	}

	notes, ok := byName["notes"]
	if !ok {
		t.Fatal("expected a main-schema virtual table to appear in GetTables")
	}
	if !notes.IsVirtual {
		t.Error("expected IsVirtual=true for a main-schema CREATE VIRTUAL TABLE")
	}

	real, ok := byName["real_table"]
	if !ok {
		t.Fatal("expected real_table to appear in GetTables")
	}
	if real.IsVirtual {
		t.Error("expected IsVirtual=false for an ordinary table")
	}
}
