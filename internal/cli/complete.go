package cli

import (
	"strings"
)

// sqlKeywords is the standard SQL clause/keyword completion set (SPEC item 3).
var sqlKeywords = []string{
	"SELECT", "FROM", "WHERE", "GROUP BY", "ORDER BY", "LIMIT", "JOIN",
	"INNER JOIN", "LEFT JOIN", "ON", "INSERT INTO", "VALUES", "UPDATE", "SET",
	"DELETE FROM", "CREATE TABLE", "CREATE INDEX", "DROP TABLE", "ALTER TABLE",
	"AND", "OR", "NOT", "NULL", "AS", "DISTINCT", "HAVING", "IN", "LIKE",
	"BETWEEN", "IS",
}

// completer implements readline.AutoCompleter. Line starting with "." gets
// dot-command completion (with a second level for .mode/.schema/.indexes/
// .import); anything else gets SQL keyword + table + column completion.
type completer struct {
	state *State
}

func newCompleter(s *State) *completer {
	return &completer{state: s}
}

// Do implements readline.AutoCompleter: given the full line and cursor
// offset, return candidate completions (the suffix each would append) and
// how many trailing runes of line they replace.
func (c *completer) Do(line []rune, pos int) ([][]rune, int) {
	text := string(line[:pos])

	if strings.HasPrefix(strings.TrimLeft(text, " "), ".") {
		return c.completeDot(text)
	}
	return c.completeSQL(text)
}

func currentWord(text string) (word string, wordStart int) {
	i := len(text)
	for i > 0 {
		r := text[i-1]
		if r == ' ' || r == '\t' || r == '\n' || r == '(' || r == ',' {
			break
		}
		i--
	}
	return text[i:], i
}

func toCandidates(word string, options []string) ([][]rune, int) {
	var out [][]rune
	lowerWord := strings.ToLower(word)
	for _, opt := range options {
		if strings.HasPrefix(strings.ToLower(opt), lowerWord) {
			out = append(out, []rune(opt[len(word):]))
		}
	}
	return out, len(word)
}

func (c *completer) completeDot(text string) ([][]rune, int) {
	fields := strings.Fields(text)
	endsWithSpace := strings.HasSuffix(text, " ")

	if len(fields) <= 1 && !endsWithSpace {
		word, _ := currentWord(text)
		return toCandidates(word, dotCommandNames)
	}

	cmd := fields[0]
	word := ""
	if !endsWithSpace {
		word, _ = currentWord(text)
	}

	switch cmd {
	case ".mode":
		return toCandidates(word, modeNames())
	case ".schema", ".indexes", ".import", ".rest", ".clone", ".diff", ".constraints", ".seed", ".backup", ".grep":
		return toCandidates(word, c.state.cachedTables())
	case ".stat":
		return toCandidates(word, []string{"db"})
	case ".listener":
		return toCandidates(word, []string{"start", "stop"})
	case ".timer", ".stats", ".headers":
		return toCandidates(word, []string{"on", "off"})
	case ".bookmark":
		return toCandidates(word, []string{"save", "load"})
	default:
		return nil, 0
	}
}

// completeSQL offers keywords plus table names, and (once a table name has
// appeared earlier in the buffer) that table's columns; ambiguous multi-table
// buffers offer the union of all referenced tables' columns.
func (c *completer) completeSQL(text string) ([][]rune, int) {
	word, _ := currentWord(text)

	tables := c.state.cachedTables()
	var options []string
	options = append(options, sqlKeywords...)
	options = append(options, tables...)

	referenced := referencedTables(text, tables)
	seen := map[string]bool{}
	for _, t := range referenced {
		for _, col := range c.state.cachedColumns(t) {
			if !seen[col] {
				seen[col] = true
				options = append(options, col)
			}
		}
	}

	return toCandidates(word, options)
}

// referencedTables does a best-effort scan for table names that already
// appear as whole words in the buffer, for column completion.
func referencedTables(text string, tables []string) []string {
	upper := strings.ToUpper(text)
	var found []string
	for _, t := range tables {
		if strings.Contains(upper, strings.ToUpper(t)) {
			found = append(found, t)
		}
	}
	return found
}
