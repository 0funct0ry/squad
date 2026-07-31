// Package modules implements the ten M10e virtual table modules: the six
// structured-data file readers (csv, jsonl, parquet, xlsx, yaml, xml) and the
// four pure generators (series, calendar, fake, tokens). Every module here
// reads the whole source into memory once in Create/Connect and serves it
// through the shared simpleTable/simpleCursor helpers below — none of them
// need incremental/streaming cursors or index pushdown beyond a full scan
// (series/calendar/tokens override BestIndex to note ordering, but still
// scan the precomputed row slice).
package modules

import (
	"errors"
	"strconv"
	"strings"

	vtabdriver "modernc.org/sqlite/vtab"
)

// UsingArgs strips the module-name/schema/table-name prefix the driver
// prepends to Module.Create/Connect's args (mirroring xCreate's argv in the
// SQLite C API — see modernc.org/sqlite/vtab's own module_test.go), leaving
// just the raw USING(...) argument strings for ParseArgs.
func UsingArgs(args []string) []string {
	if len(args) <= 3 {
		return nil
	}
	return args[3:]
}

// ModuleArgs is the parsed key=value form of a CREATE VIRTUAL TABLE ...
// USING module(k1=v1, k2='v2') argument list. Values have surrounding single
// or double quotes stripped.
type ModuleArgs map[string]string

// ParseArgs parses the raw per-argument strings the driver passes as
// args[3:] to Module.Create/Connect (args[0]=module name, args[1]=schema,
// args[2]=table name — see modernc.org/sqlite/vtab's own module_test.go).
func ParseArgs(raw []string) (ModuleArgs, error) {
	out := make(ModuleArgs, len(raw))
	for _, item := range raw {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		eq := strings.IndexByte(item, '=')
		if eq < 0 {
			return nil, errors.New("malformed module argument: " + item)
		}
		key := strings.TrimSpace(item[:eq])
		val := strings.TrimSpace(item[eq+1:])
		val = unquote(val)
		out[strings.ToLower(key)] = val
	}
	return out, nil
}

func unquote(s string) string {
	if len(s) >= 2 {
		if (s[0] == '\'' && s[len(s)-1] == '\'') || (s[0] == '"' && s[len(s)-1] == '"') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

// Get returns the value for key, or def if the key was never passed. An
// explicitly empty value (key=”) is a present value, not "unset".
func (a ModuleArgs) Get(key, def string) string {
	if v, ok := a[key]; ok {
		return v
	}
	return def
}

// GetRequired returns the value for key, or an error naming it if it was
// never passed. An explicitly empty value (key=”) satisfies "required".
func (a ModuleArgs) GetRequired(key string) (string, error) {
	v, ok := a[key]
	if !ok {
		return "", errors.New("missing required argument: " + key)
	}
	return v, nil
}

// GetBool parses a boolean argument, defaulting to def when unset.
func (a ModuleArgs) GetBool(key string, def bool) (bool, error) {
	v, ok := a[key]
	if !ok {
		return def, nil
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return false, errors.New("argument " + key + " must be a boolean: " + v)
	}
	return b, nil
}

// GetFloat parses a numeric argument, defaulting to def when unset.
func (a ModuleArgs) GetFloat(key string, def float64) (float64, error) {
	v, ok := a[key]
	if !ok {
		return def, nil
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return 0, errors.New("argument " + key + " must be a number: " + v)
	}
	return f, nil
}

// GetInt parses an integer argument, defaulting to def when unset.
func (a ModuleArgs) GetInt(key string, def int) (int, error) {
	v, ok := a[key]
	if !ok {
		return def, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, errors.New("argument " + key + " must be an integer: " + v)
	}
	return n, nil
}

// quoteIdent quotes an identifier for use inside a CREATE TABLE declaration.
func quoteIdent(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

// DeclareColumns builds and issues a `CREATE TABLE x(...)` schema
// declaration from an ordered column name list, with optional per-column
// SQLite type affinities (types[i] == "" leaves the column untyped).
func DeclareColumns(ctx vtabdriver.Context, cols []string, types []string) error {
	var b strings.Builder
	b.WriteString("CREATE TABLE x(")
	for i, c := range cols {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString(quoteIdent(c))
		if i < len(types) && types[i] != "" {
			b.WriteString(" ")
			b.WriteString(types[i])
		}
	}
	b.WriteString(")")
	return ctx.Declare(b.String())
}

// simpleTable is a materialized, read-only virtual table: every module in
// this package computes its full row set once in Create/Connect (these are
// local files and pure generators, never unbounded streams) and hands it to
// simpleTable for scanning. No module implements vtab.Updater.
type simpleTable struct {
	rows [][]vtabdriver.Value
}

// NewSimpleTable wraps a precomputed row set as a vtab.Table.
func NewSimpleTable(rows [][]vtabdriver.Value) vtabdriver.Table {
	return &simpleTable{rows: rows}
}

func (t *simpleTable) BestIndex(info *vtabdriver.IndexInfo) error {
	info.EstimatedRows = int64(len(t.rows))
	info.EstimatedCost = float64(len(t.rows)) + 1
	return nil
}

func (t *simpleTable) Open() (vtabdriver.Cursor, error) {
	return &simpleCursor{table: t, pos: 0}, nil
}

func (t *simpleTable) Disconnect() error { return nil }
func (t *simpleTable) Destroy() error    { return nil }

type simpleCursor struct {
	table *simpleTable
	pos   int
}

func (c *simpleCursor) Filter(idxNum int, idxStr string, vals []vtabdriver.Value) error {
	c.pos = 0
	return nil
}

func (c *simpleCursor) Next() error {
	c.pos++
	return nil
}

func (c *simpleCursor) Eof() bool {
	return c.pos >= len(c.table.rows)
}

func (c *simpleCursor) Column(col int) (vtabdriver.Value, error) {
	return c.table.rows[c.pos][col], nil
}

func (c *simpleCursor) Rowid() (int64, error) {
	return int64(c.pos), nil
}

func (c *simpleCursor) Close() error { return nil }

// sniffValue converts a raw text field (e.g. a CSV cell or XML text node)
// into an int64/float64 when it sniffs cleanly, else leaves it as text —
// used by the file-reader modules per VTABS.md's csv/xml column contract.
func sniffValue(s string) vtabdriver.Value {
	if s == "" {
		return s
	}
	if n, err := strconv.ParseInt(s, 10, 64); err == nil {
		return n
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return f
	}
	return s
}
