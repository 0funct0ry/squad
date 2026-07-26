package restserver

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/0funct0ry/squad/internal/db"
	"github.com/gin-gonic/gin"
)

func jsonBody(t *testing.T, v interface{}) io.Reader {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("failed to marshal json body: %v", err)
	}
	return bytes.NewReader(b)
}

func toStr(v interface{}) string {
	return fmt.Sprintf("%v", v)
}

// newTestConn opens an in-memory DB seeded with a simple table, a view, and
// an internal sqlite_sequence table (created implicitly by AUTOINCREMENT).
func newTestConn(t *testing.T) *sql.DB {
	t.Helper()
	conn, err := db.OpenDB(":memory:", false)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	stmts := []string{
		`CREATE TABLE users (id INTEGER PRIMARY KEY AUTOINCREMENT, email TEXT, active INTEGER)`,
		`INSERT INTO users (email, active) VALUES ('ada@example.com', 1)`,
		`INSERT INTO users (email, active) VALUES ('bob@example.com', 0)`,
		`CREATE VIEW active_users AS SELECT * FROM users WHERE active = 1`,
	}
	for _, s := range stmts {
		if _, err := conn.Exec(s); err != nil {
			t.Fatalf("failed to exec %q: %v", s, err)
		}
	}
	return conn
}

func buildSnapshotEngine(t *testing.T, conn *sql.DB, write bool, cfgByTable map[string]TableConfig) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)

	tables, err := db.GetTables(conn)
	if err != nil {
		t.Fatalf("GetTables: %v", err)
	}

	routeTables := make(map[string]RouteInfo)
	for _, tbl := range tables {
		cfg, ok := cfgByTable[tbl.Name]
		if !ok || !cfg.Exposed {
			continue
		}
		schema, err := db.GetTableSchema(conn, tbl.Name)
		if err != nil {
			t.Fatalf("GetTableSchema(%s): %v", tbl.Name, err)
		}
		routeTables[tbl.Name] = ResolveRouteInfo(tbl, schema, cfg, write)
	}

	engine := gin.New()
	registerRoutes(engine, conn, routeTables)
	return engine
}

func TestHandleList_ReturnsColumnKeyedBareArray(t *testing.T) {
	conn := newTestConn(t)
	engine := buildSnapshotEngine(t, conn, false, map[string]TableConfig{
		"users": {Exposed: true},
	})
	ts := httptest.NewServer(engine)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/rest/users")
	if err != nil {
		t.Fatalf("GET /rest/users: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var rows []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&rows); err != nil {
		t.Fatalf("expected a bare JSON array, got decode error: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	if rows[0]["email"] != "ada@example.com" {
		t.Errorf("expected column-keyed row, got: %+v", rows[0])
	}
}

func TestHandleList_ExactMatchFilter(t *testing.T) {
	conn := newTestConn(t)
	engine := buildSnapshotEngine(t, conn, false, map[string]TableConfig{"users": {Exposed: true}})
	ts := httptest.NewServer(engine)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/rest/users?active=1")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	var rows []map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&rows)
	if len(rows) != 1 || rows[0]["email"] != "ada@example.com" {
		t.Errorf("expected exact-match filter to return only ada, got: %+v", rows)
	}
}

func TestHandleGet_404OnMissingRow(t *testing.T) {
	conn := newTestConn(t)
	engine := buildSnapshotEngine(t, conn, false, map[string]TableConfig{"users": {Exposed: true}})
	ts := httptest.NewServer(engine)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/rest/users/9999")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
	var body map[string]string
	json.NewDecoder(resp.Body).Decode(&body)
	if body["error"] == "" {
		t.Errorf("expected {error,message} shape, got: %+v", body)
	}
}

func TestWriteRoutes_AbsentWhenNotEnabled(t *testing.T) {
	conn := newTestConn(t)
	// write=false at the manager level means Create/Update/Delete are false
	// in ResolveRouteInfo, so these routes are never mounted at all.
	engine := buildSnapshotEngine(t, conn, false, map[string]TableConfig{
		"users": {Exposed: true, Create: true, Update: true, Delete: true},
	})
	ts := httptest.NewServer(engine)
	defer ts.Close()

	resp, err := http.Post(ts.URL+"/rest/users", "application/json", nil)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected POST to 404 when write routes aren't mounted, got %d", resp.StatusCode)
	}
}

func TestWriteRoutes_CRUDRoundTrip(t *testing.T) {
	conn := newTestConn(t)
	engine := buildSnapshotEngine(t, conn, true, map[string]TableConfig{
		"users": {Exposed: true, Create: true, Update: true, Delete: true},
	})
	ts := httptest.NewServer(engine)
	defer ts.Close()

	// Create
	resp, err := http.Post(ts.URL+"/rest/users", "application/json", jsonBody(t, map[string]interface{}{
		"email": "carol@example.com", "active": 1,
	}))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}
	var created map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&created)
	resp.Body.Close()
	if created["email"] != "carol@example.com" {
		t.Fatalf("expected created row echoed back, got: %+v", created)
	}
	newID := created["id"]

	// Update
	req, _ := http.NewRequest(http.MethodPatch, ts.URL+"/rest/users/"+toStr(newID), jsonBody(t, map[string]interface{}{"active": 0}))
	req.Header.Set("Content-Type", "application/json")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PATCH: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var updated map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&updated)
	resp.Body.Close()
	if av, ok := updated["active"].(float64); !ok || int(av) != 0 {
		t.Errorf("expected active updated to 0, got: %+v", updated)
	}

	// Delete
	req, _ = http.NewRequest(http.MethodDelete, ts.URL+"/rest/users/"+toStr(newID), nil)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// Confirm gone
	resp, _ = http.Get(ts.URL + "/rest/users/" + toStr(newID))
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected deleted row to 404, got %d", resp.StatusCode)
	}
}

func TestHandleSchema_OnlyReflectsRunningSnapshot(t *testing.T) {
	conn := newTestConn(t)
	// active_users view exists but is NOT exposed in this snapshot.
	engine := buildSnapshotEngine(t, conn, false, map[string]TableConfig{"users": {Exposed: true}})
	ts := httptest.NewServer(engine)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/rest/_schema")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	var out []schemaTableView
	json.NewDecoder(resp.Body).Decode(&out)
	if len(out) != 1 || out[0].Table != "users" {
		t.Errorf("expected only the exposed 'users' table in _schema, got: %+v", out)
	}
}

func TestInternalTablesNeverExposable(t *testing.T) {
	conn := newTestConn(t)
	tables, err := db.GetTables(conn)
	if err != nil {
		t.Fatalf("GetTables: %v", err)
	}
	for _, tbl := range tables {
		if tbl.Name == "sqlite_sequence" {
			t.Fatalf("sqlite_sequence should never be listed as exposable, but GetTables returned it")
		}
	}
}
