package db

import (
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
)

// ErrNotFound is returned when a named table/view does not exist in sqlite_master.
var ErrNotFound = errors.New("table or view not found")

type TableInfo struct {
	Name     string `json:"name"`
	Type     string `json:"type"` // "table" or "view"
	RowCount int64  `json:"rowCount"`
	// IsVirtual is true for a virtual table declared directly in the user's
	// own schema (e.g. `CREATE VIRTUAL TABLE ... USING csv(...)` typed into
	// the SQL editor), as opposed to a mount, which lives in temp and is
	// never listed here — see GetTableSchema's IsVirtual for the same
	// substring-on-DDL detection.
	IsVirtual bool `json:"isVirtual"`
}

type ColumnInfo struct {
	Name       string  `json:"name"`
	Type       string  `json:"type"`
	NotNull    bool    `json:"notnull"`
	DefaultVal *string `json:"defaultVal"`
	PK         int     `json:"pk"` // 0 if not pk, otherwise 1-based index
	Hidden     int     `json:"hidden"`
	Generated  *string `json:"generated"` // nil | "virtual" | "stored"
	// IsHiddenVtabColumn is true for a virtual table's HIDDEN column
	// (table_xinfo's hidden==1 case), as opposed to hidden==2/3's generated
	// columns — e.g. a mount alias's implicit constraint-only columns.
	IsHiddenVtabColumn bool `json:"isHiddenVtabColumn,omitempty"`
}

type IndexInfo struct {
	Name    string   `json:"name"`
	Unique  bool     `json:"unique"`
	Origin  string   `json:"origin"`
	Partial bool     `json:"partial"`
	Columns []string `json:"columns"`
	SQL     *string  `json:"sql"`
}

type ForeignKeyInfo struct {
	ID       int    `json:"id"`
	Seq      int    `json:"seq"`
	Table    string `json:"table"`
	From     string `json:"from"`
	To       string `json:"to"`
	OnUpdate string `json:"onUpdate"`
	OnDelete string `json:"onDelete"`
	Match    string `json:"match"`
}

type TriggerInfo struct {
	Name string `json:"name"`
	SQL  string `json:"sql"`
}

type TableSchema struct {
	Name         string `json:"name"`
	Type         string `json:"type"` // "table" or "view"
	RowCount     int64  `json:"rowCount"`
	WithoutRowid bool   `json:"withoutRowid"`
	// IsVirtual is true for a virtual table (a mount, or any other `CREATE
	// VIRTUAL TABLE`), detected the same way WithoutRowid is: a substring
	// check on the table's own DDL. Most virtual tables have no usable
	// rowid, so BuildTableQuery must not prepend one for these.
	IsVirtual   bool             `json:"isVirtual"`
	Columns     []ColumnInfo     `json:"columns"`
	PrimaryKey  []string         `json:"primaryKey"`
	Indexes     []IndexInfo      `json:"indexes"`
	ForeignKeys []ForeignKeyInfo `json:"foreignKeys"`
	Triggers    []TriggerInfo    `json:"triggers"`
	DDL         string           `json:"ddl"`
}

type RowResult struct {
	Columns []string        `json:"columns"`
	Rows    [][]interface{} `json:"rows"`
	Total   int64           `json:"total"`
}

type RowQueryParams struct {
	Limit   int
	Offset  int
	OrderBy string
	Dir     string // "asc" or "desc"
	Filters map[string]string
}

// GetTables returns a list of tables and views with their row counts.
func GetTables(db Queryer) ([]TableInfo, error) {
	rows, err := db.Query(`
		SELECT name, type, sql
		FROM sqlite_master
		WHERE type IN ('table', 'view') AND name NOT LIKE 'sqlite_%'
		ORDER BY name
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to query tables: %w", err)
	}
	defer rows.Close()

	tables := make([]TableInfo, 0)
	for rows.Next() {
		var name, ttype string
		var sqlText sql.NullString
		if err := rows.Scan(&name, &ttype, &sqlText); err != nil {
			return nil, fmt.Errorf("failed to scan table row: %w", err)
		}
		tables = append(tables, TableInfo{
			Name:      name,
			Type:      ttype,
			IsVirtual: strings.Contains(strings.ToLower(sqlText.String), "virtual table"),
		})
	}

	// Fetch row count for each table/view
	for i, t := range tables {
		var count int64
		// We use QuoteIdentifier to avoid SQL injection on table name
		query := fmt.Sprintf("SELECT COUNT(*) FROM %s", QuoteIdentifier(t.Name))
		err := db.QueryRow(query).Scan(&count)
		if err != nil {
			// If counting fails (e.g. invalid view or locked table), default to 0 instead of crashing
			count = 0
		}
		tables[i].RowCount = count
	}

	return tables, nil
}

// GetTableSchema retrieves column info, indexes, foreign keys, triggers, and DDL for a table.
func GetTableSchema(db Queryer, tableName string) (*TableSchema, error) {
	quotedName := QuoteIdentifier(tableName)

	// 0. Resolve against sqlite_master. Mounts live in the temp schema, not
	// main, so this has to search both — unqualified "sqlite_master" only
	// ever resolves to main.sqlite_master.
	var objType string
	err := db.QueryRow(
		`SELECT type FROM (
			SELECT type, name FROM sqlite_master
			UNION ALL
			SELECT type, name FROM temp.sqlite_master
		) WHERE type IN ('table', 'view') AND name = ?`,
		tableName,
	).Scan(&objType)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to resolve table: %w", err)
	}

	// 1. Columns (table_xinfo includes hidden/generated columns)
	colRows, err := db.Query(fmt.Sprintf("PRAGMA table_xinfo(%s)", quotedName))
	if err != nil {
		return nil, fmt.Errorf("failed to get table info: %w", err)
	}
	defer colRows.Close()

	columns := []ColumnInfo{}
	var primaryKey []struct {
		name string
		pk   int
	}
	for colRows.Next() {
		var cid int
		var name, ctype string
		var notnull int
		var dfltVal sql.NullString
		var pk int
		var hidden int

		if err := colRows.Scan(&cid, &name, &ctype, &notnull, &dfltVal, &pk, &hidden); err != nil {
			return nil, fmt.Errorf("failed to scan column info: %w", err)
		}

		var dflt *string
		if dfltVal.Valid {
			dflt = &dfltVal.String
		}

		var generated *string
		switch hidden {
		case 2:
			v := "virtual"
			generated = &v
		case 3:
			v := "stored"
			generated = &v
		}

		columns = append(columns, ColumnInfo{
			Name:               name,
			Type:               ctype,
			NotNull:            notnull != 0,
			DefaultVal:         dflt,
			PK:                 pk,
			Hidden:             hidden,
			Generated:          generated,
			IsHiddenVtabColumn: hidden == 1,
		})

		if pk > 0 {
			primaryKey = append(primaryKey, struct {
				name string
				pk   int
			}{name, pk})
		}
	}

	sort.Slice(primaryKey, func(i, j int) bool { return primaryKey[i].pk < primaryKey[j].pk })
	pkNames := make([]string, len(primaryKey))
	for i, p := range primaryKey {
		pkNames[i] = p.name
	}

	// 2. Indexes
	idxRows, err := db.Query(fmt.Sprintf("PRAGMA index_list(%s)", quotedName))
	if err != nil {
		return nil, fmt.Errorf("failed to get index list: %w", err)
	}
	defer idxRows.Close()

	indexes := []IndexInfo{}
	for idxRows.Next() {
		var seq int
		var name string
		var unique int
		var origin string
		var partial int

		if err := idxRows.Scan(&seq, &name, &unique, &origin, &partial); err != nil {
			return nil, fmt.Errorf("failed to scan index list: %w", err)
		}

		// Get columns for index
		infoRows, err := db.Query(fmt.Sprintf("PRAGMA index_info(%s)", QuoteIdentifier(name)))
		if err != nil {
			return nil, fmt.Errorf("failed to get index info: %w", err)
		}

		cols := []string{}
		for infoRows.Next() {
			var seqno, cid int
			var colName sql.NullString
			if err := infoRows.Scan(&seqno, &cid, &colName); err != nil {
				infoRows.Close()
				return nil, fmt.Errorf("failed to scan index info: %w", err)
			}
			if colName.Valid {
				cols = append(cols, colName.String)
			}
		}
		infoRows.Close()

		var indexSQL *string
		var sqlStr sql.NullString
		err = db.QueryRow(
			"SELECT sql FROM sqlite_master WHERE type = 'index' AND name = ?", name,
		).Scan(&sqlStr)
		if err != nil && err != sql.ErrNoRows {
			return nil, fmt.Errorf("failed to get index DDL: %w", err)
		}
		if sqlStr.Valid {
			indexSQL = &sqlStr.String
		}

		indexes = append(indexes, IndexInfo{
			Name:    name,
			Unique:  unique != 0,
			Origin:  origin,
			Partial: partial != 0,
			Columns: cols,
			SQL:     indexSQL,
		})
	}

	// 3. Foreign keys
	fkRows, err := db.Query(fmt.Sprintf("PRAGMA foreign_key_list(%s)", quotedName))
	if err != nil {
		return nil, fmt.Errorf("failed to get foreign key list: %w", err)
	}
	defer fkRows.Close()

	fkeys := []ForeignKeyInfo{}
	for fkRows.Next() {
		var id, seq int
		var table string
		var from, to sql.NullString
		var onUpdate, onDelete, match string

		if err := fkRows.Scan(&id, &seq, &table, &from, &to, &onUpdate, &onDelete, &match); err != nil {
			return nil, fmt.Errorf("failed to scan foreign key: %w", err)
		}

		fkeys = append(fkeys, ForeignKeyInfo{
			ID:       id,
			Seq:      seq,
			Table:    table,
			From:     from.String,
			To:       to.String,
			OnUpdate: onUpdate,
			OnDelete: onDelete,
			Match:    match,
		})
	}

	// 4. Triggers
	trigRows, err := db.Query("SELECT name, sql FROM sqlite_master WHERE type = 'trigger' AND tbl_name = ? ORDER BY name", tableName)
	if err != nil {
		return nil, fmt.Errorf("failed to query triggers: %w", err)
	}
	defer trigRows.Close()

	triggers := []TriggerInfo{}
	for trigRows.Next() {
		var name, sqlStr string
		if err := trigRows.Scan(&name, &sqlStr); err != nil {
			return nil, fmt.Errorf("failed to scan trigger: %w", err)
		}
		triggers = append(triggers, TriggerInfo{
			Name: name,
			SQL:  sqlStr,
		})
	}

	// 5. DDL (same main+temp union as step 0, for the same reason)
	var ddl string
	err = db.QueryRow(
		`SELECT sql FROM (
			SELECT sql, type, name FROM sqlite_master
			UNION ALL
			SELECT sql, type, name FROM temp.sqlite_master
		) WHERE type IN ('table', 'view') AND name = ?`,
		tableName,
	).Scan(&ddl)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("failed to get DDL: %w", err)
	}
	ddlLower := strings.ToLower(ddl)
	withoutRowid := strings.Contains(ddlLower, "without rowid")
	isVirtual := strings.Contains(ddlLower, "virtual table")

	// 6. Row count. Errors are swallowed to 0 rather than propagated: some
	// virtual table modules reject an unconstrained COUNT(*) (no pushdown
	// possible), and a failed count shouldn't block the rest of the schema.
	var rowCount int64
	if err := db.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM %s", quotedName)).Scan(&rowCount); err != nil {
		rowCount = 0
	}

	return &TableSchema{
		Name:         tableName,
		Type:         objType,
		RowCount:     rowCount,
		WithoutRowid: withoutRowid,
		IsVirtual:    isVirtual,
		Columns:      columns,
		PrimaryKey:   pkNames,
		Indexes:      indexes,
		ForeignKeys:  fkeys,
		Triggers:     triggers,
		DDL:          ddl,
	}, nil
}

// BuildTableQuery constructs the SQL query, count query, and arguments for a table based on filtering and sorting parameters, WITHOUT limit/offset.
func BuildTableQuery(db Queryer, tableName string, params RowQueryParams) (string, string, []interface{}, *TableSchema, error) {
	quotedTable := QuoteIdentifier(tableName)

	// Validate orderby is one of the columns to prevent SQL injection
	schema, err := GetTableSchema(db, tableName)
	if err != nil {
		return "", "", nil, nil, err
	}

	columnMap := make(map[string]bool)
	for _, col := range schema.Columns {
		columnMap[col.Name] = true
	}

	// Construct dynamic query
	var whereClauses []string
	var args []interface{}

	for col, val := range params.Filters {
		if columnMap[col] {
			whereClauses = append(whereClauses, fmt.Sprintf("%s LIKE ?", QuoteIdentifier(col)))
			args = append(args, "%"+val+"%")
		}
	}

	wherePart := ""
	if len(whereClauses) > 0 {
		wherePart = " WHERE "
		for i, clause := range whereClauses {
			if i > 0 {
				wherePart += " AND "
			}
			wherePart += clause
		}
	}

	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM %s%s", quotedTable, wherePart)
	selectFields := "*"
	if len(schema.PrimaryKey) == 0 && !schema.WithoutRowid && !schema.IsVirtual {
		selectFields = "rowid, *"
	}
	selectQuery := fmt.Sprintf("SELECT %s FROM %s%s", selectFields, quotedTable, wherePart)

	// Ordering
	if params.OrderBy != "" && columnMap[params.OrderBy] {
		dir := "ASC"
		if params.Dir == "desc" || params.Dir == "DESC" {
			dir = "DESC"
		}
		selectQuery += fmt.Sprintf(" ORDER BY %s %s", QuoteIdentifier(params.OrderBy), dir)
	}

	return selectQuery, countQuery, args, schema, nil
}

// GetTableRows returns a page of rows with optional sorting and filtering.
func GetTableRows(db Queryer, tableName string, params RowQueryParams) (*RowResult, error) {
	selectQuery, countQuery, args, schema, err := BuildTableQuery(db, tableName, params)
	if err != nil {
		return nil, err
	}

	// Total count matching filters
	var total int64
	err = db.QueryRow(countQuery, args...).Scan(&total)
	if err != nil {
		return nil, fmt.Errorf("failed to count rows: %w", err)
	}

	// Limit and Offset
	selectQuery += " LIMIT ? OFFSET ?"
	queryArgs := append([]interface{}{}, args...)
	queryArgs = append(queryArgs, params.Limit, params.Offset)

	rows, err := db.Query(selectQuery, queryArgs...)
	if err != nil {
		return nil, fmt.Errorf("failed to query rows: %w", err)
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("failed to get columns: %w", err)
	}

	// Map result columns to their declared type so BLOB columns can be
	// hex-encoded rather than treated as raw (possibly invalid-UTF8) text.
	colIsBlob := make([]bool, len(cols))
	for i, colName := range cols {
		for _, sc := range schema.Columns {
			if sc.Name == colName {
				colIsBlob[i] = strings.EqualFold(sc.Type, "blob")
				break
			}
		}
	}

	resultRows := [][]interface{}{}
	for rows.Next() {
		// Scan non-BLOB columns into interface{} so the driver's own decoded
		// type (int64/float64/string/nil) comes through untouched. The
		// previous implementation scanned every column into raw []byte and
		// re-parsed the text with strconv.ParseInt/ParseFloat to guess
		// whether it "looked like" a number — but that round trip silently
		// reformats any TEXT value that happens to parse as a number (e.g.
		// a zero-padded code like "00008" from a generated column or a
		// PAD_LEFT/format-string result comes back as the bare integer 8,
		// dropping the leading zeros the query itself preserved). BLOB
		// columns still scan as raw bytes so they can be hex-encoded below.
		dest := make([]interface{}, len(cols))
		blobDest := make([][]byte, len(cols))
		for i := range dest {
			if colIsBlob[i] {
				dest[i] = &blobDest[i]
			} else {
				dest[i] = new(interface{})
			}
		}

		if err := rows.Scan(dest...); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		rowVals := make([]interface{}, len(cols))
		for i := range cols {
			if colIsBlob[i] {
				if blobDest[i] == nil {
					rowVals[i] = nil
				} else {
					rowVals[i] = hex.EncodeToString(blobDest[i])
				}
				continue
			}
			switch v := (*dest[i].(*interface{})).(type) {
			case []byte:
				rowVals[i] = string(v)
			default:
				rowVals[i] = v
			}
		}
		resultRows = append(resultRows, rowVals)
	}

	return &RowResult{
		Columns: cols,
		Rows:    resultRows,
		Total:   total,
	}, nil
}
