package restserver

import (
	"fmt"
	"net/url"
	"strconv"

	"github.com/0funct0ry/squad/internal/db"
)

// defaultLimit/no upper clamp matches GET /api/tables/:name/rows' behavior
// (internal/server/server.go handleTableRows).
const defaultLimit = 100

// reservedParams are pagination keys, never treated as column filters.
var reservedParams = map[string]bool{"limit": true, "offset": true}

// parsePagination extracts limit/offset from query params, matching
// handleTableRows' parsing (default limit 100, no upper clamp; default
// offset 0).
func parsePagination(query url.Values) (limit, offset int) {
	limit = defaultLimit
	if lStr := query.Get("limit"); lStr != "" {
		if l, err := strconv.Atoi(lStr); err == nil && l > 0 {
			limit = l
		}
	}
	offset = 0
	if oStr := query.Get("offset"); oStr != "" {
		if o, err := strconv.Atoi(oStr); err == nil && o >= 0 {
			offset = o
		}
	}
	return limit, offset
}

// buildListQuery builds an exact-match `SELECT * FROM <table> WHERE col=?
// AND col2=? ... LIMIT ? OFFSET ?` query. Every non-reserved query param is
// treated as an equality filter on a column, ANDed together; unknown columns
// are rejected (400) rather than silently ignored. No operator grammar
// (_gt, _like, etc.) per SPEC §5.7.
func buildListQuery(schema *db.TableSchema, query url.Values, limit, offset int) (sqlStr string, args []interface{}, err error) {
	colMap := make(map[string]bool, len(schema.Columns))
	for _, c := range schema.Columns {
		colMap[c.Name] = true
	}

	var whereParts []string
	for key, vals := range query {
		if reservedParams[key] || len(vals) == 0 {
			continue
		}
		if !colMap[key] {
			return "", nil, fmt.Errorf("unknown column: %s", key)
		}
		whereParts = append(whereParts, fmt.Sprintf("%s = ?", db.QuoteIdentifier(key)))
		args = append(args, vals[0])
	}

	where := ""
	if len(whereParts) > 0 {
		where = " WHERE "
		for i, p := range whereParts {
			if i > 0 {
				where += " AND "
			}
			where += p
		}
	}

	sqlStr = fmt.Sprintf("SELECT * FROM %s%s LIMIT ? OFFSET ?", db.QuoteIdentifier(schema.Name), where)
	args = append(args, limit, offset)
	return sqlStr, args, nil
}

// buildGetQuery builds `SELECT * FROM <table> WHERE <keyCol> = ?` for a
// single-row lookup by the resolved key column (a real column name or the
// synthetic "rowid").
func buildGetQuery(schema *db.TableSchema, keyCol string) string {
	return fmt.Sprintf("SELECT * FROM %s WHERE %s = ?", db.QuoteIdentifier(schema.Name), db.QuoteIdentifier(keyCol))
}
