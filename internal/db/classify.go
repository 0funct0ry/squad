package db

import (
	"errors"
	"strings"
	"unicode"
)

// Classification results
const (
	ClassRead  = "READ"
	ClassWrite = "WRITE"
)

// ErrEmptyQuery is returned when the query is empty or comment-only
var ErrEmptyQuery = errors.New("BAD_REQUEST")

// SplitStatements splits a SQL string into individual statements by semicolon.
// It respects string literals, quoted identifiers, and SQL comments.
func SplitStatements(sql string) ([]string, error) {
	var statements []string
	var current strings.Builder

	inSingleQuote := false
	inDoubleQuote := false
	inBracket := false
	inBacktick := false
	inLineComment := false
	inBlockComment := false

	// CREATE TRIGGER bodies are wrapped in BEGIN ... END and contain their
	// own semicolon-terminated statements internally (e.g. an UPDATE inside
	// the trigger). A semicolon must only end the outer statement when it
	// isn't nested inside such a block, so track BEGIN/END as word-bounded
	// keywords (case-insensitive) outside of quotes/comments/identifiers.
	// CASE ... END expressions (e.g. inside an UPDATE's SET clause within a
	// trigger body) also close with a bare END, so CASE counts as an opener
	// too -- otherwise its END would prematurely close the enclosing
	// trigger's BEGIN block and split the trigger body mid-statement.
	beginDepth := 0

	runes := []rune(sql)
	n := len(runes)

	isWordChar := func(r rune) bool {
		return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
	}

	// matchKeyword reports whether runes[i:] starts with word, matched
	// case-insensitively and bounded by non-word characters on both sides.
	matchKeyword := func(i int, word string) bool {
		wr := []rune(word)
		if i+len(wr) > n {
			return false
		}
		if i > 0 && isWordChar(runes[i-1]) {
			return false
		}
		for j, w := range wr {
			if unicode.ToUpper(runes[i+j]) != w {
				return false
			}
		}
		end := i + len(wr)
		if end < n && isWordChar(runes[end]) {
			return false
		}
		return true
	}

	for i := 0; i < n; i++ {
		r := runes[i]

		// Check for exit from comments
		if inLineComment {
			current.WriteRune(r)
			if r == '\n' {
				inLineComment = false
			}
			continue
		}
		if inBlockComment {
			current.WriteRune(r)
			if r == '/' && i > 0 && runes[i-1] == '*' {
				inBlockComment = false
			}
			continue
		}

		// Check for exit from quotes/identifiers
		if inSingleQuote {
			current.WriteRune(r)
			if r == '\'' {
				// Check for escaped quote ''
				if i+1 < n && runes[i+1] == '\'' {
					current.WriteRune(runes[i+1])
					i++ // Skip next quote
				} else {
					inSingleQuote = false
				}
			}
			continue
		}
		if inDoubleQuote {
			current.WriteRune(r)
			if r == '"' {
				if i+1 < n && runes[i+1] == '"' {
					current.WriteRune(runes[i+1])
					i++
				} else {
					inDoubleQuote = false
				}
			}
			continue
		}
		if inBacktick {
			current.WriteRune(r)
			if r == '`' {
				inBacktick = false
			}
			continue
		}
		if inBracket {
			current.WriteRune(r)
			if r == ']' {
				inBracket = false
			}
			continue
		}

		// Check for entering comments
		if r == '-' && i+1 < n && runes[i+1] == '-' {
			inLineComment = true
			current.WriteRune(r)
			current.WriteRune(runes[i+1])
			i++
			continue
		}
		if r == '/' && i+1 < n && runes[i+1] == '*' {
			inBlockComment = true
			current.WriteRune(r)
			current.WriteRune(runes[i+1])
			i++
			continue
		}

		// Check for entering quotes/identifiers
		if r == '\'' {
			inSingleQuote = true
			current.WriteRune(r)
			continue
		}
		if r == '"' {
			inDoubleQuote = true
			current.WriteRune(r)
			continue
		}
		if r == '`' {
			inBacktick = true
			current.WriteRune(r)
			continue
		}
		if r == '[' {
			inBracket = true
			current.WriteRune(r)
			continue
		}

		if matchKeyword(i, "BEGIN") || matchKeyword(i, "CASE") {
			beginDepth++
		} else if matchKeyword(i, "END") {
			if beginDepth > 0 {
				beginDepth--
			}
		}

		// Check for semicolon — only ends a statement outside a BEGIN...END
		// block (e.g. a trigger body), whose internal semicolons belong to
		// the enclosing CREATE TRIGGER statement.
		if r == ';' && beginDepth == 0 {
			statements = append(statements, current.String())
			current.Reset()
			continue
		}

		current.WriteRune(r)
	}

	// Add remaining part
	if current.Len() > 0 {
		statements = append(statements, current.String())
	}

	return statements, nil
}

// StripCommentsAndWhitespace removes all line comments, block comments, and trims leading/trailing whitespace.
func StripCommentsAndWhitespace(sql string) (string, error) {
	var clean strings.Builder

	inSingleQuote := false
	inDoubleQuote := false
	inBracket := false
	inBacktick := false
	inLineComment := false
	inBlockComment := false

	runes := []rune(sql)
	n := len(runes)

	for i := 0; i < n; i++ {
		r := runes[i]

		if inLineComment {
			if r == '\n' {
				inLineComment = false
				clean.WriteRune('\n') // Keep newlines for spacing
			}
			continue
		}
		if inBlockComment {
			if r == '/' && i > 0 && runes[i-1] == '*' {
				inBlockComment = false
			}
			continue
		}

		if inSingleQuote {
			clean.WriteRune(r)
			if r == '\'' {
				if i+1 < n && runes[i+1] == '\'' {
					clean.WriteRune(runes[i+1])
					i++
				} else {
					inSingleQuote = false
				}
			}
			continue
		}
		if inDoubleQuote {
			clean.WriteRune(r)
			if r == '"' {
				if i+1 < n && runes[i+1] == '"' {
					clean.WriteRune(runes[i+1])
					i++
				} else {
					inDoubleQuote = false
				}
			}
			continue
		}
		if inBacktick {
			clean.WriteRune(r)
			if r == '`' {
				inBacktick = false
			}
			continue
		}
		if inBracket {
			clean.WriteRune(r)
			if r == ']' {
				inBracket = false
			}
			continue
		}

		// Enter comment states
		if r == '-' && i+1 < n && runes[i+1] == '-' {
			inLineComment = true
			i++
			continue
		}
		if r == '/' && i+1 < n && runes[i+1] == '*' {
			inBlockComment = true
			i++
			continue
		}

		// Enter quotes/identifiers
		if r == '\'' {
			inSingleQuote = true
			clean.WriteRune(r)
			continue
		}
		if r == '"' {
			inDoubleQuote = true
			clean.WriteRune(r)
			continue
		}
		if r == '`' {
			inBacktick = true
			clean.WriteRune(r)
			continue
		}
		if r == '[' {
			inBracket = true
			clean.WriteRune(r)
			continue
		}

		clean.WriteRune(r)
	}

	res := strings.TrimSpace(clean.String())
	return res, nil
}

func getFirstWord(s string) string {
	var word strings.Builder
	for _, r := range s {
		if unicode.IsSpace(r) {
			if word.Len() > 0 {
				break
			}
			continue
		}
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
			word.WriteRune(r)
		} else {
			if word.Len() > 0 {
				break
			}
		}
	}
	return word.String()
}

// parseAfterWith takes the SQL string after "WITH" (or "WITH RECURSIVE")
// and returns the keyword of the main query that follows the CTE definitions.
func parseAfterWith(s string) (string, error) {
	runes := []rune(s)
	n := len(runes)
	i := 0

	// Helper to skip whitespace and comments
	skipWhitespaceAndComments := func() {
		for i < n {
			if unicode.IsSpace(runes[i]) {
				i++
				continue
			}
			if runes[i] == '-' && i+1 < n && runes[i+1] == '-' {
				i += 2
				for i < n && runes[i] != '\n' {
					i++
				}
				continue
			}
			if runes[i] == '/' && i+1 < n && runes[i+1] == '*' {
				i += 2
				for i < n {
					if runes[i] == '/' && runes[i-1] == '*' {
						i++
						break
					}
					i++
				}
				continue
			}
			break
		}
	}

	// Helper to read a word
	readWord := func() string {
		skipWhitespaceAndComments()
		var word strings.Builder
		for i < n {
			r := runes[i]
			if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
				word.WriteRune(r)
				i++
			} else {
				break
			}
		}
		return word.String()
	}

	// Helper to skip balanced parentheses
	skipBalancedParentheses := func() error {
		skipWhitespaceAndComments()
		if i >= n || runes[i] != '(' {
			return errors.New("expected opening parenthesis")
		}
		i++ // consume '('
		depth := 1
		inSingleQuote := false
		inDoubleQuote := false
		inBracket := false
		inBacktick := false
		inLineComment := false
		inBlockComment := false

		for i < n {
			r := runes[i]
			if inLineComment {
				if r == '\n' {
					inLineComment = false
				}
				i++
				continue
			}
			if inBlockComment {
				if r == '/' && i > 0 && runes[i-1] == '*' {
					inBlockComment = false
				}
				i++
				continue
			}
			if inSingleQuote {
				if r == '\'' {
					if i+1 < n && runes[i+1] == '\'' {
						i++ // skip escaped quote
					} else {
						inSingleQuote = false
					}
				}
				i++
				continue
			}
			if inDoubleQuote {
				if r == '"' {
					if i+1 < n && runes[i+1] == '"' {
						i++
					} else {
						inDoubleQuote = false
					}
				}
				i++
				continue
			}
			if inBacktick {
				if r == '`' {
					inBacktick = false
				}
				i++
				continue
			}
			if inBracket {
				if r == ']' {
					inBracket = false
				}
				i++
				continue
			}

			// Enter comments/quotes
			if r == '-' && i+1 < n && runes[i+1] == '-' {
				inLineComment = true
				i += 2
				continue
			}
			if r == '/' && i+1 < n && runes[i+1] == '*' {
				inBlockComment = true
				i += 2
				continue
			}
			if r == '\'' {
				inSingleQuote = true
				i++
				continue
			}
			if r == '"' {
				inDoubleQuote = true
				i++
				continue
			}
			if r == '`' {
				inBacktick = true
				i++
				continue
			}
			if r == '[' {
				inBracket = true
				i++
				continue
			}

			if r == '(' {
				depth++
			} else if r == ')' {
				depth--
				if depth == 0 {
					i++ // consume ')'
					return nil
				}
			}
			i++
		}
		return errors.New("unbalanced parentheses")
	}

	skipWhitespaceAndComments()
	startWord := readWord()
	if strings.ToUpper(startWord) == "RECURSIVE" {
		// Consume first CTE name
		_ = readWord()
	}

	for {
		skipWhitespaceAndComments()

		if i < n && runes[i] == '(' {
			if err := skipBalancedParentheses(); err != nil {
				return "", err
			}
		}

		asWord := readWord()
		if strings.ToUpper(asWord) != "AS" {
			return "", errors.New("expected AS keyword in CTE")
		}

		if err := skipBalancedParentheses(); err != nil {
			return "", err
		}

		skipWhitespaceAndComments()
		if i < n && runes[i] == ',' {
			i++ // consume comma
			_ = readWord()
			continue
		}

		finalKeyword := readWord()
		return strings.ToUpper(finalKeyword), nil
	}
}

func classifyPragma(pragmaSql string) string {
	runes := []rune(pragmaSql)
	n := len(runes)

	i := 6 // skip "PRAGMA"

	skipWhitespaceAndComments := func() {
		for i < n {
			if unicode.IsSpace(runes[i]) {
				i++
				continue
			}
			if runes[i] == '-' && i+1 < n && runes[i+1] == '-' {
				i += 2
				for i < n && runes[i] != '\n' {
					i++
				}
				continue
			}
			if runes[i] == '/' && i+1 < n && runes[i+1] == '*' {
				i += 2
				for i < n {
					if runes[i] == '/' && runes[i-1] == '*' {
						i++
						break
					}
					i++
				}
				continue
			}
			break
		}
	}

	skipWhitespaceAndComments()

	var nameBuilder strings.Builder
	for i < n {
		r := runes[i]
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '.' {
			nameBuilder.WriteRune(r)
			i++
		} else {
			break
		}
	}

	pragmaName := nameBuilder.String()
	if idx := strings.Index(pragmaName, "."); idx != -1 {
		pragmaName = pragmaName[idx+1:]
	}
	pragmaName = strings.ToLower(pragmaName)

	readOnlyPragmas := map[string]bool{
		"table_info":       true,
		"index_list":       true,
		"index_info":       true,
		"foreign_key_list": true,
		"database_list":    true,
		"table_xinfo":      true,
		"index_xinfo":      true,
		"compile_options":  true,
		"user_version":     true,
		"application_id":   true,
	}

	if !readOnlyPragmas[pragmaName] {
		return ClassWrite
	}

	bareReadOnly := map[string]bool{
		"user_version":    true,
		"application_id":  true,
		"database_list":   true,
		"compile_options": true,
	}

	skipWhitespaceAndComments()
	if i < n {
		r := runes[i]
		if r == '=' {
			return ClassWrite
		}
		if r == '(' && bareReadOnly[pragmaName] {
			return ClassWrite
		}
	}

	return ClassRead
}

// Classify determines whether a SQL string is a "READ" or "WRITE" statement.
func Classify(sql string) (string, error) {
	clean, err := StripCommentsAndWhitespace(sql)
	if err != nil {
		return "", err
	}
	if clean == "" {
		return "", ErrEmptyQuery
	}

	keyword := strings.ToUpper(getFirstWord(clean))
	if keyword == "" {
		return ClassWrite, nil // Fail closed
	}

	switch keyword {
	case "SELECT", "EXPLAIN", "VALUES":
		return ClassRead, nil
	case "WITH":
		// Find CTE content length or skip CTE keyword and recurse on remainder
		afterWith := clean[4:]
		finalKeyword, err := parseAfterWith(afterWith)
		if err != nil {
			return ClassWrite, nil // Fail closed on parsing error
		}
		if finalKeyword == "SELECT" || finalKeyword == "VALUES" {
			return ClassRead, nil
		}
		return ClassWrite, nil
	case "PRAGMA":
		return classifyPragma(clean), nil
	default:
		return ClassWrite, nil
	}
}
