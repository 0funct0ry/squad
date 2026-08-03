// Package seed provides fake-data generation for the seed table feature (M6).
package seed

import (
	"crypto/rand"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/brianvoe/gofakeit/v7"
)

// GeneratorFunc produces a single value for a column, given the column's SQLite
// type affinity and any user-supplied options. The returned value must be
// ready to bind as a database/sql driver parameter (string, int64, float64,
// bool, or []byte).
type GeneratorFunc func(affinity string, opts map[string]any) (any, error)

// OptionKind identifies the UI control used to edit a generator option.
type OptionKind string

const (
	OptKindInt       OptionKind = "int"
	OptKindFloat     OptionKind = "float"
	OptKindBool      OptionKind = "bool"
	OptKindString    OptionKind = "string"
	OptKindDateTime  OptionKind = "datetime" // RFC3339 wire format, matches optTime
	OptKindDate      OptionKind = "date"     // plain YYYY-MM-DD, no time component (e.g. vtab's calendar module)
	OptKindSelect    OptionKind = "select"
	OptKindColumns   OptionKind = "columns"   // formula-only: multi-select of sibling column names
	OptKindTextarea  OptionKind = "textarea"  // multi-line free text, e.g. value lists / DSL blocks
	OptKindGenerator OptionKind = "generator" // nested {generator, options} value, e.g. nullWithProbability
)

// OptionField describes one declarative, UI-renderable option for a generator.
type OptionField struct {
	Key         string     `json:"key"`
	Label       string     `json:"label"`
	Kind        OptionKind `json:"kind"`
	Default     any        `json:"default,omitempty"`
	Choices     []string   `json:"choices,omitempty"`
	Min         *float64   `json:"min,omitempty"` // UI hint only
	Max         *float64   `json:"max,omitempty"` // UI hint only
	Required    bool       `json:"required,omitempty"`
	Description string     `json:"description,omitempty"`
}

// GeneratorDef describes one entry in the generator registry.
type GeneratorDef struct {
	Name          string
	Group         string   // category group, e.g. "person", "geo"
	Aliases       []string // alternate names resolving to this generator
	Description   string   // picker card description
	Affinities    []string
	OptionsSchema []OptionField // nil = no options
	Stateful      bool          // true only for sequence/rowNumber/characterSequence/digitSequence
	Fn            GeneratorFunc
}

// GeneratorMeta is the JSON-serializable projection of a GeneratorDef used by
// the generatorCatalog API field.
type GeneratorMeta struct {
	Name          string        `json:"name"`
	Group         string        `json:"group"`
	Aliases       []string      `json:"aliases,omitempty"`
	Description   string        `json:"description,omitempty"`
	Affinities    []string      `json:"affinities"`
	OptionsSchema []OptionField `json:"optionsSchema,omitempty"`
	Stateful      bool          `json:"stateful,omitempty"`
}

func floatPtr(f float64) *float64 { return &f }

// ForeignKeyGeneratorName is handled specially by the server/generate layer
// (it needs live DB access to sample referenced values), but is still listed
// in the registry so it appears in availableGenerators and its affinity
// applicability can be queried generically.
const ForeignKeyGeneratorName = "foreignKey"

var registry = buildRegistry()

func buildRegistry() map[string]GeneratorDef {
	defs := []GeneratorDef{
		{Name: "email", Group: "person", Description: "Email address", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return gofakeit.Email(), nil
		}},
		{Name: "firstName", Group: "person", Description: "First name", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return gofakeit.FirstName(), nil
		}},
		{Name: "lastName", Group: "person", Description: "Last name", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return gofakeit.LastName(), nil
		}},
		{Name: "name", Group: "person", Description: "Full name", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return gofakeit.Name(), nil
		}},
		{Name: "username", Group: "person", Description: "Username", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return gofakeit.Username(), nil
		}},
		{Name: "uuid", Group: "identifier", Description: "UUID v4", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return gofakeit.UUID(), nil
		}},
		{Name: "datetime", Group: "datetime", Description: "Date/time within a range", Affinities: []string{"TEXT", "INTEGER"}, OptionsSchema: []OptionField{
			{Key: "from", Label: "From", Kind: OptKindDateTime},
			{Key: "to", Label: "To", Kind: OptKindDateTime},
			{Key: "onlyDate", Label: "Only date", Kind: OptKindBool, Default: false},
		}, Fn: genDatetime},
		{Name: "price", Group: "numeric", Description: "Price between min and max", Affinities: []string{"REAL", "NUMERIC"}, OptionsSchema: []OptionField{
			{Key: "min", Label: "Min", Kind: OptKindFloat, Default: 1.0},
			{Key: "max", Label: "Max", Kind: OptKindFloat, Default: 1000.0},
		}, Fn: func(_ string, opts map[string]any) (any, error) {
			min := optFloat(opts, "min", 1)
			max := optFloat(opts, "max", 1000)
			return gofakeit.Price(min, max), nil
		}},
		{Name: "url", Group: "internet", Description: "URL", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return gofakeit.URL(), nil
		}},
		{Name: "phone", Group: "person", Description: "Phone number", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return gofakeit.Phone(), nil
		}},
		{Name: "bool", Group: "misc", Description: "Boolean value", Affinities: []string{"INTEGER", "TEXT"}, Fn: genBool},
		{Name: "sentence", Group: "text", Description: "Sentence with a given word count", Affinities: []string{"TEXT"}, OptionsSchema: []OptionField{
			{Key: "wordCount", Label: "Word count", Kind: OptKindInt, Default: 8},
		}, Fn: func(_ string, opts map[string]any) (any, error) {
			wordCount := optInt(opts, "wordCount", 8)
			return sentenceWithWordCount(wordCount), nil
		}},
		{Name: "word", Group: "text", Description: "Single word", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return gofakeit.Word(), nil
		}},
		{Name: "paragraph", Group: "text", Description: "Paragraph with a given sentence count", Affinities: []string{"TEXT"}, OptionsSchema: []OptionField{
			{Key: "sentences", Label: "Sentences", Kind: OptKindInt, Default: 3},
		}, Fn: func(_ string, opts map[string]any) (any, error) {
			sentences := optInt(opts, "sentences", 3)
			return gofakeit.LoremIpsumParagraph(1, sentences, 10, " "), nil
		}},
		{Name: "int", Group: "numeric", Description: "Integer between min and max", Affinities: []string{"INTEGER"}, OptionsSchema: []OptionField{
			{Key: "min", Label: "Min", Kind: OptKindInt, Default: 0},
			{Key: "max", Label: "Max", Kind: OptKindInt, Default: 10000},
		}, Fn: func(_ string, opts map[string]any) (any, error) {
			min := optInt(opts, "min", 0)
			max := optInt(opts, "max", 10000)
			return gofakeit.IntRange(min, max), nil
		}},
		{Name: "float", Group: "numeric", Description: "Float between min and max", Affinities: []string{"REAL", "NUMERIC"}, OptionsSchema: []OptionField{
			{Key: "min", Label: "Min", Kind: OptKindFloat, Default: 0.0},
			{Key: "max", Label: "Max", Kind: OptKindFloat, Default: 1000.0},
		}, Fn: func(_ string, opts map[string]any) (any, error) {
			min := optFloat(opts, "min", 0)
			max := optFloat(opts, "max", 1000)
			return gofakeit.Float64Range(min, max), nil
		}},
		{Name: "company", Group: "company", Description: "Company name", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return gofakeit.Company(), nil
		}},
		{Name: "address", Group: "geo", Description: "Street address", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return gofakeit.Address().Address, nil
		}},
		{Name: "city", Group: "geo", Description: "City name", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return gofakeit.City(), nil
		}},
		{Name: "country", Group: "geo", Description: "Country name", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return gofakeit.Country(), nil
		}},
		{Name: "zipCode", Group: "geo", Description: "Zip / postal code", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return gofakeit.Zip(), nil
		}},
		{Name: "ipv4", Group: "internet", Description: "IPv4 address", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return gofakeit.IPv4Address(), nil
		}},
		{Name: "bytes", Group: "identifier", Description: "Random byte string", Affinities: []string{"BLOB"}, OptionsSchema: []OptionField{
			{Key: "length", Label: "Length", Kind: OptKindInt, Default: 16},
		}, Fn: func(_ string, opts map[string]any) (any, error) {
			length := optInt(opts, "length", 16)
			b := make([]byte, length)
			if _, err := rand.Read(b); err != nil {
				return nil, err
			}
			return b, nil
		}},
		{Name: ForeignKeyGeneratorName, Group: "special", Description: "Reference an existing row in another table", Affinities: nil, Fn: nil},
		{Name: "enumFromColumn", Group: "special", Description: "Pick from the real distinct values already present in another table/column", Affinities: []string{"TEXT", "INTEGER", "REAL"}, OptionsSchema: []OptionField{
			{Key: "table", Label: "Table", Kind: OptKindString, Required: true},
			{Key: "column", Label: "Column", Kind: OptKindString, Required: true},
		}, Fn: nil},
	}

	defs = append(defs, personGenerators()...)
	defs = append(defs, geoGenerators()...)
	defs = append(defs, datetimeGenerators()...)
	defs = append(defs, numericGenerators()...)
	defs = append(defs, internetGenerators()...)
	defs = append(defs, financeGenerators()...)
	defs = append(defs, companyGenerators()...)
	defs = append(defs, colorGenerators()...)
	defs = append(defs, textGenerators()...)
	defs = append(defs, foodGenerators()...)
	defs = append(defs, productGenerators()...)
	defs = append(defs, identifierGenerators()...)
	defs = append(defs, securityGenerators()...)
	defs = append(defs, distributionGenerators()...)
	defs = append(defs, noveltyGenerators()...)
	defs = append(defs, domainGenerators()...)
	defs = append(defs, sequenceGenerators()...)
	defs = append(defs, formulaGenerators()...)
	defs = append(defs, mediaGenerators()...)
	defs = append(defs, customListGenerators()...)
	defs = append(defs, crossColumnExtraGenerators()...)
	defs = append(defs, nullWithProbabilityGenerators()...)
	defs = append(defs, misc2Generators()...)
	defs = append(defs, gitGenerators()...)
	defs = append(defs, dockerGenerators()...)
	defs = append(defs, unixGenerators()...)
	defs = append(defs, stripeGenerators()...)

	m := make(map[string]GeneratorDef, len(defs))
	for _, d := range defs {
		if _, dup := m[d.Name]; dup {
			panic(fmt.Sprintf("seed: duplicate generator name %q", d.Name))
		}
		m[d.Name] = d
	}
	return m
}

func genBool(affinity string, _ map[string]any) (any, error) {
	v := gofakeit.Bool()
	if affinity == "TEXT" {
		if v {
			return "true", nil
		}
		return "false", nil
	}
	if v {
		return int64(1), nil
	}
	return int64(0), nil
}

func genDatetime(affinity string, opts map[string]any) (any, error) {
	from := optTime(opts, "from", time.Now().AddDate(-5, 0, 0))
	to := optTime(opts, "to", time.Now())
	onlyDate := optBool(opts, "onlyDate", false)

	t := gofakeit.DateRange(from, to)
	if affinity == "INTEGER" {
		return t.Unix(), nil
	}
	if onlyDate {
		return t.Format("2006-01-02"), nil
	}
	return t.Format(time.RFC3339), nil
}

// AvailableGenerators returns the sorted list of generator names, including
// foreignKey, for use in the plan endpoint's availableGenerators field.
func AvailableGenerators() []string {
	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Exists reports whether name is a known generator (including foreignKey).
func Exists(name string) bool {
	_, ok := registry[name]
	return ok
}

// GeneratorCatalog returns the JSON-serializable metadata for every registered
// generator, sorted by name, for the seed/plan API response.
func GeneratorCatalog() []GeneratorMeta {
	names := AvailableGenerators()
	out := make([]GeneratorMeta, 0, len(names))
	for _, name := range names {
		d := registry[name]
		out = append(out, GeneratorMeta{
			Name:          d.Name,
			Group:         d.Group,
			Aliases:       d.Aliases,
			Description:   d.Description,
			Affinities:    d.Affinities,
			OptionsSchema: d.OptionsSchema,
			Stateful:      d.Stateful,
		})
	}
	return out
}

// GeneratorMetaByName returns the metadata for a single generator by name.
func GeneratorMetaByName(name string) (GeneratorMeta, bool) {
	d, ok := registry[name]
	if !ok {
		return GeneratorMeta{}, false
	}
	return GeneratorMeta{
		Name:          d.Name,
		Group:         d.Group,
		Aliases:       d.Aliases,
		Description:   d.Description,
		Affinities:    d.Affinities,
		OptionsSchema: d.OptionsSchema,
		Stateful:      d.Stateful,
	}, true
}

// Generate produces a value for the named generator. It must not be called
// with ForeignKeyGeneratorName; that generator is handled by the caller since
// it requires live DB access to sample referenced values.
func Generate(name string, affinity string, opts map[string]any) (any, error) {
	def, ok := registry[name]
	if !ok {
		return nil, fmt.Errorf("unknown generator: %s", name)
	}
	if def.Fn == nil {
		return nil, fmt.Errorf("generator %s must be handled by the caller", name)
	}
	return def.Fn(affinity, opts)
}

// sentenceWithWordCount builds a sentence of exactly wordCount words.
// gofakeit.Sentence's wordCount parameter is a documented no-op in v7.15.0
// (it always delegates to a fixed internal template regardless of the
// argument), so word count is enforced here by joining that many
// gofakeit.Word() calls instead.
func sentenceWithWordCount(wordCount int) string {
	if wordCount <= 0 {
		wordCount = 1
	}
	words := make([]string, wordCount)
	for i := range words {
		words[i] = gofakeit.Word()
	}
	words[0] = strings.ToUpper(words[0][:1]) + words[0][1:]
	return strings.Join(words, " ") + "."
}

func optFloat(opts map[string]any, key string, def float64) float64 {
	if v, ok := opts[key]; ok {
		switch n := v.(type) {
		case float64:
			return n
		case int:
			return float64(n)
		case int64:
			return float64(n)
		}
	}
	return def
}

func optInt(opts map[string]any, key string, def int) int {
	if v, ok := opts[key]; ok {
		switch n := v.(type) {
		case float64:
			return int(n)
		case int:
			return n
		case int64:
			return int(n)
		}
	}
	return def
}

func optBool(opts map[string]any, key string, def bool) bool {
	if v, ok := opts[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return def
}

func optString(opts map[string]any, key string, def string) string {
	if v, ok := opts[key]; ok {
		if s, ok := v.(string); ok && s != "" {
			return s
		}
	}
	return def
}

func optStringSlice(opts map[string]any, key string) []string {
	v, ok := opts[key]
	if !ok {
		return nil
	}
	switch s := v.(type) {
	case []string:
		return s
	case []any:
		out := make([]string, 0, len(s))
		for _, e := range s {
			if str, ok := e.(string); ok {
				out = append(out, str)
			}
		}
		return out
	}
	return nil
}

func optTime(opts map[string]any, key string, def time.Time) time.Time {
	if v, ok := opts[key]; ok {
		if s, ok := v.(string); ok && s != "" {
			if t, err := time.Parse(time.RFC3339, s); err == nil {
				return t
			}
		}
	}
	return def
}
