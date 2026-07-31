package server

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"strings"

	"github.com/0funct0ry/squad/internal/db"
	"github.com/0funct0ry/squad/internal/importer"
	"github.com/gin-gonic/gin"
)

const importPreviewSampleRows = 20

// parseImportFile opens the uploaded multipart file and dispatches to the
// matching internal/importer parser based on the (case-insensitive) format
// string. Format auto-detection from filename/content is a frontend
// concern; the server requires an explicit format value.
func parseImportFile(fh *multipart.FileHeader, format string) (importer.ParsedFile, error) {
	f, err := fh.Open()
	if err != nil {
		return importer.ParsedFile{}, fmt.Errorf("failed to open uploaded file: %w", err)
	}
	defer f.Close()

	switch strings.ToLower(format) {
	case "csv":
		return importer.ParseCSV(f)
	case "json":
		return importer.ParseJSON(f)
	case "yaml", "yml":
		return importer.ParseYAML(f)
	default:
		return importer.ParsedFile{}, fmt.Errorf("unsupported import format %q, must be csv, json, or yaml", format)
	}
}

// orderedTargetColumns returns the target (table-side) column names implied
// by mapping, in a deterministic order derived from the file's own column
// order (map iteration order is not stable in Go).
func orderedTargetColumns(pf importer.ParsedFile, mapping map[string]string) []string {
	var out []string
	for _, fileCol := range pf.Columns {
		target, ok := mapping[fileCol]
		if !ok || target == "" || target == importer.SkipMapping {
			continue
		}
		out = append(out, target)
	}
	return out
}

// POST /api/tables/:name/import/preview
//
// Parses the uploaded file server-side and returns its detected columns and
// a small sample of rows, plus an inferred create-table column spec, so the
// frontend's mapping/create-table UI never has to re-implement CSV/JSON/
// YAML parsing itself. Read-only: it never touches the database.
func (s *Server) handleImportPreview(c *gin.Context) {
	format := c.PostForm("format")
	if format == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":    false,
			"error": gin.H{"code": "BAD_REQUEST", "message": "format is required"},
		})
		return
	}

	fh, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":    false,
			"error": gin.H{"code": "BAD_REQUEST", "message": "file is required"},
		})
		return
	}

	pf, err := parseImportFile(fh, format)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":    false,
			"error": gin.H{"code": "VALIDATION", "message": err.Error()},
		})
		return
	}

	sample := pf.Rows
	truncated := false
	if len(sample) > importPreviewSampleRows {
		sample = sample[:importPreviewSampleRows]
		truncated = true
	}

	specs := importer.InferSchema(pf)
	inferred := make([]gin.H, len(specs))
	for i, spec := range specs {
		inferred[i] = gin.H{"name": spec.Name, "type": spec.Type}
	}

	c.JSON(http.StatusOK, gin.H{
		"ok": true,
		"data": gin.H{
			"columns":         pf.Columns,
			"rows":            sample,
			"truncated":       truncated,
			"totalRows":       len(pf.Rows),
			"inferredColumns": inferred,
		},
	})
}

// POST /api/tables/:name/import
//
// Imports rows from an uploaded file into an existing table, per a
// caller-supplied field mapping (file column -> target column, or
// "__skip__"). Runs the whole insert as one transaction so a validation or
// insert failure leaves the table untouched.
func (s *Server) handleImportIntoTable(c *gin.Context) {
	name := c.Param("name")

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

	format := c.PostForm("format")
	if format == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":    false,
			"error": gin.H{"code": "BAD_REQUEST", "message": "format is required"},
		})
		return
	}

	mappingRaw := c.PostForm("mapping")
	if strings.TrimSpace(mappingRaw) == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":    false,
			"error": gin.H{"code": "BAD_REQUEST", "message": "mapping is required"},
		})
		return
	}
	var mapping map[string]string
	if err := json.Unmarshal([]byte(mappingRaw), &mapping); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":    false,
			"error": gin.H{"code": "BAD_REQUEST", "message": fmt.Sprintf("invalid mapping JSON: %s", err.Error())},
		})
		return
	}

	fh, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":    false,
			"error": gin.H{"code": "BAD_REQUEST", "message": "file is required"},
		})
		return
	}

	pf, err := parseImportFile(fh, format)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":    false,
			"error": gin.H{"code": "VALIDATION", "message": err.Error()},
		})
		return
	}

	mappedRows, err := importer.ApplyFieldMapping(pf, mapping, schema)
	if err != nil {
		var verr *importer.ValidationError
		if errors.As(err, &verr) {
			c.JSON(http.StatusBadRequest, gin.H{
				"ok": false,
				"error": gin.H{
					"code":           "VALIDATION",
					"message":        verr.Error(),
					"missingColumns": verr.MissingColumns,
				},
			})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":    false,
			"error": gin.H{"code": "VALIDATION", "message": err.Error()},
		})
		return
	}

	targetColumns := orderedTargetColumns(pf, mapping)
	if len(targetColumns) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":    false,
			"error": gin.H{"code": "BAD_REQUEST", "message": "mapping does not target any columns"},
		})
		return
	}

	tx, err := s.db.Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"ok":    false,
			"error": gin.H{"code": "DB_ERROR", "message": err.Error()},
		})
		return
	}
	defer tx.Rollback()

	inserted, err := importer.BulkInsertRows(tx, name, targetColumns, mappedRows)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":    false,
			"error": gin.H{"code": "SQL_ERROR", "message": err.Error()},
		})
		return
	}

	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"ok":    false,
			"error": gin.H{"code": "DB_ERROR", "message": err.Error()},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"ok":   true,
		"data": gin.H{"inserted": inserted},
	})
}

// ImportColumnOverride lets the caller adjust an inferred column's name
// and/or SQLite type before the create-table-from-file path creates the
// table; overrides are matched positionally to the parsed file's columns.
type ImportColumnOverride struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// POST /api/tables/import
//
// Creates a brand-new table from an uploaded file's inferred (or
// user-adjusted) schema, then inserts every row - all inside one
// transaction, so a failure anywhere leaves neither the table nor any rows
// behind.
func (s *Server) handleImportCreateTable(c *gin.Context) {
	tableName := c.PostForm("name")
	if strings.TrimSpace(tableName) == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":    false,
			"error": gin.H{"code": "BAD_REQUEST", "message": "table name is required"},
		})
		return
	}

	format := c.PostForm("format")
	if format == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":    false,
			"error": gin.H{"code": "BAD_REQUEST", "message": "format is required"},
		})
		return
	}

	fh, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":    false,
			"error": gin.H{"code": "BAD_REQUEST", "message": "file is required"},
		})
		return
	}

	pf, err := parseImportFile(fh, format)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":    false,
			"error": gin.H{"code": "VALIDATION", "message": err.Error()},
		})
		return
	}
	if len(pf.Columns) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":    false,
			"error": gin.H{"code": "VALIDATION", "message": "file has no columns to create a table from"},
		})
		return
	}

	specs := importer.InferSchema(pf)

	if overrideRaw := c.PostForm("columns"); strings.TrimSpace(overrideRaw) != "" {
		var overrides []ImportColumnOverride
		if err := json.Unmarshal([]byte(overrideRaw), &overrides); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"ok":    false,
				"error": gin.H{"code": "BAD_REQUEST", "message": fmt.Sprintf("invalid columns JSON: %s", err.Error())},
			})
			return
		}
		if len(overrides) != len(specs) {
			c.JSON(http.StatusBadRequest, gin.H{
				"ok":    false,
				"error": gin.H{"code": "BAD_REQUEST", "message": fmt.Sprintf("columns override must have exactly %d entries (one per file column)", len(specs))},
			})
			return
		}
		for i, o := range overrides {
			if strings.TrimSpace(o.Name) == "" || strings.TrimSpace(o.Type) == "" {
				c.JSON(http.StatusBadRequest, gin.H{
					"ok":    false,
					"error": gin.H{"code": "BAD_REQUEST", "message": fmt.Sprintf("columns[%d]: name and type are required", i)},
				})
				return
			}
			specs[i].Name = o.Name
			specs[i].Type = o.Type
		}
	}

	seen := make(map[string]bool, len(specs))
	for _, spec := range specs {
		lower := strings.ToLower(spec.Name)
		if seen[lower] {
			c.JSON(http.StatusBadRequest, gin.H{
				"ok":    false,
				"error": gin.H{"code": "BAD_REQUEST", "message": fmt.Sprintf("duplicate column name: %s", spec.Name)},
			})
			return
		}
		seen[lower] = true
	}

	// Mounted virtual tables live in the temp schema, not main, so this has
	// to search both - an unqualified "sqlite_master" lookup (as used
	// previously) only ever resolves to main.sqlite_master and would miss a
	// name collision with a mount. Missing that collision here doesn't just
	// mean a worse error message: CREATE TABLE would still succeed (SQLite
	// allows same-named objects in different schemas), but temp objects
	// shadow main ones for unqualified access, so every subsequent insert
	// against tableName would silently hit the mount's virtual table
	// instead of the table just created, surfacing a confusing low-level
	// "no column named X" error instead of this clear one.
	var existingType string
	err = s.db.QueryRow(
		`SELECT type FROM (
			SELECT type, name FROM sqlite_master
			UNION ALL
			SELECT type, name FROM temp.sqlite_master
		) WHERE type IN ('table', 'view') AND name = ?`,
		tableName,
	).Scan(&existingType)
	if err != nil && err != sql.ErrNoRows {
		c.JSON(http.StatusInternalServerError, gin.H{
			"ok":    false,
			"error": gin.H{"code": "DB_ERROR", "message": err.Error()},
		})
		return
	}
	if err == nil {
		kind := existingType
		if kind == "" {
			kind = "object"
		}
		c.JSON(http.StatusConflict, gin.H{
			"ok":    false,
			"error": gin.H{"code": "ALREADY_EXISTS", "message": fmt.Sprintf("a %s named %q already exists - pick a different name, or use \"Import into existing table\" instead", kind, tableName)},
		})
		return
	}

	// Optional primary key: a JSON array of column names (matched against
	// the final, possibly-overridden spec names), rendered as a table-level
	// PRIMARY KEY (...) constraint so both single and composite keys work
	// the same way.
	var primaryKey []string
	if pkRaw := c.PostForm("primaryKey"); strings.TrimSpace(pkRaw) != "" {
		if err := json.Unmarshal([]byte(pkRaw), &primaryKey); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"ok":    false,
				"error": gin.H{"code": "BAD_REQUEST", "message": fmt.Sprintf("invalid primaryKey JSON: %s", err.Error())},
			})
			return
		}
		specNames := make(map[string]bool, len(specs))
		for _, spec := range specs {
			specNames[strings.ToLower(spec.Name)] = true
		}
		seenPK := make(map[string]bool, len(primaryKey))
		for _, pkCol := range primaryKey {
			lower := strings.ToLower(pkCol)
			if !specNames[lower] {
				c.JSON(http.StatusBadRequest, gin.H{
					"ok":    false,
					"error": gin.H{"code": "BAD_REQUEST", "message": fmt.Sprintf("primaryKey column %q not found among the table's columns", pkCol)},
				})
				return
			}
			if seenPK[lower] {
				c.JSON(http.StatusBadRequest, gin.H{
					"ok":    false,
					"error": gin.H{"code": "BAD_REQUEST", "message": fmt.Sprintf("duplicate primaryKey column %q", pkCol)},
				})
				return
			}
			seenPK[lower] = true
		}
	}

	colDefs := make([]string, len(specs))
	targetColumns := make([]string, len(specs))
	for i, spec := range specs {
		colDefs[i] = fmt.Sprintf("%s %s", db.QuoteIdentifier(spec.Name), spec.Type)
		targetColumns[i] = spec.Name
	}
	if len(primaryKey) > 0 {
		quoted := make([]string, len(primaryKey))
		for i, pkCol := range primaryKey {
			quoted[i] = db.QuoteIdentifier(pkCol)
		}
		colDefs = append(colDefs, fmt.Sprintf("PRIMARY KEY (%s)", strings.Join(quoted, ", ")))
	}
	createDDL := fmt.Sprintf("CREATE TABLE %s (%s)", db.QuoteIdentifier(tableName), strings.Join(colDefs, ", "))

	// Remap each parsed row from file-column keys to target-column keys
	// (positional, matching specs/targetColumns order against pf.Columns).
	rows := make([]map[string]any, len(pf.Rows))
	for i, row := range pf.Rows {
		out := make(map[string]any, len(pf.Columns))
		for j, fileCol := range pf.Columns {
			out[targetColumns[j]] = row[fileCol]
		}
		rows[i] = out
	}

	tx, err := s.db.Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"ok":    false,
			"error": gin.H{"code": "DB_ERROR", "message": err.Error()},
		})
		return
	}
	defer tx.Rollback()

	if _, err := tx.Exec(createDDL); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":    false,
			"error": gin.H{"code": "SQL_ERROR", "message": err.Error()},
		})
		return
	}

	inserted, err := importer.BulkInsertRows(tx, tableName, targetColumns, rows)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":    false,
			"error": gin.H{"code": "SQL_ERROR", "message": err.Error()},
		})
		return
	}

	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"ok":    false,
			"error": gin.H{"code": "DB_ERROR", "message": err.Error()},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"ok":   true,
		"data": gin.H{"table": tableName, "inserted": inserted},
	})
}
