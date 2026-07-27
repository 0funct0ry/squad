package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/0funct0ry/squad/internal/db"
)

func newSandboxTestServer(t *testing.T, restEnabled bool) (*Server, *db.Registry) {
	t.Helper()
	registry := db.NewRegistry(t.TempDir(), 512*1024*1024)
	t.Cleanup(registry.CloseAll)
	srv := NewSandboxServer(registry, false, restEnabled, "127.0.0.1", 0, "info")
	return srv, registry
}

// TestSandboxRestWriteAllowedWithoutWriteFlag guards against a regression
// where the top-level sandbox Server's `write` field (read directly by the
// REST control handlers) stayed false even though sandbox databases are
// always read-write — which greyed out the create/update/delete toggles in
// the REST tab and rejected PATCHes enabling them, despite no --write flag
// ever being applicable to `squad sandbox`.
func TestSandboxRestWriteAllowedWithoutWriteFlag(t *testing.T) {
	srv, registry := newSandboxTestServer(t, true)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	entry, err := registry.Create("demo")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := entry.DB.Exec(`CREATE TABLE authors (id INTEGER PRIMARY KEY, name TEXT)`); err != nil {
		t.Fatalf("create table: %v", err)
	}
	doJSON(t, ts.Client(), http.MethodPost, ts.URL+"/api/sandbox/dbs/active", map[string]interface{}{"id": entry.ID}).Body.Close()

	resp, err := ts.Client().Get(ts.URL + "/api/rest/tables")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	env := decodeEnvelope(t, resp)
	var tables []struct {
		Name         string `json:"name"`
		WriteAllowed bool   `json:"writeAllowed"`
	}
	if err := jsonUnmarshalData(env, &tables); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(tables) != 1 || !tables[0].WriteAllowed {
		t.Fatalf("expected writeAllowed=true for sandbox mode (no --write flag applicable), got: %+v", tables)
	}

	patchResp := doJSON(t, ts.Client(), http.MethodPatch, ts.URL+"/api/rest/tables/authors", map[string]interface{}{
		"exposed": true, "create": true, "update": true, "delete": true,
	})
	patchEnv := decodeEnvelope(t, patchResp)
	if !patchEnv.Ok {
		t.Fatalf("expected write toggles to be settable in sandbox mode without --write, got error: %+v", patchEnv.Error)
	}
}

func TestSandboxSetActive_UnknownIDReturns404(t *testing.T) {
	srv, _ := newSandboxTestServer(t, true)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp := doJSON(t, ts.Client(), http.MethodPost, ts.URL+"/api/sandbox/dbs/active", map[string]interface{}{"id": "nope"})
	env := decodeEnvelope(t, resp)
	if env.Ok {
		t.Fatal("expected ok=false for an unknown sandbox id")
	}
}

func TestSandboxSetActive_SwitchingWhileStoppedIsInert(t *testing.T) {
	srv, registry := newSandboxTestServer(t, true)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	entryA, err := registry.Create("a")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	resp := doJSON(t, ts.Client(), http.MethodPost, ts.URL+"/api/sandbox/dbs/active", map[string]interface{}{"id": entryA.ID})
	env := decodeEnvelope(t, resp)
	if !env.Ok {
		t.Fatalf("expected ok=true, got: %+v", env.Error)
	}

	var data struct {
		RestStopped bool `json:"restStopped"`
	}
	if err := jsonUnmarshalData(env, &data); err != nil {
		t.Fatalf("decode data: %v", err)
	}
	if data.RestStopped {
		t.Error("expected restStopped=false when the listener was never running")
	}
}

func TestSandboxSetActive_StopsRunningListenerOnSwitch(t *testing.T) {
	srv, registry := newSandboxTestServer(t, true)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	entryA, _ := registry.Create("a")
	entryB, _ := registry.Create("b")

	// Make A active, then expose a table and start the REST listener
	// against it.
	doJSON(t, ts.Client(), http.MethodPost, ts.URL+"/api/sandbox/dbs/active", map[string]interface{}{"id": entryA.ID}).Body.Close()

	resp := doJSON(t, ts.Client(), http.MethodPost, ts.URL+"/api/rest/start", nil)
	env := decodeEnvelope(t, resp)
	if !env.Ok {
		t.Fatalf("expected REST start to succeed, got: %+v", env.Error)
	}

	// Switching the active DB to B while running against A must stop it.
	resp = doJSON(t, ts.Client(), http.MethodPost, ts.URL+"/api/sandbox/dbs/active", map[string]interface{}{"id": entryB.ID})
	env = decodeEnvelope(t, resp)
	if !env.Ok {
		t.Fatalf("expected ok=true, got: %+v", env.Error)
	}
	var data struct {
		RestStopped bool `json:"restStopped"`
	}
	if err := jsonUnmarshalData(env, &data); err != nil {
		t.Fatalf("decode data: %v", err)
	}
	if !data.RestStopped {
		t.Error("expected restStopped=true when switching away from the DB the listener was running against")
	}

	statusResp, err := ts.Client().Get(ts.URL + "/api/rest/status")
	if err != nil {
		t.Fatalf("GET status: %v", err)
	}
	statusEnv := decodeEnvelope(t, statusResp)
	var status struct {
		Running bool `json:"running"`
	}
	if err := jsonUnmarshalData(statusEnv, &status); err != nil {
		t.Fatalf("decode status: %v", err)
	}
	if status.Running {
		t.Error("expected the REST listener to be stopped after the active DB changed")
	}
}
