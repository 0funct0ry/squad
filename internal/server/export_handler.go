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
	"github.com/gin-gonic/gin"
)

type QueryExportRequest struct {
	SQL string `json:"sql" binding:"required"`
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
	if format != "csv" && format != "json" && format != "sql" {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":    false,
			"error": gin.H{"code": "BAD_REQUEST", "message": "invalid format, must be csv, json, or sql"},
		})
		return
	}

	// 1. Resolve table schema first to ensure table exists (and get DDL)
	schema, err := db.GetTableSchema(s.db, name)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{
				"ok":    false,
				"error": gin.H{"code": "NOT_FOUND", "message": "table or view not found"},
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"ok":    false,
			"error": gin.H{"code": "DB_ERROR", "message": err.Error()},
		})
		return
	}

	// 2. Build row query
	filtered := c.Query("filtered") == "true"
	var selectQuery string
	var args []interface{}
	if filtered {
		// Re-use M1 rows query-param logic
		orderBy := c.Query("orderBy")
		dir := c.Query("dir")
		filters := make(map[string]string)
		queries := c.Request.URL.Query()
		for k, v := range queries {
			if strings.HasPrefix(k, "filter[") && strings.HasSuffix(k, "]") && len(v) > 0 {
				col := k[7 : len(k)-1]
				filters[col] = v[0]
			}
		}
		params := db.RowQueryParams{
			OrderBy: orderBy,
			Dir:     dir,
			Filters: filters,
		}
		var err error
		selectQuery, _, args, _, err = db.BuildTableQuery(s.db, name, params)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"ok":    false,
				"error": gin.H{"code": "DB_ERROR", "message": err.Error()},
			})
			return
		}
	} else {
		selectQuery = fmt.Sprintf("SELECT * FROM %s", db.QuoteIdentifier(name))
	}

	// Execute query
	rows, err := s.db.Query(selectQuery, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"ok":    false,
			"error": gin.H{"code": "DB_ERROR", "message": err.Error()},
		})
		return
	}
	defer rows.Close()

	source, cols, err := makeRowSource(rows, schema)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"ok":    false,
			"error": gin.H{"code": "DB_ERROR", "message": err.Error()},
		})
		return
	}

	// 3. Set headers
	filename := fmt.Sprintf("%s.%s", sanitizeFilename(name), format)
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))

	switch format {
	case "csv":
		c.Header("Content-Type", "text/csv; charset=utf-8")
		headersParam := c.DefaultQuery("headers", "true") != "false"
		if err := export.ExportCSV(cols, source, c.Writer, headersParam); err != nil {
			_ = c.Error(err)
		}
	case "json":
		c.Header("Content-Type", "application/json")
		if err := export.ExportJSON(cols, source, c.Writer); err != nil {
			_ = c.Error(err)
		}
	case "sql":
		c.Header("Content-Type", "application/sql")
		includeSchema := c.Query("includeSchema") == "true"
		if err := export.ExportSQL(name, cols, source, c.Writer, includeSchema, schema.DDL); err != nil {
			_ = c.Error(err)
		}
	}
}

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
	if format != "csv" && format != "json" {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":    false,
			"error": gin.H{"code": "BAD_REQUEST", "message": "invalid format, must be csv or json"},
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

	rows, err := s.db.Query(req.SQL)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":    false,
			"error": gin.H{"code": "SQL_ERROR", "message": err.Error()},
		})
		return
	}
	defer rows.Close()

	source, cols, err := makeRowSource(rows, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"ok":    false,
			"error": gin.H{"code": "DB_ERROR", "message": err.Error()},
		})
		return
	}

	filename := fmt.Sprintf("query-export.%s", format)
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))

	switch format {
	case "csv":
		c.Header("Content-Type", "text/csv; charset=utf-8")
		headersParam := c.DefaultQuery("headers", "true") != "false"
		if err := export.ExportCSV(cols, source, c.Writer, headersParam); err != nil {
			_ = c.Error(err)
		}
	case "json":
		c.Header("Content-Type", "application/json")
		if err := export.ExportJSON(cols, source, c.Writer); err != nil {
			_ = c.Error(err)
		}
	}
}
