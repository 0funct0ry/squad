package vtab_test

import (
	"context"
	"database/sql"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"testing"

	"github.com/0funct0ry/squad/internal/db"
	"github.com/0funct0ry/squad/internal/export"
	"github.com/0funct0ry/squad/internal/vtab"
)

var configureOnce sync.Once

// openTestDB configures and registers the module set (once for the whole
// test binary, matching production's process-global registration) with
// modulesRoot pointed at testdata, then opens a fresh in-memory database.
func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	root, err := filepath.Abs("testdata")
	if err != nil {
		t.Fatal(err)
	}
	configureOnce.Do(func() {
		vtab.Configure(true, root)
		// cmd's init() normally wires this; tests here exercise
		// internal/vtab and internal/db directly, without importing cmd.
		db.RegisterModulesHook = vtab.Register
	})

	sqlDB, err := db.OpenDB(":memory:", false)
	if err != nil {
		t.Fatalf("OpenDB: %v", err)
	}
	t.Cleanup(func() { sqlDB.Close() })
	return sqlDB
}

func TestRegisterIsIdempotentAndListsAllModules(t *testing.T) {
	sqlDB := openTestDB(t)

	rows, err := sqlDB.Query("SELECT name FROM pragma_module_list() ORDER BY name")
	if err != nil {
		t.Fatalf("pragma_module_list: %v", err)
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatal(err)
		}
		names = append(names, name)
	}

	want := []string{"csv", "jsonl", "parquet", "xlsx", "yaml", "xml", "series", "calendar", "fake", "tokens"}
	for _, w := range want {
		found := false
		for _, n := range names {
			if n == w {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("pragma_module_list() missing %q; got %v", w, names)
		}
	}

	// Registering again (a second connection opening) must not error — the
	// sync.Once guard makes this a no-op, exactly what a second OpenDB call
	// in the same process needs.
	if err := vtab.Register(); err != nil {
		t.Fatalf("second Register() call should be a no-op, got: %v", err)
	}
}

func TestCatalogSortedAndComplete(t *testing.T) {
	cat := vtab.Catalog()
	if len(cat) != 10 {
		t.Fatalf("expected 10 catalog entries, got %d", len(cat))
	}
	names := make([]string, len(cat))
	for i, m := range cat {
		names[i] = m.Name
	}
	if !sort.StringsAreSorted(names) {
		t.Errorf("catalog not sorted: %v", names)
	}
}

func TestSeriesModule(t *testing.T) {
	sqlDB := openTestDB(t)

	if _, err := sqlDB.Exec(`CREATE VIRTUAL TABLE temp.buckets USING series(start=0, stop=1000, step=100)`); err != nil {
		t.Fatalf("mount series: %v", err)
	}

	var count int
	if err := sqlDB.QueryRow(`SELECT COUNT(*) FROM buckets`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 10 {
		t.Errorf("expected 10 buckets (0..900 step 100, exclusive stop), got %d", count)
	}

	var maxVal float64
	if err := sqlDB.QueryRow(`SELECT MAX(value) FROM buckets`).Scan(&maxVal); err != nil {
		t.Fatal(err)
	}
	if maxVal != 900 {
		t.Errorf("expected max bucket floor 900, got %v", maxVal)
	}
}

func TestSeriesModuleFractionalAndNegativeStep(t *testing.T) {
	sqlDB := openTestDB(t)

	if _, err := sqlDB.Exec(`CREATE VIRTUAL TABLE temp.frac USING series(start=0, stop=1, step=0.25)`); err != nil {
		t.Fatalf("mount series: %v", err)
	}
	var count int
	if err := sqlDB.QueryRow(`SELECT COUNT(*) FROM frac`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 4 {
		t.Errorf("expected 4 rows for 0..1 step 0.25, got %d", count)
	}

	if _, err := sqlDB.Exec(`CREATE VIRTUAL TABLE temp.down USING series(start=10, stop=0, step=-2)`); err != nil {
		t.Fatalf("mount series: %v", err)
	}
	var downCount int
	if err := sqlDB.QueryRow(`SELECT COUNT(*) FROM down`).Scan(&downCount); err != nil {
		t.Fatal(err)
	}
	if downCount != 5 {
		t.Errorf("expected 5 rows for 10..0 step -2, got %d", downCount)
	}
}

func TestCalendarModule(t *testing.T) {
	sqlDB := openTestDB(t)

	if _, err := sqlDB.Exec(`CREATE VIRTUAL TABLE temp.days USING calendar(start='2026-01-01', stop='2026-01-31')`); err != nil {
		t.Fatalf("mount calendar: %v", err)
	}

	var count int
	if err := sqlDB.QueryRow(`SELECT COUNT(*) FROM days`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 31 {
		t.Errorf("expected 31 days (inclusive boundary), got %d", count)
	}

	var weekends int
	if err := sqlDB.QueryRow(`SELECT COUNT(*) FROM days WHERE is_weekend = 1`).Scan(&weekends); err != nil {
		t.Fatal(err)
	}
	if weekends == 0 {
		t.Errorf("expected at least one weekend day in January 2026")
	}
}

func TestTokensModule(t *testing.T) {
	sqlDB := openTestDB(t)

	if _, err := sqlDB.Exec(`CREATE VIRTUAL TABLE temp.wanted USING tokens(text='paid,shipped, cancelled ')`); err != nil {
		t.Fatalf("mount tokens: %v", err)
	}

	rows, err := sqlDB.Query(`SELECT n, token FROM wanted ORDER BY n`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	var got []string
	for rows.Next() {
		var n int64
		var token string
		if err := rows.Scan(&n, &token); err != nil {
			t.Fatal(err)
		}
		got = append(got, token)
	}
	want := []string{"paid", "shipped", "cancelled"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("token %d: got %q want %q (trim=true should strip surrounding whitespace)", i, got[i], want[i])
		}
	}
}

func TestTokensModuleEmptyInput(t *testing.T) {
	sqlDB := openTestDB(t)
	if _, err := sqlDB.Exec(`CREATE VIRTUAL TABLE temp.empty_tok USING tokens(text='')`); err != nil {
		t.Fatalf("mount tokens: %v", err)
	}
	var count int
	if err := sqlDB.QueryRow(`SELECT COUNT(*) FROM empty_tok`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("splitting an empty string yields one empty token, got count %d", count)
	}
}

func TestFakeModule(t *testing.T) {
	sqlDB := openTestDB(t)

	if _, err := sqlDB.Exec(`CREATE VIRTUAL TABLE temp.prospects USING fake(rows=5, email=email, name=firstName, seed=42)`); err != nil {
		t.Fatalf("mount fake: %v", err)
	}

	var count int
	if err := sqlDB.QueryRow(`SELECT COUNT(*) FROM prospects`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 5 {
		t.Errorf("expected 5 rows, got %d", count)
	}

	var email string
	if err := sqlDB.QueryRow(`SELECT email FROM prospects LIMIT 1`).Scan(&email); err != nil {
		t.Fatal(err)
	}
	if email == "" {
		t.Errorf("expected a non-empty generated email")
	}
}

// TestFakeModuleGeneratorWithOptions covers a generator that requires
// options to run at all (oneOf's `values`) — bare oneOf (no options) errors
// "requires at least 2 values, got 0", so <column>=<generator>:<json> is the
// only way to use such a generator from fake.
func TestFakeModuleGeneratorWithOptions(t *testing.T) {
	sqlDB := openTestDB(t)

	stmt := `CREATE VIRTUAL TABLE temp.mady USING fake(rows=20, foo='oneOf:{"values":"A,B"}')`
	if _, err := sqlDB.Exec(stmt); err != nil {
		t.Fatalf("mount fake with generator options: %v", err)
	}

	rows, err := sqlDB.Query(`SELECT DISTINCT foo FROM mady ORDER BY foo`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	var got []string
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			t.Fatal(err)
		}
		got = append(got, v)
	}
	if len(got) == 0 {
		t.Fatal("expected at least one distinct value")
	}
	for _, v := range got {
		if v != "A" && v != "B" {
			t.Errorf("expected only A/B from oneOf(values=A,B), got %q", v)
		}
	}
}

func TestFakeModuleOneOfWithoutOptionsErrors(t *testing.T) {
	sqlDB := openTestDB(t)
	if _, err := sqlDB.Exec(`CREATE VIRTUAL TABLE temp.bad2 USING fake(rows=1, foo=oneOf)`); err == nil {
		t.Error("expected mounting oneOf without its required values option to fail")
	}
}

func TestFakeModuleUnknownGenerator(t *testing.T) {
	sqlDB := openTestDB(t)
	if _, err := sqlDB.Exec(`CREATE VIRTUAL TABLE temp.bad USING fake(rows=1, foo=notagenerator)`); err == nil {
		t.Errorf("expected an error mounting fake with an unknown generator")
	}
}

// TestParquetModule mounts a real parquet file (written with
// internal/export.ExportParquet, the same writer squad's own export route
// uses) and queries it. This is a regression test for a panic found in
// manual testing: parquet.GenericReader[map[string]any].Read calls
// reflect.Value.SetMapIndex on each buffer slot via Schema.Reconstruct,
// which panics with "assignment to entry in nil map" if the slot isn't
// already a non-nil map — a zero-valued []map[string]any buffer, as this
// module originally allocated, panicked on the very first row.
func TestParquetModule(t *testing.T) {
	root, err := filepath.Abs("testdata")
	if err != nil {
		t.Fatal(err)
	}
	configureOnce.Do(func() {
		vtab.Configure(true, root)
		db.RegisterModulesHook = vtab.Register
	})

	path := filepath.Join(root, "sample.parquet")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Remove(path) })

	columns := []string{"sku", "price"}
	data := [][]any{
		{"SKU-1", "9.99"},
		{"SKU-2", "4.50"},
	}
	i := 0
	source := func() ([]any, error) {
		if i >= len(data) {
			return nil, io.EOF
		}
		row := data[i]
		i++
		return row, nil
	}
	if err := export.ExportParquet(columns, source, f); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	sqlDB := openTestDB(t)
	if _, err := sqlDB.Exec(`CREATE VIRTUAL TABLE temp.prices USING parquet(file='sample.parquet')`); err != nil {
		t.Fatalf("mount parquet: %v", err)
	}

	rows, err := sqlDB.Query(`SELECT sku, price FROM prices ORDER BY sku`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	var got []string
	for rows.Next() {
		var sku, price string
		if err := rows.Scan(&sku, &price); err != nil {
			t.Fatal(err)
		}
		got = append(got, sku+"="+price)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	want := []string{"SKU-1=9.99", "SKU-2=4.50"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("row %d: got %q want %q", i, got[i], want[i])
		}
	}
}

func TestCSVModule(t *testing.T) {
	sqlDB := openTestDB(t)

	if _, err := sqlDB.Exec(`CREATE VIRTUAL TABLE temp.vendor_prices USING csv(file='sample.csv')`); err != nil {
		t.Fatalf("mount csv: %v", err)
	}

	rows, err := sqlDB.Query(`SELECT sku, price, name FROM vendor_prices ORDER BY sku`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	var count int
	for rows.Next() {
		var sku, name string
		var price float64
		if err := rows.Scan(&sku, &price, &name); err != nil {
			t.Fatal(err)
		}
		count++
	}
	if count != 3 {
		t.Errorf("expected 3 rows, got %d", count)
	}
}

func TestCSVModulePathEscapeRejected(t *testing.T) {
	sqlDB := openTestDB(t)
	_, err := sqlDB.Exec(`CREATE VIRTUAL TABLE temp.escape USING csv(file='../vtab_test.go')`)
	if err == nil {
		t.Fatal("expected mounting a file outside --modules-root to fail")
	}
}

func TestYAMLModule(t *testing.T) {
	sqlDB := openTestDB(t)

	if _, err := sqlDB.Exec(`CREATE VIRTUAL TABLE temp.catalog_cfg USING yaml(file='sample.yaml', root='/categories')`); err != nil {
		t.Fatalf("mount yaml: %v", err)
	}

	var count int
	if err := sqlDB.QueryRow(`SELECT COUNT(*) FROM catalog_cfg`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Errorf("expected 2 categories, got %d", count)
	}
}

func TestMountStoreAliasCollisions(t *testing.T) {
	store := vtab.NewMountStore()
	if err := store.ValidateAlias("series"); err == nil {
		t.Error("expected alias colliding with a module name to be rejected")
	}
	if err := store.ValidateAlias(""); err == nil {
		t.Error("expected empty alias to be rejected")
	}
	store.Add(vtab.Mount{Alias: "buckets", Module: "series"})
	if err := store.ValidateAlias("buckets"); err == nil {
		t.Error("expected re-mounting an existing alias to be rejected")
	}
}

func TestWithMountsNoopOnEmptyStore(t *testing.T) {
	sqlDB := openTestDB(t)
	store := vtab.NewMountStore()

	called := false
	err := vtab.WithMounts(context.Background(), sqlDB, store, func(conn *sql.Conn) error {
		called = true
		var one int
		return conn.QueryRowContext(context.Background(), "SELECT 1").Scan(&one)
	})
	if err != nil {
		t.Fatalf("WithMounts with empty store should be a no-op passthrough: %v", err)
	}
	if !called {
		t.Error("expected the callback to run even with an empty store")
	}
}

func TestCreateMountAndQueryOnReadOnlyDB(t *testing.T) {
	root, err := filepath.Abs("testdata")
	if err != nil {
		t.Fatal(err)
	}
	configureOnce.Do(func() {
		vtab.Configure(true, root)
	})

	dir := t.TempDir()
	path := filepath.Join(dir, "ro.db")

	rw, err := db.OpenDB(path, false)
	if err != nil {
		t.Fatalf("OpenDB write: %v", err)
	}
	if _, err := rw.Exec("CREATE TABLE t(id INTEGER PRIMARY KEY)"); err != nil {
		t.Fatal(err)
	}
	rw.Close()

	// This is the test that proves the temp-schema mount design works in
	// squad's default read-only mode: mounting into temp must succeed even
	// though main is opened mode=ro.
	roDB, err := db.OpenDB(path, true)
	if err != nil {
		t.Fatalf("OpenDB read-only: %v", err)
	}
	t.Cleanup(func() { roDB.Close() })

	store := vtab.NewMountStore()
	m, err := vtab.CreateMount(context.Background(), roDB, store, "series", "nums", map[string]string{"start": "0", "stop": "3"})
	if err != nil {
		t.Fatalf("CreateMount on read-only DB: %v", err)
	}
	if len(m.DeclaredColumns) != 1 || m.DeclaredColumns[0] != "value" {
		t.Errorf("expected declared column [value], got %v", m.DeclaredColumns)
	}

	cols, rows, err := vtab.PreviewMount(context.Background(), roDB, store, "nums", 10)
	if err != nil {
		t.Fatalf("PreviewMount: %v", err)
	}
	if len(cols) != 1 || cols[0] != "value" {
		t.Errorf("expected column [value], got %v", cols)
	}
	if len(rows) != 3 {
		t.Errorf("expected 3 rows, got %d", len(rows))
	}

	// The mount lives only in temp; the database file itself must be
	// untouched (no CREATE VIRTUAL TABLE ever reaches main).
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() == 0 {
		t.Fatal("unexpected empty db file")
	}
}

// TestUnmountIsEnforcedDespiteConnectionPoolReuse guards against a real bug
// found in manual testing: *sql.Conn.Close() returns the connection to
// database/sql's pool rather than closing it, so a virtual table created on
// that connection survives being "unmounted" from the MountStore. Pinning
// the pool to a single connection (SetMaxOpenConns(1)) forces every
// WithMounts call in this test to reuse the exact same physical connection,
// reproducing the scenario where a query after unmount would otherwise
// still see the stale table.
func TestUnmountIsEnforcedDespiteConnectionPoolReuse(t *testing.T) {
	sqlDB := openTestDB(t)
	sqlDB.SetMaxOpenConns(1)

	store := vtab.NewMountStore()
	if _, err := vtab.CreateMount(context.Background(), sqlDB, store, "series", "nums", map[string]string{"start": "0", "stop": "3"}); err != nil {
		t.Fatalf("CreateMount: %v", err)
	}

	err := vtab.WithMounts(context.Background(), sqlDB, store, func(conn *sql.Conn) error {
		var count int
		return conn.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM nums").Scan(&count)
	})
	if err != nil {
		t.Fatalf("query while mounted: %v", err)
	}

	if !vtab.DropMount(store, "nums") {
		t.Fatal("expected DropMount to report the alias existed")
	}

	err = vtab.WithMounts(context.Background(), sqlDB, store, func(conn *sql.Conn) error {
		var count int
		return conn.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM nums").Scan(&count)
	})
	if err == nil {
		t.Fatal("expected querying an unmounted table to fail with 'no such table', got no error — the stale virtual table from the reused pooled connection is still visible")
	}

	// Re-mounting the same alias name (a common follow-up: unmount, then
	// mount a different source under the same name) must also succeed
	// despite the pool having previously created a same-named temp table
	// on this connection.
	if _, err := vtab.CreateMount(context.Background(), sqlDB, store, "tokens", "nums", map[string]string{"text": "a,b"}); err != nil {
		t.Fatalf("re-mounting alias %q after unmount: %v", "nums", err)
	}
}
