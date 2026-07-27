package cli

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/0funct0ry/squad/internal/db"
)

// splitFileAndQuery splits ".save FILE QUERY" style text into the file token
// and the remaining query text, honoring quotes/brackets/backticks the same
// way internal/db/classify.go's scanners do: a whitespace run inside
// '...'/"..."/[...]/`...` does not count as the file/query separator.
func splitFileAndQuery(text string) (file string, query string, err error) {
	var inSingle, inDouble, inBracket, inBacktick bool
	i := 0
	n := len(text)
	for i < n {
		c := text[i]
		switch {
		case c == '\'' && !inDouble && !inBracket && !inBacktick:
			inSingle = !inSingle
		case c == '"' && !inSingle && !inBracket && !inBacktick:
			inDouble = !inDouble
		case c == '[' && !inSingle && !inDouble && !inBacktick:
			inBracket = true
		case c == ']' && inBracket:
			inBracket = false
		case c == '`' && !inSingle && !inDouble && !inBracket:
			inBacktick = !inBacktick
		case (c == ' ' || c == '\t') && !inSingle && !inDouble && !inBracket && !inBacktick:
			return text[:i], strings.TrimSpace(text[i:]), nil
		}
		i++
	}
	if inSingle || inDouble || inBracket || inBacktick {
		return "", "", fmt.Errorf("unterminated quote/bracket in .save arguments")
	}
	return text, "", nil
}

// cmdSave executes a quoted query and writes its rendered output to a file
// using the current .mode/.headers/.nullvalue, without touching
// State.Out/.once -- unlike .output/.once, this is a one-shot,
// self-contained redirect around a single query it runs itself.
func (s *State) cmdSave(rest string) {
	file, query, err := splitFileAndQuery(rest)
	if err != nil {
		s.shellError(err)
		return
	}
	if file == "" || query == "" {
		s.shellError(fmt.Errorf(`usage: .save FILE "QUERY"`))
		return
	}
	query = unquoteDotCommandText(query)

	rendered, err := preprocessTemplate(query)
	if err != nil {
		s.shellError(err)
		return
	}
	class, err := db.Classify(rendered)
	if err != nil {
		s.shellError(err)
		return
	}
	if class == db.ClassWrite {
		s.shellError(fmt.Errorf(".save only supports SELECT-shaped queries"))
		return
	}

	f, err := os.Create(file)
	if err != nil {
		s.shellError(err)
		return
	}
	defer f.Close()

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
	if err := s.renderTo(f, cols, resultRows); err != nil {
		s.shellError(err)
	}
}

// cmdGrep filters the last SELECT-shaped result set's rows (State.LastColumns/
// LastRows, populated by Execute) by a substring (default) or regex (-r/
// --regex) match across all columns, re-rendering only matching rows in the
// current .mode. It never re-queries the DB.
func (s *State) cmdGrep(args []string) {
	useRegex := false
	var patternParts []string
	for _, a := range args {
		if a == "-r" || a == "--regex" {
			useRegex = true
			continue
		}
		patternParts = append(patternParts, a)
	}
	if len(patternParts) == 0 {
		s.shellError(fmt.Errorf("usage: .grep ?-r|--regex? PATTERN"))
		return
	}
	pattern := strings.Join(patternParts, " ")
	if s.LastColumns == nil {
		s.shellError(fmt.Errorf("no prior result set to grep (run a SELECT first)"))
		return
	}

	var matcher func(string) bool
	if useRegex {
		re, err := regexp.Compile(pattern)
		if err != nil {
			s.shellError(err)
			return
		}
		matcher = re.MatchString
	} else {
		matcher = func(v string) bool { return strings.Contains(v, pattern) }
	}

	var matched [][]any
	for _, row := range s.LastRows {
		for _, v := range row {
			if matcher(s.formatValue(v)) {
				matched = append(matched, row)
				break
			}
		}
	}
	if err := s.Render(s.LastColumns, matched); err != nil {
		s.shellError(err)
	}
}
