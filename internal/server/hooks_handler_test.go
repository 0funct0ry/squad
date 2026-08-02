package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/0funct0ry/squad/internal/db"
	"github.com/0funct0ry/squad/internal/hooks"
)

var hooksRegisterOnce sync.Once

// newHooksTestServer wires internal/db.OpenDB's hook-dispatcher registration
// the same way cmd's init() does in production (this test binary doesn't
// import cmd) and attaches hooks to a throwaway file database.
func newHooksTestServer(t *testing.T, write bool) *Server {
	t.Helper()
	hooksRegisterOnce.Do(func() { db.RegisterHooksHook = hooks.RegisterAll })
	hooks.Configure("sync", false, write)

	path := t.TempDir() + "/hooks.db"
	database, err := db.OpenDB(path, false)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { hooks.Drain(); database.Close() })
	if _, err := database.Exec(`CREATE TABLE users(id INTEGER PRIMARY KEY, name TEXT, email TEXT)`); err != nil {
		t.Fatal(err)
	}
	if err := hooks.Init(database); err != nil {
		t.Fatalf("hooks.Init: %v", err)
	}
	return NewServer(database, path, write, false, false, "127.0.0.1", 0, "info")
}

func doHooksJSON(t *testing.T, ts *httptest.Server, method, path string, body any) (*http.Response, okEnvelope) {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		reader = bytes.NewReader(b)
	} else {
		reader = bytes.NewReader(nil)
	}
	req, err := http.NewRequest(method, ts.URL+path, reader)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var env okEnvelope
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		t.Fatal(err)
	}
	return resp, env
}

func TestHooksListEnvelopeAndStatus(t *testing.T) {
	srv := newHooksTestServer(t, false)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp, env := doHooksJSON(t, ts, http.MethodGet, "/api/hooks", nil)
	if resp.StatusCode != http.StatusOK || !env.Ok {
		t.Fatalf("GET /api/hooks = %d ok=%v", resp.StatusCode, env.Ok)
	}
	var data struct {
		Hooks    []map[string]any `json:"hooks"`
		HookMode string           `json:"hookMode"`
		Write    bool             `json:"write"`
		AllowNet bool             `json:"allowNet"`
	}
	if err := json.Unmarshal(env.Data, &data); err != nil {
		t.Fatal(err)
	}
	if data.HookMode != "sync" || data.Write || data.AllowNet {
		t.Fatalf("status strip fields = %+v", data)
	}
	if len(data.Hooks) != 0 {
		t.Fatalf("expected no hooks, got %d", len(data.Hooks))
	}
}

func TestHooksWriteGate(t *testing.T) {
	srv := newHooksTestServer(t, false)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	body := map[string]any{"table": "users", "event": "insert", "timing": "after", "scope": "row", "source": "return true"}
	for _, tc := range []struct{ method, path string }{
		{http.MethodPost, "/api/hooks"},
		{http.MethodPatch, "/api/hooks/1"},
		{http.MethodDelete, "/api/hooks/1"},
	} {
		resp, env := doHooksJSON(t, ts, tc.method, tc.path, body)
		if resp.StatusCode != http.StatusForbidden || env.Error.Code != "READ_ONLY" {
			t.Errorf("%s %s = %d/%q, want 403/READ_ONLY", tc.method, tc.path, resp.StatusCode, env.Error.Code)
		}
	}
}

func TestHooksCRUDAndLuaSyntaxRejection(t *testing.T) {
	srv := newHooksTestServer(t, true)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// A Lua syntax error is rejected at save time with VALIDATION.
	resp, env := doHooksJSON(t, ts, http.MethodPost, "/api/hooks", map[string]any{
		"table": "users", "event": "insert", "timing": "after", "scope": "row",
		"source": "if then end",
	})
	if resp.StatusCode != http.StatusBadRequest || env.Error.Code != "VALIDATION" {
		t.Fatalf("syntax error = %d/%q, want 400/VALIDATION", resp.StatusCode, env.Error.Code)
	}

	// A valid hook saves.
	resp, env = doHooksJSON(t, ts, http.MethodPost, "/api/hooks", map[string]any{
		"table": "users", "event": "insert", "timing": "before", "scope": "row",
		"name": "require-email", "source": `if new.email == nil then return false, "email required" end return true`,
	})
	if resp.StatusCode != http.StatusOK || !env.Ok {
		t.Fatalf("create = %d %s", resp.StatusCode, env.Error.Message)
	}
	var created struct {
		ID     int64  `json:"id"`
		Source string `json:"source"`
	}
	json.Unmarshal(env.Data, &created)
	if created.ID == 0 {
		t.Fatal("expected an id")
	}
	if created.Source != "" {
		t.Error("the list/summary payload must not carry Lua source")
	}

	// GET by id includes the source.
	_, env = doHooksJSON(t, ts, http.MethodGet, "/api/hooks/1", nil)
	var full struct {
		Source string `json:"source"`
	}
	json.Unmarshal(env.Data, &full)
	if full.Source == "" {
		t.Error("GET /api/hooks/:id should include the Lua source")
	}

	// PATCH toggles enabled.
	resp, env = doHooksJSON(t, ts, http.MethodPatch, "/api/hooks/1", map[string]any{"enabled": false})
	if resp.StatusCode != http.StatusOK || !env.Ok {
		t.Fatalf("patch = %d %s", resp.StatusCode, env.Error.Message)
	}

	// DELETE removes it.
	resp, _ = doHooksJSON(t, ts, http.MethodDelete, "/api/hooks/1", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("delete = %d", resp.StatusCode)
	}
	resp, _ = doHooksJSON(t, ts, http.MethodGet, "/api/hooks/1", nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("get after delete = %d, want 404", resp.StatusCode)
	}
}

func TestHooksTestRunDoesNotMutate(t *testing.T) {
	srv := newHooksTestServer(t, true)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	doHooksJSON(t, ts, http.MethodPost, "/api/hooks", map[string]any{
		"table": "users", "event": "insert", "timing": "before", "scope": "row",
		"source": `print("checking " .. tostring(new.email)) if new.email == nil then return false, "email required" end return true`,
	})

	resp, env := doHooksJSON(t, ts, http.MethodPost, "/api/hooks/1/test", map[string]any{
		"old": nil, "new": map[string]any{"name": "bob"},
	})
	if resp.StatusCode != http.StatusOK || !env.Ok {
		t.Fatalf("test = %d %s", resp.StatusCode, env.Error.Message)
	}
	var data struct {
		Result     *bool    `json:"result"`
		Message    *string  `json:"message"`
		Logs       []string `json:"logs"`
		DurationMs int64    `json:"durationMs"`
	}
	if err := json.Unmarshal(env.Data, &data); err != nil {
		t.Fatal(err)
	}
	if data.Result == nil || *data.Result {
		t.Fatalf("result = %v, want false", data.Result)
	}
	if data.Message == nil || *data.Message != "email required" {
		t.Fatalf("message = %v", data.Message)
	}
	if len(data.Logs) != 1 {
		t.Fatalf("captured print() logs = %#v", data.Logs)
	}

	// The dry run must not have touched the real table.
	var n int
	if err := srv.db.QueryRow(`SELECT count(*) FROM users`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("test run mutated real rows: count=%d err=%v", n, err)
	}
}

// TestHooksTestReportsReadOnlyInData asserts a gated write inside a test run
// reports what would have happened instead of throwing an HTTP error.
func TestHooksTestReportsReadOnlyInData(t *testing.T) {
	srv := newHooksTestServer(t, true)
	// Create with --write on, then flip the server to read-only for the run.
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()
	doHooksJSON(t, ts, http.MethodPost, "/api/hooks", map[string]any{
		"table": "users", "event": "insert", "timing": "after", "scope": "row",
		"source": `db.exec("INSERT INTO users(name) VALUES('x')")`,
	})
	srv.write = false

	resp, env := doHooksJSON(t, ts, http.MethodPost, "/api/hooks/1/test", map[string]any{"new": map[string]any{"name": "x"}})
	if resp.StatusCode != http.StatusOK || !env.Ok {
		t.Fatalf("test = %d %s", resp.StatusCode, env.Error.Message)
	}
	var data struct {
		Error string `json:"error"`
	}
	json.Unmarshal(env.Data, &data)
	if data.Error == "" || !bytes.Contains([]byte(data.Error), []byte("READ_ONLY")) {
		t.Fatalf("expected a READ_ONLY failure reported in data, got %q", data.Error)
	}
}

func TestHooksLogPaginationAndCap(t *testing.T) {
	srv := newHooksTestServer(t, true)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	doHooksJSON(t, ts, http.MethodPost, "/api/hooks", map[string]any{
		"table": "users", "event": "insert", "timing": "after", "scope": "row",
		"source": "return true",
	})
	// 205 test runs, each of which records an execution-log row.
	for i := 0; i < 205; i++ {
		doHooksJSON(t, ts, http.MethodPost, "/api/hooks/1/test", map[string]any{"new": map[string]any{}})
	}
	hooks.Drain()

	_, env := doHooksJSON(t, ts, http.MethodGet, "/api/hooks/1/log?limit=10&offset=5", nil)
	var data struct {
		Runs   []map[string]any `json:"runs"`
		Total  int              `json:"total"`
		Limit  int              `json:"limit"`
		Offset int              `json:"offset"`
	}
	if err := json.Unmarshal(env.Data, &data); err != nil {
		t.Fatal(err)
	}
	if len(data.Runs) != 10 || data.Limit != 10 || data.Offset != 5 {
		t.Fatalf("pagination = %d rows limit=%d offset=%d", len(data.Runs), data.Limit, data.Offset)
	}
	if data.Total > 200 {
		t.Fatalf("execution log should be capped at 200 rows per hook, got %d", data.Total)
	}
}

func TestHooksClearLog(t *testing.T) {
	srv := newHooksTestServer(t, true)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	doHooksJSON(t, ts, http.MethodPost, "/api/hooks", map[string]any{
		"table": "users", "event": "insert", "timing": "after", "scope": "row",
		"source": "return true",
	})
	doHooksJSON(t, ts, http.MethodPost, "/api/hooks/1/test", map[string]any{"new": map[string]any{}})
	hooks.Drain()

	_, env := doHooksJSON(t, ts, http.MethodGet, "/api/hooks/1/log", nil)
	var before struct {
		Total int `json:"total"`
	}
	if err := json.Unmarshal(env.Data, &before); err != nil {
		t.Fatal(err)
	}
	if before.Total == 0 {
		t.Fatalf("expected at least one recorded run before clearing, got %d", before.Total)
	}

	resp, _ := doHooksJSON(t, ts, http.MethodDelete, "/api/hooks/1/log", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("DELETE /api/hooks/1/log = %d, want 200", resp.StatusCode)
	}

	_, env = doHooksJSON(t, ts, http.MethodGet, "/api/hooks/1/log", nil)
	var after struct {
		Total int              `json:"total"`
		Runs  []map[string]any `json:"runs"`
	}
	if err := json.Unmarshal(env.Data, &after); err != nil {
		t.Fatal(err)
	}
	if after.Total != 0 || len(after.Runs) != 0 {
		t.Fatalf("expected 0 runs after clearing, got total=%d runs=%d", after.Total, len(after.Runs))
	}
}

func TestHooksAsyncRejectsBeforeTiming(t *testing.T) {
	srv := newHooksTestServer(t, true)
	hooks.Configure("async", false, true)
	defer hooks.Configure("sync", false, true)
	srv.ConfigureHooks("async", false)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp, env := doHooksJSON(t, ts, http.MethodPost, "/api/hooks", map[string]any{
		"table": "users", "event": "insert", "timing": "before", "scope": "row", "source": "return true",
	})
	if resp.StatusCode != http.StatusBadRequest || env.Error.Code != "VALIDATION" {
		t.Fatalf("before hook under async = %d/%q, want 400/VALIDATION", resp.StatusCode, env.Error.Code)
	}
}
