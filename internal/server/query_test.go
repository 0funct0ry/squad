package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/0funct0ry/squad/internal/db"
)

func TestQueryHandlers(t *testing.T) {
	// Create an in-memory database for testing
	database, err := db.OpenDB(":memory:", false)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer database.Close()

	// Seed database
	_, err = database.Exec(`
		CREATE TABLE users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			email TEXT NOT NULL UNIQUE
		);
		INSERT INTO users (email) VALUES ('ada@example.com');
		INSERT INTO users (email) VALUES ('linus@example.com');
	`)
	if err != nil {
		t.Fatalf("failed to seed db: %v", err)
	}

	// 1. Test plain SELECT in read-only server
	srv := NewServer(database, ":memory:", false, false, false, "127.0.0.1", 7072) // write = false
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	{
		body, _ := json.Marshal(map[string]any{"sql": "SELECT id, email FROM users ORDER BY id ASC"})
		resp, err := http.Post(ts.URL+"/api/query", "application/json", bytes.NewBuffer(body))
		if err != nil {
			t.Fatalf("POST /api/query failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected status OK, got %d", resp.StatusCode)
		}

		var res struct {
			Ok   bool `json:"ok"`
			Data struct {
				Columns      []string `json:"columns"`
				Rows         [][]any  `json:"rows"`
				RowsAffected int      `json:"rowsAffected"`
				DurationMs   float64  `json:"durationMs"`
				Limit        int      `json:"limit"`
				Truncated    bool     `json:"truncated"`
			} `json:"data"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if !res.Ok {
			t.Errorf("expected Ok to be true")
		}
		if len(res.Data.Columns) != 2 || res.Data.Columns[0] != "id" || res.Data.Columns[1] != "email" {
			t.Errorf("unexpected columns: %v", res.Data.Columns)
		}
		if len(res.Data.Rows) != 2 {
			t.Errorf("expected 2 rows, got %d", len(res.Data.Rows))
		}
		if res.Data.RowsAffected != 0 {
			t.Errorf("expected rowsAffected to be 0 for SELECT")
		}
		if res.Data.DurationMs <= 0 {
			t.Errorf("expected durationMs to be > 0, got %f", res.Data.DurationMs)
		}
		if res.Data.Truncated {
			t.Errorf("expected truncated to be false")
		}
	}

	// 2. Test SELECT with limit truncation
	{
		body, _ := json.Marshal(map[string]any{"sql": "SELECT id, email FROM users ORDER BY id ASC", "limit": 1})
		resp, err := http.Post(ts.URL+"/api/query", "application/json", bytes.NewBuffer(body))
		if err != nil {
			t.Fatalf("POST /api/query failed: %v", err)
		}
		defer resp.Body.Close()

		var res struct {
			Ok   bool `json:"ok"`
			Data struct {
				Rows      [][]any `json:"rows"`
				Truncated bool    `json:"truncated"`
				Limit     int     `json:"limit"`
			} `json:"data"`
		}
		json.NewDecoder(resp.Body).Decode(&res)

		if len(res.Data.Rows) != 1 {
			t.Errorf("expected 1 row due to limit, got %d", len(res.Data.Rows))
		}
		if !res.Data.Truncated {
			t.Errorf("expected truncated to be true")
		}
		if res.Data.Limit != 1 {
			t.Errorf("expected limit to be 1, got %d", res.Data.Limit)
		}
	}

	// 3. Test INSERT in read-only mode (should be blocked)
	{
		body, _ := json.Marshal(map[string]any{"sql": "INSERT INTO users (email) VALUES ('grace@example.com')"})
		resp, err := http.Post(ts.URL+"/api/query", "application/json", bytes.NewBuffer(body))
		if err != nil {
			t.Fatalf("POST /api/query failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("expected status Forbidden (403) for write query in read-only mode, got %d", resp.StatusCode)
		}

		var res struct {
			Ok    bool `json:"ok"`
			Error struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		json.NewDecoder(resp.Body).Decode(&res)

		if res.Ok {
			t.Errorf("expected Ok to be false")
		}
		if res.Error.Code != "READ_ONLY" {
			t.Errorf("expected error code READ_ONLY, got %s", res.Error.Code)
		}

		// Verify database did not mutate
		var count int
		database.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
		if count != 2 {
			t.Errorf("expected user count to remain 2, got %d", count)
		}
	}

	// 4. Test PRAGMAs in read-only mode
	{
		// PRAGMA table_info is READ -> should succeed
		body, _ := json.Marshal(map[string]any{"sql": "PRAGMA table_info(users)"})
		resp, err := http.Post(ts.URL+"/api/query", "application/json", bytes.NewBuffer(body))
		if err != nil {
			t.Fatalf("POST /api/query failed: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected PRAGMA table_info to be allowed, got status %d", resp.StatusCode)
		}

		// PRAGMA user_version = 5 is WRITE -> should be blocked
		bodySet, _ := json.Marshal(map[string]any{"sql": "PRAGMA user_version = 5"})
		respSet, err := http.Post(ts.URL+"/api/query", "application/json", bytes.NewBuffer(bodySet))
		if err != nil {
			t.Fatalf("POST /api/query failed: %v", err)
		}
		defer respSet.Body.Close()
		if respSet.StatusCode != http.StatusForbidden {
			t.Errorf("expected PRAGMA user_version = 5 to be blocked, got status %d", respSet.StatusCode)
		}
	}

	// 5. Test syntax error in read-only mode
	{
		body, _ := json.Marshal(map[string]any{"sql": "SELECT FROM user_version"})
		resp, err := http.Post(ts.URL+"/api/query", "application/json", bytes.NewBuffer(body))
		if err != nil {
			t.Fatalf("POST /api/query failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("expected status BadRequest (400) for syntax error, got %d", resp.StatusCode)
		}

		var res struct {
			Ok    bool `json:"ok"`
			Error struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		json.NewDecoder(resp.Body).Decode(&res)

		if res.Ok {
			t.Errorf("expected Ok to be false")
		}
		if res.Error.Code != "SQL_ERROR" {
			t.Errorf("expected error code SQL_ERROR, got %s", res.Error.Code)
		}
	}

	// Now start a write-enabled server
	srvWrite := NewServer(database, ":memory:", true, false, false, "127.0.0.1", 7072) // write = true
	tsWrite := httptest.NewServer(srvWrite.Handler())
	defer tsWrite.Close()

	// 6. Test successful INSERT in write mode
	{
		body, _ := json.Marshal(map[string]any{"sql": "INSERT INTO users (email) VALUES ('grace@example.com')"})
		resp, err := http.Post(tsWrite.URL+"/api/query", "application/json", bytes.NewBuffer(body))
		if err != nil {
			t.Fatalf("POST /api/query failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected status OK, got %d", resp.StatusCode)
		}

		var res struct {
			Ok   bool `json:"ok"`
			Data struct {
				Columns      []string `json:"columns"`
				Rows         [][]any  `json:"rows"`
				RowsAffected int      `json:"rowsAffected"`
			} `json:"data"`
		}
		json.NewDecoder(resp.Body).Decode(&res)

		if res.Data.RowsAffected != 1 {
			t.Errorf("expected rowsAffected to be 1, got %d", res.Data.RowsAffected)
		}
		if len(res.Data.Rows) != 0 || len(res.Data.Columns) != 0 {
			t.Errorf("expected empty columns/rows for write operation")
		}

		// Verify database did mutate
		var count int
		database.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
		if count != 3 {
			t.Errorf("expected user count to be 3, got %d", count)
		}
	}

	// 7. Test atomic rollback of multi-statement write batch
	{
		// Try a batch where the second statement fails
		body, _ := json.Marshal(map[string]any{
			"sql": `
				INSERT INTO users (email) VALUES ('bob@example.com');
				INSERT INTO users (email) VALUES ('ada@example.com'); -- this fails because of UNIQUE constraint
			`,
		})
		resp, err := http.Post(tsWrite.URL+"/api/query", "application/json", bytes.NewBuffer(body))
		if err != nil {
			t.Fatalf("POST /api/query failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("expected status BadRequest (400) for transaction rollback, got %d", resp.StatusCode)
		}

		// Verify no partial insert of 'bob@example.com' happened
		var count int
		database.QueryRow("SELECT COUNT(*) FROM users WHERE email = 'bob@example.com'").Scan(&count)
		if count != 0 {
			t.Errorf("expected 'bob@example.com' to not be inserted (rolled back), but it exists")
		}
	}
}
