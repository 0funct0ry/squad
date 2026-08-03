package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/0funct0ry/squad/internal/db"
	"github.com/ergochat/readline"
	"github.com/fatih/color"
)

// historyFilePath returns ~/.squad_history, or "" if the home dir can't be
// resolved -- readline treats an empty HistoryFile as "disable persistence",
// which is exactly the silent-skip behavior required here.
func historyFilePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	path := filepath.Join(home, ".squad_history")
	if _, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600); err != nil {
		return ""
	}
	return path
}

// RunREPL starts the interactive readline-based shell.
func RunREPL(s *State) error {
	s.Colorized = color.NoColor == false

	printBanner(s)

	rl, err := readline.NewEx(&readline.Config{
		Prompt:          RenderPrompt(s, s.Prompt),
		HistoryFile:     historyFilePath(),
		AutoComplete:    newCompleter(s),
		InterruptPrompt: "^C",
		EOFPrompt:       "^D",
		Listener:        abbrListener(s),
	})
	if err != nil {
		return err
	}
	defer rl.Close()
	defer func() {
		if s.RestManager != nil {
			_ = s.RestManager.Stop("cli exit")
		}
	}()

	var buf strings.Builder
	for {
		if s.pendingDefault != "" {
			rl.SetDefault(s.pendingDefault)
			s.pendingDefault = ""
		}

		rl.SetPrompt(RenderPrompt(s, s.Prompt))
		if buf.Len() > 0 {
			rl.SetPrompt(RenderPrompt(s, s.ContinuationPrompt))
		}

		line, err := rl.Readline()
		if errors.Is(err, readline.ErrInterrupt) {
			buf.Reset()
			continue
		}
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}

		if buf.Len() == 0 {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, ".") {
				s.History = append(s.History, trimmed)
				s.Execute(trimmed)
				if s.Quit {
					return nil
				}
				continue
			}
			if trimmed == "" {
				continue
			}
		}

		buf.WriteString(line)
		buf.WriteString("\n")

		if !statementComplete(buf.String()) {
			continue
		}

		statement := buf.String()
		buf.Reset()
		s.History = append(s.History, strings.TrimSpace(statement))
		s.Execute(statement)
		if s.Quit {
			return nil
		}
	}
}

// abbrListener returns a readline.Listener that expands a
// "<trigger>name " token into its defined abbreviation live in the input
// buffer, where <trigger> is s.AbbrTrigger (default ":", configurable via
// `squad cli --abbr-trigger`/`-A`). It only ever acts on the Space keypress;
// every other key is a cheap no-op (ok=false, buffer unchanged) since Do is
// invoked on literally every keystroke.
func abbrListener(s *State) readline.Listener {
	return func(line []rune, pos int, key rune) ([]rune, int, bool) {
		if key != ' ' {
			return nil, 0, false
		}
		s.loadAbbrs()
		if len(s.Abbrs) == 0 {
			return nil, 0, false
		}
		if pos == 0 || line[pos-1] != ' ' {
			// The just-typed space is already written into the buffer at
			// pos-1 by the time the Listener runs; if it isn't there,
			// something else changed the buffer (paste, etc.) -- no-op.
			return nil, 0, false
		}

		trigger := s.abbrTrigger()
		before := string(line[:pos-1])
		start := len(before)
		for start > 0 {
			c := before[start-1]
			if c == ' ' || c == '\t' {
				break
			}
			start--
		}
		token := before[start:]
		if !strings.HasPrefix(token, trigger) {
			return nil, 0, false
		}
		expansion, ok := s.Abbrs[strings.TrimPrefix(token, trigger)]
		if !ok {
			return nil, 0, false
		}

		newLine := before[:start] + expansion + " " + string(line[pos:])
		newPos := start + len(expansion) + 1
		return []rune(newLine), newPos, true
	}
}

// statementComplete reports whether buf contains at least one semicolon
// terminator outside quotes/comments/BEGIN...END bodies, reusing
// internal/db's own statement splitter rather than a second implementation.
func statementComplete(buf string) bool {
	stripped, err := db.StripCommentsAndWhitespace(buf)
	if err != nil {
		return false
	}
	if stripped == "" {
		return false
	}
	statements, err := db.SplitStatements(buf)
	if err != nil {
		return false
	}
	// SplitStatements only appends a statement once it sees the terminating
	// semicolon (any trailing partial statement is never appended), so at
	// least one complete statement means the trailing ";" was reached.
	if len(statements) == 0 {
		return false
	}
	return strings.TrimRight(buf, " \t\n") != "" && strings.HasSuffix(strings.TrimRight(buf, " \t\n\r"), ";")
}

func printBanner(s *State) {
	version, _, err := db.Meta(s.DB, s.Path)
	if err != nil {
		version = "unknown"
	}
	mode := "READ-ONLY"
	modeColor := color.New(color.FgWhite)
	if s.Write {
		mode = "WRITE"
		modeColor = color.New(color.FgYellow)
	}
	if s.Colorized {
		mode = modeColor.Sprint(mode)
	}
	fmt.Fprintf(s.Out, "squad CLI (SQLite version %s)\n", version)
	fmt.Fprintf(s.Out, "connected to: %s (%s)\n", s.Path, mode)
	fmt.Fprintln(s.Out, `Enter ".help" for usage hints.`)
}
