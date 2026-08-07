package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/0funct0ry/squad/internal/db"
	"github.com/gin-gonic/gin"
)

// WriteGateMiddleware ensures that write operations require the --write flag.
func (s *Server) WriteGateMiddleware(op string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !s.write {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"ok": false,
				"error": gin.H{
					"code":    "READ_ONLY",
					"message": fmt.Sprintf("%s requires --write mode", op),
				},
			})
			return
		}
		c.Next()
	}
}

// POST /api/ddl
//
// Dropping an index or trigger from the Schema tab is intentionally routed
// through this general DDL path (`DROP INDEX "<name>"` / `DROP TRIGGER
// "<name>"`) rather than duplicated as bespoke table-CRUD-style endpoints —
// there's no row/schema state to resolve beyond executing the statement.
type DDLRequest struct {
	SQL string `json:"sql" binding:"required"`
}

func (s *Server) handlePostDDL(c *gin.Context) {
	var req DDLRequest
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.SQL) == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":    false,
			"error": gin.H{"code": "BAD_REQUEST", "message": "SQL statement is required"},
		})
		return
	}

	rawStatements, err := db.SplitStatements(req.SQL)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":    false,
			"error": gin.H{"code": "SQL_ERROR", "message": err.Error()},
		})
		return
	}

	var statements []string
	var originalIndices []int
	for idx, stmt := range rawStatements {
		clean, err := db.StripCommentsAndWhitespace(stmt)
		if err == nil && clean != "" {
			statements = append(statements, stmt)
			originalIndices = append(originalIndices, idx+1)
		}
	}

	if len(statements) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":    false,
			"error": gin.H{"code": "BAD_REQUEST", "message": "SQL statement is required"},
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

	var totalRowsAffected int64
	for idx, stmt := range statements {
		res, err := tx.Exec(stmt)
		if err != nil {
			msg := err.Error()
			if len(statements) > 1 {
				msg = fmt.Sprintf("statement %d: %s", originalIndices[idx], msg)
			}
			c.JSON(http.StatusBadRequest, gin.H{
				"ok":    false,
				"error": gin.H{"code": "SQL_ERROR", "message": msg},
			})
			return
		}
		affected, err := res.RowsAffected()
		if err == nil {
			totalRowsAffected += affected
		}
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
		"data": gin.H{"rowsAffected": totalRowsAffected},
	})
}

// POST /api/tables (structured create)
type CreateTableColumn struct {
	Name    string  `json:"name"`
	Type    string  `json:"type"`
	PK      bool    `json:"pk"`
	NotNull bool    `json:"notnull"`
	Unique  bool    `json:"unique"`
	Default *string `json:"default"`
}

type CreateTableForeignKey struct {
	Columns    []string `json:"columns"`
	RefTable   string   `json:"refTable"`
	RefColumns []string `json:"refColumns"`
	OnDelete   string   `json:"onDelete"`
	OnUpdate   string   `json:"onUpdate"`
	Match      string   `json:"match"`
}

type CreateTableRequest struct {
	Name        string                  `json:"name"`
	Columns     []CreateTableColumn     `json:"columns"`
	PrimaryKey  []string                `json:"primaryKey"`
	ForeignKeys []CreateTableForeignKey `json:"foreignKeys"`
}

var validFKActions = map[string]bool{
	"NO ACTION":   true,
	"RESTRICT":    true,
	"CASCADE":     true,
	"SET NULL":    true,
	"SET DEFAULT": true,
}

var validFKMatch = map[string]bool{
	"NONE":    true,
	"SIMPLE":  true,
	"PARTIAL": true,
	"FULL":    true,
}

// validateForeignKey checks a single foreign key entry against the local table's
// column set and the referenced table's schema, returning a descriptive error
// naming the specific validation failure, or nil if the entry is valid. It also
// returns the normalized (defaulted) onDelete/onUpdate/match values.
func validateForeignKey(fk CreateTableForeignKey, localColumns map[string]bool, refSchema *db.TableSchema) (onDelete, onUpdate, match string, err error) {
	if len(fk.Columns) == 0 || len(fk.RefColumns) == 0 {
		return "", "", "", fmt.Errorf("foreign key must specify at least one column and refColumns")
	}
	if len(fk.Columns) != len(fk.RefColumns) {
		return "", "", "", fmt.Errorf("foreign key columns and refColumns must be the same length")
	}

	seenLocal := make(map[string]bool)
	for _, c := range fk.Columns {
		lower := strings.ToLower(c)
		if seenLocal[lower] {
			return "", "", "", fmt.Errorf("duplicate column %q in foreign key", c)
		}
		seenLocal[lower] = true
		if !localColumns[lower] {
			return "", "", "", fmt.Errorf("foreign key column %q not found in table", c)
		}
	}

	if refSchema.Type == "view" {
		return "", "", "", fmt.Errorf("foreign key refTable %q must be a table, not a view", refSchema.Name)
	}

	refColMap := make(map[string]bool)
	for _, c := range refSchema.Columns {
		refColMap[strings.ToLower(c.Name)] = true
	}

	seenRef := make(map[string]bool)
	for _, c := range fk.RefColumns {
		lower := strings.ToLower(c)
		if seenRef[lower] {
			return "", "", "", fmt.Errorf("duplicate refColumn %q in foreign key", c)
		}
		seenRef[lower] = true
		if !refColMap[lower] {
			return "", "", "", fmt.Errorf("refColumn %q not found on table %q", c, refSchema.Name)
		}
	}

	// refColumns must be covered by a PK or a unique index on refTable
	pkSet := make(map[string]bool)
	for _, c := range refSchema.PrimaryKey {
		pkSet[strings.ToLower(c)] = true
	}
	coveredByPK := len(refSchema.PrimaryKey) == len(fk.RefColumns)
	if coveredByPK {
		for _, c := range fk.RefColumns {
			if !pkSet[strings.ToLower(c)] {
				coveredByPK = false
				break
			}
		}
	}

	coveredByUnique := false
	if !coveredByPK {
		for _, idx := range refSchema.Indexes {
			if !idx.Unique || len(idx.Columns) != len(fk.RefColumns) {
				continue
			}
			idxSet := make(map[string]bool)
			for _, c := range idx.Columns {
				idxSet[strings.ToLower(c)] = true
			}
			match := true
			for _, c := range fk.RefColumns {
				if !idxSet[strings.ToLower(c)] {
					match = false
					break
				}
			}
			if match {
				coveredByUnique = true
				break
			}
		}
	}

	if !coveredByPK && !coveredByUnique {
		return "", "", "", fmt.Errorf("refColumns %v on table %q must be covered by a primary key or unique constraint", fk.RefColumns, refSchema.Name)
	}

	onDelete = strings.ToUpper(strings.TrimSpace(fk.OnDelete))
	if onDelete == "" {
		onDelete = "NO ACTION"
	}
	if !validFKActions[onDelete] {
		return "", "", "", fmt.Errorf("invalid onDelete value: %q", fk.OnDelete)
	}

	onUpdate = strings.ToUpper(strings.TrimSpace(fk.OnUpdate))
	if onUpdate == "" {
		onUpdate = "NO ACTION"
	}
	if !validFKActions[onUpdate] {
		return "", "", "", fmt.Errorf("invalid onUpdate value: %q", fk.OnUpdate)
	}

	match = strings.ToUpper(strings.TrimSpace(fk.Match))
	if match == "" {
		match = "NONE"
	}
	if !validFKMatch[match] {
		return "", "", "", fmt.Errorf("invalid match value: %q", fk.Match)
	}

	return onDelete, onUpdate, match, nil
}

// buildForeignKeyClause renders a validated foreign key entry as a table-level
// FOREIGN KEY (...) REFERENCES ... clause, omitting MATCH when it's the default NONE.
func buildForeignKeyClause(fk CreateTableForeignKey, onDelete, onUpdate, match string) string {
	var localCols, refCols []string
	for _, c := range fk.Columns {
		localCols = append(localCols, db.QuoteIdentifier(c))
	}
	for _, c := range fk.RefColumns {
		refCols = append(refCols, db.QuoteIdentifier(c))
	}
	clause := fmt.Sprintf("FOREIGN KEY (%s) REFERENCES %s (%s) ON DELETE %s ON UPDATE %s",
		strings.Join(localCols, ", "), db.QuoteIdentifier(fk.RefTable), strings.Join(refCols, ", "), onDelete, onUpdate)
	if match != "NONE" {
		clause += " MATCH " + match
	}
	return clause
}

func (s *Server) handleCreateTable(c *gin.Context) {
	var req CreateTableRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":    false,
			"error": gin.H{"code": "BAD_REQUEST", "message": err.Error()},
		})
		return
	}

	if strings.TrimSpace(req.Name) == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":    false,
			"error": gin.H{"code": "BAD_REQUEST", "message": "table name is required"},
		})
		return
	}

	if len(req.Columns) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":    false,
			"error": gin.H{"code": "BAD_REQUEST", "message": "at least one column is required"},
		})
		return
	}

	seenCols := make(map[string]bool)
	pkCount := 0
	for _, col := range req.Columns {
		if strings.TrimSpace(col.Name) == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"ok":    false,
				"error": gin.H{"code": "BAD_REQUEST", "message": "column name cannot be empty"},
			})
			return
		}
		lowerName := strings.ToLower(col.Name)
		if seenCols[lowerName] {
			c.JSON(http.StatusBadRequest, gin.H{
				"ok":    false,
				"error": gin.H{"code": "BAD_REQUEST", "message": fmt.Sprintf("duplicate column name: %s", col.Name)},
			})
			return
		}
		seenCols[lowerName] = true

		if strings.TrimSpace(col.Type) == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"ok":    false,
				"error": gin.H{"code": "BAD_REQUEST", "message": fmt.Sprintf("column type is required for column: %s", col.Name)},
			})
			return
		}

		if col.PK {
			pkCount++
		}
	}

	if pkCount > 0 && len(req.PrimaryKey) > 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":    false,
			"error": gin.H{"code": "BAD_REQUEST", "message": "ambiguous primary key: both per-column pk and top-level primaryKey provided"},
		})
		return
	}

	if pkCount > 1 {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":    false,
			"error": gin.H{"code": "BAD_REQUEST", "message": "multiple columns marked as primary key; use primaryKey array for composite keys"},
		})
		return
	}

	if len(req.PrimaryKey) > 0 {
		for _, pkCol := range req.PrimaryKey {
			if !seenCols[strings.ToLower(pkCol)] {
				c.JSON(http.StatusBadRequest, gin.H{
					"ok":    false,
					"error": gin.H{"code": "BAD_REQUEST", "message": fmt.Sprintf("primary key column %q not found in columns", pkCol)},
				})
				return
			}
		}
	}

	// Check if table or view already exists
	var exists bool
	err := s.db.QueryRow("SELECT EXISTS(SELECT 1 FROM sqlite_master WHERE name = ?)", req.Name).Scan(&exists)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"ok":    false,
			"error": gin.H{"code": "DB_ERROR", "message": err.Error()},
		})
		return
	}
	if exists {
		c.JSON(http.StatusConflict, gin.H{
			"ok":    false,
			"error": gin.H{"code": "ALREADY_EXISTS", "message": fmt.Sprintf("table or view %q already exists", req.Name)},
		})
		return
	}

	// Build DDL
	var colDefs []string
	for _, col := range req.Columns {
		def := fmt.Sprintf("%s %s", db.QuoteIdentifier(col.Name), col.Type)
		if col.PK && len(req.PrimaryKey) == 0 {
			def += " PRIMARY KEY"
		}
		if col.NotNull {
			def += " NOT NULL"
		}
		if col.Unique {
			def += " UNIQUE"
		}
		if col.Default != nil {
			def += " DEFAULT " + *col.Default
		}
		colDefs = append(colDefs, def)
	}

	if len(req.PrimaryKey) > 0 {
		var pkParts []string
		for _, pkc := range req.PrimaryKey {
			pkParts = append(pkParts, db.QuoteIdentifier(pkc))
		}
		colDefs = append(colDefs, fmt.Sprintf("PRIMARY KEY (%s)", strings.Join(pkParts, ", ")))
	}

	if len(req.ForeignKeys) > 0 {
		var selfPK []string
		if len(req.PrimaryKey) > 0 {
			selfPK = req.PrimaryKey
		} else {
			for _, col := range req.Columns {
				if col.PK {
					selfPK = append(selfPK, col.Name)
				}
			}
		}
		selfSchema := &db.TableSchema{Name: req.Name, Type: "table", PrimaryKey: selfPK}
		for _, col := range req.Columns {
			selfSchema.Columns = append(selfSchema.Columns, db.ColumnInfo{Name: col.Name})
			if col.Unique {
				selfSchema.Indexes = append(selfSchema.Indexes, db.IndexInfo{Unique: true, Columns: []string{col.Name}})
			}
		}

		for i, fk := range req.ForeignKeys {
			var refSchema *db.TableSchema
			if strings.EqualFold(fk.RefTable, req.Name) {
				refSchema = selfSchema
			} else {
				refSchema, err = db.GetTableSchema(s.db, fk.RefTable)
				if err != nil {
					if errors.Is(err, db.ErrNotFound) {
						c.JSON(http.StatusBadRequest, gin.H{
							"ok":    false,
							"error": gin.H{"code": "BAD_REQUEST", "message": fmt.Sprintf("foreign key %d: refTable %q does not exist", i, fk.RefTable)},
						})
						return
					}
					c.JSON(http.StatusInternalServerError, gin.H{
						"ok":    false,
						"error": gin.H{"code": "DB_ERROR", "message": err.Error()},
					})
					return
				}
			}

			onDelete, onUpdate, match, verr := validateForeignKey(fk, seenCols, refSchema)
			if verr != nil {
				c.JSON(http.StatusBadRequest, gin.H{
					"ok":    false,
					"error": gin.H{"code": "BAD_REQUEST", "message": fmt.Sprintf("foreign key %d: %s", i, verr.Error())},
				})
				return
			}
			colDefs = append(colDefs, buildForeignKeyClause(fk, onDelete, onUpdate, match))
		}
	}

	query := fmt.Sprintf("CREATE TABLE %s (%s)", db.QuoteIdentifier(req.Name), strings.Join(colDefs, ", "))
	if _, err := s.db.Exec(query); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":    false,
			"error": gin.H{"code": "SQL_ERROR", "message": err.Error()},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"ok":   true,
		"data": gin.H{"name": req.Name},
	})
}

// PATCH /api/tables/:name (alter table)
type AlterTableRequest struct {
	Op         string          `json:"op"`
	NewName    string          `json:"newName,omitempty"`
	Column     json.RawMessage `json:"column,omitempty"`
	From       string          `json:"from,omitempty"`
	To         string          `json:"to,omitempty"`
	ForeignKey json.RawMessage `json:"foreignKey,omitempty"`
}

type DropForeignKeyData struct {
	ID int `json:"id"`
}

// renderExistingForeignKeys renders a table's current foreign keys (grouped by
// their shared PRAGMA foreign_key_list id) back into table-level FOREIGN KEY
// clauses, optionally excluding one id (used when dropping a foreign key).
func renderExistingForeignKeys(fks []db.ForeignKeyInfo, excludeID int, hasExclude bool) []string {
	groups := make(map[int][]db.ForeignKeyInfo)
	var order []int
	for _, fk := range fks {
		if hasExclude && fk.ID == excludeID {
			continue
		}
		if _, ok := groups[fk.ID]; !ok {
			order = append(order, fk.ID)
		}
		groups[fk.ID] = append(groups[fk.ID], fk)
	}

	var clauses []string
	for _, id := range order {
		rows := groups[id]
		sort.Slice(rows, func(i, j int) bool { return rows[i].Seq < rows[j].Seq })
		var from, to []string
		for _, r := range rows {
			from = append(from, db.QuoteIdentifier(r.From))
			to = append(to, db.QuoteIdentifier(r.To))
		}
		first := rows[0]
		clause := fmt.Sprintf("FOREIGN KEY (%s) REFERENCES %s (%s) ON DELETE %s ON UPDATE %s",
			strings.Join(from, ", "), db.QuoteIdentifier(first.Table), strings.Join(to, ", "), first.OnDelete, first.OnUpdate)
		if first.Match != "" && !strings.EqualFold(first.Match, "NONE") {
			clause += " MATCH " + strings.ToUpper(first.Match)
		}
		clauses = append(clauses, clause)
	}
	return clauses
}

// runForeignKeyRebuild rebuilds a table in place with an explicit set of
// table-level foreign key clauses, preserving all columns, the primary key,
// and recreating every existing index and trigger. Used by add_foreign_key
// and drop_foreign_key, which both need to add/remove a table-level
// constraint that SQLite's ALTER TABLE cannot express directly.
func runForeignKeyRebuild(s *Server, name string, schema *db.TableSchema, fkClauses []string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var colDefs []string
	for _, col := range schema.Columns {
		def := fmt.Sprintf("%s %s", db.QuoteIdentifier(col.Name), col.Type)
		if col.NotNull {
			def += " NOT NULL"
		}
		if col.DefaultVal != nil {
			def += " DEFAULT " + *col.DefaultVal
		}
		colDefs = append(colDefs, def)
	}
	if len(schema.PrimaryKey) > 0 {
		var pkParts []string
		for _, pkc := range schema.PrimaryKey {
			pkParts = append(pkParts, db.QuoteIdentifier(pkc))
		}
		colDefs = append(colDefs, fmt.Sprintf("PRIMARY KEY (%s)", strings.Join(pkParts, ", ")))
	}
	colDefs = append(colDefs, fkClauses...)

	rebuildTable := name + "__rebuild"
	createSQL := fmt.Sprintf("CREATE TABLE %s (%s)", db.QuoteIdentifier(rebuildTable), strings.Join(colDefs, ", "))
	if schema.WithoutRowid {
		createSQL += " WITHOUT ROWID"
	}
	if _, err := tx.Exec(createSQL); err != nil {
		return fmt.Errorf("failed to create rebuild table: %w", err)
	}

	var cols []string
	for _, col := range schema.Columns {
		cols = append(cols, db.QuoteIdentifier(col.Name))
	}
	colsJoined := strings.Join(cols, ", ")
	copySQL := fmt.Sprintf("INSERT INTO %s (%s) SELECT %s FROM %s",
		db.QuoteIdentifier(rebuildTable), colsJoined, colsJoined, db.QuoteIdentifier(name))
	if _, err := tx.Exec(copySQL); err != nil {
		return fmt.Errorf("failed to copy data to rebuild table: %w", err)
	}

	if _, err := tx.Exec(fmt.Sprintf("DROP TABLE %s", db.QuoteIdentifier(name))); err != nil {
		return fmt.Errorf("failed to drop original table: %w", err)
	}

	if _, err := tx.Exec(fmt.Sprintf("ALTER TABLE %s RENAME TO %s", db.QuoteIdentifier(rebuildTable), db.QuoteIdentifier(name))); err != nil {
		return fmt.Errorf("failed to rename rebuild table: %w", err)
	}

	for _, idx := range schema.Indexes {
		if idx.SQL != nil {
			if _, err := tx.Exec(*idx.SQL); err != nil {
				return fmt.Errorf("failed to recreate index %q: %w", idx.Name, err)
			}
		}
	}

	for _, trg := range schema.Triggers {
		if _, err := tx.Exec(trg.SQL); err != nil {
			return fmt.Errorf("failed to recreate trigger %q: %w", trg.Name, err)
		}
	}

	return tx.Commit()
}

type AddColumnData struct {
	Name    string  `json:"name"`
	Type    string  `json:"type"`
	NotNull bool    `json:"notnull"`
	Default *string `json:"default"`
}

func (s *Server) handleAlterTable(c *gin.Context) {
	name := c.Param("name")

	// Verify table exists
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

	var req AlterTableRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":    false,
			"error": gin.H{"code": "BAD_REQUEST", "message": err.Error()},
		})
		return
	}

	switch req.Op {
	case "rename_table":
		if strings.TrimSpace(req.NewName) == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"ok":    false,
				"error": gin.H{"code": "BAD_REQUEST", "message": "newName is required"},
			})
			return
		}
		var exists bool
		err := s.db.QueryRow("SELECT EXISTS(SELECT 1 FROM sqlite_master WHERE name = ?)", req.NewName).Scan(&exists)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"ok":    false,
				"error": gin.H{"code": "DB_ERROR", "message": err.Error()},
			})
			return
		}
		if exists {
			c.JSON(http.StatusConflict, gin.H{
				"ok":    false,
				"error": gin.H{"code": "ALREADY_EXISTS", "message": fmt.Sprintf("table or view %q already exists", req.NewName)},
			})
			return
		}

		query := fmt.Sprintf("ALTER TABLE %s RENAME TO %s", db.QuoteIdentifier(name), db.QuoteIdentifier(req.NewName))
		if _, err := s.db.Exec(query); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"ok":    false,
				"error": gin.H{"code": "SQL_ERROR", "message": err.Error()},
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"ok":   true,
			"data": gin.H{"name": req.NewName},
		})
		return

	case "add_column":
		var colData AddColumnData
		if err := json.Unmarshal(req.Column, &colData); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"ok":    false,
				"error": gin.H{"code": "BAD_REQUEST", "message": "invalid column definition"},
			})
			return
		}
		if strings.TrimSpace(colData.Name) == "" || strings.TrimSpace(colData.Type) == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"ok":    false,
				"error": gin.H{"code": "BAD_REQUEST", "message": "column name and type are required"},
			})
			return
		}

		query := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", db.QuoteIdentifier(name), db.QuoteIdentifier(colData.Name), colData.Type)
		if colData.NotNull {
			query += " NOT NULL"
		}
		if colData.Default != nil {
			query += " DEFAULT " + *colData.Default
		}

		if _, err := s.db.Exec(query); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"ok":    false,
				"error": gin.H{"code": "SQL_ERROR", "message": err.Error()},
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"ok":   true,
			"data": gin.H{"name": name},
		})
		return

	case "rename_column":
		if strings.TrimSpace(req.From) == "" || strings.TrimSpace(req.To) == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"ok":    false,
				"error": gin.H{"code": "BAD_REQUEST", "message": "from and to column names are required"},
			})
			return
		}

		query := fmt.Sprintf("ALTER TABLE %s RENAME COLUMN %s TO %s", db.QuoteIdentifier(name), db.QuoteIdentifier(req.From), db.QuoteIdentifier(req.To))
		if _, err := s.db.Exec(query); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"ok":    false,
				"error": gin.H{"code": "SQL_ERROR", "message": err.Error()},
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"ok":   true,
			"data": gin.H{"name": name},
		})
		return

	case "drop_column":
		var colName string
		if err := json.Unmarshal(req.Column, &colName); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"ok":    false,
				"error": gin.H{"code": "BAD_REQUEST", "message": "invalid column name format"},
			})
			return
		}

		if strings.TrimSpace(colName) == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"ok":    false,
				"error": gin.H{"code": "BAD_REQUEST", "message": "column name to drop is required"},
			})
			return
		}

		// 1. Reject if it is the last remaining column
		if len(schema.Columns) <= 1 {
			c.JSON(http.StatusBadRequest, gin.H{
				"ok":    false,
				"error": gin.H{"code": "BAD_REQUEST", "message": "cannot drop the last remaining column"},
			})
			return
		}

		// 2. Reject if another table has an FK referencing this column
		tables, err := db.GetTables(s.db)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"ok":    false,
				"error": gin.H{"code": "DB_ERROR", "message": err.Error()},
			})
			return
		}
		for _, t := range tables {
			if strings.EqualFold(t.Name, name) {
				continue
			}
			tSchema, err := db.GetTableSchema(s.db, t.Name)
			if err != nil {
				continue
			}
			for _, fk := range tSchema.ForeignKeys {
				if strings.EqualFold(fk.Table, name) && strings.EqualFold(fk.To, colName) {
					c.JSON(http.StatusBadRequest, gin.H{
						"ok":    false,
						"error": gin.H{"code": "BAD_REQUEST", "message": fmt.Sprintf("cannot drop column %q because table %q has a foreign key referencing it", colName, t.Name)},
					})
					return
				}
			}
		}

		// 3. Determine if column participates in PK, local FK, or Index
		participates := false
		for _, pkCol := range schema.PrimaryKey {
			if strings.EqualFold(pkCol, colName) {
				participates = true
				break
			}
		}
		for _, fk := range schema.ForeignKeys {
			if strings.EqualFold(fk.From, colName) {
				participates = true
				break
			}
		}
		for _, idx := range schema.Indexes {
			for _, cName := range idx.Columns {
				if strings.EqualFold(cName, colName) {
					participates = true
					break
				}
			}
		}

		var warnings []string

		// Rebuild pattern or native drop column
		runRebuildPattern := func() error {
			tx, err := s.db.Begin()
			if err != nil {
				return err
			}
			defer tx.Rollback()

			// Build create table query for rebuild
			var colDefs []string
			var survivingPKs []string
			for _, col := range schema.Columns {
				if strings.EqualFold(col.Name, colName) {
					continue
				}
				def := fmt.Sprintf("%s %s", db.QuoteIdentifier(col.Name), col.Type)
				if col.NotNull {
					def += " NOT NULL"
				}
				if col.DefaultVal != nil {
					def += " DEFAULT " + *col.DefaultVal
				}
				colDefs = append(colDefs, def)

				if col.PK > 0 {
					survivingPKs = append(survivingPKs, col.Name)
				}
			}

			if len(survivingPKs) > 0 {
				var pkParts []string
				for _, pkc := range survivingPKs {
					pkParts = append(pkParts, db.QuoteIdentifier(pkc))
				}
				colDefs = append(colDefs, fmt.Sprintf("PRIMARY KEY (%s)", strings.Join(pkParts, ", ")))
			}

			rebuildTable := name + "__rebuild"
			createSQL := fmt.Sprintf("CREATE TABLE %s (%s)", db.QuoteIdentifier(rebuildTable), strings.Join(colDefs, ", "))
			if schema.WithoutRowid {
				createSQL += " WITHOUT ROWID"
			}

			if _, err := tx.Exec(createSQL); err != nil {
				return fmt.Errorf("failed to create rebuild table: %w", err)
			}

			// Copy data
			var survivingCols []string
			for _, col := range schema.Columns {
				if !strings.EqualFold(col.Name, colName) {
					survivingCols = append(survivingCols, db.QuoteIdentifier(col.Name))
				}
			}
			colsJoined := strings.Join(survivingCols, ", ")
			copySQL := fmt.Sprintf("INSERT INTO %s (%s) SELECT %s FROM %s",
				db.QuoteIdentifier(rebuildTable), colsJoined, colsJoined, db.QuoteIdentifier(name))

			if _, err := tx.Exec(copySQL); err != nil {
				return fmt.Errorf("failed to copy data to rebuild table: %w", err)
			}

			// Drop original table
			if _, err := tx.Exec(fmt.Sprintf("DROP TABLE %s", db.QuoteIdentifier(name))); err != nil {
				return fmt.Errorf("failed to drop original table: %w", err)
			}

			// Rename rebuild
			if _, err := tx.Exec(fmt.Sprintf("ALTER TABLE %s RENAME TO %s", db.QuoteIdentifier(rebuildTable), db.QuoteIdentifier(name))); err != nil {
				return fmt.Errorf("failed to rename rebuild table: %w", err)
			}

			// Recreate indexes
			for _, idx := range schema.Indexes {
				hasDroppedCol := false
				for _, cName := range idx.Columns {
					if strings.EqualFold(cName, colName) {
						hasDroppedCol = true
						break
					}
				}
				if hasDroppedCol {
					warnings = append(warnings, fmt.Sprintf("Index %q dropped because it referenced dropped column %q", idx.Name, colName))
					continue
				}
				if idx.SQL != nil {
					if _, err := tx.Exec(*idx.SQL); err != nil {
						// treated as warning
						warnings = append(warnings, fmt.Sprintf("Failed to recreate index %q: %s", idx.Name, err.Error()))
					}
				}
			}

			return tx.Commit()
		}

		if participates {
			if err := runRebuildPattern(); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{
					"ok":    false,
					"error": gin.H{"code": "SQL_ERROR", "message": err.Error()},
				})
				return
			}
		} else {
			// Try native ALTER TABLE DROP COLUMN
			nativeQuery := fmt.Sprintf("ALTER TABLE %s DROP COLUMN %s", db.QuoteIdentifier(name), db.QuoteIdentifier(colName))
			if _, err := s.db.Exec(nativeQuery); err != nil {
				// Fall back to rebuild pattern
				if err := runRebuildPattern(); err != nil {
					c.JSON(http.StatusBadRequest, gin.H{
						"ok":    false,
						"error": gin.H{"code": "SQL_ERROR", "message": err.Error()},
					})
					return
				}
			}
		}

		resData := gin.H{"name": name}
		if len(warnings) > 0 {
			resData["warnings"] = warnings
		}

		c.JSON(http.StatusOK, gin.H{
			"ok":   true,
			"data": resData,
		})
		return

	case "add_foreign_key":
		var fk CreateTableForeignKey
		if err := json.Unmarshal(req.ForeignKey, &fk); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"ok":    false,
				"error": gin.H{"code": "BAD_REQUEST", "message": "invalid foreign key definition"},
			})
			return
		}

		localColumns := make(map[string]bool)
		for _, col := range schema.Columns {
			localColumns[strings.ToLower(col.Name)] = true
		}

		var refSchema *db.TableSchema
		if strings.EqualFold(fk.RefTable, name) {
			refSchema = schema
		} else {
			refSchema, err = db.GetTableSchema(s.db, fk.RefTable)
			if err != nil {
				if errors.Is(err, db.ErrNotFound) {
					c.JSON(http.StatusBadRequest, gin.H{
						"ok":    false,
						"error": gin.H{"code": "BAD_REQUEST", "message": fmt.Sprintf("refTable %q does not exist", fk.RefTable)},
					})
					return
				}
				c.JSON(http.StatusInternalServerError, gin.H{
					"ok":    false,
					"error": gin.H{"code": "DB_ERROR", "message": err.Error()},
				})
				return
			}
		}

		onDelete, onUpdate, match, verr := validateForeignKey(fk, localColumns, refSchema)
		if verr != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"ok":    false,
				"error": gin.H{"code": "BAD_REQUEST", "message": verr.Error()},
			})
			return
		}

		// Pre-check: existing data must not already violate the proposed FK.
		var localParts, joinParts, nullChecks []string
		for i := range fk.Columns {
			lc := db.QuoteIdentifier(fk.Columns[i])
			rc := db.QuoteIdentifier(fk.RefColumns[i])
			localParts = append(localParts, "t."+lc)
			joinParts = append(joinParts, fmt.Sprintf("t.%s = r.%s", lc, rc))
			nullChecks = append(nullChecks, fmt.Sprintf("t.%s IS NOT NULL", lc))
		}
		antiJoinQuery := fmt.Sprintf(
			"SELECT COUNT(*) FROM %s t LEFT JOIN %s r ON %s WHERE %s AND r.%s IS NULL",
			db.QuoteIdentifier(name), db.QuoteIdentifier(fk.RefTable), strings.Join(joinParts, " AND "),
			strings.Join(nullChecks, " AND "), db.QuoteIdentifier(fk.RefColumns[0]),
		)
		var violationCount int64
		if err := s.db.QueryRow(antiJoinQuery).Scan(&violationCount); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"ok":    false,
				"error": gin.H{"code": "DB_ERROR", "message": err.Error()},
			})
			return
		}
		if violationCount > 0 {
			sampleQuery := fmt.Sprintf(
				"SELECT %s FROM %s t LEFT JOIN %s r ON %s WHERE %s AND r.%s IS NULL LIMIT 5",
				strings.Join(localParts, ", "), db.QuoteIdentifier(name), db.QuoteIdentifier(fk.RefTable),
				strings.Join(joinParts, " AND "), strings.Join(nullChecks, " AND "), db.QuoteIdentifier(fk.RefColumns[0]),
			)
			sampleRows, err := s.db.Query(sampleQuery)
			var samples []map[string]interface{}
			if err == nil {
				defer sampleRows.Close()
				for sampleRows.Next() {
					vals := make([]interface{}, len(fk.Columns))
					ptrs := make([]interface{}, len(fk.Columns))
					for i := range vals {
						ptrs[i] = &vals[i]
					}
					if sampleRows.Scan(ptrs...) == nil {
						row := make(map[string]interface{})
						for i, colName := range fk.Columns {
							row[colName] = vals[i]
						}
						samples = append(samples, row)
					}
				}
			}
			c.JSON(http.StatusConflict, gin.H{
				"ok": false,
				"error": gin.H{
					"code":    "FK_VIOLATION",
					"message": fmt.Sprintf("%d existing row(s) in %q would violate the proposed foreign key", violationCount, name),
				},
				"data": gin.H{"violatingRowCount": violationCount, "sampleKeys": samples},
			})
			return
		}

		existingClauses := renderExistingForeignKeys(schema.ForeignKeys, 0, false)
		newClause := buildForeignKeyClause(fk, onDelete, onUpdate, match)
		allClauses := append(existingClauses, newClause)

		if err := runForeignKeyRebuild(s, name, schema, allClauses); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"ok":    false,
				"error": gin.H{"code": "SQL_ERROR", "message": err.Error()},
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"ok":   true,
			"data": gin.H{"name": name},
		})
		return

	case "drop_foreign_key":
		var dropData DropForeignKeyData
		if err := json.Unmarshal(req.ForeignKey, &dropData); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"ok":    false,
				"error": gin.H{"code": "BAD_REQUEST", "message": "invalid foreign key id"},
			})
			return
		}

		found := false
		for _, fk := range schema.ForeignKeys {
			if fk.ID == dropData.ID {
				found = true
				break
			}
		}
		if !found {
			c.JSON(http.StatusBadRequest, gin.H{
				"ok":    false,
				"error": gin.H{"code": "BAD_REQUEST", "message": fmt.Sprintf("no such foreign key id %d", dropData.ID)},
			})
			return
		}

		remainingClauses := renderExistingForeignKeys(schema.ForeignKeys, dropData.ID, true)
		if err := runForeignKeyRebuild(s, name, schema, remainingClauses); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"ok":    false,
				"error": gin.H{"code": "SQL_ERROR", "message": err.Error()},
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"ok":   true,
			"data": gin.H{"name": name},
		})
		return

	default:
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":    false,
			"error": gin.H{"code": "BAD_REQUEST", "message": fmt.Sprintf("unsupported operation: %s", req.Op)},
		})
	}
}

// DELETE /api/tables/:name (drop table or view)
func (s *Server) handleDropTable(c *gin.Context) {
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

	var ddl string
	if strings.EqualFold(schema.Type, "view") {
		ddl = fmt.Sprintf("DROP VIEW %s", db.QuoteIdentifier(name))
	} else {
		ddl = fmt.Sprintf("DROP TABLE %s", db.QuoteIdentifier(name))
	}

	if _, err := s.db.Exec(ddl); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":    false,
			"error": gin.H{"code": "SQL_ERROR", "message": err.Error()},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"ok":   true,
		"data": gin.H{"name": name},
	})
}

// POST /api/tables/:name/duplicate
//
// Uses CREATE TABLE ... AS SELECT, which only copies columns and data — it
// intentionally does not preserve the original's indexes, constraints, or
// triggers (SQLite's CTAS semantics). This is a deliberate simplification
// over extracting and rewriting the original DDL; the frontend's copy makes
// the limitation explicit to the user.
type DuplicateTableRequest struct {
	NewName     string `json:"newName" binding:"required"`
	IncludeData bool   `json:"includeData"`
}

func (s *Server) handleDuplicateTable(c *gin.Context) {
	name := c.Param("name")

	if _, err := db.GetTableSchema(s.db, name); err != nil {
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

	var req DuplicateTableRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":    false,
			"error": gin.H{"code": "BAD_REQUEST", "message": err.Error()},
		})
		return
	}
	if strings.TrimSpace(req.NewName) == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":    false,
			"error": gin.H{"code": "BAD_REQUEST", "message": "newName is required"},
		})
		return
	}

	var exists bool
	if err := s.db.QueryRow("SELECT EXISTS(SELECT 1 FROM sqlite_master WHERE name = ?)", req.NewName).Scan(&exists); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"ok":    false,
			"error": gin.H{"code": "DB_ERROR", "message": err.Error()},
		})
		return
	}
	if exists {
		c.JSON(http.StatusConflict, gin.H{
			"ok":    false,
			"error": gin.H{"code": "ALREADY_EXISTS", "message": fmt.Sprintf("table or view %q already exists", req.NewName)},
		})
		return
	}

	selectClause := fmt.Sprintf("SELECT * FROM %s", db.QuoteIdentifier(name))
	if !req.IncludeData {
		selectClause += " WHERE 0"
	}
	query := fmt.Sprintf("CREATE TABLE %s AS %s", db.QuoteIdentifier(req.NewName), selectClause)
	if _, err := s.db.Exec(query); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":    false,
			"error": gin.H{"code": "SQL_ERROR", "message": err.Error()},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"ok":   true,
		"data": gin.H{"name": req.NewName},
	})
}

// POST /api/tables/:name/rows (insert row)
type InsertRowRequest struct {
	Values map[string]interface{} `json:"values" binding:"required"`
}

func (s *Server) handleInsertRow(c *gin.Context) {
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

	var req InsertRowRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":    false,
			"error": gin.H{"code": "BAD_REQUEST", "message": err.Error()},
		})
		return
	}

	if len(req.Values) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":    false,
			"error": gin.H{"code": "BAD_REQUEST", "message": "values are required"},
		})
		return
	}

	// Verify column names against table columns
	colMap := make(map[string]bool)
	for _, col := range schema.Columns {
		colMap[col.Name] = true
	}

	var cols []string
	var placeHolders []string
	var args []interface{}
	for k, v := range req.Values {
		if !colMap[k] {
			c.JSON(http.StatusBadRequest, gin.H{
				"ok":    false,
				"error": gin.H{"code": "BAD_REQUEST", "message": fmt.Sprintf("unknown column: %s", k)},
			})
			return
		}
		cols = append(cols, db.QuoteIdentifier(k))
		placeHolders = append(placeHolders, "?")
		args = append(args, v)
	}

	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
		db.QuoteIdentifier(name), strings.Join(cols, ", "), strings.Join(placeHolders, ", "))

	res, err := s.db.Exec(query, args...)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":    false,
			"error": gin.H{"code": "SQL_ERROR", "message": err.Error()},
		})
		return
	}

	var lastID interface{} = nil
	if !schema.WithoutRowid {
		if id, err := res.LastInsertId(); err == nil {
			lastID = id
		}
	}

	affected, _ := res.RowsAffected()

	c.JSON(http.StatusOK, gin.H{
		"ok": true,
		"data": gin.H{
			"rowsAffected":    affected,
			"lastInsertRowid": lastID,
		},
	})
}

// PATCH /api/tables/:name/rows (update row)
type UpdateRowRequest struct {
	Key    map[string]interface{} `json:"key" binding:"required"`
	Values map[string]interface{} `json:"values" binding:"required"`
}

func (s *Server) handleUpdateRow(c *gin.Context) {
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

	var req UpdateRowRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":    false,
			"error": gin.H{"code": "BAD_REQUEST", "message": err.Error()},
		})
		return
	}

	if len(req.Key) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":    false,
			"error": gin.H{"code": "BAD_REQUEST", "message": "key is required"},
		})
		return
	}

	if len(req.Values) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":    false,
			"error": gin.H{"code": "BAD_REQUEST", "message": "values are required"},
		})
		return
	}

	// Validate key requirements
	var whereParts []string
	var args []interface{}
	useRowid := false

	if len(schema.PrimaryKey) == 0 && !schema.WithoutRowid {
		// Normal rowid table with no explicit PK -> require rowid
		rowidVal, ok := req.Key["rowid"]
		if !ok {
			c.JSON(http.StatusBadRequest, gin.H{
				"ok":    false,
				"error": gin.H{"code": "BAD_REQUEST", "message": "table has no primary key; key must specify 'rowid'"},
			})
			return
		}
		whereParts = append(whereParts, "rowid = ?")
		args = append(args, rowidVal)
		useRowid = true
	} else {
		// Check that ALL PK columns are present in the key
		for _, pkCol := range schema.PrimaryKey {
			val, ok := req.Key[pkCol]
			if !ok {
				c.JSON(http.StatusBadRequest, gin.H{
					"ok":    false,
					"error": gin.H{"code": "BAD_REQUEST", "message": fmt.Sprintf("missing primary key column %q in update key", pkCol)},
				})
				return
			}
			whereParts = append(whereParts, fmt.Sprintf("%s = ?", db.QuoteIdentifier(pkCol)))
			args = append(args, val)
		}
	}

	// Verify update values column names
	colMap := make(map[string]bool)
	for _, col := range schema.Columns {
		colMap[col.Name] = true
	}

	var updateParts []string
	var updateArgs []interface{}
	for k, v := range req.Values {
		if !colMap[k] {
			c.JSON(http.StatusBadRequest, gin.H{
				"ok":    false,
				"error": gin.H{"code": "BAD_REQUEST", "message": fmt.Sprintf("unknown column: %s", k)},
			})
			return
		}
		updateParts = append(updateParts, fmt.Sprintf("%s = ?", db.QuoteIdentifier(k)))
		updateArgs = append(updateArgs, v)
	}

	// Begin transaction to protect against ambiguous keys safely
	tx, err := s.db.Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"ok":    false,
			"error": gin.H{"code": "DB_ERROR", "message": err.Error()},
		})
		return
	}
	defer tx.Rollback()

	// Count rows matching the key first
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE %s",
		db.QuoteIdentifier(name), strings.Join(whereParts, " AND "))
	var count int
	if err := tx.QueryRow(countQuery, args...).Scan(&count); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":    false,
			"error": gin.H{"code": "SQL_ERROR", "message": err.Error()},
		})
		return
	}

	if count == 0 {
		c.JSON(http.StatusNotFound, gin.H{
			"ok":    false,
			"error": gin.H{"code": "NOT_FOUND", "message": "row not found"},
		})
		return
	}

	if count > 1 {
		c.JSON(http.StatusConflict, gin.H{
			"ok":    false,
			"error": gin.H{"code": "AMBIGUOUS_KEY", "message": "update matches multiple rows"},
		})
		return
	}

	// Perform UPDATE
	updateSQL := fmt.Sprintf("UPDATE %s SET %s WHERE %s",
		db.QuoteIdentifier(name), strings.Join(updateParts, ", "), strings.Join(whereParts, " AND "))
	finalArgs := append(updateArgs, args...)

	if _, err := tx.Exec(updateSQL, finalArgs...); err != nil {
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
		"data": gin.H{"rowsAffected": 1, "useRowid": useRowid},
	})
}

// DELETE /api/tables/:name/rows (delete row)
type DeleteRowRequest struct {
	Key map[string]interface{} `json:"key" binding:"required"`
}

// buildRowWhereClause builds the parameterized WHERE clause used to address
// a single row by its primary key (or "rowid" for rowid tables with no
// explicit PK), mirroring the key-resolution rules used across row
// update/delete. Returns useRowid=true when the "rowid" fallback was used.
func buildRowWhereClause(schema *db.TableSchema, key map[string]interface{}) (whereSQL string, args []interface{}, useRowid bool, err error) {
	var whereParts []string

	if len(schema.PrimaryKey) == 0 && !schema.WithoutRowid {
		// Normal rowid table with no explicit PK -> require rowid
		rowidVal, ok := key["rowid"]
		if !ok {
			return "", nil, false, fmt.Errorf("table has no primary key; key must specify 'rowid'")
		}
		whereParts = append(whereParts, "rowid = ?")
		args = append(args, rowidVal)
		useRowid = true
	} else {
		// Check that ALL PK columns are present in the key
		for _, pkCol := range schema.PrimaryKey {
			val, ok := key[pkCol]
			if !ok {
				return "", nil, false, fmt.Errorf("missing primary key column %q in delete key", pkCol)
			}
			whereParts = append(whereParts, fmt.Sprintf("%s = ?", db.QuoteIdentifier(pkCol)))
			args = append(args, val)
		}
	}

	return strings.Join(whereParts, " AND "), args, useRowid, nil
}

func (s *Server) handleDeleteRow(c *gin.Context) {
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

	var req DeleteRowRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":    false,
			"error": gin.H{"code": "BAD_REQUEST", "message": err.Error()},
		})
		return
	}

	if len(req.Key) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":    false,
			"error": gin.H{"code": "BAD_REQUEST", "message": "key is required"},
		})
		return
	}

	whereClause, args, useRowid, err := buildRowWhereClause(schema, req.Key)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":    false,
			"error": gin.H{"code": "BAD_REQUEST", "message": err.Error()},
		})
		return
	}
	whereParts := []string{whereClause}

	// Begin transaction
	tx, err := s.db.Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"ok":    false,
			"error": gin.H{"code": "DB_ERROR", "message": err.Error()},
		})
		return
	}
	defer tx.Rollback()

	// Count rows matching the key first
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE %s",
		db.QuoteIdentifier(name), strings.Join(whereParts, " AND "))
	var count int
	if err := tx.QueryRow(countQuery, args...).Scan(&count); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":    false,
			"error": gin.H{"code": "SQL_ERROR", "message": err.Error()},
		})
		return
	}

	if count == 0 {
		c.JSON(http.StatusNotFound, gin.H{
			"ok":    false,
			"error": gin.H{"code": "NOT_FOUND", "message": "row not found"},
		})
		return
	}

	if count > 1 {
		c.JSON(http.StatusConflict, gin.H{
			"ok":    false,
			"error": gin.H{"code": "AMBIGUOUS_KEY", "message": "delete matches multiple rows"},
		})
		return
	}

	// Perform DELETE
	deleteSQL := fmt.Sprintf("DELETE FROM %s WHERE %s",
		db.QuoteIdentifier(name), strings.Join(whereParts, " AND "))

	if _, err := tx.Exec(deleteSQL, args...); err != nil {
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
		"data": gin.H{"rowsAffected": 1, "useRowid": useRowid},
	})
}

// POST /api/tables/:name/rows/bulk-delete
type BulkDeleteRowsRequest struct {
	Keys []map[string]interface{} `json:"keys" binding:"required"`
}

// handleBulkDeleteRows deletes a set of rows, identified by their PK/rowid
// keys, in a single transaction. Any key that fails to resolve to exactly
// one row (missing key column, no match, or an ambiguous match) aborts the
// whole batch — nothing is deleted — matching the same all-or-nothing
// discipline as the single-row delete's count-check-before-delete guard.
func (s *Server) handleBulkDeleteRows(c *gin.Context) {
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

	var req BulkDeleteRowsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":    false,
			"error": gin.H{"code": "BAD_REQUEST", "message": err.Error()},
		})
		return
	}

	if len(req.Keys) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":    false,
			"error": gin.H{"code": "BAD_REQUEST", "message": "keys is required and must be non-empty"},
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

	var deleted int64
	for idx, key := range req.Keys {
		if len(key) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{
				"ok":    false,
				"error": gin.H{"code": "BAD_REQUEST", "message": fmt.Sprintf("keys[%d]: key is required", idx)},
			})
			return
		}

		whereClause, args, _, err := buildRowWhereClause(schema, key)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"ok":    false,
				"error": gin.H{"code": "BAD_REQUEST", "message": fmt.Sprintf("keys[%d]: %s", idx, err.Error())},
			})
			return
		}

		countQuery := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE %s",
			db.QuoteIdentifier(name), whereClause)
		var count int
		if err := tx.QueryRow(countQuery, args...).Scan(&count); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"ok":    false,
				"error": gin.H{"code": "SQL_ERROR", "message": err.Error()},
			})
			return
		}

		if count == 0 {
			c.JSON(http.StatusNotFound, gin.H{
				"ok":    false,
				"error": gin.H{"code": "NOT_FOUND", "message": fmt.Sprintf("keys[%d]: row not found", idx)},
			})
			return
		}
		if count > 1 {
			c.JSON(http.StatusConflict, gin.H{
				"ok":    false,
				"error": gin.H{"code": "AMBIGUOUS_KEY", "message": fmt.Sprintf("keys[%d]: delete matches multiple rows", idx)},
			})
			return
		}

		deleteSQL := fmt.Sprintf("DELETE FROM %s WHERE %s",
			db.QuoteIdentifier(name), whereClause)
		if _, err := tx.Exec(deleteSQL, args...); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"ok":    false,
				"error": gin.H{"code": "SQL_ERROR", "message": err.Error()},
			})
			return
		}
		deleted++
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
		"data": gin.H{"deleted": deleted},
	})
}
