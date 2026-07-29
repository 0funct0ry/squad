package db

import (
	"strings"
	"unicode"
)

// AnalyzeSelect inspects a single SQL statement and, when it is a plain
// `SELECT ... FROM <single base table>` with no joins, set operations,
// grouping, aggregates, or computed columns, returns the source table name
// and the set of columns needed to uniquely address a row (its primary key,
// or "rowid" for rowid tables with no explicit PK).
//
// This is intentionally conservative: it only ever returns ok=true for
// queries the UI can safely treat as row-editable. Anything it can't prove
// safe (joins, CTEs, subqueries in FROM, GROUP BY/DISTINCT, aggregate or
// window functions, computed expressions, multi-table FROM lists, or views)
// is rejected rather than partially supported. schemaLookup must return
// ErrNotFound (or any error) for names that aren't base tables; views are
// deliberately excluded from v1.
func AnalyzeSelect(sql string, schemaLookup func(table string) (*TableSchema, error)) (sourceTable string, primaryKeyColumns []string, ok bool) {
	clean, err := StripCommentsAndWhitespace(sql)
	if err != nil || clean == "" {
		return "", nil, false
	}

	runes := []rune(clean)
	n := len(runes)
	i := 0

	isWordChar := func(r rune) bool {
		return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
	}

	skipSpace := func() {
		for i < n && unicode.IsSpace(runes[i]) {
			i++
		}
	}

	// peekWord returns the upper-cased word starting at i without advancing.
	peekWordAt := func(pos int) string {
		if pos > 0 && isWordChar(runes[pos-1]) {
			return ""
		}
		j := pos
		for j < n && isWordChar(runes[j]) {
			j++
		}
		return strings.ToUpper(string(runes[pos:j]))
	}

	readWord := func() string {
		skipSpace()
		start := i
		for i < n && isWordChar(runes[i]) {
			i++
		}
		return string(runes[start:i])
	}

	// readIdentifier reads a bare or quoted identifier, returning the
	// unquoted name.
	readIdentifier := func() (string, bool) {
		skipSpace()
		if i >= n {
			return "", false
		}
		switch runes[i] {
		case '"', '`':
			quote := runes[i]
			i++
			start := i
			var sb strings.Builder
			for i < n {
				if runes[i] == quote {
					if i+1 < n && runes[i+1] == quote {
						sb.WriteRune(quote)
						i += 2
						continue
					}
					name := sb.String()
					i++
					return name, true
				}
				sb.WriteRune(runes[i])
				i++
			}
			_ = start
			return "", false
		case '[':
			i++
			start := i
			for i < n && runes[i] != ']' {
				i++
			}
			name := string(runes[start:i])
			if i < n {
				i++
			}
			return name, true
		default:
			if !isWordChar(runes[i]) {
				return "", false
			}
			w := readWord()
			if w == "" {
				return "", false
			}
			return w, true
		}
	}

	// skipBalancedParens assumes runes[i] == '(' and advances i past the
	// matching ')', tracking quotes so embedded parens/quotes don't confuse
	// depth tracking. Returns false on unbalanced input.
	skipBalancedParens := func() bool {
		if i >= n || runes[i] != '(' {
			return false
		}
		depth := 0
		for i < n {
			r := runes[i]
			switch r {
			case '\'':
				i++
				for i < n {
					if runes[i] == '\'' {
						if i+1 < n && runes[i+1] == '\'' {
							i += 2
							continue
						}
						i++
						break
					}
					i++
				}
				continue
			case '"':
				i++
				for i < n && runes[i] != '"' {
					i++
				}
				if i < n {
					i++
				}
				continue
			case '(':
				depth++
				i++
				continue
			case ')':
				depth--
				i++
				if depth == 0 {
					return true
				}
				continue
			default:
				i++
			}
		}
		return false
	}

	// ---- 1. First keyword must be SELECT (reject WITH/CTEs and everything else) ----
	skipSpace()
	firstWord := strings.ToUpper(peekWordAt(i))
	if firstWord != "SELECT" {
		return "", nil, false
	}
	i += len([]rune(firstWord))

	// Reject SELECT DISTINCT outright (changes row identity semantics).
	skipSpace()
	if strings.ToUpper(peekWordAt(i)) == "DISTINCT" {
		return "", nil, false
	}
	if strings.ToUpper(peekWordAt(i)) == "ALL" {
		i += len("ALL")
	}

	// ---- 2. Scan the column list up to the top-level FROM keyword ----
	colListStart := i
	depth := 0
	fromIdx := -1
	for i < n {
		r := runes[i]
		switch r {
		case '\'':
			i++
			for i < n {
				if runes[i] == '\'' {
					if i+1 < n && runes[i+1] == '\'' {
						i += 2
						continue
					}
					i++
					break
				}
				i++
			}
			continue
		case '"', '`':
			quote := r
			i++
			for i < n && runes[i] != quote {
				i++
			}
			if i < n {
				i++
			}
			continue
		case '(':
			depth++
			i++
			continue
		case ')':
			depth--
			i++
			continue
		default:
			if depth == 0 {
				if peekWordAt(i) == "FROM" {
					fromIdx = i
				}
			}
		}
		if fromIdx != -1 {
			break
		}
		i++
	}
	if fromIdx == -1 {
		return "", nil, false
	}
	colListEnd := fromIdx
	colListRaw := strings.TrimSpace(string(runes[colListStart:colListEnd]))
	if colListRaw == "" {
		return "", nil, false
	}

	// Split the column list on top-level commas and reject anything that
	// isn't a bare (optionally qualified/quoted) column reference or "*".
	var selectedCols []string
	selectAll := false
	{
		clRunes := []rune(colListRaw)
		cn := len(clRunes)
		start := 0
		d := 0
		items := []string{}
		for k := 0; k <= cn; k++ {
			if k == cn || (d == 0 && clRunes[k] == ',') {
				items = append(items, strings.TrimSpace(string(clRunes[start:k])))
				start = k + 1
				continue
			}
			switch clRunes[k] {
			case '(':
				d++
			case ')':
				d--
			}
		}
		for _, item := range items {
			if item == "" {
				return "", nil, false
			}
			if item == "*" {
				selectAll = true
				continue
			}
			if strings.Contains(item, "(") {
				// function call, window function, or expression with parens.
				return "", nil, false
			}
			// Reject arithmetic/concat operators anywhere in the item
			// (e.g. "email || name") and anything with a space beyond a
			// single alias token (col AS alias / col alias).
			for _, op := range []string{"||", "+", "-", "*", "/", "%"} {
				if strings.Contains(item, op) {
					return "", nil, false
				}
			}
			fields := strings.Fields(item)
			if len(fields) == 0 {
				return "", nil, false
			}
			colToken := fields[0]
			if strings.HasSuffix(colToken, ".*") {
				selectAll = true
				continue
			}
			// Strip a possible table-qualifier prefix ("t.col" -> "col").
			name := colToken
			if idx := strings.LastIndex(name, "."); idx != -1 {
				name = name[idx+1:]
			}
			name = strings.Trim(name, `"`+"`"+`[]`)
			if name == "" {
				return "", nil, false
			}
			selectedCols = append(selectedCols, name)
		}
	}

	// ---- 3. FROM clause: must be exactly one bare table reference ----
	i = fromIdx
	i += len("FROM")
	skipSpace()

	tableName, okName := readIdentifier()
	if !okName {
		// Subquery or table-valued function in FROM.
		return "", nil, false
	}

	// Optional alias: `AS ident` or a bare trailing identifier.
	skipSpace()
	if strings.ToUpper(peekWordAt(i)) == "AS" {
		i += len("AS")
		skipSpace()
		if _, ok := readIdentifier(); !ok {
			return "", nil, false
		}
	} else if i < n && isWordChar(runes[i]) {
		word := peekWordAt(i)
		if word != "" && !isClauseKeyword(word) && !isJoinKeyword(word) {
			i += len([]rune(word))
		}
	}

	// ---- 4. Reject anything indicating a join, set op, grouping, or multi-table FROM ----
	skipSpace()
	if i < n && runes[i] == ',' {
		// Comma-separated FROM list (old-style join).
		return "", nil, false
	}

	rest := string(runes[i:])
	restUpper := strings.ToUpper(rest)
	if containsTopLevelKeyword(rest, "JOIN") ||
		containsTopLevelKeyword(restUpper, "GROUP") ||
		containsTopLevelKeyword(restUpper, "UNION") ||
		containsTopLevelKeyword(restUpper, "INTERSECT") ||
		containsTopLevelKeyword(restUpper, "EXCEPT") {
		return "", nil, false
	}

	// Also guard against a subquery snuck in via a parenthesized FROM item
	// (readIdentifier already rejects a leading '(', so this is covered).
	_ = skipBalancedParens

	// ---- 5. Resolve schema, validate PK coverage ----
	schema, err := schemaLookup(tableName)
	if err != nil || schema == nil {
		return "", nil, false
	}
	if schema.Type != "" && schema.Type != "table" {
		// Views excluded from v1.
		return "", nil, false
	}

	allColumnNames := make(map[string]bool, len(schema.Columns))
	for _, c := range schema.Columns {
		allColumnNames[strings.ToLower(c.Name)] = true
	}

	selectedSet := make(map[string]bool)
	if selectAll {
		for _, c := range schema.Columns {
			selectedSet[strings.ToLower(c.Name)] = true
		}
	} else {
		for _, c := range selectedCols {
			selectedSet[strings.ToLower(c)] = true
		}
	}

	var pkCols []string
	if len(schema.PrimaryKey) > 0 {
		for _, pk := range schema.PrimaryKey {
			if !selectedSet[strings.ToLower(pk)] {
				return "", nil, false
			}
		}
		pkCols = append(pkCols, schema.PrimaryKey...)
	} else if !schema.WithoutRowid {
		if !selectedSet["rowid"] && !selectedSet["_rowid_"] && !selectedSet["oid"] {
			return "", nil, false
		}
		pkCols = []string{"rowid"}
	} else {
		// WITHOUT ROWID table with no discoverable PK - shouldn't happen,
		// but stay conservative.
		return "", nil, false
	}

	return tableName, pkCols, true
}

var clauseKeywords = map[string]bool{
	"WHERE": true, "GROUP": true, "ORDER": true, "LIMIT": true,
	"HAVING": true, "UNION": true, "INTERSECT": true, "EXCEPT": true,
	"WINDOW": true,
}

var joinKeywords = map[string]bool{
	"JOIN": true, "INNER": true, "LEFT": true, "RIGHT": true, "FULL": true,
	"CROSS": true, "NATURAL": true,
}

func isClauseKeyword(word string) bool {
	return clauseKeywords[word]
}

func isJoinKeyword(word string) bool {
	return joinKeywords[word]
}

// containsTopLevelKeyword reports whether word appears as a word-bounded
// token in s outside of quotes/parens.
func containsTopLevelKeyword(s string, word string) bool {
	runes := []rune(s)
	n := len(runes)
	depth := 0
	isWordChar := func(r rune) bool {
		return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
	}
	wr := []rune(word)
	for i := 0; i < n; i++ {
		r := runes[i]
		switch r {
		case '\'':
			i++
			for i < n {
				if runes[i] == '\'' {
					if i+1 < n && runes[i+1] == '\'' {
						i++
						continue
					}
					break
				}
				i++
			}
			continue
		case '"', '`':
			quote := r
			i++
			for i < n && runes[i] != quote {
				i++
			}
			continue
		case '(':
			depth++
			continue
		case ')':
			depth--
			continue
		}
		if depth != 0 {
			continue
		}
		if i+len(wr) <= n {
			if i > 0 && isWordChar(runes[i-1]) {
				continue
			}
			match := true
			for j, w := range wr {
				if unicode.ToUpper(runes[i+j]) != w {
					match = false
					break
				}
			}
			if match {
				end := i + len(wr)
				if end >= n || !isWordChar(runes[end]) {
					return true
				}
			}
		}
	}
	return false
}
