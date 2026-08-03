package cli

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/0funct0ry/squad/internal/db"
	"github.com/0funct0ry/squad/internal/vtab"
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

	trimmed = s.expandAlias(trimmed)
	statement = trimmed

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
		start := time.Now()
		res, err := s.DB.Exec(rendered)
		elapsed := time.Since(start)
		if err != nil {
			s.shellError(err)
			return
		}
		schemaChanged := isSchemaChangingStatement(rendered)
		if schemaChanged {
			s.invalidateSchemaCache()
		}
		// LastColumns/LastRows are deliberately left untouched by writes:
		// .grep always greps the last SELECT result, not the last statement.
		affected, _ := res.RowsAffected()
		if affected > 0 && s.Interactive {
			fmt.Fprintf(s.Out, "changes: %d\n", affected)
		}
		s.printTimerStats(elapsed, affected, schemaChanged)
		return
	}

	start := time.Now()
	var cols []string
	var resultRows [][]any
	// Routed through a pinned connection with mounts replayed, the same way
	// the web server's read routes are — a plain s.DB.Query could land on a
	// pooled connection that never saw this session's CREATE VIRTUAL TABLE.
	err = vtab.WithMounts(context.Background(), s.DB, s.MountStore, func(conn *sql.Conn) error {
		rows, err := conn.QueryContext(context.Background(), rendered)
		if err != nil {
			return err
		}
		defer rows.Close()
		cols, resultRows, err = scanRowsToValues(rows)
		return err
	})
	elapsed := time.Since(start)
	if err != nil {
		s.shellError(err)
		return
	}
	s.LastColumns = cols
	s.LastRows = resultRows
	if err := s.Render(cols, resultRows); err != nil {
		s.shellError(err)
	}
	s.printTimerStats(elapsed, int64(len(resultRows)), false)
}

// printTimerStats prints the ".timer"/".stats" lines after a statement, if
// either toggle is on. Independent toggles: both may print, timer first.
func (s *State) printTimerStats(elapsed time.Duration, rows int64, schemaChanged bool) {
	if s.TimerOn {
		fmt.Fprintf(s.Out, "Run Time: real %.3f ms\n", float64(elapsed.Microseconds())/1000.0)
	}
	if s.StatsOn {
		fmt.Fprintf(s.Out, "rows: %d  duration: %.3fms  schema-changing: %v\n", rows, float64(elapsed.Microseconds())/1000.0, schemaChanged)
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
