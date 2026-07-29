package export

import (
	"encoding/base64"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// ExportYAML streams rows as a YAML sequence of mappings, one row at a
// time, rather than building the whole document via a buffered encoder -
// keeping memory bounded the same way ExportJSON does.
func ExportYAML(columns []string, source RowSource, w io.Writer) error {
	for {
		row, err := source()
		if err != nil {
			if err == io.EOF || strings.Contains(err.Error(), "EOF") {
				break
			}
			return err
		}

		for i, col := range columns {
			prefix := "  "
			if i == 0 {
				prefix = "- "
			}
			val := row[i]
			if _, err := io.WriteString(w, prefix+yamlScalarKey(col)+": "+yamlScalarValue(val)+"\n"); err != nil {
				return err
			}
		}
	}
	return nil
}

// yamlScalarKey renders a mapping key, quoting it if it isn't a plain
// unquoted-safe YAML scalar (column names are arbitrary SQL identifiers).
func yamlScalarKey(s string) string {
	if s == "" || needsYAMLQuoting(s) {
		return yamlQuoteString(s)
	}
	return s
}

// yamlScalarValue renders val as a YAML scalar: null, a bare number/bool,
// or a quoted string (blobs are base64-encoded then quoted like a string).
func yamlScalarValue(val any) string {
	if val == nil {
		return "null"
	}
	switch v := val.(type) {
	case []byte:
		return yamlQuoteString(base64.StdEncoding.EncodeToString(v))
	case bool:
		return strconv.FormatBool(v)
	case int, int32, int64, float32, float64:
		return fmt.Sprintf("%v", v)
	case string:
		if v == "" || needsYAMLQuoting(v) {
			return yamlQuoteString(v)
		}
		return v
	default:
		return yamlQuoteString(fmt.Sprintf("%v", v))
	}
}

// needsYAMLQuoting reports whether s must be quoted to be safely read back
// as a YAML string (rather than being interpreted as a number/bool/null or
// tripping over YAML's structural characters).
func needsYAMLQuoting(s string) bool {
	if s == "" {
		return true
	}
	switch strings.ToLower(s) {
	case "null", "~", "true", "false", "yes", "no", "on", "off":
		return true
	}
	if _, err := strconv.ParseFloat(s, 64); err == nil {
		return true
	}
	first := s[0]
	if strings.ContainsAny(string(first), "!&*-?|>%@`\"'#,[]{}") || first == ' ' {
		return true
	}
	if strings.ContainsAny(s, ":\n") {
		return true
	}
	if strings.HasSuffix(s, " ") {
		return true
	}
	return false
}

func yamlQuoteString(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	return `"` + s + `"`
}
