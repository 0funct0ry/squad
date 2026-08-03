package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/0funct0ry/squad/internal/db"
	"github.com/0funct0ry/squad/internal/testfixtures"
)

// scratchExamplePath builds the named fixture database fresh into a temp
// file, so tests never depend on checked-in fixtures or external tooling.
func scratchExamplePath(t *testing.T, name string) string {
	t.Helper()
	dst := filepath.Join(t.TempDir(), name+".db")
	if name != "blog" {
		t.Fatalf("scratchExamplePath: no fixture builder registered for %q", name)
	}
	if err := testfixtures.BuildBlog(dst); err != nil {
		t.Fatalf("failed to build fixture db %s: %v", name, err)
	}
	return dst
}

func doJSON(t *testing.T, client *http.Client, method, url string, body any) *http.Response {
	t.Helper()
	var reqBody *bytes.Buffer
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal failed: %v", err)
		}
		reqBody = bytes.NewBuffer(b)
	} else {
		reqBody = bytes.NewBuffer(nil)
	}
	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	return resp
}

func TestSeedPlanEndpoint(t *testing.T) {
	dbPath := scratchExamplePath(t, "blog")
	database, err := db.OpenDB(dbPath, false)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer database.Close()

	srv := NewServer(database, dbPath, true, false, false, "127.0.0.1", 7072, "info")
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()
	client := ts.Client()

	resp := doJSON(t, client, "GET", ts.URL+"/api/tables/users/seed/plan", nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	res := parseResponse(t, resp)
	if !res.Ok {
		t.Fatalf("expected ok:true, got %+v", res)
	}

	columns, ok := res.Data["columns"].([]any)
	if !ok {
		t.Fatalf("expected columns array, got %T", res.Data["columns"])
	}

	found := map[string]map[string]any{}
	for _, c := range columns {
		col := c.(map[string]any)
		found[col["name"].(string)] = col
	}

	idCol := found["id"]
	if idCol["skip"] != true {
		t.Errorf("expected id to be skipped, got %+v", idCol)
	}

	usernameCol := found["username"]
	if usernameCol["generator"] != "username" {
		t.Errorf("expected username generator, got %+v", usernameCol)
	}

	availableGens, ok := res.Data["availableGenerators"].([]any)
	if !ok || len(availableGens) == 0 {
		t.Fatalf("expected non-empty availableGenerators, got %v", res.Data["availableGenerators"])
	}
}

func TestSeedPlan_ViewRejected(t *testing.T) {
	dbPath := scratchExamplePath(t, "blog")
	database, err := db.OpenDB(dbPath, false)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer database.Close()

	srv := NewServer(database, dbPath, true, false, false, "127.0.0.1", 7072, "info")
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()
	client := ts.Client()

	resp := doJSON(t, client, "GET", ts.URL+"/api/tables/published_posts/seed/plan", nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
	res := parseResponse(t, resp)
	if res.Error == nil || res.Error.Code != "BAD_REQUEST" {
		t.Errorf("expected BAD_REQUEST, got %+v", res.Error)
	}

	respPost := doJSON(t, client, "POST", ts.URL+"/api/tables/published_posts/seed",
		map[string]any{"count": 1, "dryRun": true, "columns": map[string]any{}})
	defer respPost.Body.Close()
	if respPost.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for POST seed on view, got %d", respPost.StatusCode)
	}
}

func TestSeedDryRunDoesNotInsert(t *testing.T) {
	dbPath := scratchExamplePath(t, "blog")
	database, err := db.OpenDB(dbPath, false)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer database.Close()

	srv := NewServer(database, dbPath, true, false, false, "127.0.0.1", 7072, "info")
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()
	client := ts.Client()

	var before int
	if err := database.QueryRow("SELECT COUNT(*) FROM tags").Scan(&before); err != nil {
		t.Fatal(err)
	}

	resp := doJSON(t, client, "POST", ts.URL+"/api/tables/tags/seed", map[string]any{
		"count":  5,
		"dryRun": true,
		"columns": map[string]any{
			"name": map[string]any{"generator": "word"},
		},
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	res := parseResponse(t, resp)
	rows, ok := res.Data["rows"].([]any)
	if !ok || len(rows) != 5 {
		t.Fatalf("expected 5 preview rows, got %v", res.Data["rows"])
	}

	var after int
	if err := database.QueryRow("SELECT COUNT(*) FROM tags").Scan(&after); err != nil {
		t.Fatal(err)
	}
	if before != after {
		t.Errorf("expected row count unchanged by dry run, before=%d after=%d", before, after)
	}
}

func TestSeedInsertExactCount(t *testing.T) {
	dbPath := scratchExamplePath(t, "blog")
	database, err := db.OpenDB(dbPath, false)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer database.Close()

	srv := NewServer(database, dbPath, true, false, false, "127.0.0.1", 7072, "info")
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()
	client := ts.Client()

	var before int
	if err := database.QueryRow("SELECT COUNT(*) FROM tags").Scan(&before); err != nil {
		t.Fatal(err)
	}

	const count = 200
	resp := doJSON(t, client, "POST", ts.URL+"/api/tables/tags/seed", map[string]any{
		"count":  count,
		"dryRun": false,
		"columns": map[string]any{
			"name": map[string]any{"generator": "uuid"},
		},
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body := parseResponse(t, resp)
		t.Fatalf("expected 200, got %d: %+v", resp.StatusCode, body.Error)
	}
	res := parseResponse(t, resp)
	if int(res.Data["inserted"].(float64)) != count {
		t.Errorf("expected inserted=%d, got %v", count, res.Data["inserted"])
	}

	var after int
	if err := database.QueryRow("SELECT COUNT(*) FROM tags").Scan(&after); err != nil {
		t.Fatal(err)
	}
	if after-before != count {
		t.Errorf("expected %d new rows, got %d", count, after-before)
	}
}

func TestSeedForeignKeyValuesValid(t *testing.T) {
	dbPath := scratchExamplePath(t, "blog")
	database, err := db.OpenDB(dbPath, false)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer database.Close()

	srv := NewServer(database, dbPath, true, false, false, "127.0.0.1", 7072, "info")
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()
	client := ts.Client()

	resp := doJSON(t, client, "POST", ts.URL+"/api/tables/posts/seed", map[string]any{
		"count":  20,
		"dryRun": false,
		"columns": map[string]any{
			"author_id": map[string]any{"generator": "foreignKey", "options": map[string]any{"table": "users", "column": "id"}},
			"title":     map[string]any{"generator": "sentence"},
			"slug":      map[string]any{"generator": "uuid"},
		},
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body := parseResponse(t, resp)
		t.Fatalf("expected 200, got %d: %+v", resp.StatusCode, body.Error)
	}

	validIDs := map[int64]bool{}
	rows, err := database.Query("SELECT id FROM users")
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		validIDs[id] = true
	}
	rows.Close()

	authorRows, err := database.Query("SELECT DISTINCT author_id FROM posts")
	if err != nil {
		t.Fatal(err)
	}
	defer authorRows.Close()
	for authorRows.Next() {
		var authorID int64
		if err := authorRows.Scan(&authorID); err != nil {
			t.Fatal(err)
		}
		if !validIDs[authorID] {
			t.Errorf("post author_id %d does not exist in users", authorID)
		}
	}
}

func TestSeedEmptyReference(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "empty_ref.db")
	database, err := db.OpenDB(dbPath, false)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	if _, err := database.Exec(`
		CREATE TABLE parent (id INTEGER PRIMARY KEY, name TEXT);
		CREATE TABLE child (id INTEGER PRIMARY KEY, parent_id INTEGER REFERENCES parent(id));
	`); err != nil {
		t.Fatal(err)
	}

	srv := NewServer(database, dbPath, true, false, false, "127.0.0.1", 7072, "info")
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()
	client := ts.Client()

	resp := doJSON(t, client, "POST", ts.URL+"/api/tables/child/seed", map[string]any{
		"count":  1,
		"dryRun": true,
		"columns": map[string]any{
			"parent_id": map[string]any{"generator": "foreignKey", "options": map[string]any{"table": "parent", "column": "id"}},
		},
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
	res := parseResponse(t, resp)
	if res.Error == nil || res.Error.Code != "EMPTY_REFERENCE" {
		t.Errorf("expected EMPTY_REFERENCE, got %+v", res.Error)
	}
}

func TestSeedUniqueExhaustionRollsBack(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "unique_exhaustion.db")
	database, err := db.OpenDB(dbPath, false)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	if _, err := database.Exec(`CREATE TABLE t (id INTEGER PRIMARY KEY, flag TEXT UNIQUE)`); err != nil {
		t.Fatal(err)
	}

	srv := NewServer(database, dbPath, true, false, false, "127.0.0.1", 7072, "info")
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()
	client := ts.Client()

	resp := doJSON(t, client, "POST", ts.URL+"/api/tables/t/seed", map[string]any{
		"count":  5,
		"dryRun": false,
		"columns": map[string]any{
			"flag": map[string]any{"generator": "bool"},
		},
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
	res := parseResponse(t, resp)
	if res.Error == nil || res.Error.Code != "SQL_ERROR" {
		t.Errorf("expected SQL_ERROR, got %+v", res.Error)
	}

	var count int
	if err := database.QueryRow("SELECT COUNT(*) FROM t").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Errorf("expected transaction to roll back with zero rows persisted, got %d", count)
	}
}

func TestSeedCompositePKRetryGroup(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "composite_pk.db")
	database, err := db.OpenDB(dbPath, false)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	if _, err := database.Exec(`CREATE TABLE pt (a INTEGER NOT NULL, b INTEGER NOT NULL, PRIMARY KEY (a, b))`); err != nil {
		t.Fatal(err)
	}

	srv := NewServer(database, dbPath, true, false, false, "127.0.0.1", 7072, "info")
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()
	client := ts.Client()

	resp := doJSON(t, client, "POST", ts.URL+"/api/tables/pt/seed", map[string]any{
		"count":  4,
		"dryRun": false,
		"columns": map[string]any{
			"a": map[string]any{"generator": "int", "options": map[string]any{"min": 0, "max": 1}},
			"b": map[string]any{"generator": "int", "options": map[string]any{"min": 0, "max": 1}},
		},
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body := parseResponse(t, resp)
		t.Fatalf("expected 200, got %d: %+v", resp.StatusCode, body.Error)
	}

	var count int
	if err := database.QueryRow("SELECT COUNT(DISTINCT a || '-' || b) FROM pt").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 4 {
		t.Errorf("expected 4 distinct (a,b) pairs, got %d", count)
	}
}

func TestSeedCountBounds(t *testing.T) {
	dbPath := scratchExamplePath(t, "blog")
	database, err := db.OpenDB(dbPath, false)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	srv := NewServer(database, dbPath, true, false, false, "127.0.0.1", 7072, "info")
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()
	client := ts.Client()

	for _, count := range []int{0, 100001} {
		resp := doJSON(t, client, "POST", ts.URL+"/api/tables/tags/seed", map[string]any{
			"count":   count,
			"dryRun":  true,
			"columns": map[string]any{"name": map[string]any{"generator": "word"}},
		})
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("count=%d: expected 400, got %d", count, resp.StatusCode)
		}
		res := parseResponse(t, resp)
		if res.Error == nil || res.Error.Code != "BAD_REQUEST" {
			t.Errorf("count=%d: expected BAD_REQUEST, got %+v", count, res.Error)
		}
	}
}

func TestSeedUnknownGenerator(t *testing.T) {
	dbPath := scratchExamplePath(t, "blog")
	database, err := db.OpenDB(dbPath, false)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	srv := NewServer(database, dbPath, true, false, false, "127.0.0.1", 7072, "info")
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()
	client := ts.Client()

	resp := doJSON(t, client, "POST", ts.URL+"/api/tables/tags/seed", map[string]any{
		"count":   1,
		"dryRun":  true,
		"columns": map[string]any{"name": map[string]any{"generator": "bogus"}},
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
	res := parseResponse(t, resp)
	if res.Error == nil || res.Error.Code != "UNKNOWN_GENERATOR" {
		t.Errorf("expected UNKNOWN_GENERATOR, got %+v", res.Error)
	}
}

func TestSeedReadOnlyGating(t *testing.T) {
	dbPath := scratchExamplePath(t, "blog")
	database, err := db.OpenDB(dbPath, false)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	var before int
	if err := database.QueryRow("SELECT COUNT(*) FROM users").Scan(&before); err != nil {
		t.Fatal(err)
	}

	srv := NewServer(database, dbPath, false, false, false, "127.0.0.1", 7072, "info") // read-only
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()
	client := ts.Client()

	resp := doJSON(t, client, "GET", ts.URL+"/api/tables/users/seed/plan", nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected 403 for GET seed/plan in read-only mode, got %d", resp.StatusCode)
	}
	res := parseResponse(t, resp)
	if res.Error == nil || res.Error.Code != "READ_ONLY" {
		t.Errorf("expected READ_ONLY, got %+v", res.Error)
	}

	respPost := doJSON(t, client, "POST", ts.URL+"/api/tables/users/seed", map[string]any{
		"count": 1, "dryRun": true, "columns": map[string]any{},
	})
	defer respPost.Body.Close()
	if respPost.StatusCode != http.StatusForbidden {
		t.Errorf("expected 403 for POST seed in read-only mode, got %d", respPost.StatusCode)
	}
	resPost := parseResponse(t, respPost)
	if resPost.Error == nil || resPost.Error.Code != "READ_ONLY" {
		t.Errorf("expected READ_ONLY, got %+v", resPost.Error)
	}

	var after int
	if err := database.QueryRow("SELECT COUNT(*) FROM users").Scan(&after); err != nil {
		t.Fatal(err)
	}
	if before != after {
		t.Errorf("expected users row count unchanged, before=%d after=%d", before, after)
	}
}

func TestSeedFormulaCycleRejected_ZeroRowsInserted(t *testing.T) {
	dbPath := scratchExamplePath(t, "blog")
	database, err := db.OpenDB(dbPath, false)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer database.Close()

	srv := NewServer(database, dbPath, true, false, false, "127.0.0.1", 7072, "info")
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()
	client := ts.Client()

	var before int
	if err := database.QueryRow("SELECT COUNT(*) FROM posts").Scan(&before); err != nil {
		t.Fatal(err)
	}

	resp := doJSON(t, client, "POST", ts.URL+"/api/tables/posts/seed", map[string]any{
		"count":  1,
		"dryRun": false,
		"columns": map[string]any{
			"title": map[string]any{"generator": "formula", "options": map[string]any{
				"columns": []any{"body"}, "expression": "body",
			}},
			"body": map[string]any{"generator": "formula", "options": map[string]any{
				"columns": []any{"title"}, "expression": "title",
			}},
		},
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for a formula cycle, got %d", resp.StatusCode)
	}
	res := parseResponse(t, resp)
	if res.Error == nil || res.Error.Code != "BAD_REQUEST" {
		t.Errorf("expected BAD_REQUEST, got %+v", res.Error)
	}

	var after int
	if err := database.QueryRow("SELECT COUNT(*) FROM posts").Scan(&after); err != nil {
		t.Fatal(err)
	}
	if before != after {
		t.Errorf("expected zero rows inserted, before=%d after=%d", before, after)
	}
}

func TestSeedGeneratorSample_ValidPair(t *testing.T) {
	dbPath := scratchExamplePath(t, "blog")
	database, err := db.OpenDB(dbPath, false)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	srv := NewServer(database, dbPath, true, false, false, "127.0.0.1", 7072, "info")
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()
	client := ts.Client()

	resp := doJSON(t, client, "GET", ts.URL+"/api/seed/generators/email/sample", nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	res := parseResponse(t, resp)
	if res.Data["sample"] == nil {
		t.Errorf("expected a non-nil sample, got %+v", res.Data)
	}
	if res.Data["affinityUsed"] != "TEXT" {
		t.Errorf("expected affinityUsed=TEXT, got %v", res.Data["affinityUsed"])
	}
}

func TestSeedGeneratorSample_UnknownGenerator(t *testing.T) {
	dbPath := scratchExamplePath(t, "blog")
	database, err := db.OpenDB(dbPath, false)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	srv := NewServer(database, dbPath, true, false, false, "127.0.0.1", 7072, "info")
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()
	client := ts.Client()

	resp := doJSON(t, client, "GET", ts.URL+"/api/seed/generators/nonsense/sample", nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
	res := parseResponse(t, resp)
	if res.Error == nil || res.Error.Code != "NOT_FOUND" {
		t.Errorf("expected NOT_FOUND, got %+v", res.Error)
	}
}

func TestSeedGeneratorSample_ForeignKeyAndFormulaRejected(t *testing.T) {
	dbPath := scratchExamplePath(t, "blog")
	database, err := db.OpenDB(dbPath, false)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	srv := NewServer(database, dbPath, true, false, false, "127.0.0.1", 7072, "info")
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()
	client := ts.Client()

	for _, name := range []string{"foreignKey", "formula", "enumFromColumn"} {
		resp := doJSON(t, client, "GET", ts.URL+"/api/seed/generators/"+name+"/sample", nil)
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("%s: expected 400, got %d", name, resp.StatusCode)
		}
		res := parseResponse(t, resp)
		if res.Error == nil || res.Error.Code != "BAD_REQUEST" {
			t.Errorf("%s: expected BAD_REQUEST, got %+v", name, res.Error)
		}
	}
}

func TestSeedEnumFromColumnValuesValid(t *testing.T) {
	dbPath := scratchExamplePath(t, "blog")
	database, err := db.OpenDB(dbPath, false)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer database.Close()

	srv := NewServer(database, dbPath, true, false, false, "127.0.0.1", 7072, "info")
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()
	client := ts.Client()

	resp := doJSON(t, client, "POST", ts.URL+"/api/tables/post_stats/seed", map[string]any{
		"count":  5,
		"dryRun": true,
		"columns": map[string]any{
			"post_id": map[string]any{"generator": "enumFromColumn", "options": map[string]any{"table": "posts", "column": "status"}},
		},
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body := parseResponse(t, resp)
		t.Fatalf("expected 200, got %d: %+v", resp.StatusCode, body.Error)
	}
}

func TestSeedEnumFromColumnUnknownTableRejected(t *testing.T) {
	dbPath := scratchExamplePath(t, "blog")
	database, err := db.OpenDB(dbPath, false)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	srv := NewServer(database, dbPath, true, false, false, "127.0.0.1", 7072, "info")
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()
	client := ts.Client()

	resp := doJSON(t, client, "POST", ts.URL+"/api/tables/posts/seed", map[string]any{
		"count":  1,
		"dryRun": true,
		"columns": map[string]any{
			"title": map[string]any{"generator": "enumFromColumn", "options": map[string]any{"table": "nope", "column": "x"}},
		},
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
	res := parseResponse(t, resp)
	if res.Error == nil || res.Error.Code != "NOT_FOUND" {
		t.Errorf("expected NOT_FOUND, got %+v", res.Error)
	}
}

func TestSeedEnumFromColumnUnknownColumnRejected(t *testing.T) {
	dbPath := scratchExamplePath(t, "blog")
	database, err := db.OpenDB(dbPath, false)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	srv := NewServer(database, dbPath, true, false, false, "127.0.0.1", 7072, "info")
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()
	client := ts.Client()

	resp := doJSON(t, client, "POST", ts.URL+"/api/tables/posts/seed", map[string]any{
		"count":  1,
		"dryRun": true,
		"columns": map[string]any{
			"title": map[string]any{"generator": "enumFromColumn", "options": map[string]any{"table": "posts", "column": "nope"}},
		},
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
	res := parseResponse(t, resp)
	if res.Error == nil || res.Error.Code != "NOT_FOUND" {
		t.Errorf("expected NOT_FOUND, got %+v", res.Error)
	}
}

func TestSeedNullWithProbabilityUnknownWrappedGeneratorRejected(t *testing.T) {
	dbPath := scratchExamplePath(t, "blog")
	database, err := db.OpenDB(dbPath, false)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	srv := NewServer(database, dbPath, true, false, false, "127.0.0.1", 7072, "info")
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()
	client := ts.Client()

	resp := doJSON(t, client, "POST", ts.URL+"/api/tables/posts/seed", map[string]any{
		"count":  1,
		"dryRun": true,
		"columns": map[string]any{
			"title": map[string]any{"generator": "nullWithProbability", "options": map[string]any{
				"generator": map[string]any{"generator": "doesNotExist"},
				"nullRate":  0.1,
			}},
		},
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
	res := parseResponse(t, resp)
	if res.Error == nil || res.Error.Code != "BAD_REQUEST" {
		t.Errorf("expected BAD_REQUEST, got %+v", res.Error)
	}
}

func TestSeedNullWithProbabilitySelfReferenceRejected(t *testing.T) {
	dbPath := scratchExamplePath(t, "blog")
	database, err := db.OpenDB(dbPath, false)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	srv := NewServer(database, dbPath, true, false, false, "127.0.0.1", 7072, "info")
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()
	client := ts.Client()

	resp := doJSON(t, client, "POST", ts.URL+"/api/tables/posts/seed", map[string]any{
		"count":  1,
		"dryRun": true,
		"columns": map[string]any{
			"title": map[string]any{"generator": "nullWithProbability", "options": map[string]any{
				"generator": map[string]any{"generator": "nullWithProbability"},
				"nullRate":  0.1,
			}},
		},
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
	res := parseResponse(t, resp)
	if res.Error == nil || res.Error.Code != "BAD_REQUEST" {
		t.Errorf("expected BAD_REQUEST, got %+v", res.Error)
	}
}

func TestSeedNullWithProbabilityValidWrapWorks(t *testing.T) {
	dbPath := scratchExamplePath(t, "blog")
	database, err := db.OpenDB(dbPath, false)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	srv := NewServer(database, dbPath, true, false, false, "127.0.0.1", 7072, "info")
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()
	client := ts.Client()

	resp := doJSON(t, client, "POST", ts.URL+"/api/tables/posts/seed", map[string]any{
		"count":  10,
		"dryRun": true,
		"columns": map[string]any{
			"title": map[string]any{"generator": "nullWithProbability", "options": map[string]any{
				"generator": map[string]any{"generator": "sentence"},
				"nullRate":  0.2,
			}},
		},
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body := parseResponse(t, resp)
		t.Fatalf("expected 200, got %d: %+v", resp.StatusCode, body.Error)
	}
}

func TestSeedGeneratorSample_MalformedOptionsJSON(t *testing.T) {
	dbPath := scratchExamplePath(t, "blog")
	database, err := db.OpenDB(dbPath, false)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	srv := NewServer(database, dbPath, true, false, false, "127.0.0.1", 7072, "info")
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()
	client := ts.Client()

	resp := doJSON(t, client, "GET", ts.URL+"/api/seed/generators/int/sample?options=not-json", nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
	res := parseResponse(t, resp)
	if res.Error == nil || res.Error.Code != "BAD_REQUEST" {
		t.Errorf("expected BAD_REQUEST, got %+v", res.Error)
	}
}

func TestSeedGeneratorSample_AffinityMismatch(t *testing.T) {
	dbPath := scratchExamplePath(t, "blog")
	database, err := db.OpenDB(dbPath, false)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	srv := NewServer(database, dbPath, true, false, false, "127.0.0.1", 7072, "info")
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()
	client := ts.Client()

	resp := doJSON(t, client, "GET", ts.URL+"/api/seed/generators/email/sample?affinity=INTEGER", nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
	res := parseResponse(t, resp)
	if res.Error == nil || res.Error.Code != "BAD_REQUEST" {
		t.Errorf("expected BAD_REQUEST, got %+v", res.Error)
	}
}
