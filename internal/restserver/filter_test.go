package restserver

import (
	"net/url"
	"strings"
	"testing"

	"github.com/0funct0ry/squad/internal/db"
)

func testSchema() *db.TableSchema {
	return &db.TableSchema{
		Name: "users",
		Type: "table",
		Columns: []db.ColumnInfo{
			{Name: "id"},
			{Name: "email"},
			{Name: "active"},
		},
	}
}

func TestParsePagination(t *testing.T) {
	limit, offset := parsePagination(url.Values{})
	if limit != defaultLimit || offset != 0 {
		t.Errorf("expected default limit=%d offset=0, got limit=%d offset=%d", defaultLimit, limit, offset)
	}

	limit, offset = parsePagination(url.Values{"limit": {"5"}, "offset": {"20"}})
	if limit != 5 || offset != 20 {
		t.Errorf("expected limit=5 offset=20, got limit=%d offset=%d", limit, offset)
	}

	// No upper clamp, matching GET /api/tables/:name/rows.
	limit, _ = parsePagination(url.Values{"limit": {"100000"}})
	if limit != 100000 {
		t.Errorf("expected no upper clamp, got limit=%d", limit)
	}

	// Invalid/non-positive values fall back to the default.
	limit, offset = parsePagination(url.Values{"limit": {"-5"}, "offset": {"-1"}})
	if limit != defaultLimit || offset != 0 {
		t.Errorf("expected defaults for invalid values, got limit=%d offset=%d", limit, offset)
	}
}

func TestBuildListQuery_SingleFilter(t *testing.T) {
	sqlStr, args, err := buildListQuery(testSchema(), url.Values{"email": {"ada@example.com"}}, 100, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(sqlStr, `"email" = ?`) {
		t.Errorf("expected exact-match filter in query, got: %s", sqlStr)
	}
	if len(args) != 3 || args[0] != "ada@example.com" || args[1] != 100 || args[2] != 0 {
		t.Errorf("unexpected args: %+v", args)
	}
}

func TestBuildListQuery_MultipleFiltersAnded(t *testing.T) {
	sqlStr, _, err := buildListQuery(testSchema(), url.Values{"email": {"ada@example.com"}, "active": {"1"}}, 100, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(sqlStr, " AND ") {
		t.Errorf("expected filters to be ANDed, got: %s", sqlStr)
	}
}

func TestBuildListQuery_UnknownColumnRejected(t *testing.T) {
	_, _, err := buildListQuery(testSchema(), url.Values{"nope": {"x"}}, 100, 0)
	if err == nil {
		t.Fatal("expected an error for an unknown column filter")
	}
}

func TestBuildListQuery_ReservedParamsExcludedFromFilters(t *testing.T) {
	sqlStr, args, err := buildListQuery(testSchema(), url.Values{"limit": {"5"}, "offset": {"1"}}, 5, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(sqlStr, "WHERE") {
		t.Errorf("expected no WHERE clause when only reserved params are present, got: %s", sqlStr)
	}
	if len(args) != 2 {
		t.Errorf("expected only limit/offset args, got: %+v", args)
	}
}

func TestBuildGetQuery(t *testing.T) {
	sqlStr := buildGetQuery(testSchema(), "id")
	if !strings.Contains(sqlStr, `"id" = ?`) {
		t.Errorf("expected key column in WHERE clause, got: %s", sqlStr)
	}
}
