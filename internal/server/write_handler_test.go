package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/0funct0ry/squad/internal/db"
)

type apiResponse struct {
	Ok    bool           `json:"ok"`
	Data  map[string]any `json:"data"`
	Error *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func parseResponse(t *testing.T, resp *http.Response) apiResponse {
	t.Helper()
	var res apiResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	return res
}

func TestWriteGateReadOnly(t *testing.T) {
	// Setup a database in a temp directory
	dbPath := filepath.Join(t.TempDir(), "readonly_test.db")
	database, err := db.OpenDB(dbPath, false)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer database.Close()

	// Seed table
	_, err = database.Exec("CREATE TABLE test_table (id INTEGER PRIMARY KEY, name TEXT)")
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	// Create server in read-only mode (write=false)
	srv := NewServer(database, dbPath, false)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	client := ts.Client()

	tests := []struct {
		method string
		url    string
		body   any
	}{
		{"POST", ts.URL + "/api/ddl", map[string]any{"sql": "CREATE TABLE another (id INT)"}},
		{"POST", ts.URL + "/api/tables", map[string]any{"name": "users", "columns": []any{map[string]any{"name": "id", "type": "INT"}}}},
		{"PATCH", ts.URL + "/api/tables/test_table", map[string]any{"op": "rename_table", "newName": "new_name"}},
		{"DELETE", ts.URL + "/api/tables/test_table", nil},
		{"POST", ts.URL + "/api/tables/test_table/rows", map[string]any{"values": map[string]any{"id": 1, "name": "foo"}}},
		{"PATCH", ts.URL + "/api/tables/test_table/rows", map[string]any{"key": map[string]any{"id": 1}, "values": map[string]any{"name": "bar"}}},
		{"DELETE", ts.URL + "/api/tables/test_table/rows", map[string]any{"key": map[string]any{"id": 1}}},
	}

	for i, tc := range tests {
		t.Run(fmt.Sprintf("tc_%d", i), func(t *testing.T) {
			var req *http.Request
			var err error
			if tc.body != nil {
				b, _ := json.Marshal(tc.body)
				req, err = http.NewRequest(tc.method, tc.url, bytes.NewBuffer(b))
			} else {
				req, err = http.NewRequest(tc.method, tc.url, nil)
			}
			if err != nil {
				t.Fatalf("failed to create request: %v", err)
			}
			req.Header.Set("Content-Type", "application/json")

			resp, err := client.Do(req)
			if err != nil {
				t.Fatalf("failed request: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusForbidden {
				t.Errorf("expected 403 Forbidden, got %d", resp.StatusCode)
			}

			res := parseResponse(t, resp)
			if res.Ok {
				t.Errorf("expected ok: false")
			}
			if res.Error == nil || res.Error.Code != "READ_ONLY" {
				t.Errorf("expected error code READ_ONLY, got %+v", res.Error)
			}
		})
	}

	// Verify database did not mutate
	var count int
	err = database.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='test_table'").Scan(&count)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if count != 1 {
		t.Errorf("expected table count to be 1, got %d", count)
	}
}

func TestPostDDLHandler(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "ddl_test.db")
	database, err := db.OpenDB(dbPath, false)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer database.Close()

	srv := NewServer(database, dbPath, true) // write=true
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()
	client := ts.Client()

	// 1. Success case: CREATE + INSERT
	sql := `
		CREATE TABLE users (id INTEGER PRIMARY KEY, email TEXT UNIQUE);
		INSERT INTO users (email) VALUES ('alice@example.com');
	`
	body, _ := json.Marshal(map[string]any{"sql": sql})
	resp, err := client.Post(ts.URL+"/api/ddl", "application/json", bytes.NewBuffer(body))
	if err != nil {
		t.Fatalf("POST /api/ddl failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}
	res := parseResponse(t, resp)
	if !res.Ok {
		t.Errorf("expected ok: true")
	}

	// 2. Transaction rollback case: CREATE + INSERT + invalid INSERT
	sqlRollback := `
		CREATE TABLE products (id INTEGER PRIMARY KEY, name TEXT UNIQUE);
		INSERT INTO products (name) VALUES ('book');
		INSERT INTO products (name) VALUES ('book'); -- duplicate key violation
	`
	bodyRollback, _ := json.Marshal(map[string]any{"sql": sqlRollback})
	respRollback, err := client.Post(ts.URL+"/api/ddl", "application/json", bytes.NewBuffer(bodyRollback))
	if err != nil {
		t.Fatalf("POST /api/ddl failed: %v", err)
	}
	defer respRollback.Body.Close()

	if respRollback.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", respRollback.StatusCode)
	}
	resRollback := parseResponse(t, respRollback)
	if resRollback.Ok {
		t.Errorf("expected ok: false")
	}
	if resRollback.Error == nil || resRollback.Error.Code != "SQL_ERROR" {
		t.Errorf("expected error code SQL_ERROR, got %+v", resRollback.Error)
	}

	// Verify roll back: 'products' table should NOT exist
	var exists bool
	database.QueryRow("SELECT EXISTS(SELECT 1 FROM sqlite_master WHERE type='table' AND name='products')").Scan(&exists)
	if exists {
		t.Errorf("expected products table to not exist due to rollback")
	}
}

func TestCreateTableHandler(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "create_table_test.db")
	database, err := db.OpenDB(dbPath, false)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer database.Close()

	srv := NewServer(database, dbPath, true)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()
	client := ts.Client()

	// 1. Create table with composite PK
	reqBody := map[string]any{
		"name": "composite_table",
		"columns": []any{
			map[string]any{"name": "col_a", "type": "TEXT", "notnull": true},
			map[string]any{"name": "col_b", "type": "INTEGER", "notnull": true},
			map[string]any{"name": "col_c", "type": "TEXT", "default": "'hello'"},
		},
		"primaryKey": []string{"col_a", "col_b"},
	}
	b, _ := json.Marshal(reqBody)
	resp, err := client.Post(ts.URL+"/api/tables", "application/json", bytes.NewBuffer(b))
	if err != nil {
		t.Fatalf("POST failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	res := parseResponse(t, resp)
	if !res.Ok {
		t.Errorf("expected ok: true")
	}

	// Verify schema composite PK
	schema, err := db.GetTableSchema(database, "composite_table")
	if err != nil {
		t.Fatalf("schema fetch failed: %v", err)
	}
	if len(schema.PrimaryKey) != 2 || schema.PrimaryKey[0] != "col_a" || schema.PrimaryKey[1] != "col_b" {
		t.Errorf("expected composite PK ['col_a', 'col_b'], got %v", schema.PrimaryKey)
	}

	// 2. Reject duplicate table name with ALREADY_EXISTS
	respDup, err := client.Post(ts.URL+"/api/tables", "application/json", bytes.NewBuffer(b))
	if err != nil {
		t.Fatalf("POST failed: %v", err)
	}
	defer respDup.Body.Close()

	if respDup.StatusCode != http.StatusConflict {
		t.Errorf("expected 409 Conflict, got %d", respDup.StatusCode)
	}
	resDup := parseResponse(t, respDup)
	if resDup.Error == nil || resDup.Error.Code != "ALREADY_EXISTS" {
		t.Errorf("expected error code ALREADY_EXISTS, got %+v", resDup.Error)
	}
}

func TestAlterTableHandler(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "alter_table_test.db")
	database, err := db.OpenDB(dbPath, false)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer database.Close()

	// Seed database with a table, index, and data
	_, err = database.Exec(`
		CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT NOT NULL, email TEXT);
		CREATE UNIQUE INDEX idx_users_email ON users(email);
		INSERT INTO users (name, email) VALUES ('Alice', 'alice@example.com');
		INSERT INTO users (name, email) VALUES ('Bob', 'bob@example.com');
	`)
	if err != nil {
		t.Fatalf("seed failed: %v", err)
	}

	srv := NewServer(database, dbPath, true)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()
	client := ts.Client()

	// 1. Rename table
	renameBody, _ := json.Marshal(map[string]any{"op": "rename_table", "newName": "customers"})
	req, _ := http.NewRequest("PATCH", ts.URL+"/api/tables/users", bytes.NewBuffer(renameBody))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := client.Do(req)
	resp.Body.Close()

	schema, err := db.GetTableSchema(database, "customers")
	if err != nil {
		t.Fatalf("expected customers table to exist, got error: %v", err)
	}

	// 2. Add column
	addBody, _ := json.Marshal(map[string]any{"op": "add_column", "column": map[string]any{"name": "age", "type": "INTEGER", "default": "30"}})
	req, _ = http.NewRequest("PATCH", ts.URL+"/api/tables/customers", bytes.NewBuffer(addBody))
	req.Header.Set("Content-Type", "application/json")
	resp, _ = client.Do(req)
	resp.Body.Close()

	schema, _ = db.GetTableSchema(database, "customers")
	foundAge := false
	for _, col := range schema.Columns {
		if col.Name == "age" {
			foundAge = true
			if col.Type != "INTEGER" {
				t.Errorf("expected column type INTEGER, got %s", col.Type)
			}
		}
	}
	if !foundAge {
		t.Errorf("failed to add column 'age'")
	}

	// 3. Rename column
	renameColBody, _ := json.Marshal(map[string]any{"op": "rename_column", "from": "email", "to": "email_address"})
	req, _ = http.NewRequest("PATCH", ts.URL+"/api/tables/customers", bytes.NewBuffer(renameColBody))
	req.Header.Set("Content-Type", "application/json")
	resp, _ = client.Do(req)
	resp.Body.Close()

	schema, _ = db.GetTableSchema(database, "customers")
	foundNewCol := false
	for _, col := range schema.Columns {
		if col.Name == "email_address" {
			foundNewCol = true
		}
	}
	if !foundNewCol {
		t.Errorf("failed to rename column 'email' to 'email_address'")
	}

	// 4. Drop plain column (uses native if supported)
	dropPlainBody, _ := json.Marshal(map[string]any{"op": "drop_column", "column": "age"})
	req, _ = http.NewRequest("PATCH", ts.URL+"/api/tables/customers", bytes.NewBuffer(dropPlainBody))
	req.Header.Set("Content-Type", "application/json")
	resp, _ = client.Do(req)
	resp.Body.Close()

	schema, _ = db.GetTableSchema(database, "customers")
	for _, col := range schema.Columns {
		if col.Name == "age" {
			t.Errorf("column 'age' was not dropped")
		}
	}

	// 5. Drop column that participates in index -> falls back to rebuild pattern
	// Wait, let's verify if 'email_address' participates in unique index (re-established after rename_column)
	// Let's drop 'email_address'
	dropIndexedBody, _ := json.Marshal(map[string]any{"op": "drop_column", "column": "email_address"})
	req, _ = http.NewRequest("PATCH", ts.URL+"/api/tables/customers", bytes.NewBuffer(dropIndexedBody))
	req.Header.Set("Content-Type", "application/json")
	resp, _ = client.Do(req)
	resObj := parseResponse(t, resp)
	resp.Body.Close()

	if !resObj.Ok {
		t.Errorf("drop indexed column failed: %+v", resObj.Error)
	}

	// Verify surviving data is intact
	var count int
	database.QueryRow("SELECT COUNT(*) FROM customers").Scan(&count)
	if count != 2 {
		t.Errorf("expected 2 rows, got %d", count)
	}

	// Verify schema: 'email_address' is gone, 'name' and 'id' remain
	schema, _ = db.GetTableSchema(database, "customers")
	if len(schema.Columns) != 2 {
		t.Errorf("expected 2 columns left, got %d", len(schema.Columns))
	}
}

func TestFKDropColumnReject(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "fk_reject_test.db")
	database, err := db.OpenDB(dbPath, false)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer database.Close()

	// Enable foreign keys
	_, _ = database.Exec("PRAGMA foreign_keys = ON")

	_, err = database.Exec(`
		CREATE TABLE parent (id INTEGER PRIMARY KEY, code TEXT UNIQUE);
		CREATE TABLE child (id INTEGER PRIMARY KEY, parent_code TEXT, FOREIGN KEY(parent_code) REFERENCES parent(code));
	`)
	if err != nil {
		t.Fatalf("seed failed: %v", err)
	}

	srv := NewServer(database, dbPath, true)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()
	client := ts.Client()

	// Attempt to drop column 'code' in 'parent' which 'child' depends on
	dropBody, _ := json.Marshal(map[string]any{"op": "drop_column", "column": "code"})
	req, _ := http.NewRequest("PATCH", ts.URL+"/api/tables/parent", bytes.NewBuffer(dropBody))
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("PATCH failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request, got %d", resp.StatusCode)
	}
	res := parseResponse(t, resp)
	if res.Error == nil || res.Error.Code != "BAD_REQUEST" {
		t.Errorf("expected BAD_REQUEST error code, got %+v", res.Error)
	}
}

func TestDeleteTableHandler(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "delete_table_test.db")
	database, err := db.OpenDB(dbPath, false)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer database.Close()

	_, err = database.Exec(`
		CREATE TABLE t1 (id INT);
		CREATE VIEW v1 AS SELECT * FROM t1;
	`)
	if err != nil {
		t.Fatalf("seed failed: %v", err)
	}

	srv := NewServer(database, dbPath, true)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()
	client := ts.Client()

	// Drop View
	reqV, _ := http.NewRequest("DELETE", ts.URL+"/api/tables/v1", nil)
	respV, _ := client.Do(reqV)
	respV.Body.Close()

	// Drop Table
	reqT, _ := http.NewRequest("DELETE", ts.URL+"/api/tables/t1", nil)
	respT, _ := client.Do(reqT)
	respT.Body.Close()

	// Verify they are gone
	var count int
	database.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type IN ('table', 'view') AND name IN ('t1', 'v1')").Scan(&count)
	if count != 0 {
		t.Errorf("expected table and view to be dropped, got count %d", count)
	}
}

func TestRowCRUDHandlers(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "row_crud_test.db")
	database, err := db.OpenDB(dbPath, false)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer database.Close()

	_, err = database.Exec(`
		CREATE TABLE users (id INTEGER PRIMARY KEY, email TEXT NOT NULL, age INTEGER);
		CREATE TABLE without_pk (name TEXT, value INTEGER);
		CREATE TABLE composite_pk (pk1 TEXT, pk2 TEXT, details TEXT, PRIMARY KEY (pk1, pk2));
	`)
	if err != nil {
		t.Fatalf("seed failed: %v", err)
	}

	srv := NewServer(database, dbPath, true)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()
	client := ts.Client()

	// 1. Row INSERT returns usable lastInsertRowid
	insertBody, _ := json.Marshal(map[string]any{"values": map[string]any{"email": "alice@example.com", "age": 30}})
	resp, _ := client.Post(ts.URL+"/api/tables/users/rows", "application/json", bytes.NewBuffer(insertBody))
	res := parseResponse(t, resp)
	resp.Body.Close()

	if !res.Ok {
		t.Fatalf("insert failed: %+v", res.Error)
	}
	lastID, ok := res.Data["lastInsertRowid"].(float64)
	if !ok || lastID != 1 {
		t.Errorf("expected lastInsertRowid 1, got %v", res.Data["lastInsertRowid"])
	}

	// 2. Row UPDATE by explicit composite key affects exactly targeted row
	// Insert composite rows
	_, _ = database.Exec(`
		INSERT INTO composite_pk (pk1, pk2, details) VALUES ('a', 'b', 'first');
		INSERT INTO composite_pk (pk1, pk2, details) VALUES ('a', 'c', 'second');
	`)

	updateBody, _ := json.Marshal(map[string]any{
		"key":    map[string]any{"pk1": "a", "pk2": "b"},
		"values": map[string]any{"details": "updated-first"},
	})
	req, _ := http.NewRequest("PATCH", ts.URL+"/api/tables/composite_pk/rows", bytes.NewBuffer(updateBody))
	req.Header.Set("Content-Type", "application/json")
	resp, _ = client.Do(req)
	resUpdate := parseResponse(t, resp)
	resp.Body.Close()

	if !resUpdate.Ok {
		t.Fatalf("update failed: %+v", resUpdate.Error)
	}

	// Verify exact row updated
	var details string
	database.QueryRow("SELECT details FROM composite_pk WHERE pk1='a' AND pk2='b'").Scan(&details)
	if details != "updated-first" {
		t.Errorf("expected 'updated-first', got %s", details)
	}
	database.QueryRow("SELECT details FROM composite_pk WHERE pk1='a' AND pk2='c'").Scan(&details)
	if details != "second" {
		t.Errorf("unrelated row was updated!")
	}

	// 3. Row UPDATE with partial key is rejected
	partialUpdate, _ := json.Marshal(map[string]any{
		"key":    map[string]any{"pk1": "a"},
		"values": map[string]any{"details": "fail"},
	})
	req, _ = http.NewRequest("PATCH", ts.URL+"/api/tables/composite_pk/rows", bytes.NewBuffer(partialUpdate))
	req.Header.Set("Content-Type", "application/json")
	resp, _ = client.Do(req)
	resPartial := parseResponse(t, resp)
	resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest || resPartial.Error == nil || resPartial.Error.Code != "BAD_REQUEST" {
		t.Errorf("expected 400 BAD_REQUEST on partial update key, got status %d, error %+v", resp.StatusCode, resPartial.Error)
	}

	// 4. Row UPDATE matching zero rows returns 404
	missingUpdate, _ := json.Marshal(map[string]any{
		"key":    map[string]any{"pk1": "x", "pk2": "y"},
		"values": map[string]any{"details": "fail"},
	})
	req, _ = http.NewRequest("PATCH", ts.URL+"/api/tables/composite_pk/rows", bytes.NewBuffer(missingUpdate))
	req.Header.Set("Content-Type", "application/json")
	resp, _ = client.Do(req)
	resMissing := parseResponse(t, resp)
	resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound || resMissing.Ok || resMissing.Error == nil || resMissing.Error.Code != "NOT_FOUND" {
		t.Errorf("expected 404 NOT_FOUND on missing row update, got status %d, error %+v", resp.StatusCode, resMissing.Error)
	}

	// 5. Update by rowid on a table with no PK
	_, _ = database.Exec("INSERT INTO without_pk (name, value) VALUES ('one', 1)")
	rowidUpdate, _ := json.Marshal(map[string]any{
		"key":    map[string]any{"rowid": 1},
		"values": map[string]any{"value": 10},
	})
	req, _ = http.NewRequest("PATCH", ts.URL+"/api/tables/without_pk/rows", bytes.NewBuffer(rowidUpdate))
	req.Header.Set("Content-Type", "application/json")
	resp, _ = client.Do(req)
	resRowid := parseResponse(t, resp)
	resp.Body.Close()

	if !resRowid.Ok {
		t.Fatalf("rowid update failed: %+v", resRowid.Error)
	}
	var val int
	database.QueryRow("SELECT value FROM without_pk WHERE rowid=1").Scan(&val)
	if val != 10 {
		t.Errorf("expected value 10, got %d", val)
	}
}
