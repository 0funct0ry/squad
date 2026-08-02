package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/0funct0ry/squad/internal/db"
)

func TestHandleTransformTemplate(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "transform_test.db")
	database, err := db.OpenDB(dbPath, false)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer database.Close()

	// Read-only server: this endpoint does no DB writes, so it must work
	// without --write.
	srv := NewServer(database, dbPath, false, false, false, "127.0.0.1", 7073, "info")
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()
	client := ts.Client()

	body, _ := json.Marshal(TransformTemplateRequest{
		Template: "{{upper .Value}}",
		Values:   []interface{}{"alice", "bob"},
	})
	resp, err := client.Post(ts.URL+"/api/transform/template", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	res := parseResponse(t, resp)
	if !res.Ok {
		t.Fatalf("expected ok response, got %+v", res)
	}
	results, ok := res.Data["results"].([]interface{})
	if !ok || len(results) != 2 {
		t.Fatalf("expected 2 results, got %+v", res.Data["results"])
	}
	if results[0] != "ALICE" || results[1] != "BOB" {
		t.Fatalf("expected [ALICE BOB], got %+v", results)
	}
}

func TestHandleTransformTemplateSyntaxError(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "transform_err_test.db")
	database, err := db.OpenDB(dbPath, false)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer database.Close()

	srv := NewServer(database, dbPath, false, false, false, "127.0.0.1", 7074, "info")
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()
	client := ts.Client()

	body, _ := json.Marshal(TransformTemplateRequest{
		Template: "{{.Value",
		Values:   []interface{}{"x"},
	})
	resp, err := client.Post(ts.URL+"/api/transform/template", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
	res := parseResponse(t, resp)
	if res.Ok {
		t.Fatalf("expected ok:false, got %+v", res)
	}
	if res.Error == nil || res.Error.Code != "VALIDATION" {
		t.Fatalf("expected VALIDATION error code, got %+v", res.Error)
	}
}
