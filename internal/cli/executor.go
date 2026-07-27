package cli

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/0funct0ry/squad/internal/db"
	"github.com/fatih/color"
)

// schemaChangingKeywords mirrors internal/server's isSchemaChangingStatement
// classification (CREATE/ALTER/DROP), used to invalidate the CLI's
// table/column completion cache after DDL.
var schemaChangingKeywords = map[string]bool{
	"CREATE": true,
	"ALTER":  true,
	"DROP":   true,
}

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

// shellError formats err the way sqlite3 does: a single "Error: <message>"
// line, in red when the current session is colorized.
func (s *State) shellError(err error) {
	msg := "Error: " + err.Error()
	if s.Colorized {
		msg = color.New(color.FgRed).Sprint(msg)
	}
	fmt.Fprintln(s.Out, msg)
}

// Execute runs one statement: dot-command dispatch, or template preprocess
// -> Classify (the same function POST /api/query uses) -> execute -> render.
// It is the single path shared by the REPL, the inline-SQL-argument mode,
// and stdin/.read script execution.
func (s *State) Execute(statement string) {
	defer s.closeOnceIfSet()

	trimmed := strings.TrimSpace(statement)
	if trimmed == "" {
		return
	}

	if strings.HasPrefix(trimmed, ".") {
		s.dispatchDotCommand(trimmed)
		return
	}

	rendered, err := preprocessTemplate(statement)
	if err != nil {
		s.shellError(err)
		return
	}

	class, err := db.Classify(rendered)
	if err != nil {
		if errors.Is(err, db.ErrEmptyQuery) {
			return
		}
		s.shellError(err)
		return
	}

	if class == db.ClassWrite && !s.Write {
		clean, _ := db.StripCommentsAndWhitespace(rendered)
		keyword := "WRITE"
		if words := strings.Fields(clean); len(words) > 0 {
			keyword = strings.ToUpper(words[0])
		}
		s.shellError(fmt.Errorf("%s is not allowed in read-only mode (READ_ONLY)", keyword))
		return
	}

	if class == db.ClassWrite {
		res, err := s.DB.Exec(rendered)
		if err != nil {
			s.shellError(err)
			return
		}
		if isSchemaChangingStatement(rendered) {
			s.invalidateSchemaCache()
		}
		if affected, err := res.RowsAffected(); err == nil && s.Interactive {
			fmt.Fprintf(s.Out, "changes: %d\n", affected)
		}
		return
	}

	rows, err := s.DB.Query(rendered)
	if err != nil {
		s.shellError(err)
		return
	}
	defer rows.Close()

	cols, resultRows, err := scanRowsToValues(rows)
	if err != nil {
		s.shellError(err)
		return
	}
	if err := s.Render(cols, resultRows); err != nil {
		s.shellError(err)
	}
}

// scanRowsToValues scans all rows, hex-encoding BLOB columns per their
// declared database type (same convention as server.scanQueryRows).
func scanRowsToValues(rows *sql.Rows) ([]string, [][]any, error) {
	cols, err := rows.Columns()
	if err != nil {
		return nil, nil, err
	}

	colTypes, err := rows.ColumnTypes()
	colIsBlob := make([]bool, len(cols))
	if err == nil {
		for i, ct := range colTypes {
			colIsBlob[i] = strings.EqualFold(ct.DatabaseTypeName(), "BLOB")
		}
	}

	var result [][]any
	for rows.Next() {
		dest := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range ptrs {
			ptrs[i] = &dest[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, nil, err
		}
		row := make([]any, len(cols))
		for i, v := range dest {
			if v == nil {
				row[i] = nil
			} else if b, ok := v.([]byte); ok {
				if colIsBlob[i] {
					row[i] = "X'" + hexEncode(b) + "'"
				} else {
					row[i] = string(b)
				}
			} else {
				row[i] = v
			}
		}
		result = append(result, row)
	}
	return cols, result, rows.Err()
}

func hexEncode(b []byte) string {
	const hextable = "0123456789abcdef"
	out := make([]byte, len(b)*2)
	for i, c := range b {
		out[i*2] = hextable[c>>4]
		out[i*2+1] = hextable[c&0x0f]
	}
	return string(out)
}
