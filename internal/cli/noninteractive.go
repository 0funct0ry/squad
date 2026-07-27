package cli

import (
	"bufio"
	"io"
	"os"
	"strings"
)

// RunInline runs a single SQL argument (`squad cli db.sqlite "SELECT ...;"`)
// and returns. No banner, no readline, .mode defaults to list.
func RunInline(s *State, sql string) {
	s.Execute(sql)
}

// RunScript reads from r (stdin, piped/heredoc, or a `.read FILE`) until EOF
// and executes each statement/dot-command in order. No banner, no readline,
// .mode defaults to list.
//
// Dot-commands are newline-terminated (like sqlite3's own shell), not
// semicolon-terminated, so this can't just hand the whole input to
// db.SplitStatements the way a pure-SQL batch would -- it buffers SQL lines
// until a semicolon terminator (same rule as the REPL loop in repl.go) but
// dispatches a "." line immediately once the buffer is empty.
func RunScript(s *State, r io.Reader) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)

	var buf strings.Builder
	for scanner.Scan() {
		line := scanner.Text()

		if buf.Len() == 0 {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" {
				continue
			}
			if strings.HasPrefix(trimmed, ".") {
				s.Execute(trimmed)
				if s.Quit {
					return nil
				}
				continue
			}
		}

		buf.WriteString(line)
		buf.WriteString("\n")

		if statementComplete(buf.String()) {
			statement := buf.String()
			buf.Reset()
			s.Execute(statement)
			if s.Quit {
				return nil
			}
		}
	}
	return scanner.Err()
}

// IsStdinTerminal reports whether stdin is a terminal (vs. piped/redirected).
func IsStdinTerminal() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}
