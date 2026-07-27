package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/0funct0ry/squad/internal/db"
)

// cmdOpen implements ".open DB": closes the current *sql.DB and reopens DB
// via the same db.OpenDB call cmd/cli.go used at startup, keeping the
// process's --write/--read-only-pragma safety mode as-is (only the path
// changes). The {db} prompt segment updates automatically next render since
// RenderPrompt reads State.Path live.
func (s *State) cmdOpen(args []string) {
	if len(args) != 1 {
		s.shellError(fmt.Errorf("usage: .open DB"))
		return
	}
	path := args[0]
	resolved := path
	if path != ":memory:" && !strings.HasPrefix(path, "file:") {
		if abs, err := filepath.Abs(path); err == nil {
			resolved = abs
		}
	}

	newDB, err := db.OpenDB(resolved, s.ReadOnly)
	if err != nil {
		s.shellError(err)
		return
	}
	old := s.DB
	s.DB = newDB
	s.Path = resolved
	old.Close()
	s.invalidateSchemaCache()
	fmt.Fprintf(s.Out, "now connected to %s\n", resolved)
}

// cmdBackup implements ".backup FILE": snapshots the currently open database
// to FILE via VACUUM INTO, which is transactionally consistent and doesn't
// need exclusive access to the source. Available in both read-only and
// --write mode (it's a read of the source), but errors if FILE already
// exists rather than silently overwriting it.
func (s *State) cmdBackup(args []string) {
	if len(args) != 1 {
		s.shellError(fmt.Errorf("usage: .backup FILE"))
		return
	}
	if _, err := os.Stat(args[0]); err == nil {
		s.shellError(fmt.Errorf("file already exists: %s", args[0]))
		return
	}
	lit, err := sqlLiteral(args[0])
	if err != nil {
		s.shellError(err)
		return
	}
	if _, err := s.DB.Exec(fmt.Sprintf("VACUUM INTO %s", lit)); err != nil {
		s.shellError(err)
	}
}

// createTableNameRE matches "CREATE TABLE <name>", where <name> is bare,
// "double-quoted", `backtick-quoted`, or 'single-quoted'.
var createTableNameRE = regexp.MustCompile(`(?i)^(CREATE\s+TABLE\s+)("([^"]+)"|` + "`([^`]+)`" + `|'([^']+)'|(\S+))`)

// replaceTableNameInDDL renames the table in a CREATE TABLE statement's own
// name only -- not a blind string replace, which would also mangle any
// column/comment text matching the same name.
func replaceTableNameInDDL(ddl, newName string) string {
	return createTableNameRE.ReplaceAllString(ddl, "${1}"+db.QuoteIdentifier(newName))
}

// cmdClone implements ".clone TABLE NEW_TABLE ?--data?": recreates TABLE's
// exact DDL under NEW_TABLE (not CREATE TABLE AS SELECT, which silently
// drops PK/NOT NULL/CHECK/FK constraints in SQLite). The whole command is
// write-gated -- even the DDL-only form runs a CREATE TABLE, a write per
// db.Classify. With --data, follows up with an INSERT ... SELECT.
func (s *State) cmdClone(args []string) {
	if !s.Write {
		s.shellError(fmt.Errorf(".clone is not allowed in read-only mode (READ_ONLY)"))
		return
	}
	withData := false
	var positional []string
	for _, a := range args {
		if a == "--data" {
			withData = true
			continue
		}
		positional = append(positional, a)
	}
	if len(positional) != 2 {
		s.shellError(fmt.Errorf("usage: .clone TABLE NEW_TABLE ?--data?"))
		return
	}
	src, dst := positional[0], positional[1]

	schema, err := db.GetTableSchema(s.DB, src)
	if err != nil {
		s.shellError(err)
		return
	}
	newDDL := replaceTableNameInDDL(schema.DDL, dst)
	if _, err := s.DB.Exec(newDDL); err != nil {
		s.shellError(err)
		return
	}
	s.invalidateSchemaCache()

	if withData {
		q := fmt.Sprintf("INSERT INTO %s SELECT * FROM %s", db.QuoteIdentifier(dst), db.QuoteIdentifier(src))
		if _, err := s.DB.Exec(q); err != nil {
			s.shellError(err)
			return
		}
	}
}

// cmdDiff implements ".diff TABLE_A TABLE_B": compares two tables' columns
// (name, type, notnull, pk) in the currently open database and prints
// additions/removals/type-changes as diff-style +/-/~ lines. Iteration is
// over sorted column names for deterministic output.
func (s *State) cmdDiff(args []string) {
	if len(args) != 2 {
		s.shellError(fmt.Errorf("usage: .diff TABLE_A TABLE_B"))
		return
	}
	schemaA, err := db.GetTableSchema(s.DB, args[0])
	if err != nil {
		s.shellError(err)
		return
	}
	schemaB, err := db.GetTableSchema(s.DB, args[1])
	if err != nil {
		s.shellError(err)
		return
	}

	colsA := map[string]db.ColumnInfo{}
	var namesA []string
	for _, c := range schemaA.Columns {
		colsA[c.Name] = c
		namesA = append(namesA, c.Name)
	}
	colsB := map[string]db.ColumnInfo{}
	var namesB []string
	for _, c := range schemaB.Columns {
		colsB[c.Name] = c
		namesB = append(namesB, c.Name)
	}
	sort.Strings(namesA)
	sort.Strings(namesB)

	for _, name := range namesA {
		ca := colsA[name]
		cb, ok := colsB[name]
		if !ok {
			fmt.Fprintf(s.Out, "- %s (only in %s)\n", name, args[0])
			continue
		}
		if ca.Type != cb.Type || ca.NotNull != cb.NotNull || ca.PK != cb.PK {
			fmt.Fprintf(s.Out, "~ %s: %s(notnull=%v,pk=%v) -> %s(notnull=%v,pk=%v)\n",
				name, ca.Type, ca.NotNull, ca.PK, cb.Type, cb.NotNull, cb.PK)
		}
	}
	for _, name := range namesB {
		if _, ok := colsA[name]; !ok {
			fmt.Fprintf(s.Out, "+ %s (only in %s)\n", name, args[1])
		}
	}
}

// isIdentRune reports whether r is a valid identifier rune (alnum or _).
func isIdentRune(r byte) bool {
	return r == '_' || (r >= '0' && r <= '9') || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
}

// parseCheckConstraints is a narrow, deliberately limited parser: it finds
// CHECK(...) clauses in a table's DDL, respecting parens/quotes so a ')'
// inside a quoted string literal in the CHECK expression doesn't prematurely
// close it. SQLite exposes no PRAGMA for CHECK constraints, so this is the
// only way to surface them.
func parseCheckConstraints(ddl string) []string {
	var out []string
	upper := strings.ToUpper(ddl)
	i := 0
	for {
		idx := strings.Index(upper[i:], "CHECK")
		if idx == -1 {
			break
		}
		pos := i + idx
		if pos > 0 && isIdentRune(ddl[pos-1]) {
			i = pos + 5
			continue
		}
		j := pos + 5
		for j < len(ddl) && ddl[j] == ' ' {
			j++
		}
		if j >= len(ddl) || ddl[j] != '(' {
			i = pos + 5
			continue
		}
		depth := 0
		inSingle, inDouble := false, false
		start := j
		closed := false
		for ; j < len(ddl); j++ {
			switch {
			case ddl[j] == '\'' && !inDouble:
				inSingle = !inSingle
			case ddl[j] == '"' && !inSingle:
				inDouble = !inDouble
			case ddl[j] == '(' && !inSingle && !inDouble:
				depth++
			case ddl[j] == ')' && !inSingle && !inDouble:
				depth--
				if depth == 0 {
					out = append(out, ddl[start:j+1])
					j++
					closed = true
				}
			}
			if closed {
				break
			}
		}
		i = j
		if !closed {
			break
		}
	}
	return out
}

// cmdConstraints implements ".constraints TABLE": a narrower, differently
// shaped view than ".schema -t" -- PK/FK/NOT NULL/UNIQUE straight from
// db.GetTableSchema, plus CHECK constraints parsed out of the DDL.
func (s *State) cmdConstraints(args []string) {
	if len(args) != 1 {
		s.shellError(fmt.Errorf("usage: .constraints TABLE"))
		return
	}
	schema, err := db.GetTableSchema(s.DB, args[0])
	if err != nil {
		s.shellError(err)
		return
	}
	if len(schema.PrimaryKey) > 0 {
		fmt.Fprintf(s.Out, "PRIMARY KEY: %s\n", strings.Join(schema.PrimaryKey, ", "))
	}
	for _, fk := range schema.ForeignKeys {
		fmt.Fprintf(s.Out, "FOREIGN KEY: %s -> %s(%s)\n", fk.From, fk.Table, fk.To)
	}
	for _, c := range schema.Columns {
		if c.NotNull {
			fmt.Fprintf(s.Out, "NOT NULL: %s\n", c.Name)
		}
	}
	for _, idx := range schema.Indexes {
		if idx.Unique {
			fmt.Fprintf(s.Out, "UNIQUE: %s\n", strings.Join(idx.Columns, ", "))
		}
	}
	for _, chk := range parseCheckConstraints(schema.DDL) {
		fmt.Fprintf(s.Out, "CHECK: %s\n", chk)
	}
}

// cmdSize implements ".size" / ".stat db": prints db.GetDBMeta's fields as a
// labeled key/value listing -- pure reuse of the exact function the web UI's
// meta endpoint already calls.
func (s *State) cmdSize(args []string) {
	if len(args) != 0 {
		s.shellError(fmt.Errorf("usage: .size"))
		return
	}
	meta, err := db.GetDBMeta(s.DB, s.Path, s.Write)
	if err != nil {
		s.shellError(err)
		return
	}
	fmt.Fprintf(s.Out, "name:           %s\n", meta.Name)
	fmt.Fprintf(s.Out, "path:           %s\n", meta.Path)
	fmt.Fprintf(s.Out, "mode:           %s\n", meta.Mode)
	fmt.Fprintf(s.Out, "sqlite version: %s\n", meta.SqliteVersion)
	fmt.Fprintf(s.Out, "size (bytes):   %d\n", meta.SizeBytes)
	fmt.Fprintf(s.Out, "page size:      %d\n", meta.PageSize)
	fmt.Fprintf(s.Out, "page count:     %d\n", meta.PageCount)
	fmt.Fprintf(s.Out, "encoding:       %s\n", meta.Encoding)
	fmt.Fprintf(s.Out, "journal mode:   %s\n", meta.JournalMode)
	fmt.Fprintf(s.Out, "tables:         %d\n", meta.TableCount)
	fmt.Fprintf(s.Out, "views:          %d\n", meta.ViewCount)
}
