package hooks

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

// openTestDB opens a real modernc.org/sqlite file database with the hook
// scalar function registered, exactly as the product does.
func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	if err := RegisterAll(); err != nil {
		t.Fatalf("RegisterAll: %v", err)
	}
	path := filepath.Join(t.TempDir(), "test.db")
	d, err := sql.Open("sqlite", "file:"+path+"?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	if err := d.Ping(); err != nil {
		t.Fatalf("ping: %v", err)
	}
	return d
}

func TestSandboxLibraryIsolation(t *testing.T) {
	cases := map[string]string{
		"os":      `os.execute("echo hi")`,
		"io":      `io.open("/etc/passwd")`,
		"require": `require("os")`,
		"load":    `load("return 1")`,
		"dofile":  `dofile("/etc/passwd")`,
		"debug":   `debug.getinfo(1)`,
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			logs := []string{}
			L := NewSandbox(SandboxConfig{Logs: &logs})
			defer L.Close()
			if err := L.DoString(src); err == nil {
				t.Fatalf("expected %s to be unreachable, but the script succeeded", name)
			}
		})
	}
}

func TestSandboxAllowedLibs(t *testing.T) {
	logs := []string{}
	L := NewSandbox(SandboxConfig{Logs: &logs})
	defer L.Close()
	if err := L.DoString(`print(string.upper("a") .. tostring(math.floor(1.5)) .. table.concat({"x"}))`); err != nil {
		t.Fatalf("allowlisted libs should work: %v", err)
	}
	if len(logs) != 1 || logs[0] != "A1x" {
		t.Fatalf("print capture = %#v", logs)
	}
}

func TestJSONModule(t *testing.T) {
	logs := []string{}
	L := NewSandbox(SandboxConfig{Logs: &logs})
	defer L.Close()
	if err := L.DoString(`print(json.decode(json.encode({a=1})).a)`); err != nil {
		t.Fatal(err)
	}
	if logs[0] != "1" {
		t.Fatalf("logs = %#v", logs)
	}
}

func TestHTTPDisabledWithoutAllowNet(t *testing.T) {
	L := NewSandbox(SandboxConfig{AllowNet: false})
	defer L.Close()
	err := L.DoString(`http.post("http://example.com", "{}")`)
	if err == nil || !strings.Contains(err.Error(), "network access disabled") {
		t.Fatalf("want network-disabled error, got %v", err)
	}
	err = L.DoString(`http.head("http://example.com")`)
	if err == nil || !strings.Contains(err.Error(), "network access disabled") {
		t.Fatalf("unknown http field should raise the same message, got %v", err)
	}
}

func TestHTTPEnabledWithAllowNet(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(201)
		w.Write([]byte("pong:" + r.Method))
	}))
	defer srv.Close()
	logs := []string{}
	L := NewSandbox(SandboxConfig{AllowNet: true, Logs: &logs})
	defer L.Close()
	if err := L.DoString(`local r = http.post("` + srv.URL + `", "{}") print(r.status) print(r.body)`); err != nil {
		t.Fatal(err)
	}
	if logs[0] != "201" || logs[1] != "pong:POST" {
		t.Fatalf("logs = %#v", logs)
	}
}

func TestDBExecWriteScoping(t *testing.T) {
	d := openTestDB(t)
	if _, err := d.Exec(`CREATE TABLE audit(msg TEXT)`); err != nil {
		t.Fatal(err)
	}

	L := NewSandbox(SandboxConfig{DB: d, Write: false})
	err := L.DoString(`db.exec("INSERT INTO audit(msg) VALUES('x')")`)
	L.Close()
	if err == nil || !strings.Contains(err.Error(), "READ_ONLY") {
		t.Fatalf("want READ_ONLY error without --write, got %v", err)
	}

	L2 := NewSandbox(SandboxConfig{DB: d, Write: true})
	defer L2.Close()
	if err := L2.DoString(`db.exec("INSERT INTO audit(msg) VALUES(?)", "y")`); err != nil {
		t.Fatalf("db.exec with --write: %v", err)
	}
	Drain()
	var n int
	if err := d.QueryRow(`SELECT count(*) FROM audit WHERE msg='y'`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("audit rows = %d err = %v", n, err)
	}

	// db.query is always available, regardless of --write.
	L3 := NewSandbox(SandboxConfig{DB: d, Write: false})
	defer L3.Close()
	if err := L3.DoString(`local rows = db.query("SELECT msg FROM audit") assert(rows[1].msg == "y")`); err != nil {
		t.Fatalf("db.query read: %v", err)
	}
}

// TestIntegrationBeforeHookAborts is the milestone's true integration test:
// a real sqlite connection, a real hook-backed trigger, a real INSERT.
func TestIntegrationBeforeHookAborts(t *testing.T) {
	Configure("sync", false, true, true)
	d := openTestDB(t)
	if _, err := d.Exec(`CREATE TABLE users(id INTEGER PRIMARY KEY, name TEXT, email TEXT, slug TEXT)`); err != nil {
		t.Fatal(err)
	}
	if err := Init(d); err != nil {
		t.Fatal(err)
	}

	if _, err := Create(d, Hook{
		Table: "users", Event: "insert", Timing: "before", Scope: "row",
		Name: "require-email", Enabled: true,
		Source: `if new.email == nil or new.email == "" then return false, "email required" end return true`,
	}); err != nil {
		t.Fatalf("create hook: %v", err)
	}

	_, err := d.Exec(`INSERT INTO users(name) VALUES('bob')`)
	if err == nil || !strings.Contains(err.Error(), "email required") {
		t.Fatalf("expected the insert to be aborted with 'email required', got %v", err)
	}
	var n int
	d.QueryRow(`SELECT count(*) FROM users`).Scan(&n)
	if n != 0 {
		t.Fatalf("row should not have been persisted, count = %d", n)
	}

	if _, err := d.Exec(`INSERT INTO users(name, email) VALUES('bob','b@example.com')`); err != nil {
		t.Fatalf("valid insert should succeed: %v", err)
	}

	Drain()
	runs, err := Logs(d, 1, 50, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) < 2 {
		t.Fatalf("expected execution-log rows, got %d", len(runs))
	}
}

// TestIntegrationAfterHookDerivedColumn covers the spec's slug scenario.
func TestIntegrationAfterHookDerivedColumn(t *testing.T) {
	Configure("sync", false, true, true)
	d := openTestDB(t)
	if _, err := d.Exec(`CREATE TABLE people(id INTEGER PRIMARY KEY, name TEXT, slug TEXT)`); err != nil {
		t.Fatal(err)
	}
	if err := Init(d); err != nil {
		t.Fatal(err)
	}
	h, err := Create(d, Hook{
		Table: "people", Event: "insert", Timing: "after", Scope: "row",
		Name: "slugify", Enabled: true,
		Source: `db.exec("UPDATE people SET slug = ? WHERE id = ?", string.lower(new.name), new.id)`,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := d.Exec(`INSERT INTO people(name) VALUES('Ada Lovelace')`); err != nil {
		t.Fatal(err)
	}
	Drain()

	deadline := time.Now().Add(3 * time.Second)
	var slug sql.NullString
	for time.Now().Before(deadline) {
		d.QueryRow(`SELECT slug FROM people WHERE name='Ada Lovelace'`).Scan(&slug)
		if slug.Valid {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if slug.String != "ada lovelace" {
		t.Fatalf("derived slug = %q", slug.String)
	}
	runs, err := Logs(d, h.ID, 50, 0)
	if err != nil || len(runs) == 0 {
		t.Fatalf("expected an execution-log row, got %d err=%v", len(runs), err)
	}
}

func TestAsyncRejectsBeforeHooks(t *testing.T) {
	Configure("async", false, true, true)
	defer Configure("sync", false, true, true)
	d := openTestDB(t)
	if _, err := d.Exec(`CREATE TABLE t(id INTEGER PRIMARY KEY)`); err != nil {
		t.Fatal(err)
	}
	if err := Init(d); err != nil {
		t.Fatal(err)
	}
	_, err := Create(d, Hook{Table: "t", Event: "insert", Timing: "before", Scope: "row", Enabled: true, Source: "return true"})
	if err == nil || !strings.Contains(err.Error(), "async") {
		t.Fatalf("want async rejection of before hooks, got %v", err)
	}
}

func TestAsyncDoesNotBlockTheWrite(t *testing.T) {
	Configure("async", false, true, true)
	defer Configure("sync", false, true, true)
	d := openTestDB(t)
	if _, err := d.Exec(`CREATE TABLE t(id INTEGER PRIMARY KEY, v TEXT)`); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Exec(`CREATE TABLE audit(v TEXT)`); err != nil {
		t.Fatal(err)
	}
	if err := Init(d); err != nil {
		t.Fatal(err)
	}
	if _, err := Create(d, Hook{
		Table: "t", Event: "insert", Timing: "after", Scope: "row", Enabled: true,
		// A deliberately slow-ish script: a busy loop the async worker runs
		// after the statement has already returned.
		Source: `local s = 0 for i = 1, 4000000 do s = s + i end db.exec("INSERT INTO audit(v) VALUES('done')")`,
	}); err != nil {
		t.Fatal(err)
	}

	start := time.Now()
	if _, err := d.Exec(`INSERT INTO t(v) VALUES('x')`); err != nil {
		t.Fatal(err)
	}
	elapsed := time.Since(start)

	Drain()
	var n int
	d.QueryRow(`SELECT count(*) FROM audit`).Scan(&n)
	if n != 1 {
		t.Fatalf("async worker should eventually have written the audit row, got %d", n)
	}
	t.Logf("insert returned in %v (async worker ran afterwards)", elapsed)
}

func TestCompileCheckRejectsSyntaxErrors(t *testing.T) {
	if err := CompileCheck(`if then`); err == nil {
		t.Fatal("expected a syntax error")
	}
	if err := CompileCheck(`return true`); err != nil {
		t.Fatalf("valid source rejected: %v", err)
	}
}

func TestRunLogCapAt200(t *testing.T) {
	Configure("sync", false, true, true)
	d := openTestDB(t)
	if err := Init(d); err != nil {
		t.Fatal(err)
	}
	if err := EnsureSchema(d); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 205; i++ {
		recordRun(d, false, 99, "after insert", true, "", 1, nil)
	}
	Drain()
	n, err := CountLogs(d, 99)
	if err != nil {
		t.Fatal(err)
	}
	if n > maxRunsPerHook {
		t.Fatalf("execution log should be capped at %d, got %d", maxRunsPerHook, n)
	}
}
