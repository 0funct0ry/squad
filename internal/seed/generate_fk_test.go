package seed

import (
	"testing"

	"github.com/0funct0ry/squad/internal/db"
)

func TestForeignKey_AutoUniqueForOneToOne_FailsFastWhenExhausted(t *testing.T) {
	sqlDB := openScratchDB(t)
	if _, err := sqlDB.Exec(`CREATE TABLE users (user_id TEXT PRIMARY KEY)`); err != nil {
		t.Fatal(err)
	}
	if _, err := sqlDB.Exec(`INSERT INTO users (user_id) VALUES ('u1'), ('u2'), ('u3')`); err != nil {
		t.Fatal(err)
	}

	// driver_id is a solo-unique PK that's also a foreignKey -- a genuine 1:1
	// relationship -- so it should auto-detect as needing unique sampling.
	schema := &db.TableSchema{
		Name:       "drivers",
		Type:       "table",
		Columns:    []db.ColumnInfo{{Name: "driver_id", Type: "TEXT", PK: 1}},
		PrimaryKey: []string{"driver_id"},
	}
	specs := map[string]ColumnSpec{
		"driver_id": {Generator: ForeignKeyGeneratorName, Options: map[string]any{"table": "users", "column": "user_id"}},
	}

	// Requesting more rows than there are users should fail fast, not
	// silently generate colliding values that only surface as a DB error
	// partway through the insert.
	if _, err := NewRowGenerator(sqlDB, schema, specs, 5); err == nil {
		t.Fatal("expected UniqueForeignKeyExhaustedError, got nil")
	} else if _, ok := err.(*UniqueForeignKeyExhaustedError); !ok {
		t.Fatalf("expected *UniqueForeignKeyExhaustedError, got %T: %v", err, err)
	}

	// Requesting exactly as many rows as there are users should succeed and
	// produce distinct values every time.
	gen, err := NewRowGenerator(sqlDB, schema, specs, 3)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for i := 0; i < 3; i++ {
		row, err := gen.GenerateRow()
		if err != nil {
			t.Fatal(err)
		}
		v := row["driver_id"].(string)
		if seen[v] {
			t.Errorf("expected unique driver_id values, got duplicate %q", v)
		}
		seen[v] = true
	}
}

func TestForeignKey_ExplicitUniqueOverride_OptsInWithoutAConstraint(t *testing.T) {
	sqlDB := openScratchDB(t)
	if _, err := sqlDB.Exec(`CREATE TABLE users (user_id TEXT PRIMARY KEY)`); err != nil {
		t.Fatal(err)
	}
	if _, err := sqlDB.Exec(`INSERT INTO users (user_id) VALUES ('u1'), ('u2')`); err != nil {
		t.Fatal(err)
	}

	// referrer_id has no unique constraint of its own, so it wouldn't
	// auto-detect as needing unique sampling -- but the user can still opt in
	// explicitly via options.unique.
	schema := simpleSchema("referrer_id")
	specs := map[string]ColumnSpec{
		"referrer_id": {Generator: ForeignKeyGeneratorName, Options: map[string]any{"table": "users", "column": "user_id", "unique": true}},
	}

	if _, err := NewRowGenerator(sqlDB, schema, specs, 5); err == nil {
		t.Fatal("expected fail-fast error when explicit unique=true can't be satisfied")
	}

	gen, err := NewRowGenerator(sqlDB, schema, specs, 2)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for i := 0; i < 2; i++ {
		row, err := gen.GenerateRow()
		if err != nil {
			t.Fatal(err)
		}
		seen[row["referrer_id"].(string)] = true
	}
	if len(seen) != 2 {
		t.Errorf("expected 2 distinct values with unique=true override, got %d", len(seen))
	}
}

func TestForeignKey_ExplicitReplacementOverride_AllowsOversampling(t *testing.T) {
	sqlDB := openScratchDB(t)
	if _, err := sqlDB.Exec(`CREATE TABLE users (user_id TEXT PRIMARY KEY)`); err != nil {
		t.Fatal(err)
	}
	if _, err := sqlDB.Exec(`INSERT INTO users (user_id) VALUES ('u1')`); err != nil {
		t.Fatal(err)
	}

	// driver_id is a solo-unique PK, so this would normally fail fast for
	// count=5 against a single user row -- but an explicit unique=false
	// override lets the caller accept the collision risk instead.
	schema := &db.TableSchema{
		Name:       "drivers",
		Type:       "table",
		Columns:    []db.ColumnInfo{{Name: "driver_id", Type: "TEXT", PK: 1}},
		PrimaryKey: []string{"driver_id"},
	}
	specs := map[string]ColumnSpec{
		"driver_id": {Generator: ForeignKeyGeneratorName, Options: map[string]any{"table": "users", "column": "user_id", "unique": false}},
	}
	if _, err := NewRowGenerator(sqlDB, schema, specs, 5); err != nil {
		t.Fatalf("expected no fail-fast error with explicit unique=false, got %v", err)
	}
}
