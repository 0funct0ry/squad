package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/0funct0ry/squad/internal/db"
)

func newRestTestServer(t *testing.T, write, restEnabled bool) *Server {
	t.Helper()
	database, err := db.OpenDB(":memory:", false)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	t.Cleanup(func() { database.Close() })

	if _, err := database.Exec(`CREATE TABLE users (id INTEGER PRIMARY KEY, email TEXT)`); err != nil {
		t.Fatalf("failed to create table: %v", err)
	}
	if _, err := database.Exec(`CREATE VIEW user_view AS SELECT * FROM users`); err != nil {
		t.Fatalf("failed to create view: %v", err)
	}

	// Port 0: these tests never actually start the listener against a real
	// port except where the test explicitly calls /api/rest/start.
	return NewServer(database, ":memory:", write, false, restEnabled, "127.0.0.1", 0, "info")
}

type okEnvelope struct {
	Ok    bool            `json:"ok"`
	Data  json.RawMessage `json:"data"`
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func decodeEnvelope(t *testing.T, resp *http.Response) okEnvelope {
	t.Helper()
	defer resp.Body.Close()
	var env okEnvelope
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		t.Fatalf("failed to decode envelope: %v", err)
	}
	return env
}

func jsonUnmarshalData(env okEnvelope, v interface{}) error {
	return json.Unmarshal(env.Data, v)
}

func TestRestStart_NoOpErrorWhenNotEnabled(t *testing.T) {
	srv := newRestTestServer(t, true, false)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp := doJSON(t, ts.Client(), http.MethodPost, ts.URL+"/api/rest/start", nil)
	env := decodeEnvelope(t, resp)
	if env.Ok {
		t.Fatal("expected ok=false when --rest was not passed at launch")
	}
	if env.Error.Code != "REST_START_FAILED" {
		t.Errorf("expected REST_START_FAILED, got %q", env.Error.Code)
	}
}

func TestRestStatus_Shape(t *testing.T) {
	srv := newRestTestServer(t, true, true)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp, err := ts.Client().Get(ts.URL + "/api/rest/status")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	env := decodeEnvelope(t, resp)
	if !env.Ok {
		t.Fatal("expected ok=true")
	}
	var status struct {
		Enabled bool `json:"enabled"`
		Running bool `json:"running"`
	}
	if err := json.Unmarshal(env.Data, &status); err != nil {
		t.Fatalf("failed to decode status: %v", err)
	}
	if !status.Enabled {
		t.Error("expected enabled=true")
	}
	if status.Running {
		t.Error("expected running=false before Start")
	}
}

func TestRestUpdateTableConfig_RejectsWriteTogglesWithoutWrite(t *testing.T) {
	srv := newRestTestServer(t, false, true) // no --write
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp := doJSON(t, ts.Client(), http.MethodPatch, ts.URL+"/api/rest/tables/users", map[string]interface{}{
		"exposed": true, "create": true,
	})
	env := decodeEnvelope(t, resp)
	if env.Ok {
		t.Fatal("expected ok=false when create is requested without --write")
	}
}

func TestRestUpdateTableConfig_RejectsWriteTogglesOnView(t *testing.T) {
	srv := newRestTestServer(t, true, true) // --write is on
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp := doJSON(t, ts.Client(), http.MethodPatch, ts.URL+"/api/rest/tables/user_view", map[string]interface{}{
		"exposed": true, "create": true,
	})
	env := decodeEnvelope(t, resp)
	if env.Ok {
		t.Fatal("expected ok=false when create is requested on a view")
	}
}

func TestRestUpdateTableConfig_AllowsExposeToggleReadOnly(t *testing.T) {
	srv := newRestTestServer(t, false, true)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp := doJSON(t, ts.Client(), http.MethodPatch, ts.URL+"/api/rest/tables/users", map[string]interface{}{
		"exposed": true,
	})
	env := decodeEnvelope(t, resp)
	if !env.Ok {
		t.Fatalf("expected ok=true, got error: %+v", env.Error)
	}
}

func TestRestListTables_ExcludesInternalReflectsConfig(t *testing.T) {
	srv := newRestTestServer(t, true, true)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// Expose users before listing.
	doJSON(t, ts.Client(), http.MethodPatch, ts.URL+"/api/rest/tables/users", map[string]interface{}{"exposed": true}).Body.Close()

	resp, err := ts.Client().Get(ts.URL + "/api/rest/tables")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	env := decodeEnvelope(t, resp)
	var tables []struct {
		Name    string `json:"name"`
		Exposed bool   `json:"exposed"`
	}
	if err := json.Unmarshal(env.Data, &tables); err != nil {
		t.Fatalf("failed to decode tables: %v", err)
	}

	found := false
	for _, tb := range tables {
		if tb.Name == "sqlite_sequence" {
			t.Fatalf("sqlite_sequence should never be listed")
		}
		if tb.Name == "users" {
			found = true
			if !tb.Exposed {
				t.Errorf("expected users to be exposed after PATCH")
			}
		}
	}
	if !found {
		t.Fatal("expected users to appear in the table list")
	}
}
