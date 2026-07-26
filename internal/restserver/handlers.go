package restserver

import (
	"database/sql"
	"encoding/hex"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/0funct0ry/squad/internal/db"
	"github.com/gin-gonic/gin"
)

// registerRoutes mounts one literal route set per exposed table into engine,
// against the connection captured at Start() time. Registering a literal
// path per table (rather than a generic "/:table" dispatcher) means a table
// or method that isn't in the snapshot is simply never routed — gin's own
// 404 handling enforces "only mounted if exposed/enabled", matching SPEC
// §5.7 without extra bookkeeping in the handlers.
func registerRoutes(engine *gin.Engine, conn *sql.DB, tables map[string]RouteInfo) {
	rest := engine.Group("/rest")
	rest.GET("/_schema", func(c *gin.Context) { handleSchema(c, tables) })

	for name, info := range tables {
		info := info
		rest.GET("/"+name, func(c *gin.Context) { handleList(c, conn, info) })
		if info.HasPKRoute {
			rest.GET("/"+name+"/:pk", func(c *gin.Context) { handleGet(c, conn, info) })
		}
		if info.Create {
			rest.POST("/"+name, func(c *gin.Context) { handleCreate(c, conn, info) })
		}
		if info.Update {
			rest.PATCH("/"+name+"/:pk", func(c *gin.Context) { handleUpdate(c, conn, info) })
		}
		if info.Delete {
			rest.DELETE("/"+name+"/:pk", func(c *gin.Context) { handleDelete(c, conn, info) })
		}
	}

	engine.NoRoute(func(c *gin.Context) {
		writeError(c, http.StatusNotFound, "NOT_FOUND", "no such REST route (table not exposed, method not enabled, or unknown path)")
	})
}

type schemaTableView struct {
	Table   string   `json:"table"`
	Type    string   `json:"type"`
	Methods []string `json:"methods"`
}

func handleSchema(c *gin.Context, tables map[string]RouteInfo) {
	out := make([]schemaTableView, 0, len(tables))
	for _, info := range tables {
		methods := []string{"GET /rest/" + info.Table}
		if info.HasPKRoute {
			methods = append(methods, "GET /rest/"+info.Table+"/:pk")
		}
		if info.Create {
			methods = append(methods, "POST /rest/"+info.Table)
		}
		if info.Update {
			methods = append(methods, "PATCH /rest/"+info.Table+"/:pk")
		}
		if info.Delete {
			methods = append(methods, "DELETE /rest/"+info.Table+"/:pk")
		}
		out = append(out, schemaTableView{Table: info.Table, Type: info.Type, Methods: methods})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Table < out[j].Table })
	c.JSON(http.StatusOK, out)
}

func handleList(c *gin.Context, conn *sql.DB, info RouteInfo) {
	schema, err := db.GetTableSchema(conn, info.Table)
	if err != nil {
		writeError(c, http.StatusNotFound, "NOT_FOUND", "table or view not found")
		return
	}

	limit, offset := parsePagination(c.Request.URL.Query())
	sqlStr, args, err := buildListQuery(schema, c.Request.URL.Query(), limit, offset)
	if err != nil {
		writeError(c, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}

	rows, err := conn.Query(sqlStr, args...)
	if err != nil {
		writeError(c, http.StatusInternalServerError, "INTERNAL", err.Error())
		return
	}
	defer rows.Close()

	results, err := scanRows(rows, schema)
	if err != nil {
		writeError(c, http.StatusInternalServerError, "INTERNAL", err.Error())
		return
	}
	c.JSON(http.StatusOK, results)
}

func handleGet(c *gin.Context, conn *sql.DB, info RouteInfo) {
	schema, err := db.GetTableSchema(conn, info.Table)
	if err != nil {
		writeError(c, http.StatusNotFound, "NOT_FOUND", "table or view not found")
		return
	}

	pk := c.Param("pk")
	rows, err := conn.Query(buildGetQuery(schema, info.PKColumn), pk)
	if err != nil {
		writeError(c, http.StatusInternalServerError, "INTERNAL", err.Error())
		return
	}
	defer rows.Close()

	results, err := scanRows(rows, schema)
	if err != nil {
		writeError(c, http.StatusInternalServerError, "INTERNAL", err.Error())
		return
	}
	if len(results) == 0 {
		writeError(c, http.StatusNotFound, "NOT_FOUND", "row not found")
		return
	}
	c.JSON(http.StatusOK, results[0])
}

func handleCreate(c *gin.Context, conn *sql.DB, info RouteInfo) {
	schema, err := db.GetTableSchema(conn, info.Table)
	if err != nil {
		writeError(c, http.StatusNotFound, "NOT_FOUND", "table or view not found")
		return
	}

	var body map[string]interface{}
	if err := c.ShouldBindJSON(&body); err != nil || len(body) == 0 {
		writeError(c, http.StatusBadRequest, "BAD_REQUEST", "request body must be a JSON object with at least one column")
		return
	}

	colMap := make(map[string]bool, len(schema.Columns))
	for _, col := range schema.Columns {
		colMap[col.Name] = true
	}

	var cols, placeholders []string
	var args []interface{}
	for k, v := range body {
		if !colMap[k] {
			writeError(c, http.StatusBadRequest, "BAD_REQUEST", fmt.Sprintf("unknown column: %s", k))
			return
		}
		cols = append(cols, db.QuoteIdentifier(k))
		placeholders = append(placeholders, "?")
		args = append(args, v)
	}

	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
		db.QuoteIdentifier(info.Table), strings.Join(cols, ", "), strings.Join(placeholders, ", "))

	res, err := conn.Exec(query, args...)
	if err != nil {
		writeError(c, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}

	if info.HasPKRoute {
		var pkVal interface{}
		if v, ok := body[info.PKColumn]; ok {
			pkVal = v
		} else if id, err := res.LastInsertId(); err == nil {
			pkVal = id
		}
		if pkVal != nil {
			if rows, err := conn.Query(buildGetQuery(schema, info.PKColumn), pkVal); err == nil {
				defer rows.Close()
				if results, err := scanRows(rows, schema); err == nil && len(results) == 1 {
					c.JSON(http.StatusCreated, results[0])
					return
				}
			}
		}
	}

	affected, _ := res.RowsAffected()
	c.JSON(http.StatusCreated, gin.H{"rowsAffected": affected})
}

func handleUpdate(c *gin.Context, conn *sql.DB, info RouteInfo) {
	schema, err := db.GetTableSchema(conn, info.Table)
	if err != nil {
		writeError(c, http.StatusNotFound, "NOT_FOUND", "table or view not found")
		return
	}

	var body map[string]interface{}
	if err := c.ShouldBindJSON(&body); err != nil || len(body) == 0 {
		writeError(c, http.StatusBadRequest, "BAD_REQUEST", "request body must be a JSON object with at least one column")
		return
	}

	colMap := make(map[string]bool, len(schema.Columns))
	for _, col := range schema.Columns {
		colMap[col.Name] = true
	}

	var setParts []string
	var args []interface{}
	for k, v := range body {
		if !colMap[k] {
			writeError(c, http.StatusBadRequest, "BAD_REQUEST", fmt.Sprintf("unknown column: %s", k))
			return
		}
		setParts = append(setParts, fmt.Sprintf("%s = ?", db.QuoteIdentifier(k)))
		args = append(args, v)
	}

	pk := c.Param("pk")
	query := fmt.Sprintf("UPDATE %s SET %s WHERE %s = ?",
		db.QuoteIdentifier(info.Table), strings.Join(setParts, ", "), db.QuoteIdentifier(info.PKColumn))
	args = append(args, pk)

	res, err := conn.Exec(query, args...)
	if err != nil {
		writeError(c, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		writeError(c, http.StatusNotFound, "NOT_FOUND", "row not found")
		return
	}

	if rows, err := conn.Query(buildGetQuery(schema, info.PKColumn), pk); err == nil {
		defer rows.Close()
		if results, err := scanRows(rows, schema); err == nil && len(results) == 1 {
			c.JSON(http.StatusOK, results[0])
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{"rowsAffected": affected})
}

func handleDelete(c *gin.Context, conn *sql.DB, info RouteInfo) {
	_, err := db.GetTableSchema(conn, info.Table)
	if err != nil {
		writeError(c, http.StatusNotFound, "NOT_FOUND", "table or view not found")
		return
	}

	pk := c.Param("pk")
	query := fmt.Sprintf("DELETE FROM %s WHERE %s = ?", db.QuoteIdentifier(info.Table), db.QuoteIdentifier(info.PKColumn))
	res, err := conn.Exec(query, pk)
	if err != nil {
		writeError(c, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		writeError(c, http.StatusNotFound, "NOT_FOUND", "row not found")
		return
	}
	c.JSON(http.StatusOK, gin.H{"rowsAffected": affected})
}

// scanRows decodes rows into column-keyed maps (one per row) for /rest/*'s
// bare-JSON-object response shape, applying the same BLOB-to-hex and
// numeric-string decoding as internal/db.GetTableRows.
func scanRows(rows *sql.Rows, schema *db.TableSchema) ([]map[string]interface{}, error) {
	cols, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("failed to get columns: %w", err)
	}

	colIsBlob := make([]bool, len(cols))
	for i, colName := range cols {
		for _, sc := range schema.Columns {
			if sc.Name == colName {
				colIsBlob[i] = strings.EqualFold(sc.Type, "blob")
				break
			}
		}
	}

	out := []map[string]interface{}{}
	for rows.Next() {
		raw := make([][]byte, len(cols))
		dest := make([]interface{}, len(cols))
		for i := range dest {
			dest[i] = &raw[i]
		}
		if err := rows.Scan(dest...); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		row := make(map[string]interface{}, len(cols))
		for i, b := range raw {
			switch {
			case b == nil:
				row[cols[i]] = nil
			case colIsBlob[i]:
				row[cols[i]] = hex.EncodeToString(b)
			default:
				s := string(b)
				if iv, err := strconv.ParseInt(s, 10, 64); err == nil {
					row[cols[i]] = iv
				} else if fv, err := strconv.ParseFloat(s, 64); err == nil {
					row[cols[i]] = fv
				} else {
					row[cols[i]] = s
				}
			}
		}
		out = append(out, row)
	}
	return out, nil
}
