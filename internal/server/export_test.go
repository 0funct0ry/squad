package server

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

// maxWriteTrackingWriter tracks the maximum size of a single Write call.
type maxWriteTrackingWriter struct {
	maxWriteSize int
	totalBytes   int64
	writeCount   int
	header       http.Header
}

func (w *maxWriteTrackingWriter) Header() http.Header {
	if w.header == nil {
		w.header = make(http.Header)
	}
	return w.header
}

func (w *maxWriteTrackingWriter) Write(p []byte) (n int, err error) {
	n = len(p)
	w.totalBytes += int64(n)
	w.writeCount++
	if n > w.maxWriteSize {
		w.maxWriteSize = n
	}
	return n, nil
}

func (w *maxWriteTrackingWriter) WriteHeader(statusCode int) {}

func TestServerExportTable(t *testing.T) {
	dbConn, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer dbConn.Close()

	// Create test table
	_, err = dbConn.Exec(`
		CREATE TABLE users (
			id INTEGER PRIMARY KEY,
			name TEXT,
			role TEXT
		);
		INSERT INTO users (id, name, role) VALUES 
			(1, 'Alice', 'admin'),
			(2, 'Bob', 'user'),
			(3, 'Charlie', 'user');
	`)
	if err != nil {
		t.Fatalf("failed to seed database: %v", err)
	}

	srv := NewServer(dbConn, "test.db", false, false, false, "127.0.0.1", 7072)

	t.Run("CSV Export Default", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/tables/users/export?format=csv", nil)
		resp := httptest.NewRecorder()
		srv.Handler().ServeHTTP(resp, req)

		if resp.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", resp.Code)
		}
		expectedCSV := "id,name,role\n1,Alice,admin\n2,Bob,user\n3,Charlie,user\n"
		if resp.Body.String() != expectedCSV {
			t.Errorf("expected body %q, got %q", expectedCSV, resp.Body.String())
		}
		disp := resp.Header().Get("Content-Disposition")
		if !strings.Contains(disp, `filename="users.csv"`) {
			t.Errorf("unexpected Content-Disposition: %s", disp)
		}
	})

	t.Run("CSV Export Headers False", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/tables/users/export?format=csv&headers=false", nil)
		resp := httptest.NewRecorder()
		srv.Handler().ServeHTTP(resp, req)

		if resp.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", resp.Code)
		}
		expectedCSV := "1,Alice,admin\n2,Bob,user\n3,Charlie,user\n"
		if resp.Body.String() != expectedCSV {
			t.Errorf("expected body %q, got %q", expectedCSV, resp.Body.String())
		}
	})

	t.Run("CSV Export Filtered", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/tables/users/export?format=csv&filtered=true&filter[role]=user&orderBy=id&dir=desc", nil)
		resp := httptest.NewRecorder()
		srv.Handler().ServeHTTP(resp, req)

		if resp.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", resp.Code)
		}
		expectedCSV := "id,name,role\n3,Charlie,user\n2,Bob,user\n"
		if resp.Body.String() != expectedCSV {
			t.Errorf("expected body %q, got %q", expectedCSV, resp.Body.String())
		}
	})

	t.Run("JSON Export", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/tables/users/export?format=json", nil)
		resp := httptest.NewRecorder()
		srv.Handler().ServeHTTP(resp, req)

		if resp.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", resp.Code)
		}
		var list []map[string]any
		if err := json.Unmarshal(resp.Body.Bytes(), &list); err != nil {
			t.Fatalf("failed to parse JSON response: %v", err)
		}
		if len(list) != 3 {
			t.Errorf("expected 3 rows, got %d", len(list))
		}
		if list[0]["name"] != "Alice" || list[1]["name"] != "Bob" {
			t.Errorf("unexpected json contents: %v", list)
		}
	})

	t.Run("SQL Export Only Inserts", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/tables/users/export?format=sql", nil)
		resp := httptest.NewRecorder()
		srv.Handler().ServeHTTP(resp, req)

		if resp.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", resp.Code)
		}
		sqlBody := resp.Body.String()
		if strings.Contains(sqlBody, "CREATE TABLE") {
			t.Errorf("should not contain CREATE TABLE, got:\n%s", sqlBody)
		}
		if !strings.Contains(sqlBody, `INSERT INTO "users" ("id","name","role") VALUES (1,'Alice','admin');`) {
			t.Errorf("missing insert statement, got:\n%s", sqlBody)
		}
	})

	t.Run("SQL Export With Schema", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/tables/users/export?format=sql&includeSchema=true", nil)
		resp := httptest.NewRecorder()
		srv.Handler().ServeHTTP(resp, req)

		if resp.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", resp.Code)
		}
		sqlBody := resp.Body.String()
		if !strings.Contains(sqlBody, "CREATE TABLE users") {
			t.Errorf("should contain CREATE TABLE, got:\n%s", sqlBody)
		}
		if !strings.Contains(sqlBody, `INSERT INTO "users" ("id","name","role") VALUES (1,'Alice','admin');`) {
			t.Errorf("missing insert statement, got:\n%s", sqlBody)
		}
	})

	t.Run("Export Nonexistent Table", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/tables/missing_table/export?format=csv", nil)
		resp := httptest.NewRecorder()
		srv.Handler().ServeHTTP(resp, req)

		if resp.Code != http.StatusNotFound {
			t.Errorf("expected 404, got %d", resp.Code)
		}
		var errResp map[string]any
		if err := json.Unmarshal(resp.Body.Bytes(), &errResp); err != nil {
			t.Fatalf("failed to parse JSON error: %v", err)
		}
		if errResp["ok"] != false {
			t.Errorf("expected ok to be false, got %v", errResp["ok"])
		}
	})
}

func TestServerQueryExport(t *testing.T) {
	dbConn, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer dbConn.Close()

	_, err = dbConn.Exec(`
		CREATE TABLE items (id INTEGER PRIMARY KEY, name TEXT);
		INSERT INTO items (id, name) VALUES (1, 'item1'), (2, 'item2');
	`)
	if err != nil {
		t.Fatalf("failed to seed: %v", err)
	}

	srv := NewServer(dbConn, "test.db", true, false, false, "127.0.0.1", 7072) // server in write mode

	t.Run("Query Export CSV", func(t *testing.T) {
		body := `{"sql": "SELECT name FROM items ORDER BY id DESC"}`
		req := httptest.NewRequest("POST", "/api/export/query?format=csv", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp := httptest.NewRecorder()
		srv.Handler().ServeHTTP(resp, req)

		if resp.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", resp.Code)
		}
		expected := "name\nitem2\nitem1\n"
		if resp.Body.String() != expected {
			t.Errorf("expected body %q, got %q", expected, resp.Body.String())
		}
		disp := resp.Header().Get("Content-Disposition")
		if !strings.Contains(disp, `filename="query-export.csv"`) {
			t.Errorf("unexpected Content-Disposition: %s", disp)
		}
	})

	t.Run("Query Export Rejects Write", func(t *testing.T) {
		body := `{"sql": "INSERT INTO items (name) VALUES ('item3')"}`
		req := httptest.NewRequest("POST", "/api/export/query?format=csv", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp := httptest.NewRecorder()
		srv.Handler().ServeHTTP(resp, req)

		if resp.Code != http.StatusForbidden {
			t.Errorf("expected 403, got %d", resp.Code)
		}
		var errResp map[string]any
		if err := json.Unmarshal(resp.Body.Bytes(), &errResp); err != nil {
			t.Fatalf("failed to parse JSON error: %v", err)
		}
		if errResp["ok"] != false {
			t.Errorf("expected ok to be false, got %v", errResp["ok"])
		}
	})
}

func TestStreamingLargeExportMemory(t *testing.T) {
	dbConn, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer dbConn.Close()

	srv := NewServer(dbConn, "test.db", false, false, false, "127.0.0.1", 7072)

	// We'll write to a custom tracker to measure write sizes and count
	tracker := &maxWriteTrackingWriter{}

	// Prepare query parameter for ad-hoc export
	body := `{"sql": "WITH RECURSIVE cnt(x) AS (SELECT 1 UNION ALL SELECT x+1 FROM cnt LIMIT 100000) SELECT x, 'large_text_payload_to_stress_buffers_abcdefghijklmnopqrstuvwxyz' FROM cnt"}`
	req := httptest.NewRequest("POST", "/api/export/query?format=csv", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	srv.Handler().ServeHTTP(tracker, req)

	if tracker.totalBytes < 5000000 {
		t.Errorf("expected at least ~5MB of data, got only %d bytes", tracker.totalBytes)
	}

	// Because we stream to the response writer, the maximum size of a single write
	// should be bounded (typically Go's internal buffer or gin writer chunk is 32KB/64KB).
	// If it was fully buffered and written in one go, maxWriteSize would equal totalBytes (>= 5MB).
	// Let's assert maxWriteSize is less than 500KB (typically much smaller like 4KB or 32KB).
	const limitSize = 500 * 1024
	if tracker.maxWriteSize > limitSize {
		t.Errorf("Export was fully buffered! Max single write size was %d bytes out of %d total", tracker.maxWriteSize, tracker.totalBytes)
	}

	if tracker.writeCount < 10 {
		t.Errorf("expected many small writes, got only %d write calls", tracker.writeCount)
	}

	t.Logf("Total bytes: %d, Max write size: %d, Write count: %d", tracker.totalBytes, tracker.maxWriteSize, tracker.writeCount)
}
