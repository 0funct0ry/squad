package cli

import (
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"text/template"
	"time"

	"github.com/0funct0ry/squad/internal/seed"
)

// buildTemplateFuncMap exposes internal/seed's generator registry and
// formulaFuncs whitelist as text/template functions, so CLI SQL like
// `INSERT INTO users (name) VALUES ({{person.name}});` can call the exact
// same generators the web UI's GeneratorPicker offers -- no gofakeit wiring
// or string/crypto/math helpers are duplicated here.
func buildTemplateFuncMap() template.FuncMap {
	fm := template.FuncMap{}

	for _, meta := range seed.GeneratorCatalog() {
		name := meta.Name
		affinity := "TEXT"
		if len(meta.Affinities) > 0 {
			affinity = meta.Affinities[0]
		}
		schema := meta.OptionsSchema
		// text/template function names must be valid Go identifiers (no
		// dots), so dot-namespaced generator names (e.g. "git.branchName",
		// added in M12) are exposed under a dot-stripped camelCase alias
		// (e.g. {{gitBranchName}}) instead of the registry name verbatim.
		fm[templateFuncName(name)] = makeGeneratorTemplateFunc(name, affinity, schema)
	}

	for _, name := range seed.FormulaFuncNames() {
		n := name
		fm[n] = func(args ...any) (string, error) {
			result, err, ok := seed.CallFormulaFunc(n, args)
			if !ok {
				return "", fmt.Errorf("unknown function: %s", n)
			}
			if err != nil {
				return "", err
			}
			return templateRawValue(result)
		}
	}

	return fm
}

// templateFuncName converts a dot-namespaced generator name (e.g.
// "git.branchName") into a valid Go identifier for use as a text/template
// function name (e.g. "gitBranchName") by removing the dots and
// capitalizing the following segment. Names without dots pass through
// unchanged.
func templateFuncName(name string) string {
	if !strings.Contains(name, ".") {
		return name
	}
	parts := strings.Split(name, ".")
	out := parts[0]
	for _, p := range parts[1:] {
		if p == "" {
			continue
		}
		out += strings.ToUpper(p[:1]) + p[1:]
	}
	return out
}

// makeGeneratorTemplateFunc returns a template function that calls the named
// generator via seed.Generate (reused verbatim) and renders the result as
// raw text ready to splice inline. Positional template args are zipped
// against the generator's declared OptionsSchema keys, e.g.
// {{int 1 100}} -> opts{"min":1,"max":100} for the "int" generator.
func makeGeneratorTemplateFunc(name, affinity string, schema []seed.OptionField) func(args ...any) (string, error) {
	return func(args ...any) (string, error) {
		opts := map[string]any{}
		for i, a := range args {
			if i >= len(schema) {
				break
			}
			opts[schema[i].Key] = a
		}
		val, err := seed.Generate(name, affinity, opts)
		if err != nil {
			return "", err
		}
		return templateRawValue(val)
	}
}

// templateRawValue renders a generator/formula return value as raw text for
// splicing into a {{ }} block, deliberately WITHOUT surrounding SQL quotes --
// the caller writes the quotes in their own SQL, e.g.
// `VALUES ('{{name}}', '{{firstName}}@{{lastName}}.in')`, the same way they'd
// write a literal by hand. This is why a string value only has its embedded
// single quotes escaped (doubled), not wrapped: the surrounding '...' is the
// user's, and doubling still produces correct SQL whether or not they added
// their own quotes. Numbers/booleans render bare (no quotes needed either
// way). NULL renders as the bare keyword NULL, since a user-supplied
// '{{ }}' around a NULL-valued generator can't become a real SQL NULL --
// nullable columns should be handled through their own DEFAULT/nullability,
// not through quoting. []byte is the one exception that still needs its own
// delimiters: SQLite has no other blob literal syntax, so it renders as a
// complete `X'...'` literal -- don't wrap a blob-returning generator/formula
// call in your own quotes.
func templateRawValue(val any) (string, error) {
	switch v := val.(type) {
	case nil:
		return "NULL", nil
	case string:
		return strings.ReplaceAll(v, "'", "''"), nil
	case bool:
		if v {
			return "1", nil
		}
		return "0", nil
	case int, int64, int32, float32, float64:
		return fmt.Sprintf("%v", v), nil
	case []byte:
		return "X'" + hex.EncodeToString(v) + "'", nil
	case time.Time:
		return v.Format(time.RFC3339), nil
	default:
		return strings.ReplaceAll(fmt.Sprintf("%v", v), "'", "''"), nil
	}
}

// sqlLiteral formats a value as a complete, self-quoted SQL literal: a
// quoted string, a bare number, or an X'...' blob literal, per its Go type.
// Used by .dump (dotcommands.go), which emits full standalone INSERT
// statements from real row data -- not a {{ }} template value the user is
// about to wrap in their own quotes, so it must fully quote itself.
func sqlLiteral(val any) (string, error) {
	switch v := val.(type) {
	case nil:
		return "NULL", nil
	case string:
		return "'" + strings.ReplaceAll(v, "'", "''") + "'", nil
	case bool:
		if v {
			return "1", nil
		}
		return "0", nil
	case int, int64, int32, float32, float64:
		return fmt.Sprintf("%v", v), nil
	case []byte:
		return "X'" + hex.EncodeToString(v) + "'", nil
	case time.Time:
		return "'" + v.Format(time.RFC3339) + "'", nil
	default:
		return "'" + strings.ReplaceAll(fmt.Sprintf("%v", v), "'", "''") + "'", nil
	}
}

// TemplateFunctionInfo describes one function callable inside a {{ }} block,
// for the .templates dot-command. Quoting is a short hint on how the
// caller's SQL should treat the function's output (see templateRawValue):
// "add quotes" for raw text that must be wrapped in '...' by hand, "bare" for
// numbers/booleans that need no quotes, or "self-quoted" for the one case
// (blob-returning calls) that already renders a complete X'...' literal and
// must NOT be wrapped in extra quotes.
type TemplateFunctionInfo struct {
	Name        string
	Description string
	Usage       string
	Quoting     string
}

const (
	quoteAdd  = "add quotes"
	quoteBare = "bare"
	quoteSelf = "self-quoted"
)

// formulaFuncDoc is formulaFuncDocs' entry shape; embeds into
// TemplateFunctionInfo below.
type formulaFuncDoc struct {
	Description string
	Usage       string
	Quoting     string
}

// formulaFuncDocs supplies the description/usage/quoting text for the
// formulaFuncs whitelist, which internal/seed exposes by name only (no
// metadata). Kept here rather than in internal/seed since it's CLI-display
// concern, not part of the whitelist itself; a name missing from this map
// still gets a reasonable generic fallback in ListTemplateFunctions.
var formulaFuncDocs = map[string]formulaFuncDoc{
	"upper":      {Description: "Uppercase a string", Usage: "{{upper s}}", Quoting: quoteAdd},
	"lower":      {Description: "Lowercase a string", Usage: "{{lower s}}", Quoting: quoteAdd},
	"concat":     {Description: "Concatenate two or more values", Usage: "{{concat a b ...}}", Quoting: quoteAdd},
	"trim":       {Description: "Trim leading/trailing whitespace", Usage: "{{trim s}}", Quoting: quoteAdd},
	"len":        {Description: "Rune length of a string", Usage: "{{len s}}", Quoting: quoteBare},
	"capitalize": {Description: "Uppercase the first letter", Usage: "{{capitalize s}}", Quoting: quoteAdd},
	"kebabCase":  {Description: "Convert to kebab-case", Usage: "{{kebabCase s}}", Quoting: quoteAdd},
	"camelCase":  {Description: "Convert to camelCase", Usage: "{{camelCase s}}", Quoting: quoteAdd},
	"hex":        {Description: "Hex-encode a string", Usage: "{{hex s}}", Quoting: quoteAdd},
	"base32":     {Description: "Base32-encode a string", Usage: "{{base32 s}}", Quoting: quoteAdd},
	"base64":     {Description: "Base64-encode a string", Usage: "{{base64 s}}", Quoting: quoteAdd},
	"sha1":       {Description: "SHA-1 hex digest", Usage: "{{sha1 s}}", Quoting: quoteAdd},
	"md5":        {Description: "MD5 hex digest", Usage: "{{md5 s}}", Quoting: quoteAdd},
	"sha256":     {Description: "SHA-256 hex digest", Usage: "{{sha256 s}}", Quoting: quoteAdd},
	"sha512":     {Description: "SHA-512 hex digest", Usage: "{{sha512 s}}", Quoting: quoteAdd},
	"abs":        {Description: "Absolute value", Usage: "{{abs x}}", Quoting: quoteBare},
	"round":      {Description: "Round to the nearest integer", Usage: "{{round x}}", Quoting: quoteBare},
	"floor":      {Description: "Round down", Usage: "{{floor x}}", Quoting: quoteBare},
	"ceil":       {Description: "Round up", Usage: "{{ceil x}}", Quoting: quoteBare},
	"min":        {Description: "Minimum of two or more numbers", Usage: "{{min a b ...}}", Quoting: quoteBare},
	"max":        {Description: "Maximum of two or more numbers", Usage: "{{max a b ...}}", Quoting: quoteBare},
	"pow":        {Description: "x raised to the power y", Usage: "{{pow x y}}", Quoting: quoteBare},
	"mod":        {Description: "Floating-point remainder of x/y", Usage: "{{mod x y}}", Quoting: quoteBare},
}

// ListTemplateFunctions returns every function callable inside a {{ }}
// template block -- internal/seed's generator registry plus its
// formulaFuncs whitelist -- sorted by name, for the .templates dot-command.
func ListTemplateFunctions() []TemplateFunctionInfo {
	var out []TemplateFunctionInfo

	for _, meta := range seed.GeneratorCatalog() {
		usage := "{{" + meta.Name + "}}"
		if len(meta.OptionsSchema) > 0 {
			keys := make([]string, len(meta.OptionsSchema))
			for i, opt := range meta.OptionsSchema {
				keys[i] = opt.Key
			}
			usage = "{{" + meta.Name + " " + strings.Join(keys, " ") + "}}"
		}
		desc := meta.Description
		if desc == "" {
			desc = "Seed generator"
		}
		// Mirrors the affinity buildTemplateFuncMap actually calls the
		// generator with (meta.Affinities[0], default "TEXT"), which is what
		// determines the Go type -- and therefore the quoting -- of the
		// value templateRawValue renders.
		affinity := "TEXT"
		if len(meta.Affinities) > 0 {
			affinity = meta.Affinities[0]
		}
		quoting := quoteAdd
		switch affinity {
		case "INTEGER", "REAL", "NUMERIC":
			quoting = quoteBare
		case "BLOB":
			quoting = quoteSelf
		}
		out = append(out, TemplateFunctionInfo{Name: meta.Name, Description: desc, Usage: usage, Quoting: quoting})
	}

	for _, name := range seed.FormulaFuncNames() {
		if doc, ok := formulaFuncDocs[name]; ok {
			out = append(out, TemplateFunctionInfo{Name: name, Description: doc.Description, Usage: doc.Usage, Quoting: doc.Quoting})
		} else {
			out = append(out, TemplateFunctionInfo{Name: name, Description: "Whitelisted formula function", Usage: "{{" + name + " args...}}", Quoting: quoteAdd})
		}
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// preprocessTemplate runs stmt through text/template when it contains "{{",
// using the seed-backed FuncMap above. Statements without "{{" pass through
// untouched. Template errors are returned as-is; the caller surfaces them as
// a shell error before Classify/execution ever sees the statement.
func preprocessTemplate(stmt string) (string, error) {
	if !strings.Contains(stmt, "{{") {
		return stmt, nil
	}
	tmpl, err := template.New("stmt").Funcs(buildTemplateFuncMap()).Parse(stmt)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	if err := tmpl.Execute(&b, nil); err != nil {
		return "", err
	}
	return b.String(), nil
}
