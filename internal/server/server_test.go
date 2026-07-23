package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/0funct0ry/squad/internal/db"
)

func TestServerMeta(t *testing.T) {
	// Create an in-memory database for testing
	database, err := db.OpenDB(":memory:", false)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer database.Close()

	srv := NewServer(database, ":memory:", false)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/meta")
	if err != nil {
		t.Fatalf("failed to GET /api/meta: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status OK, got %d", resp.StatusCode)
	}

	var result struct {
		Ok   bool `json:"ok"`
		Data struct {
			Name          string `json:"name"`
			Mode          string `json:"mode"`
			SqliteVersion string `json:"sqliteVersion"`
			SizeBytes     int64  `json:"sizeBytes"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !result.Ok {
		t.Error("expected ok to be true")
	}
	if result.Data.Name != ":memory:" {
		t.Errorf("expected name to be :memory:, got %q", result.Data.Name)
	}
	if result.Data.Mode != "ro" {
		t.Errorf("expected mode to be ro, got %q", result.Data.Mode)
	}
	if result.Data.SqliteVersion == "" {
		t.Error("expected non-empty sqliteVersion")
	}
}
