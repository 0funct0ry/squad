package seed

import (
	"testing"
)

func TestEnumFromColumn_OnlyProducesRealDistinctValues(t *testing.T) {
	sqlDB := openScratchExample(t, "blog")

	realValues := map[string]bool{}
	rows, err := sqlDB.Query("SELECT DISTINCT status FROM posts WHERE status IS NOT NULL")
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			t.Fatal(err)
		}
		realValues[s] = true
	}
	rows.Close()
	if len(realValues) == 0 {
		t.Fatal("expected posts.status to have at least 1 distinct value in the blog fixture")
	}

	schema := simpleSchema("status")
	specs := map[string]ColumnSpec{
		"status": {Generator: "enumFromColumn", Options: map[string]any{"table": "posts", "column": "status"}},
	}
	gen, err := NewRowGenerator(sqlDB, schema, specs)
	if err != nil {
		t.Fatal(err)
	}
	seenSampled := map[string]bool{}
	for i := 0; i < 100; i++ {
		row, err := gen.GenerateRow()
		if err != nil {
			t.Fatal(err)
		}
		v, ok := row["status"].(string)
		if !ok {
			t.Fatalf("expected string, got %T", row["status"])
		}
		if !realValues[v] {
			t.Errorf("sampled value %q is not among the real distinct posts.status values %v", v, realValues)
		}
		seenSampled[v] = true
	}
	if len(seenSampled) < 2 && len(realValues) >= 2 {
		t.Errorf("expected multiple distinct sampled values given %d real distinct values, only saw %v", len(realValues), seenSampled)
	}
}

// TestEnumFromColumn_ErrorsOnMissingTableOrColumn covers the table case at
// the seed package level: querying a nonexistent table is always a real SQL
// error. A nonexistent *column* name is intentionally not exercised here --
// SQLite's quoted-identifier fallback silently treats an unresolvable
// double-quoted identifier as a string literal rather than erroring, so that
// check only actually happens at the HTTP layer's schema validation (see
// internal/server's TestSeedEnumFromColumnUnknownTableRejected and its
// column-not-found counterpart), exactly like foreignKey's equivalent check.
func TestEnumFromColumn_ErrorsOnMissingTableOrColumn(t *testing.T) {
	sqlDB := openScratchExample(t, "blog")
	schema := simpleSchema("status")

	specs := map[string]ColumnSpec{
		"status": {Generator: "enumFromColumn", Options: map[string]any{"table": "does_not_exist", "column": "status"}},
	}
	if _, err := NewRowGenerator(sqlDB, schema, specs); err == nil {
		t.Error("expected an error for a nonexistent table")
	}
}

func TestEnumFromColumn_ErrorsWhenColumnIsAllNull(t *testing.T) {
	sqlDB := openScratchDB(t)
	if _, err := sqlDB.Exec("CREATE TABLE t (id INTEGER PRIMARY KEY, val TEXT)"); err != nil {
		t.Fatal(err)
	}
	if _, err := sqlDB.Exec("INSERT INTO t (val) VALUES (NULL), (NULL)"); err != nil {
		t.Fatal(err)
	}

	schema := simpleSchema("val")
	specs := map[string]ColumnSpec{
		"val": {Generator: "enumFromColumn", Options: map[string]any{"table": "t", "column": "val"}},
	}
	if _, err := NewRowGenerator(sqlDB, schema, specs); err == nil {
		t.Error("expected an EmptyReferenceError-shaped error when the referenced column is entirely NULL")
	}
}

func TestEnumFromColumn_PoolIsGenuinelyDeduplicated(t *testing.T) {
	sqlDB := openScratchDB(t)
	if _, err := sqlDB.Exec("CREATE TABLE t (id INTEGER PRIMARY KEY, category TEXT)"); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 500; i++ {
		if _, err := sqlDB.Exec("INSERT INTO t (category) VALUES ('same-category')"); err != nil {
			t.Fatal(err)
		}
	}

	schema := simpleSchema("category")
	specs := map[string]ColumnSpec{
		"category": {Generator: "enumFromColumn", Options: map[string]any{"table": "t", "column": "category"}},
	}
	gen, err := NewRowGenerator(sqlDB, schema, specs)
	if err != nil {
		t.Fatal(err)
	}
	if len(gen.enumPools["t\x00category"].values) != 1 {
		t.Errorf("expected the pool to be deduplicated to 1 distinct value, got %d", len(gen.enumPools["t\x00category"].values))
	}
}
