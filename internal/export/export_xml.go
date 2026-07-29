package export

import (
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode"
)

// XMLCaseStyle controls how column/tag names are rendered as XML element
// names, independent of however the underlying SQL identifier is cased.
type XMLCaseStyle string

const (
	XMLCaseSnake  XMLCaseStyle = "snake"  // order_id
	XMLCaseCamel  XMLCaseStyle = "camel"  // orderId
	XMLCasePascal XMLCaseStyle = "pascal" // OrderId
	XMLCaseKebab  XMLCaseStyle = "kebab"  // order-id
)

// XMLNullHandling controls how a NULL column value is rendered.
type XMLNullHandling string

const (
	// XMLNullEmpty renders a NULL value as an empty element, e.g. <notes></notes>.
	XMLNullEmpty XMLNullHandling = "empty"
	// XMLNullOmit skips the element entirely for a NULL value.
	XMLNullOmit XMLNullHandling = "omit"
)

// XMLOptions configures ExportXML's tag naming, formatting, and null
// handling. Zero-value fields are NOT valid on their own - use
// DefaultXMLOptions to get a sane baseline and override individual fields.
type XMLOptions struct {
	// RootTag is the document's root element name (before case conversion
	// and sanitization), e.g. a table name like "orders".
	RootTag string
	// RowTag is the per-row element name (before case conversion and
	// sanitization), e.g. a singularized table name like "order".
	RowTag string
	// CaseStyle is applied uniformly to RootTag, RowTag, and every column
	// name when rendering element names.
	CaseStyle XMLCaseStyle
	// Pretty enables indented, multi-line output. When false, output is
	// compact (no inserted whitespace).
	Pretty bool
	// IndentSize is the number of spaces per nesting level when Pretty is
	// true. Ignored otherwise.
	IndentSize int
	// IncludeDeclaration prepends the standard <?xml version="1.0"
	// encoding="UTF-8"?> declaration.
	IncludeDeclaration bool
	// NullHandling controls how NULL column values are rendered.
	NullHandling XMLNullHandling
}

// DefaultXMLOptions returns the default export configuration: RootTag from
// tableName (or "rows" if empty, e.g. exporting an ad hoc query with no
// single source table), RowTag naively singularized from RootTag, snake_case
// tag names (i.e. column names unchanged), pretty-printed with a 4-space
// indent, an XML declaration, and NULL values rendered as empty elements.
func DefaultXMLOptions(tableName string) XMLOptions {
	root := strings.TrimSpace(tableName)
	if root == "" {
		root = "rows"
	}
	return XMLOptions{
		RootTag:            root,
		RowTag:             singularize(root),
		CaseStyle:          XMLCaseSnake,
		Pretty:             true,
		IndentSize:         4,
		IncludeDeclaration: true,
		NullHandling:       XMLNullEmpty,
	}
}

// normalize applies opts.CaseStyle then sanitizes the result into a valid
// XML element name (NCName-ish: starts with a letter or underscore,
// followed by letters/digits/-/_/.).
func (opts XMLOptions) normalize(name string) string {
	return sanitizeXMLName(convertCase(name, opts.CaseStyle))
}

// ExportXML streams rows as a configurable XML document: a root element
// (opts.RootTag) containing one row element (opts.RowTag) per row, each
// with one child element per column, named and cased per opts.
func ExportXML(columns []string, source RowSource, w io.Writer, opts XMLOptions) error {
	if opts.IndentSize <= 0 {
		opts.IndentSize = 2
	}

	if opts.IncludeDeclaration {
		if _, err := io.WriteString(w, xml.Header); err != nil {
			return err
		}
	}

	enc := xml.NewEncoder(w)
	if opts.Pretty {
		enc.Indent("", strings.Repeat(" ", opts.IndentSize))
	}

	rootName := opts.normalize(opts.RootTag)
	rowName := opts.normalize(opts.RowTag)
	colNames := make([]string, len(columns))
	for i, c := range columns {
		colNames[i] = opts.normalize(c)
	}

	rootStart := xml.StartElement{Name: xml.Name{Local: rootName}}
	if err := enc.EncodeToken(rootStart); err != nil {
		return err
	}

	for {
		row, err := source()
		if err != nil {
			if err == io.EOF || strings.Contains(err.Error(), "EOF") {
				break
			}
			return err
		}

		rowStart := xml.StartElement{Name: xml.Name{Local: rowName}}
		if err := enc.EncodeToken(rowStart); err != nil {
			return err
		}

		for i, val := range row {
			if val == nil && opts.NullHandling == XMLNullOmit {
				continue
			}

			colStart := xml.StartElement{Name: xml.Name{Local: colNames[i]}}
			if err := enc.EncodeToken(colStart); err != nil {
				return err
			}

			if val != nil {
				var text string
				if b, ok := val.([]byte); ok {
					text = base64.StdEncoding.EncodeToString(b)
				} else {
					text = fmt.Sprintf("%v", val)
				}
				if err := enc.EncodeToken(xml.CharData(text)); err != nil {
					return err
				}
			}

			if err := enc.EncodeToken(colStart.End()); err != nil {
				return err
			}
		}

		if err := enc.EncodeToken(rowStart.End()); err != nil {
			return err
		}
	}

	if err := enc.EncodeToken(rootStart.End()); err != nil {
		return err
	}
	if err := enc.Flush(); err != nil {
		return err
	}
	if opts.Pretty {
		_, err := io.WriteString(w, "\n")
		return err
	}
	return nil
}

// splitWords breaks an identifier into its constituent words, treating
// underscores/hyphens/spaces as separators and camelCase humps as implicit
// word boundaries, so any input casing convention can be re-cased uniformly.
func splitWords(s string) []string {
	var words []string
	var cur strings.Builder
	runes := []rune(s)
	for i, r := range runes {
		if r == '_' || r == '-' || r == ' ' || r == '.' {
			if cur.Len() > 0 {
				words = append(words, cur.String())
				cur.Reset()
			}
			continue
		}
		if i > 0 && unicode.IsUpper(r) && !unicode.IsUpper(runes[i-1]) && cur.Len() > 0 {
			words = append(words, cur.String())
			cur.Reset()
		}
		cur.WriteRune(r)
	}
	if cur.Len() > 0 {
		words = append(words, cur.String())
	}
	return words
}

func titleCaseWord(w string) string {
	if w == "" {
		return w
	}
	r := []rune(strings.ToLower(w))
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}

// convertCase re-cases name into the requested style. Unrecognized styles
// fall back to XMLCaseSnake.
func convertCase(name string, style XMLCaseStyle) string {
	words := splitWords(name)
	if len(words) == 0 {
		return name
	}
	switch style {
	case XMLCaseCamel:
		var sb strings.Builder
		sb.WriteString(strings.ToLower(words[0]))
		for _, w := range words[1:] {
			sb.WriteString(titleCaseWord(w))
		}
		return sb.String()
	case XMLCasePascal:
		var sb strings.Builder
		for _, w := range words {
			sb.WriteString(titleCaseWord(w))
		}
		return sb.String()
	case XMLCaseKebab:
		lower := make([]string, len(words))
		for i, w := range words {
			lower[i] = strings.ToLower(w)
		}
		return strings.Join(lower, "-")
	default: // XMLCaseSnake and anything unrecognized
		lower := make([]string, len(words))
		for i, w := range words {
			lower[i] = strings.ToLower(w)
		}
		return strings.Join(lower, "_")
	}
}

// sanitizeXMLName coerces name into a valid XML element name: any
// character that isn't a letter, underscore, digit, hyphen, or period is
// replaced with an underscore, and a leading digit is prefixed with an
// underscore (element names can't start with a digit).
func sanitizeXMLName(name string) string {
	if name == "" {
		return "_"
	}
	var sb strings.Builder
	for _, r := range name {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-' || r == '.' {
			sb.WriteRune(r)
		} else {
			sb.WriteRune('_')
		}
	}
	out := sb.String()
	if out == "" {
		return "_"
	}
	if unicode.IsDigit([]rune(out)[0]) {
		out = "_" + out
	}
	return out
}

// singularize applies a small set of common English pluralization rules in
// reverse. It's a best-effort heuristic for deriving a default row tag from
// a table name (e.g. "orders" -> "order", "categories" -> "category") - not
// a general-purpose linguistic singularizer. Names it doesn't recognize as
// plural are returned unchanged.
func singularize(s string) string {
	lower := strings.ToLower(s)
	switch {
	case strings.HasSuffix(lower, "ies") && len(s) > 3:
		return s[:len(s)-3] + "y"
	case strings.HasSuffix(lower, "ses"), strings.HasSuffix(lower, "xes"), strings.HasSuffix(lower, "zes"),
		strings.HasSuffix(lower, "ches"), strings.HasSuffix(lower, "shes"):
		return s[:len(s)-2]
	case strings.HasSuffix(lower, "s") && !strings.HasSuffix(lower, "ss") && len(s) > 1:
		return s[:len(s)-1]
	default:
		return s
	}
}

// ParseXMLCaseStyle validates a case-style query/form value, defaulting to
// XMLCaseSnake for an empty or unrecognized input.
func ParseXMLCaseStyle(s string) XMLCaseStyle {
	switch XMLCaseStyle(strings.ToLower(strings.TrimSpace(s))) {
	case XMLCaseCamel:
		return XMLCaseCamel
	case XMLCasePascal:
		return XMLCasePascal
	case XMLCaseKebab:
		return XMLCaseKebab
	default:
		return XMLCaseSnake
	}
}

// ParseXMLNullHandling validates a null-handling query/form value,
// defaulting to XMLNullEmpty for an empty or unrecognized input.
func ParseXMLNullHandling(s string) XMLNullHandling {
	if XMLNullHandling(strings.ToLower(strings.TrimSpace(s))) == XMLNullOmit {
		return XMLNullOmit
	}
	return XMLNullEmpty
}

// ParseXMLIndentSize parses an indent-size query/form value, defaulting to
// fallback for an empty, invalid, or non-positive input.
func ParseXMLIndentSize(s string, fallback int) int {
	if s == "" {
		return fallback
	}
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return fallback
	}
	return n
}
