package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/0funct0ry/squad/internal/db"
)

func TestTablesHandlers(t *testing.T) {
	// Create an in-memory database for testing
	database, err := db.OpenDB(":memory:", false)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer database.Close()

	// Create test table
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

	srv := NewServer(database, ":memory:", false, false)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// 1. GET /api/tables
	resp, err := http.Get(ts.URL + "/api/tables")
	if err != nil {
		t.Fatalf("failed to GET /api/tables: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status OK, got %d", resp.StatusCode)
	}

	var tablesResult struct {
		Ok   bool           `json:"ok"`
		Data []db.TableInfo `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tablesResult); err != nil {
		t.Fatalf("failed to decode tables response: %v", err)
	}
	if !tablesResult.Ok || len(tablesResult.Data) != 1 || tablesResult.Data[0].Name != "users" || tablesResult.Data[0].RowCount != 2 {
		t.Errorf("unexpected tables result: %+v", tablesResult)
	}

	// 2. GET /api/tables/users/schema
	resp, err = http.Get(ts.URL + "/api/tables/users/schema")
	if err != nil {
		t.Fatalf("failed to GET schema: %v", err)
	}
	defer resp.Body.Close()

	var schemaResult struct {
		Ok   bool           `json:"ok"`
		Data db.TableSchema `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&schemaResult); err != nil {
		t.Fatalf("failed to decode schema response: %v", err)
	}
	if !schemaResult.Ok || len(schemaResult.Data.Columns) != 2 {
		t.Errorf("unexpected schema result: %+v", schemaResult)
	}

	// 3. GET /api/tables/users/rows
	resp, err = http.Get(ts.URL + "/api/tables/users/rows?limit=1&offset=1")
	if err != nil {
		t.Fatalf("failed to GET rows: %v", err)
	}
	defer resp.Body.Close()

	var rowsResult struct {
		Ok   bool `json:"ok"`
		Data struct {
			Columns []string        `json:"columns"`
			Rows    [][]interface{} `json:"rows"`
			Total   int64           `json:"total"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rowsResult); err != nil {
		t.Fatalf("failed to decode rows response: %v", err)
	}
	if !rowsResult.Ok || rowsResult.Data.Total != 2 || len(rowsResult.Data.Rows) != 1 || rowsResult.Data.Rows[0][1] != "linus@example.com" {
		t.Errorf("unexpected rows result: %+v", rowsResult)
	}

	// 4. GET /api/tables/users/rows with filter
	resp, err = http.Get(ts.URL + "/api/tables/users/rows?filter[email]=ada")
	if err != nil {
		t.Fatalf("failed to GET filtered rows: %v", err)
	}
	defer resp.Body.Close()

	if err := json.NewDecoder(resp.Body).Decode(&rowsResult); err != nil {
		t.Fatalf("failed to decode filtered rows response: %v", err)
	}
	if !rowsResult.Ok || rowsResult.Data.Total != 1 || rowsResult.Data.Rows[0][1] != "ada@example.com" {
		t.Errorf("unexpected filtered rows result: %+v", rowsResult)
	}

	// 5. GET /api/tables/does_not_exist/schema -> 404 NOT_FOUND
	resp, err = http.Get(ts.URL + "/api/tables/does_not_exist/schema")
	if err != nil {
		t.Fatalf("failed to GET missing schema: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", resp.StatusCode)
	}

	var notFoundResult struct {
		Ok    bool `json:"ok"`
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&notFoundResult); err != nil {
		t.Fatalf("failed to decode not-found response: %v", err)
	}
	if notFoundResult.Ok || notFoundResult.Error.Code != "NOT_FOUND" {
		t.Errorf("unexpected not-found result: %+v", notFoundResult)
	}
}
