package server

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/0funct0ry/squad/internal/db"
)

type metaResponse struct {
	Ok   bool      `json:"ok"`
	Data db.DBMeta `json:"data"`
	Err  *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func TestMetaHandler_NormalAndEmpty(t *testing.T) {
	// Create a temp db
	tmpDir, err := os.MkdirTemp("", "squad-meta-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "test.db")
	database, err := db.OpenDB(dbPath, false)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer database.Close()

	// 1. Test empty schema-less database returns tableCount:0, viewCount:0
	srv := NewServer(database, dbPath, false, false, false, "127.0.0.1", 7072)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/meta")
	if err != nil {
		t.Fatalf("failed GET /api/meta: %v", err)
	}
	defer resp.Body.Close()

	var res metaResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		t.Fatalf("failed decode response: %v", err)
	}

	if !res.Ok {
		t.Fatalf("expected response OK, got error: %+v", res.Err)
	}

	if res.Data.TableCount != 0 || res.Data.ViewCount != 0 {
		t.Errorf("expected 0 tables/views for empty db, got tables=%d, views=%d", res.Data.TableCount, res.Data.ViewCount)
	}

	// 2. Add some schema and test counts
	_, err = database.Exec(`
		CREATE TABLE users (id INTEGER PRIMARY KEY);
		CREATE VIEW active_users AS SELECT * FROM users;
	`)
	if err != nil {
		t.Fatalf("failed to create tables: %v", err)
	}

	resp2, err := http.Get(ts.URL + "/api/meta")
	if err != nil {
		t.Fatalf("failed GET /api/meta: %v", err)
	}
	defer resp2.Body.Close()

	var res2 metaResponse
	if err := json.NewDecoder(resp2.Body).Decode(&res2); err != nil {
		t.Fatalf("failed decode response: %v", err)
	}

	if res2.Data.TableCount != 1 || res2.Data.ViewCount != 1 {
		t.Errorf("expected 1 table and 1 view, got tables=%d, views=%d", res2.Data.TableCount, res2.Data.ViewCount)
	}
	if res2.Data.Mode != "ro" {
		t.Errorf("expected mode to be ro, got %q", res2.Data.Mode)
	}
}

func TestMetaHandler_JournalModes(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "squad-meta-journal-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Test default / delete mode
	dbPathDel := filepath.Join(tmpDir, "delete.db")
	dbDel, err := db.OpenDB(dbPathDel, false)
	if err != nil {
		t.Fatalf("failed to open delete db: %v", err)
	}
	defer dbDel.Close()

	srvDel := NewServer(dbDel, dbPathDel, false, false, false, "127.0.0.1", 7072)
	tsDel := httptest.NewServer(srvDel.Handler())
	defer tsDel.Close()

	respDel, err := http.Get(tsDel.URL + "/api/meta")
	if err != nil {
		t.Fatalf("failed GET: %v", err)
	}
	defer respDel.Body.Close()

	var resDel metaResponse
	json.NewDecoder(respDel.Body).Decode(&resDel)
	if resDel.Data.JournalMode == "" {
		t.Error("expected non-empty journal mode")
	}

	// Test WAL mode
	dbPathWal := filepath.Join(tmpDir, "wal.db")
	dbWal, err := db.OpenDB(dbPathWal, false)
	if err != nil {
		t.Fatalf("failed to open wal db: %v", err)
	}
	defer dbWal.Close()

	_, err = dbWal.Exec("PRAGMA journal_mode = WAL")
	if err != nil {
		t.Fatalf("failed to set WAL: %v", err)
	}

	srvWal := NewServer(dbWal, dbPathWal, false, false, false, "127.0.0.1", 7072)
	tsWal := httptest.NewServer(srvWal.Handler())
	defer tsWal.Close()

	respWal, err := http.Get(tsWal.URL + "/api/meta")
	if err != nil {
		t.Fatalf("failed GET: %v", err)
	}
	defer respWal.Body.Close()

	var resWal metaResponse
	json.NewDecoder(respWal.Body).Decode(&resWal)
	if resWal.Data.JournalMode != "wal" {
		t.Errorf("expected journal mode 'wal', got %q", resWal.Data.JournalMode)
	}
}

func TestMetaHandler_SizeBytesUpdates(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "squad-meta-size-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "size.db")
	database, err := db.OpenDB(dbPath, false)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer database.Close()

	srv := NewServer(database, dbPath, true, false, false, "127.0.0.1", 7072) // Write mode
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// Initial GET
	resp1, err := http.Get(ts.URL + "/api/meta")
	if err != nil {
		t.Fatalf("failed GET: %v", err)
	}
	var res1 metaResponse
	json.NewDecoder(resp1.Body).Decode(&res1)
	resp1.Body.Close()

	size1 := res1.Data.SizeBytes

	// Insert large row to grow file
	_, err = database.Exec("CREATE TABLE t (val TEXT)")
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}
	// Insert large blob of data
	stmt, err := database.Prepare("INSERT INTO t VALUES (?)")
	if err != nil {
		t.Fatalf("failed prepare: %v", err)
	}
	largeVal := make([]byte, 1024*100) // 100KB
	_, err = stmt.Exec(string(largeVal))
	if err != nil {
		t.Fatalf("failed exec: %v", err)
	}
	stmt.Close()

	// Second GET
	resp2, err := http.Get(ts.URL + "/api/meta")
	if err != nil {
		t.Fatalf("failed GET: %v", err)
	}
	var res2 metaResponse
	json.NewDecoder(resp2.Body).Decode(&res2)
	resp2.Body.Close()

	size2 := res2.Data.SizeBytes

	if size2 <= size1 {
		t.Errorf("expected size to increase, was %d -> %d", size1, size2)
	}
}

func TestMetaHandler_PathAbsolute(t *testing.T) {
	// Test that relative path resolves to absolute in resolvedPath / API meta response
	relPath := "./test_rel.db"
	absPath, err := filepath.Abs(relPath)
	if err != nil {
		t.Fatalf("failed to get absolute path: %v", err)
	}

	database, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory db: %v", err)
	}
	defer database.Close()

	// In the real app, cmd/root.go resolves absolute path at open time.
	// Let's pass the absolute path as the DB path to NewServer.
	srv := NewServer(database, absPath, false, false, false, "127.0.0.1", 7072)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/meta")
	if err != nil {
		t.Fatalf("failed GET: %v", err)
	}
	defer resp.Body.Close()

	var res metaResponse
	json.NewDecoder(resp.Body).Decode(&res)

	if res.Data.Path != absPath {
		t.Errorf("expected path to be absolute: %q, got %q", absPath, res.Data.Path)
	}
}
