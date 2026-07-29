package server

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/0funct0ry/squad/internal/db"
)

// buildMultipartRequest builds a multipart/form-data request with an
// optional "file" field (fileContent, when non-empty) plus arbitrary text
// fields.
func buildMultipartRequest(t *testing.T, method, url, fileContent string, fields map[string]string) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	if fileContent != "" {
		fw, err := w.CreateFormFile("file", "upload.dat")
		if err != nil {
			t.Fatalf("failed to create form file: %v", err)
		}
		if _, err := fw.Write([]byte(fileContent)); err != nil {
			t.Fatalf("failed to write file content: %v", err)
		}
	}
	for k, v := range fields {
		if err := w.WriteField(k, v); err != nil {
			t.Fatalf("failed to write field %s: %v", k, err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("failed to close multipart writer: %v", err)
	}

	req := httptest.NewRequest(method, url, &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	return req
}

func TestImportPreviewHandler(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "import_preview_test.db")
	database, err := db.OpenDB(dbPath, false)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer database.Close()

	srv := NewServer(database, dbPath, false, false, false, "127.0.0.1", 7072, "info")

	csvContent := "id,name,price\n1,widget,9.99\n2,gadget,19.99\n"
	req := buildMultipartRequest(t, "POST", "/api/import/preview", csvContent, map[string]string{"format": "csv"})
	resp := httptest.NewRecorder()
	srv.Handler().ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}
	var res apiResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !res.Ok {
		t.Fatalf("expected ok, got %+v", res.Error)
	}
	cols, _ := res.Data["columns"].([]any)
	if len(cols) != 3 {
		t.Errorf("expected 3 columns, got %v", res.Data["columns"])
	}
	if totalRows, _ := res.Data["totalRows"].(float64); totalRows != 2 {
		t.Errorf("expected totalRows=2, got %v", res.Data["totalRows"])
	}
}

func TestImportIntoTableHandler(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "import_into_table_test.db")
	database, err := db.OpenDB(dbPath, false)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer database.Close()

	_, err = database.Exec(`CREATE TABLE users (id INTEGER PRIMARY KEY, email TEXT NOT NULL, bio TEXT)`)
	if err != nil {
		t.Fatalf("seed failed: %v", err)
	}

	srv := NewServer(database, dbPath, true, false, false, "127.0.0.1", 7072, "info")

	t.Run("field mapping with mismatched headers", func(t *testing.T) {
		csvContent := "user_id,user_email,notes\n1,a@example.com,hi\n2,b@example.com,yo\n"
		mapping, _ := json.Marshal(map[string]string{
			"user_id":    "id",
			"user_email": "email",
			"notes":      "bio",
		})
		req := buildMultipartRequest(t, "POST", "/api/tables/users/import", csvContent, map[string]string{
			"format":  "csv",
			"mapping": string(mapping),
		})
		resp := httptest.NewRecorder()
		srv.Handler().ServeHTTP(resp, req)

		if resp.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
		}
		var count int
		database.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
		if count != 2 {
			t.Errorf("expected 2 rows inserted, got %d", count)
		}
		var bio string
		database.QueryRow("SELECT bio FROM users WHERE id = 1").Scan(&bio)
		if bio != "hi" {
			t.Errorf("expected mapped bio 'hi', got %q", bio)
		}
	})

	t.Run("missing required column blocked", func(t *testing.T) {
		csvContent := "user_id,notes\n3,x\n"
		mapping, _ := json.Marshal(map[string]string{
			"user_id": "id",
			"notes":   "bio",
		})
		req := buildMultipartRequest(t, "POST", "/api/tables/users/import", csvContent, map[string]string{
			"format":  "csv",
			"mapping": string(mapping),
		})
		resp := httptest.NewRecorder()
		srv.Handler().ServeHTTP(resp, req)

		if resp.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d: %s", resp.Code, resp.Body.String())
		}
		var res apiResponse
		json.Unmarshal(resp.Body.Bytes(), &res)
		if res.Error == nil || res.Error.Code != "VALIDATION" {
			t.Errorf("expected VALIDATION error, got %+v", res.Error)
		}
		var count int
		database.QueryRow("SELECT COUNT(*) FROM users WHERE id = 3").Scan(&count)
		if count != 0 {
			t.Errorf("expected no row inserted on validation failure, got %d", count)
		}
	})

	t.Run("read-only mode rejects import", func(t *testing.T) {
		roDBPath := filepath.Join(t.TempDir(), "import_ro_test.db")
		roDatabase, err := db.OpenDB(roDBPath, false)
		if err != nil {
			t.Fatalf("failed to open database: %v", err)
		}
		defer roDatabase.Close()
		roDatabase.Exec(`CREATE TABLE users (id INTEGER PRIMARY KEY, email TEXT NOT NULL)`)
		roSrv := NewServer(roDatabase, roDBPath, false, false, false, "127.0.0.1", 7073, "info")

		mapping, _ := json.Marshal(map[string]string{"id": "id", "email": "email"})
		req := buildMultipartRequest(t, "POST", "/api/tables/users/import", "id,email\n1,a@example.com\n", map[string]string{
			"format":  "csv",
			"mapping": string(mapping),
		})
		resp := httptest.NewRecorder()
		roSrv.Handler().ServeHTTP(resp, req)

		if resp.Code != http.StatusForbidden {
			t.Errorf("expected 403, got %d: %s", resp.Code, resp.Body.String())
		}
	})
}

func TestImportCreateTableHandler(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "import_create_table_test.db")
	database, err := db.OpenDB(dbPath, false)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer database.Close()

	srv := NewServer(database, dbPath, true, false, false, "127.0.0.1", 7072, "info")

	t.Run("creates table and inserts rows", func(t *testing.T) {
		csvContent := "id,name,price\n1,widget,9.99\n2,gadget,19.99\n"
		req := buildMultipartRequest(t, "POST", "/api/tables/import", csvContent, map[string]string{
			"format": "csv",
			"name":   "products",
		})
		resp := httptest.NewRecorder()
		srv.Handler().ServeHTTP(resp, req)

		if resp.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
		}

		var count int
		database.QueryRow("SELECT COUNT(*) FROM products").Scan(&count)
		if count != 2 {
			t.Errorf("expected 2 rows, got %d", count)
		}
		var name string
		database.QueryRow("SELECT name FROM products WHERE id = 1").Scan(&name)
		if name != "widget" {
			t.Errorf("expected name 'widget', got %q", name)
		}
	})

	t.Run("creates table with a designated primary key", func(t *testing.T) {
		csvContent := "sku,name\nABC-1,widget\nABC-2,gadget\n"
		req := buildMultipartRequest(t, "POST", "/api/tables/import", csvContent, map[string]string{
			"format":     "csv",
			"name":       "skus",
			"primaryKey": `["sku"]`,
		})
		resp := httptest.NewRecorder()
		srv.Handler().ServeHTTP(resp, req)

		if resp.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
		}

		schema, err := db.GetTableSchema(database, "skus")
		if err != nil {
			t.Fatalf("failed to fetch schema: %v", err)
		}
		if len(schema.PrimaryKey) != 1 || schema.PrimaryKey[0] != "sku" {
			t.Errorf("expected primary key [sku], got %v", schema.PrimaryKey)
		}

		// A duplicate sku must now violate the primary key.
		_, err = database.Exec(`INSERT INTO skus (sku, name) VALUES ('ABC-1', 'dup')`)
		if err == nil {
			t.Error("expected inserting a duplicate sku to fail")
		}
	})

	t.Run("creates table with a composite primary key", func(t *testing.T) {
		csvContent := "region,sku,name\nUK,ABC-1,widget\nUS,ABC-1,widget-us\n"
		req := buildMultipartRequest(t, "POST", "/api/tables/import", csvContent, map[string]string{
			"format":     "csv",
			"name":       "regional_skus",
			"primaryKey": `["region","sku"]`,
		})
		resp := httptest.NewRecorder()
		srv.Handler().ServeHTTP(resp, req)

		if resp.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
		}

		schema, err := db.GetTableSchema(database, "regional_skus")
		if err != nil {
			t.Fatalf("failed to fetch schema: %v", err)
		}
		if len(schema.PrimaryKey) != 2 {
			t.Errorf("expected a 2-column composite primary key, got %v", schema.PrimaryKey)
		}
	})

	t.Run("rejects primaryKey column not present among the table's columns", func(t *testing.T) {
		req := buildMultipartRequest(t, "POST", "/api/tables/import", "id\n1\n", map[string]string{
			"format":     "csv",
			"name":       "bad_pk_table",
			"primaryKey": `["bogus"]`,
		})
		resp := httptest.NewRecorder()
		srv.Handler().ServeHTTP(resp, req)

		if resp.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d: %s", resp.Code, resp.Body.String())
		}
		var exists bool
		database.QueryRow("SELECT EXISTS(SELECT 1 FROM sqlite_master WHERE name = 'bad_pk_table')").Scan(&exists)
		if exists {
			t.Error("expected bad_pk_table to not exist after rejected primaryKey")
		}
	})

	t.Run("rejects existing table name", func(t *testing.T) {
		req := buildMultipartRequest(t, "POST", "/api/tables/import", "id\n1\n", map[string]string{
			"format": "csv",
			"name":   "products",
		})
		resp := httptest.NewRecorder()
		srv.Handler().ServeHTTP(resp, req)

		if resp.Code != http.StatusConflict {
			t.Errorf("expected 409, got %d: %s", resp.Code, resp.Body.String())
		}
	})

	t.Run("forced DDL-level failure leaves no partial table or rows", func(t *testing.T) {
		// A duplicate-name column in the override forces the CREATE TABLE
		// DDL itself to fail, verifying the whole transaction (including
		// the table) is rolled back rather than left half-created.
		csvContent := "a,b\n1,2\n"
		overrides, _ := json.Marshal([]ImportColumnOverride{
			{Name: "dup", Type: "TEXT"},
			{Name: "dup", Type: "TEXT"},
		})
		req := buildMultipartRequest(t, "POST", "/api/tables/import", csvContent, map[string]string{
			"format":  "csv",
			"name":    "broken_table",
			"columns": string(overrides),
		})
		resp := httptest.NewRecorder()
		srv.Handler().ServeHTTP(resp, req)

		if resp.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d: %s", resp.Code, resp.Body.String())
		}

		var exists bool
		database.QueryRow("SELECT EXISTS(SELECT 1 FROM sqlite_master WHERE name = 'broken_table')").Scan(&exists)
		if exists {
			t.Errorf("expected broken_table to not exist after forced failure")
		}
	})

	t.Run("forced mid-insert failure rolls back the created table too", func(t *testing.T) {
		// The type override smuggles a CHECK constraint into the DDL so the
		// CREATE TABLE succeeds but the second row's insert violates it -
		// verifying the table itself doesn't survive the rollback either.
		csvContent := "name\nshort\nthis-name-is-far-too-long-to-pass\n"
		overrides, _ := json.Marshal([]ImportColumnOverride{
			{Name: "name", Type: "TEXT CHECK(length(name) < 10)"},
		})
		req := buildMultipartRequest(t, "POST", "/api/tables/import", csvContent, map[string]string{
			"format":  "csv",
			"name":    "checked_table",
			"columns": string(overrides),
		})
		resp := httptest.NewRecorder()
		srv.Handler().ServeHTTP(resp, req)

		if resp.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d: %s", resp.Code, resp.Body.String())
		}

		var exists bool
		database.QueryRow("SELECT EXISTS(SELECT 1 FROM sqlite_master WHERE name = 'checked_table')").Scan(&exists)
		if exists {
			t.Errorf("expected checked_table to not exist after mid-insert rollback")
		}
	})

	t.Run("read-only mode rejects create-table import", func(t *testing.T) {
		roDBPath := filepath.Join(t.TempDir(), "import_create_ro_test.db")
		roDatabase, err := db.OpenDB(roDBPath, false)
		if err != nil {
			t.Fatalf("failed to open database: %v", err)
		}
		defer roDatabase.Close()
		roSrv := NewServer(roDatabase, roDBPath, false, false, false, "127.0.0.1", 7074, "info")

		req := buildMultipartRequest(t, "POST", "/api/tables/import", "id\n1\n", map[string]string{
			"format": "csv",
			"name":   "whatever",
		})
		resp := httptest.NewRecorder()
		roSrv.Handler().ServeHTTP(resp, req)

		if resp.Code != http.StatusForbidden {
			t.Errorf("expected 403, got %d: %s", resp.Code, resp.Body.String())
		}
	})
}
