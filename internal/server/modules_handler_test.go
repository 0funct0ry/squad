package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/0funct0ry/squad/internal/db"
	"github.com/0funct0ry/squad/internal/vtab"
)

var modulesConfigureOnce sync.Once

// ensureModulesRegistered wires internal/db.OpenDB's registration hook the
// same way cmd's init() does in production, and configures vtab with a
// throwaway root — needed since this test binary doesn't import cmd.
func ensureModulesRegistered(t *testing.T) {
	t.Helper()
	modulesConfigureOnce.Do(func() {
		vtab.Configure(true, t.TempDir())
		db.RegisterModulesHook = vtab.Register
	})
}

func newModulesTestServer(t *testing.T, enableModules, write bool) *Server {
	t.Helper()
	ensureModulesRegistered(t)

	database, err := db.OpenDB(":memory:", false)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	t.Cleanup(func() { database.Close() })

	srv := NewServer(database, ":memory:", write, false, false, "127.0.0.1", 0, "info")
	if enableModules {
		srv.EnableModules(t.TempDir())
	}
	return srv
}

func doModulesJSON(t *testing.T, ts *httptest.Server, method, path string, body any) (*http.Response, okEnvelope) {
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

func TestModulesRoutesDisabledByDefault(t *testing.T) {
	srv := newModulesTestServer(t, false, false)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp, env := doModulesJSON(t, ts, http.MethodGet, "/api/modules/mounts", nil)
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected 403, got %d", resp.StatusCode)
	}
	if env.Error.Code != "MODULES_DISABLED" {
		t.Errorf("expected MODULES_DISABLED, got %q", env.Error.Code)
	}

	resp2, env2 := doModulesJSON(t, ts, http.MethodPost, "/api/modules/mounts", map[string]any{
		"module": "series", "alias": "nums", "args": map[string]string{"stop": "3"},
	})
	if resp2.StatusCode != http.StatusForbidden || env2.Error.Code != "MODULES_DISABLED" {
		t.Errorf("expected MODULES_DISABLED on mount, got %d/%q", resp2.StatusCode, env2.Error.Code)
	}

	resp3, env3 := doModulesJSON(t, ts, http.MethodDelete, "/api/modules/mounts/nums", nil)
	if resp3.StatusCode != http.StatusForbidden || env3.Error.Code != "MODULES_DISABLED" {
		t.Errorf("expected MODULES_DISABLED on unmount, got %d/%q", resp3.StatusCode, env3.Error.Code)
	}

	// GET /api/modules itself always works (it's how the UI knows to render
	// the disabled banner rather than hide the tab).
	resp4, env4 := doModulesJSON(t, ts, http.MethodGet, "/api/modules", nil)
	if resp4.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 from /api/modules, got %d", resp4.StatusCode)
	}
	var data struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.Unmarshal(env4.Data, &data); err != nil {
		t.Fatal(err)
	}
	if data.Enabled {
		t.Error("expected enabled: false without --modules")
	}
}

func TestModulesMountUnmountPreviewWithoutWrite(t *testing.T) {
	// The important case: mount routes must work without --write, since
	// mounts are temp-scoped and mutate nothing in the user's database.
	srv := newModulesTestServer(t, true, false)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp, env := doModulesJSON(t, ts, http.MethodPost, "/api/modules/mounts", map[string]any{
		"module": "series",
		"alias":  "nums",
		"args":   map[string]string{"start": "0", "stop": "5"},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 mounting series, got %d: %s", resp.StatusCode, env.Error.Message)
	}

	respList, envList := doModulesJSON(t, ts, http.MethodGet, "/api/modules/mounts", nil)
	if respList.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 listing mounts, got %d", respList.StatusCode)
	}
	var mounts []map[string]any
	if err := json.Unmarshal(envList.Data, &mounts); err != nil {
		t.Fatal(err)
	}
	if len(mounts) != 1 || mounts[0]["alias"] != "nums" {
		t.Errorf("expected one mount named nums, got %v", mounts)
	}

	respPrev, envPrev := doModulesJSON(t, ts, http.MethodPost, "/api/modules/mounts/nums/preview", nil)
	if respPrev.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 previewing mount, got %d: %s", respPrev.StatusCode, envPrev.Error.Message)
	}
	var preview struct {
		Columns []string `json:"columns"`
		Rows    [][]any  `json:"rows"`
	}
	if err := json.Unmarshal(envPrev.Data, &preview); err != nil {
		t.Fatal(err)
	}
	if len(preview.Rows) != 5 {
		t.Errorf("expected 5 rows in preview, got %d", len(preview.Rows))
	}

	// Query the mount through the ordinary /api/query path (proves
	// WithMounts wiring reaches the query handler, not just modules routes).
	respQ, envQ := doModulesJSON(t, ts, http.MethodPost, "/api/query", map[string]string{"sql": "SELECT COUNT(*) AS n FROM nums"})
	if respQ.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 querying mounted table, got %d: %s", respQ.StatusCode, envQ.Error.Message)
	}

	respDel, envDel := doModulesJSON(t, ts, http.MethodDelete, "/api/modules/mounts/nums", nil)
	if respDel.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 unmounting, got %d: %s", respDel.StatusCode, envDel.Error.Message)
	}

	respDel2, _ := doModulesJSON(t, ts, http.MethodDelete, "/api/modules/mounts/nums", nil)
	if respDel2.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 unmounting an already-removed alias, got %d", respDel2.StatusCode)
	}
}

func TestModulesMountRejectsAliasCollisionWithRealTable(t *testing.T) {
	srv := newModulesTestServer(t, true, false)
	if _, err := srv.db.Exec(`CREATE TABLE users(id INTEGER PRIMARY KEY)`); err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp, env := doModulesJSON(t, ts, http.MethodPost, "/api/modules/mounts", map[string]any{
		"module": "series", "alias": "users", "args": map[string]string{"stop": "1"},
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 mounting alias colliding with a real table, got %d: %s", resp.StatusCode, env.Error.Message)
	}
}
