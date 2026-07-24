package export

import (
	"bytes"
	"database/sql"
	"encoding/base64"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"io"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func TestExportCSV(t *testing.T) {
	columns := []string{"id", "name", "data", "null_col"}
	rows := [][]any{
		{int64(1), "Alice, Builder", []byte("hello"), nil},
		{int64(2), "Bob\n\"The King\"", []byte("world"), "not null"},
	}

	idx := 0
	rowSource := func() ([]any, error) {
		if idx >= len(rows) {
			return nil, io.EOF
		}
		r := rows[idx]
		idx++
		return r, nil
	}

	var buf bytes.Buffer
	err := ExportCSV(columns, rowSource, &buf, true)
	if err != nil {
		t.Fatalf("ExportCSV failed: %v", err)
	}

	// Verify using csv.Reader
	r := csv.NewReader(&buf)
	records, err := r.ReadAll()
	if err != nil {
		t.Fatalf("Failed to read CSV: %v", err)
	}

	if len(records) != 3 {
		t.Fatalf("Expected 3 CSV records, got %d", len(records))
	}

	// Header check
	if records[0][0] != "id" || records[0][1] != "name" || records[0][2] != "data" || records[0][3] != "null_col" {
		t.Errorf("Unexpected header record: %v", records[0])
	}

	// First row
	if records[1][0] != "1" {
		t.Errorf("Expected id '1', got '%s'", records[1][0])
	}
	if records[1][1] != "Alice, Builder" {
		t.Errorf("Expected name 'Alice, Builder', got '%s'", records[1][1])
	}
	if records[1][2] != base64.StdEncoding.EncodeToString([]byte("hello")) {
		t.Errorf("Expected base64 bytes, got '%s'", records[1][2])
	}
	if records[1][3] != "" {
		t.Errorf("Expected null_col to be empty, got '%s'", records[1][3])
	}

	// Second row
	if records[2][0] != "2" {
		t.Errorf("Expected id '2', got '%s'", records[2][0])
	}
	if records[2][1] != "Bob\n\"The King\"" {
		t.Errorf("Expected name 'Bob\n\"The King\"', got '%s'", records[2][1])
	}
	if records[2][2] != base64.StdEncoding.EncodeToString([]byte("world")) {
		t.Errorf("Expected base64 bytes, got '%s'", records[2][2])
	}
	if records[2][3] != "not null" {
		t.Errorf("Expected null_col to be 'not null', got '%s'", records[2][3])
	}
}

func TestExportJSON(t *testing.T) {
	columns := []string{"id", "name", "data", "null_col"}
	rows := [][]any{
		{int64(1), "Alice", []byte("hello"), nil},
		{int64(2), "Bob", []byte("world"), "not null"},
	}

	idx := 0
	rowSource := func() ([]any, error) {
		if idx >= len(rows) {
			return nil, io.EOF
		}
		r := rows[idx]
		idx++
		return r, nil
	}

	var buf bytes.Buffer
	err := ExportJSON(columns, rowSource, &buf)
	if err != nil {
		t.Fatalf("ExportJSON failed: %v", err)
	}

	var data []map[string]any
	if err := json.Unmarshal(buf.Bytes(), &data); err != nil {
		t.Fatalf("Failed to parse JSON: %v. Output was:\n%s", err, buf.String())
	}

	if len(data) != 2 {
		t.Fatalf("Expected 2 items, got %d", len(data))
	}

	if data[0]["id"] != float64(1) { // json unmarshals numbers to float64
		t.Errorf("id mismatch: %v", data[0]["id"])
	}
	if data[0]["name"] != "Alice" {
		t.Errorf("name mismatch: %v", data[0]["name"])
	}
	if data[0]["data"] != base64.StdEncoding.EncodeToString([]byte("hello")) {
		t.Errorf("data mismatch: %v", data[0]["data"])
	}
	if data[0]["null_col"] != nil {
		t.Errorf("null_col mismatch: %v", data[0]["null_col"])
	}
}

func TestExportSQL(t *testing.T) {
	columns := []string{"id", "name", "data", "null_col"}
	rows := [][]any{
		{int64(1), "Alice's Shop", []byte("hello"), nil},
		{int64(2), "Bob", []byte("world"), "not null"},
	}

	idx := 0
	rowSource := func() ([]any, error) {
		if idx >= len(rows) {
			return nil, io.EOF
		}
		r := rows[idx]
		idx++
		return r, nil
	}

	var buf bytes.Buffer
	ddl := "CREATE TABLE test_table (id INTEGER PRIMARY KEY, name TEXT, data BLOB, null_col TEXT);"
	err := ExportSQL("test_table", columns, rowSource, &buf, true, ddl)
	if err != nil {
		t.Fatalf("ExportSQL failed: %v", err)
	}

	sqlOutput := buf.String()
	if !strings.HasPrefix(sqlOutput, ddl) {
		t.Errorf("Expected output to start with DDL, got: %s", sqlOutput)
	}

	// Verify by running it in modernc sqlite
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open temp db: %v", err)
	}
	defer db.Close()

	// Execute the full DDL + INSERTS
	_, err = db.Exec(sqlOutput)
	if err != nil {
		t.Fatalf("Failed to execute exported SQL: %v\nSQL was:\n%s", err, sqlOutput)
	}

	// Query it back and assert
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM test_table").Scan(&count)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if count != 2 {
		t.Errorf("Expected 2 rows in DB, got %d", count)
	}

	var id int
	var name string
	var dataBytes []byte
	var nullCol sql.NullString
	err = db.QueryRow("SELECT id, name, data, null_col FROM test_table WHERE id = 1").Scan(&id, &name, &dataBytes, &nullCol)
	if err != nil {
		t.Fatalf("Querying row 1 failed: %v", err)
	}
	if name != "Alice's Shop" {
		t.Errorf("Expected name Alice's Shop, got %q", name)
	}
	if string(dataBytes) != "hello" {
		t.Errorf("Expected data 'hello', got %q (hex: %s)", string(dataBytes), hex.EncodeToString(dataBytes))
	}
	if nullCol.Valid {
		t.Errorf("Expected nullCol to be null, got %v", nullCol)
	}
}
