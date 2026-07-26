package server

import (
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/0funct0ry/squad/internal/db"
	"github.com/gin-gonic/gin"
)

type QueryRequest struct {
	SQL   string `json:"sql" binding:"required"`
	Limit *int   `json:"limit"`
}

type QueryResponseData struct {
	Columns       []string `json:"columns"`
	Rows          [][]any  `json:"rows"`
	RowsAffected  int64    `json:"rowsAffected"`
	DurationMs    float64  `json:"durationMs"`
	Limit         int      `json:"limit"`
	Truncated     bool     `json:"truncated"`
	SchemaChanged bool     `json:"schemaChanged"`
}

// schemaChangingKeywords are the statement keywords that alter the set of
// tables/views/indexes/triggers in the database, as opposed to just their
// contents. Used to tell the frontend when it needs to refresh the table
// list even though a DDL statement reports 0 rowsAffected (e.g. CREATE
// TABLE), such as after running an --examples DDL script.
var schemaChangingKeywords = map[string]bool{
	"CREATE": true,
	"ALTER":  true,
	"DROP":   true,
}

// isSchemaChangingStatement reports whether stmt is DDL that adds, removes,
// or restructures a schema object.
func isSchemaChangingStatement(stmt string) bool {
	clean, err := db.StripCommentsAndWhitespace(stmt)
	if err != nil {
		return false
	}
	words := strings.Fields(clean)
	if len(words) == 0 {
		return false
	}
	return schemaChangingKeywords[strings.ToUpper(words[0])]
}

func (s *Server) handleQuery(c *gin.Context) {
	var req QueryRequest
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.SQL) == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok": false,
			"error": gin.H{
				"code":    "BAD_REQUEST",
				"message": "SQL statement is required",
			},
		})
		return
	}

	// Split statements and classify
	rawStatements, err := db.SplitStatements(req.SQL)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok": false,
			"error": gin.H{
				"code":    "SQL_ERROR",
				"message": err.Error(),
			},
		})
		return
	}

	// Filter out empty statements
	var statements []string
	var originalIndices []int // Keep track of the original 1-based index in the raw batch
	for idx, stmt := range rawStatements {
		clean, err := db.StripCommentsAndWhitespace(stmt)
		if err == nil && clean != "" {
			statements = append(statements, stmt)
			originalIndices = append(originalIndices, idx+1)
		}
	}

	if len(statements) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok": false,
			"error": gin.H{
				"code":    "BAD_REQUEST",
				"message": "SQL statement is required",
			},
		})
		return
	}

	// Validate against read-only mode
	for i, stmt := range statements {
		class, err := db.Classify(stmt)
		if err != nil {
			if errors.Is(err, db.ErrEmptyQuery) {
				c.JSON(http.StatusBadRequest, gin.H{
					"ok": false,
					"error": gin.H{
						"code":    "BAD_REQUEST",
						"message": "Empty query",
					},
				})
				return
			}
			c.JSON(http.StatusBadRequest, gin.H{
				"ok": false,
				"error": gin.H{
					"code":    "SQL_ERROR",
					"message": err.Error(),
				},
			})
			return
		}

		if class == db.ClassWrite && !s.write {
			clean, _ := db.StripCommentsAndWhitespace(stmt)
			// Extract keyword
			words := strings.Fields(clean)
			keyword := "WRITE"
			if len(words) > 0 {
				keyword = strings.ToUpper(words[0])
			}

			c.JSON(http.StatusForbidden, gin.H{
				"ok": false,
				"error": gin.H{
					"code":    "READ_ONLY",
					"message": fmt.Sprintf("%s is not allowed in read-only mode (statement %d)", keyword, originalIndices[i]),
				},
			})
			return
		}
	}

	// Parse and clamp limit
	limit := 1000
	if req.Limit != nil {
		limit = *req.Limit
	}
	if limit < 0 {
		limit = 0
	}
	if limit > 1000 {
		limit = 1000
	}

	// Setup execution
	var (
		cols          []string
		resultRows    [][]any
		rowsAffected  int64
		truncated     bool
		execDuration  float64
		schemaChanged bool
	)

	// If we are running write mode queries or multiple statements, run inside a single transaction
	isMulti := len(statements) > 1
	var hasWrite bool
	for _, stmt := range statements {
		class, _ := db.Classify(stmt)
		if class == db.ClassWrite {
			hasWrite = true
			break
		}
	}

	if isMulti || hasWrite {
		// Run batch in a transaction
		tx, err := s.db.Begin()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"ok":    false,
				"error": gin.H{"code": "DB_ERROR", "message": err.Error()},
			})
			return
		}
		defer tx.Rollback()

		startTime := time.Now()
		var totalRowsAffected int64

		for idx, stmt := range statements {
			class, _ := db.Classify(stmt)
			isLast := idx == len(statements)-1

			if class == db.ClassWrite {
				res, err := tx.Exec(stmt)
				if err != nil {
					c.JSON(http.StatusBadRequest, gin.H{
						"ok":    false,
						"error": gin.H{"code": "SQL_ERROR", "message": err.Error()},
					})
					return
				}
				affected, err := res.RowsAffected()
				if err == nil {
					totalRowsAffected += affected
				}
				if isSchemaChangingStatement(stmt) {
					schemaChanged = true
				}
			} else {
				// Read statement inside transaction (e.g. SELECT at the end of a batch)
				if isLast {
					// We only need to fetch results of the final statement
					rows, err := tx.Query(stmt)
					if err != nil {
						c.JSON(http.StatusBadRequest, gin.H{
							"ok":    false,
							"error": gin.H{"code": "SQL_ERROR", "message": err.Error()},
						})
						return
					}
					cols, resultRows, truncated, err = scanQueryRows(rows, limit)
					rows.Close()
					if err != nil {
						c.JSON(http.StatusBadRequest, gin.H{
							"ok":    false,
							"error": gin.H{"code": "SQL_ERROR", "message": err.Error()},
						})
						return
					}
				} else {
					// Execute but discard result if it's not the final statement
					rows, err := tx.Query(stmt)
					if err != nil {
						c.JSON(http.StatusBadRequest, gin.H{
							"ok":    false,
							"error": gin.H{"code": "SQL_ERROR", "message": err.Error()},
						})
						return
					}
					rows.Close()
				}
			}
		}

		if err := tx.Commit(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"ok":    false,
				"error": gin.H{"code": "DB_ERROR", "message": err.Error()},
			})
			return
		}

		execDuration = float64(time.Since(startTime).Nanoseconds()) / 1e6
		rowsAffected = totalRowsAffected
	} else {
		// Single read-only statement: run directly without a transaction
		stmt := statements[0]
		startTime := time.Now()

		rows, err := s.db.Query(stmt)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"ok":    false,
				"error": gin.H{"code": "SQL_ERROR", "message": err.Error()},
			})
			return
		}
		cols, resultRows, truncated, err = scanQueryRows(rows, limit)
		rows.Close()
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"ok":    false,
				"error": gin.H{"code": "SQL_ERROR", "message": err.Error()},
			})
			return
		}

		execDuration = float64(time.Since(startTime).Nanoseconds()) / 1e6
	}

	if cols == nil {
		cols = []string{}
	}
	if resultRows == nil {
		resultRows = [][]any{}
	}

	c.JSON(http.StatusOK, gin.H{
		"ok": true,
		"data": QueryResponseData{
			Columns:       cols,
			Rows:          resultRows,
			RowsAffected:  rowsAffected,
			DurationMs:    execDuration,
			Limit:         limit,
			Truncated:     truncated,
			SchemaChanged: schemaChanged,
		},
	})
}

// scanQueryRows reads rows up to limit and scans them into typed slices
func scanQueryRows(rows *sql.Rows, limit int) ([]string, [][]any, bool, error) {
	cols, err := rows.Columns()
	if err != nil {
		return nil, nil, false, err
	}

	colTypes, err := rows.ColumnTypes()
	colIsBlob := make([]bool, len(cols))
	if err == nil {
		for i, ct := range colTypes {
			colIsBlob[i] = strings.EqualFold(ct.DatabaseTypeName(), "BLOB")
		}
	}

	var resultRows [][]any
	truncated := false

	for rows.Next() {
		if len(resultRows) >= limit {
			truncated = true
			break
		}

		dest := make([]any, len(cols))
		destPtrs := make([]any, len(cols))
		for i := range destPtrs {
			destPtrs[i] = &dest[i]
		}

		if err := rows.Scan(destPtrs...); err != nil {
			return nil, nil, false, err
		}

		rowVals := make([]any, len(cols))
		for i, val := range dest {
			if val == nil {
				rowVals[i] = nil
			} else if bytes, ok := val.([]byte); ok {
				if colIsBlob[i] {
					rowVals[i] = hex.EncodeToString(bytes)
				} else {
					rowVals[i] = string(bytes)
				}
			} else {
				rowVals[i] = val
			}
		}
		resultRows = append(resultRows, rowVals)
	}

	return cols, resultRows, truncated, nil
}
