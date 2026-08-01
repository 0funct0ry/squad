package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/0funct0ry/squad/internal/db"
	"github.com/0funct0ry/squad/internal/udf"
)

var udfRegisterOnce sync.Once

// ensureUDFRegistered wires internal/db.OpenDB's UDF registration hook the
// same way cmd's init() does in production — needed since this test binary
// doesn't import cmd.
func ensureUDFRegistered(t *testing.T) {
	t.Helper()
	udfRegisterOnce.Do(func() {
		db.RegisterUDFHook = udf.RegisterAll
	})
}

func newFunctionsTestServer(t *testing.T) *Server {
	t.Helper()
	ensureUDFRegistered(t)
	database, err := db.OpenDB(":memory:", false)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	return NewServer(database, ":memory:", false, false, false, "127.0.0.1", 0, "info")
}

func TestFunctionsCatalog(t *testing.T) {
	srv := newFunctionsTestServer(t)
	ts := httptest.NewServer(srv.router)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/functions")
	if err != nil {
		t.Fatal(err)
	}
	env := decodeEnvelope(t, resp)
	if resp.StatusCode != http.StatusOK || !env.Ok {
		t.Fatalf("expected ok=true 200, got %d ok=%v", resp.StatusCode, env.Ok)
	}
	if !strings.Contains(string(env.Data), "SLUGIFY") {
		t.Fatalf("expected catalog to contain SLUGIFY, got %s", env.Data)
	}
}

func TestFunctionsTrySuccess(t *testing.T) {
	srv := newFunctionsTestServer(t)
	ts := httptest.NewServer(srv.router)
	defer ts.Close()

	resp, env := doModulesJSON(t, ts, http.MethodPost, "/api/functions/try", map[string]any{
		"name": "SLUGIFY",
		"args": []any{"Hello, World!"},
	})
	if resp.StatusCode != http.StatusOK || !env.Ok {
		t.Fatalf("expected ok=true 200, got %d ok=%v (%s)", resp.StatusCode, env.Ok, env.Data)
	}
	if !strings.Contains(string(env.Data), "hello-world") {
		t.Fatalf("expected result hello-world, got %s", env.Data)
	}
}

func TestFunctionsTryNotFound(t *testing.T) {
	srv := newFunctionsTestServer(t)
	ts := httptest.NewServer(srv.router)
	defer ts.Close()

	resp, env := doModulesJSON(t, ts, http.MethodPost, "/api/functions/try", map[string]any{
		"name": "NOT_A_REAL_FUNCTION",
		"args": []any{},
	})
	if resp.StatusCode != http.StatusNotFound || env.Ok {
		t.Fatalf("expected NOT_FOUND, got %d ok=%v", resp.StatusCode, env.Ok)
	}
	if env.Error.Code != "NOT_FOUND" {
		t.Fatalf("expected error code NOT_FOUND, got %q", env.Error.Code)
	}
}

func TestFunctionsTryWrongArgCount(t *testing.T) {
	srv := newFunctionsTestServer(t)
	ts := httptest.NewServer(srv.router)
	defer ts.Close()

	resp, env := doModulesJSON(t, ts, http.MethodPost, "/api/functions/try", map[string]any{
		"name": "SLUGIFY",
		"args": []any{"a", "b"},
	})
	if resp.StatusCode != http.StatusBadRequest || env.Ok {
		t.Fatalf("expected an error for wrong arg count, got %d ok=%v", resp.StatusCode, env.Ok)
	}
}
