package server

import (
	"database/sql"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/0funct0ry/squad/internal/db"
	"github.com/0funct0ry/squad/internal/export"
	"github.com/0funct0ry/squad/internal/vtab"
	"github.com/gin-gonic/gin"
)

type QueryExportRequest struct {
	SQL string `json:"sql" binding:"required"`
	// ColumnLabels optionally renames the exported header/JSON-key for each
	// column, positionally matching the final column list (i.e. after any
	// columns= subset/reorder is applied). Useful when the query's own
	// column names are raw, unaliased SQL expressions (e.g.
	// "concat(first_name,' ',last_name)") rather than simple identifiers.
	ColumnLabels []string `json:"columnLabels"`
}

// tableExportFormats are the formats accepted by GET /tables/:name/export.
var tableExportFormats = map[string]bool{
	"csv": true, "json": true, "sql": true,
	"yaml": true, "xml": true, "toml": true,
	"bson": true, "protobuf": true, "xlsx": true, "parquet": true,
}

// queryExportFormats are the formats accepted by POST /export/query. SQL is
// intentionally excluded here (it's meaningless for an arbitrary read query
// result rather than a named table).
var queryExportFormats = map[string]bool{
	"csv": true, "json": true,
	"yaml": true, "xml": true, "toml": true,
	"bson": true, "protobuf": true, "xlsx": true, "parquet": true,
}

// exportContentType maps a format to its HTTP Content-Type header.
var exportContentType = map[string]string{
	"csv":      "text/csv; charset=utf-8",
	"json":     "application/json",
	"sql":      "application/sql",
	"yaml":     "application/yaml",
	"xml":      "application/xml",
	"toml":     "application/toml",
	"bson":     "application/octet-stream",
	"protobuf": "application/octet-stream",
	"xlsx":     "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
	"parquet":  "application/octet-stream",
}

// writeExport dispatches to the matching internal/export writer for
// formats shared by both export routes (i.e. everything except "sql",
// which only handleTableExport supports since it needs a table/DDL, and
// "xml", which needs its own export.XMLOptions and is handled by the
// caller via buildXMLOptions + export.ExportXML directly).
func writeExport(format string, cols []string, source export.RowSource, w io.Writer, csvHeaders bool) error {
	switch format {
	case "csv":
		return export.ExportCSV(cols, source, w, csvHeaders)
	case "json":
		return export.ExportJSON(cols, source, w)
	case "yaml":
		return export.ExportYAML(cols, source, w)
	case "toml":
		return export.ExportTOML(cols, source, w)
	case "bson":
		return export.ExportBSON(cols, source, w)
	case "protobuf":
		return export.ExportProtobuf(cols, source, w)
	case "xlsx":
		return export.ExportXLSX(cols, source, w)
	case "parquet":
		return export.ExportParquet(cols, source, w)
	default:
		return fmt.Errorf("unsupported export format %q", format)
	}
}

func sanitizeFilename(name string) string {
	var sb strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '.' || r == '-' {
			sb.WriteRune(r)
		} else {
			sb.WriteRune('_')
		}
	}
	return sb.String()
}

func makeRowSource(rows *sql.Rows, schema *db.TableSchema) (export.RowSource, []string, error) {
	cols, err := rows.Columns()
	if err != nil {
		return nil, nil, err
	}

	colIsBlob := make([]bool, len(cols))
	colTypes, err := rows.ColumnTypes()
	if err == nil {
		for i, ct := range colTypes {
			colIsBlob[i] = strings.EqualFold(ct.DatabaseTypeName(), "BLOB")
		}
	}

	if schema != nil {
		for i, colName := range cols {
			for _, sc := range schema.Columns {
				if sc.Name == colName {
					if strings.EqualFold(sc.Type, "blob") {
						colIsBlob[i] = true
					}
					break
				}
			}
		}
	}

	source := func() ([]any, error) {
		if !rows.Next() {
			if err := rows.Err(); err != nil {
				return nil, err
			}
			return nil, io.EOF
		}

		dest := make([]any, len(cols))
		destPtrs := make([]any, len(cols))
		for i := range destPtrs {
			destPtrs[i] = &dest[i]
		}

		if err := rows.Scan(destPtrs...); err != nil {
			return nil, err
		}

		rowVals := make([]any, len(cols))
		for i, val := range dest {
			if val == nil {
				rowVals[i] = nil
			} else if bytes, ok := val.([]byte); ok {
				if colIsBlob[i] {
					rowVals[i] = bytes
				} else {
					rowVals[i] = string(bytes)
				}
			} else {
				rowVals[i] = val
			}
		}
		return rowVals, nil
	}

	return source, cols, nil
}

// buildXMLOptions constructs export.XMLOptions from optional "xml*" query
// params, layered on top of export.DefaultXMLOptions(defaultTableName).
// defaultTableName should be the source table name for a table export, or
// "" for an ad hoc query export (which has no single named source).
func buildXMLOptions(c *gin.Context, defaultTableName string) export.XMLOptions {
	opts := export.DefaultXMLOptions(defaultTableName)
	if v := c.Query("xmlRootTag"); v != "" {
		opts.RootTag = v
	}
	if v := c.Query("xmlRowTag"); v != "" {
		opts.RowTag = v
	}
	if v := c.Query("xmlCase"); v != "" {
		opts.CaseStyle = export.ParseXMLCaseStyle(v)
	}
	if v := c.Query("xmlPretty"); v != "" {
		opts.Pretty = v == "true"
	}
	opts.IndentSize = export.ParseXMLIndentSize(c.Query("xmlIndent"), opts.IndentSize)
	if v := c.Query("xmlDeclaration"); v != "" {
		opts.IncludeDeclaration = v == "true"
	}
	if v := c.Query("xmlNullHandling"); v != "" {
		opts.NullHandling = export.ParseXMLNullHandling(v)
	}
	return opts
}

// parseColumnsParam splits a "columns=col1,col2" query param into a
// trimmed, non-empty column-name list, preserving requested order.
func parseColumnsParam(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	cols := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			cols = append(cols, p)
		}
	}
	return cols
}

// validateColumnsAgainstSchema whitelists requested column names against a
// table's real columns, so an export's columns= param can never be used to
// smuggle arbitrary SQL into the generated SELECT list.
func validateColumnsAgainstSchema(requested []string, schema *db.TableSchema) error {
	known := make(map[string]bool, len(schema.Columns))
	for _, c := range schema.Columns {
		known[c.Name] = true
	}
	for _, c := range requested {
		if !known[c] {
			return fmt.Errorf("unknown column %q", c)
		}
	}
	return nil
}

// projectRowSource wraps a RowSource/column-list pair to only yield the
// requested subset of columns, in the requested order. Used for
// post-execution column selection over an already-executed arbitrary query
// (handleQueryExport), where rewriting the user's own SQL to select a
// column subset would be unsafe/complex.
func projectRowSource(cols []string, source export.RowSource, requested []string) (export.RowSource, []string, error) {
	indices := make([]int, len(requested))
	colIndex := make(map[string]int, len(cols))
	for i, c := range cols {
		colIndex[c] = i
	}
	for i, name := range requested {
		idx, ok := colIndex[name]
		if !ok {
			return nil, nil, fmt.Errorf("unknown column %q in result set", name)
		}
		indices[i] = idx
	}

	projected := func() ([]any, error) {
		row, err := source()
		if err != nil {
			return nil, err
		}
		out := make([]any, len(indices))
		for i, idx := range indices {
			out[i] = row[idx]
		}
		return out, nil
	}

	return projected, requested, nil
}

func (s *Server) handleTableExport(c *gin.Context) {
	name := c.Param("name")
	format := c.Query("format")
	if format == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":    false,
			"error": gin.H{"code": "BAD_REQUEST", "message": "format query parameter is required"},
		})
		return
	}
	format = strings.ToLower(format)
	if !tableExportFormats[format] {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":    false,
			"error": gin.H{"code": "BAD_REQUEST", "message": "invalid format, must be one of csv, json, sql, yaml, xml, toml, bson, protobuf, xlsx, parquet"},
		})
		return
	}

	err := vtab.WithMounts(c.Request.Context(), s.db, s.mountStore, func(conn *sql.Conn) error {
		q := db.WrapConn(conn)

		// 1. Resolve table schema first to ensure table exists (and get DDL)
		schema, err := db.GetTableSchema(q, name)
		if err != nil {
			return err
		}

		// 2. Build row query
		filtered := c.Query("filtered") == "true"
		var selectQuery string
		var args []interface{}
		if filtered {
			// Re-use M1 rows query-param logic
			orderBy := c.Query("orderBy")
			dir := c.Query("dir")
			filters, ferr := parseRowFilters(c)
			if ferr != nil {
				return ferr
			}
			params := db.RowQueryParams{
				OrderBy: orderBy,
				Dir:     dir,
				Filters: filters,
			}
			selectQuery, _, args, _, err = db.BuildTableQuery(q, name, params)
			if err != nil {
				return err
			}
		} else {
			selectQuery = fmt.Sprintf("SELECT * FROM %s", db.QuoteIdentifier(name))
		}

		// 2b. Optional column subset: whitelist against the real schema, then
		// replace the generated SELECT's column list (everything before the
		// first " FROM ") rather than the caller-visible WHERE/ORDER BY/args,
		// since both the filtered and unfiltered branches above always produce
		// a "SELECT <fields> FROM ..." string.
		if requestedCols := parseColumnsParam(c.Query("columns")); len(requestedCols) > 0 {
			if err := validateColumnsAgainstSchema(requestedCols, schema); err != nil {
				return &exportValidationError{err}
			}
			quoted := make([]string, len(requestedCols))
			for i, col := range requestedCols {
				quoted[i] = db.QuoteIdentifier(col)
			}
			fromIdx := strings.Index(selectQuery, " FROM ")
			selectQuery = "SELECT " + strings.Join(quoted, ", ") + selectQuery[fromIdx:]
		}

		// Execute query
		rows, err := conn.QueryContext(c.Request.Context(), selectQuery, args...)
		if err != nil {
			return err
		}
		defer rows.Close()

		source, cols, err := makeRowSource(rows, schema)
		if err != nil {
			return err
		}

		// 3. Set headers
		filename := fmt.Sprintf("%s.%s", sanitizeFilename(name), format)
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))

		c.Header("Content-Type", exportContentType[format])
		if format == "sql" {
			return export.ExportSQL(name, cols, source, c.Writer, c.Query("includeSchema") == "true", schema.DDL)
		}
		if format == "xml" {
			return export.ExportXML(cols, source, c.Writer, buildXMLOptions(c, name))
		}
		headersParam := c.DefaultQuery("headers", "true") != "false"
		return writeExport(format, cols, source, c.Writer, headersParam)
	})
	if err != nil {
		var verr *exportValidationError
		if errors.As(err, &verr) {
			c.JSON(http.StatusBadRequest, gin.H{
				"ok":    false,
				"error": gin.H{"code": "VALIDATION", "message": verr.Error()},
			})
			return
		}
		if errors.Is(err, db.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{
				"ok":    false,
				"error": gin.H{"code": "NOT_FOUND", "message": "table or view not found"},
			})
			return
		}
		if !c.Writer.Written() {
			c.JSON(http.StatusInternalServerError, gin.H{
				"ok":    false,
				"error": gin.H{"code": "DB_ERROR", "message": err.Error()},
			})
			return
		}
		_ = c.Error(err)
	}
}

// exportValidationError distinguishes a client-caused VALIDATION error
// (invalid columns=) from any other DB/write error inside the WithMounts
// closure above, so the outer handler can map it to 400 instead of 500.
type exportValidationError struct{ err error }

func (e *exportValidationError) Error() string { return e.err.Error() }
func (e *exportValidationError) Unwrap() error { return e.err }

func (s *Server) handleQueryExport(c *gin.Context) {
	format := c.Query("format")
	if format == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":    false,
			"error": gin.H{"code": "BAD_REQUEST", "message": "format query parameter is required"},
		})
		return
	}
	format = strings.ToLower(format)
	if !queryExportFormats[format] {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":    false,
			"error": gin.H{"code": "BAD_REQUEST", "message": "invalid format, must be one of csv, json, yaml, xml, toml, bson, protobuf, xlsx, parquet"},
		})
		return
	}

	var req QueryExportRequest
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.SQL) == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":    false,
			"error": gin.H{"code": "BAD_REQUEST", "message": "SQL statement is required"},
		})
		return
	}

	// 1. Run sql through classify
	class, err := db.Classify(req.SQL)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":    false,
			"error": gin.H{"code": "SQL_ERROR", "message": err.Error()},
		})
		return
	}

	// Exporting is a read operation; reject ANY write statement
	if class == db.ClassWrite {
		c.JSON(http.StatusForbidden, gin.H{
			"ok":    false,
			"error": gin.H{"code": "READ_ONLY", "message": "mutating queries are not permitted in export"},
		})
		return
	}

	err = vtab.WithMounts(c.Request.Context(), s.db, s.mountStore, func(conn *sql.Conn) error {
		rows, err := conn.QueryContext(c.Request.Context(), req.SQL)
		if err != nil {
			return err
		}
		defer rows.Close()

		source, cols, err := makeRowSource(rows, nil)
		if err != nil {
			return err
		}

		// Optional column subset: since this route executes arbitrary user SQL,
		// selection is applied as a post-execution projection over whatever
		// columns the query already returned, rather than rewriting the SQL
		// text (unsafe/complex for arbitrary queries).
		if requestedCols := parseColumnsParam(c.Query("columns")); len(requestedCols) > 0 {
			projected, projectedCols, err := projectRowSource(cols, source, requestedCols)
			if err != nil {
				return &exportValidationError{err}
			}
			source, cols = projected, projectedCols
		}

		// Optional column relabeling: lets the caller override the exported
		// header/JSON-key names (e.g. when the query's own columns are raw,
		// unaliased expressions), positionally matched to the final column list.
		if len(req.ColumnLabels) > 0 {
			if len(req.ColumnLabels) != len(cols) {
				return &exportValidationError{fmt.Errorf("columnLabels must have exactly %d entries (one per exported column)", len(cols))}
			}
			seen := make(map[string]bool, len(req.ColumnLabels))
			for _, label := range req.ColumnLabels {
				if strings.TrimSpace(label) == "" {
					return &exportValidationError{fmt.Errorf("columnLabels entries cannot be empty")}
				}
				if seen[label] {
					return &exportValidationError{fmt.Errorf("duplicate columnLabels entry %q", label)}
				}
				seen[label] = true
			}
			cols = req.ColumnLabels
		}

		filename := fmt.Sprintf("query-export.%s", format)
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))

		c.Header("Content-Type", exportContentType[format])
		if format == "xml" {
			return export.ExportXML(cols, source, c.Writer, buildXMLOptions(c, ""))
		}
		headersParam := c.DefaultQuery("headers", "true") != "false"
		return writeExport(format, cols, source, c.Writer, headersParam)
	})
	if err != nil {
		var verr *exportValidationError
		if errors.As(err, &verr) {
			c.JSON(http.StatusBadRequest, gin.H{
				"ok":    false,
				"error": gin.H{"code": "VALIDATION", "message": verr.Error()},
			})
			return
		}
		if !c.Writer.Written() {
			c.JSON(http.StatusBadRequest, gin.H{
				"ok":    false,
				"error": gin.H{"code": "SQL_ERROR", "message": err.Error()},
			})
			return
		}
		_ = c.Error(err)
	}
}
