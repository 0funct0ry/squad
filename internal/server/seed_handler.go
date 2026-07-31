package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/0funct0ry/squad/internal/db"
	"github.com/0funct0ry/squad/internal/seed"
	"github.com/gin-gonic/gin"
)

const (
	seedMinCount   = 1
	seedMaxCount   = 100000
	seedMaxRetries = 20
)

// resolveSeedableTable loads the schema for name and rejects views, writing
// an error response and returning ok=false if the table can't be seeded.
func (s *Server) resolveSeedableTable(c *gin.Context, name string) (*db.TableSchema, bool) {
	schema, err := db.GetTableSchema(s.db, name)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			c.JSON(404, gin.H{
				"ok":    false,
				"error": gin.H{"code": "NOT_FOUND", "message": "table or view not found"},
			})
			return nil, false
		}
		c.JSON(500, gin.H{
			"ok":    false,
			"error": gin.H{"code": "DB_ERROR", "message": err.Error()},
		})
		return nil, false
	}
	if schema.Type == "view" {
		c.JSON(400, gin.H{
			"ok":    false,
			"error": gin.H{"code": "BAD_REQUEST", "message": "cannot seed a view"},
		})
		return nil, false
	}
	return schema, true
}

// GET /api/tables/:name/seed/plan
func (s *Server) handleSeedPlan(c *gin.Context) {
	name := c.Param("name")
	schema, ok := s.resolveSeedableTable(c, name)
	if !ok {
		return
	}

	columns, err := seed.BuildPlan(s.db, schema)
	if err != nil {
		c.JSON(500, gin.H{
			"ok":    false,
			"error": gin.H{"code": "DB_ERROR", "message": err.Error()},
		})
		return
	}

	c.JSON(200, gin.H{
		"ok": true,
		"data": gin.H{
			"columns":             columns,
			"availableGenerators": seed.AvailableGenerators(),
			"generatorCatalog":    seed.GeneratorCatalog(),
		},
	})
}

// SeedColumnRequest is the caller-supplied generator + options for one column.
type SeedColumnRequest struct {
	Generator string         `json:"generator"`
	Options   map[string]any `json:"options"`
}

// SeedRequest is the body of POST /api/tables/:name/seed.
type SeedRequest struct {
	Count   int                          `json:"count"`
	DryRun  bool                         `json:"dryRun"`
	Columns map[string]SeedColumnRequest `json:"columns"`
}

// POST /api/tables/:name/seed
func (s *Server) handleSeedTable(c *gin.Context) {
	name := c.Param("name")
	schema, ok := s.resolveSeedableTable(c, name)
	if !ok {
		return
	}

	var req SeedRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{
			"ok":    false,
			"error": gin.H{"code": "BAD_REQUEST", "message": err.Error()},
		})
		return
	}

	if req.Count < seedMinCount || req.Count > seedMaxCount {
		c.JSON(400, gin.H{
			"ok":    false,
			"error": gin.H{"code": "BAD_REQUEST", "message": fmt.Sprintf("count must be between %d and %d", seedMinCount, seedMaxCount)},
		})
		return
	}

	colMap := make(map[string]db.ColumnInfo, len(schema.Columns))
	for _, col := range schema.Columns {
		colMap[col.Name] = col
	}

	specs := make(map[string]seed.ColumnSpec, len(req.Columns))
	for colName, colReq := range req.Columns {
		col, exists := colMap[colName]
		if !exists {
			c.JSON(400, gin.H{
				"ok":    false,
				"error": gin.H{"code": "BAD_REQUEST", "message": fmt.Sprintf("unknown column: %s", colName)},
			})
			return
		}
		if col.Hidden == 2 || col.Hidden == 3 {
			c.JSON(400, gin.H{
				"ok":    false,
				"error": gin.H{"code": "BAD_REQUEST", "message": fmt.Sprintf("cannot seed generated column: %s", colName)},
			})
			return
		}
		if !seed.Exists(colReq.Generator) {
			c.JSON(400, gin.H{
				"ok":    false,
				"error": gin.H{"code": "UNKNOWN_GENERATOR", "message": fmt.Sprintf("unknown generator: %s", colReq.Generator)},
			})
			return
		}

		options := colReq.Options
		if options == nil {
			options = map[string]any{}
		}

		if colReq.Generator == seed.ForeignKeyGeneratorName {
			table, _ := options["table"].(string)
			column, _ := options["column"].(string)
			if table == "" || column == "" {
				c.JSON(400, gin.H{
					"ok":    false,
					"error": gin.H{"code": "BAD_REQUEST", "message": "foreignKey generator requires options.table and options.column"},
				})
				return
			}
			refSchema, err := db.GetTableSchema(s.db, table)
			if err != nil {
				if errors.Is(err, db.ErrNotFound) {
					c.JSON(404, gin.H{
						"ok":    false,
						"error": gin.H{"code": "NOT_FOUND", "message": fmt.Sprintf("referenced table not found: %s", table)},
					})
					return
				}
				c.JSON(500, gin.H{
					"ok":    false,
					"error": gin.H{"code": "DB_ERROR", "message": err.Error()},
				})
				return
			}
			found := false
			for _, rc := range refSchema.Columns {
				if rc.Name == column {
					found = true
					break
				}
			}
			if !found {
				c.JSON(404, gin.H{
					"ok":    false,
					"error": gin.H{"code": "NOT_FOUND", "message": fmt.Sprintf("referenced column not found: %s.%s", table, column)},
				})
				return
			}
		}

		if colReq.Generator == "enumFromColumn" {
			table, _ := options["table"].(string)
			column, _ := options["column"].(string)
			if table == "" || column == "" {
				c.JSON(400, gin.H{
					"ok":    false,
					"error": gin.H{"code": "BAD_REQUEST", "message": "enumFromColumn generator requires options.table and options.column"},
				})
				return
			}
			refSchema, err := db.GetTableSchema(s.db, table)
			if err != nil {
				if errors.Is(err, db.ErrNotFound) {
					c.JSON(404, gin.H{
						"ok":    false,
						"error": gin.H{"code": "NOT_FOUND", "message": fmt.Sprintf("referenced table not found: %s", table)},
					})
					return
				}
				c.JSON(500, gin.H{
					"ok":    false,
					"error": gin.H{"code": "DB_ERROR", "message": err.Error()},
				})
				return
			}
			found := false
			for _, rc := range refSchema.Columns {
				if rc.Name == column {
					found = true
					break
				}
			}
			if !found {
				c.JSON(404, gin.H{
					"ok":    false,
					"error": gin.H{"code": "NOT_FOUND", "message": fmt.Sprintf("referenced column not found: %s.%s", table, column)},
				})
				return
			}
		}

		if colReq.Generator == "nullWithProbability" {
			wrapped, _ := options["generator"].(map[string]any)
			wrappedName, _ := wrapped["generator"].(string)
			if wrappedName == "" {
				c.JSON(400, gin.H{
					"ok":    false,
					"error": gin.H{"code": "BAD_REQUEST", "message": "nullWithProbability generator requires options.generator.generator"},
				})
				return
			}
			if wrappedName == "nullWithProbability" {
				c.JSON(400, gin.H{
					"ok":    false,
					"error": gin.H{"code": "BAD_REQUEST", "message": "nullWithProbability cannot wrap itself"},
				})
				return
			}
			if !seed.Exists(wrappedName) {
				c.JSON(400, gin.H{
					"ok":    false,
					"error": gin.H{"code": "BAD_REQUEST", "message": fmt.Sprintf("nullWithProbability: unknown wrapped generator: %s", wrappedName)},
				})
				return
			}
		}

		specs[colName] = seed.ColumnSpec{Generator: colReq.Generator, Options: options}
	}

	if err := seed.ValidateFormulaDependencies(specs); err != nil {
		c.JSON(400, gin.H{
			"ok":    false,
			"error": gin.H{"code": "BAD_REQUEST", "message": err.Error()},
		})
		return
	}

	gen, err := seed.NewRowGenerator(s.db, schema, specs)
	if err != nil {
		var emptyRef *seed.EmptyReferenceError
		if errors.As(err, &emptyRef) {
			c.JSON(400, gin.H{
				"ok":    false,
				"error": gin.H{"code": "EMPTY_REFERENCE", "message": err.Error()},
			})
			return
		}
		c.JSON(500, gin.H{
			"ok":    false,
			"error": gin.H{"code": "DB_ERROR", "message": err.Error()},
		})
		return
	}

	if req.DryRun {
		rows := make([]map[string]any, 0, req.Count)
		for i := 0; i < req.Count; i++ {
			row, err := gen.GenerateRow()
			if err != nil {
				c.JSON(400, gin.H{
					"ok":    false,
					"error": gin.H{"code": "SQL_ERROR", "message": err.Error()},
				})
				return
			}
			rows = append(rows, row)
		}
		c.JSON(200, gin.H{
			"ok":   true,
			"data": gin.H{"rows": rows},
		})
		return
	}

	// Deterministic column order for the INSERT statement, reused every row.
	colNames := make([]string, 0, len(specs))
	for colName := range specs {
		colNames = append(colNames, colName)
	}
	sort.Strings(colNames)

	var quotedCols, placeholders []string
	for _, colName := range colNames {
		quotedCols = append(quotedCols, db.QuoteIdentifier(colName))
		placeholders = append(placeholders, "?")
	}
	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
		db.QuoteIdentifier(name), strings.Join(quotedCols, ", "), strings.Join(placeholders, ", "))

	tx, err := s.db.Begin()
	if err != nil {
		c.JSON(500, gin.H{
			"ok":    false,
			"error": gin.H{"code": "DB_ERROR", "message": err.Error()},
		})
		return
	}

	for i := 0; i < req.Count; i++ {
		row, err := gen.GenerateRow()
		if err != nil {
			tx.Rollback()
			c.JSON(400, gin.H{
				"ok":    false,
				"error": gin.H{"code": "SQL_ERROR", "message": fmt.Sprintf("row %d: %s", i+1, err.Error())},
			})
			return
		}

		var insertErr error
		for attempt := 0; attempt <= seedMaxRetries; attempt++ {
			args := make([]interface{}, len(colNames))
			for j, colName := range colNames {
				args[j] = row[colName]
			}

			_, insertErr = tx.Exec(query, args...)
			if insertErr == nil {
				break
			}
			if !isUniqueConstraintError(insertErr) || attempt == seedMaxRetries {
				break
			}
			for _, group := range gen.UniqueGroups() {
				if err := gen.RegenerateGroup(row, group); err != nil {
					insertErr = err
					break
				}
			}
		}

		if insertErr != nil {
			tx.Rollback()
			c.JSON(400, gin.H{
				"ok":    false,
				"error": gin.H{"code": "SQL_ERROR", "message": fmt.Sprintf("row %d: %s", i+1, insertErr.Error())},
			})
			return
		}
	}

	if err := tx.Commit(); err != nil {
		c.JSON(500, gin.H{
			"ok":    false,
			"error": gin.H{"code": "DB_ERROR", "message": err.Error()},
		})
		return
	}

	c.JSON(200, gin.H{
		"ok":   true,
		"data": gin.H{"inserted": req.Count},
	})
}

// GET /api/seed/generators/catalog — the same generator catalog embedded in
// a seed plan response, but standalone and ungated (no table, no --write
// requirement), so callers that aren't table-scoped — currently the Modules
// tab's `fake` mount form, which needs generator names for its repeatable
// <column>=<generator> pairs — can list generators without a real table to
// build a seed plan against.
func (s *Server) handleSeedGeneratorsCatalog(c *gin.Context) {
	c.JSON(200, gin.H{
		"ok":   true,
		"data": gin.H{"generatorCatalog": seed.GeneratorCatalog()},
	})
}

// GET /api/seed/generators/:name/sample
func (s *Server) handleSeedGeneratorSample(c *gin.Context) {
	name := c.Param("name")

	meta, ok := seed.GeneratorMetaByName(name)
	if !ok {
		c.JSON(404, gin.H{
			"ok":    false,
			"error": gin.H{"code": "NOT_FOUND", "message": fmt.Sprintf("unknown generator: %s", name)},
		})
		return
	}

	if name == seed.ForeignKeyGeneratorName || name == "formula" || name == "enumFromColumn" {
		c.JSON(400, gin.H{
			"ok":    false,
			"error": gin.H{"code": "BAD_REQUEST", "message": fmt.Sprintf("generator %s cannot be previewed without table/row context", name)},
		})
		return
	}

	affinity := c.Query("affinity")
	if affinity == "" {
		if len(meta.Affinities) == 0 {
			c.JSON(400, gin.H{
				"ok":    false,
				"error": gin.H{"code": "BAD_REQUEST", "message": fmt.Sprintf("generator %s has no declared affinities", name)},
			})
			return
		}
		affinity = meta.Affinities[0]
	} else {
		found := false
		for _, a := range meta.Affinities {
			if a == affinity {
				found = true
				break
			}
		}
		if !found {
			c.JSON(400, gin.H{
				"ok":    false,
				"error": gin.H{"code": "BAD_REQUEST", "message": fmt.Sprintf("affinity %s is not valid for generator %s", affinity, name)},
			})
			return
		}
	}

	opts := map[string]any{}
	if raw := c.Query("options"); raw != "" {
		if err := json.Unmarshal([]byte(raw), &opts); err != nil {
			c.JSON(400, gin.H{
				"ok":    false,
				"error": gin.H{"code": "BAD_REQUEST", "message": fmt.Sprintf("malformed options JSON: %s", err.Error())},
			})
			return
		}
	}

	sample, err := seed.GenerateSample(name, affinity, opts)
	if err != nil {
		c.JSON(400, gin.H{
			"ok":    false,
			"error": gin.H{"code": "BAD_REQUEST", "message": err.Error()},
		})
		return
	}

	c.JSON(200, gin.H{
		"ok": true,
		"data": gin.H{
			"sample":       sample,
			"affinityUsed": affinity,
		},
	})
}

func isUniqueConstraintError(err error) bool {
	return strings.Contains(err.Error(), "UNIQUE constraint failed")
}
