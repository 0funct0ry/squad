// Package seed provides fake-data generation for the seed table feature (M6).
package seed

import (
	"crypto/rand"
	"fmt"
	"sort"
	"time"

	"github.com/brianvoe/gofakeit/v7"
)

// GeneratorFunc produces a single value for a column, given the column's SQLite
// type affinity and any user-supplied options. The returned value must be
// ready to bind as a database/sql driver parameter (string, int64, float64,
// bool, or []byte).
type GeneratorFunc func(affinity string, opts map[string]any) (any, error)

// GeneratorDef describes one entry in the generator registry.
type GeneratorDef struct {
	Name       string
	Affinities []string
	Fn         GeneratorFunc
}

// ForeignKeyGeneratorName is handled specially by the server/generate layer
// (it needs live DB access to sample referenced values), but is still listed
// in the registry so it appears in availableGenerators and its affinity
// applicability can be queried generically.
const ForeignKeyGeneratorName = "foreignKey"

var registry = buildRegistry()

func buildRegistry() map[string]GeneratorDef {
	defs := []GeneratorDef{
		{Name: "email", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return gofakeit.Email(), nil
		}},
		{Name: "firstName", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return gofakeit.FirstName(), nil
		}},
		{Name: "lastName", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return gofakeit.LastName(), nil
		}},
		{Name: "name", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return gofakeit.Name(), nil
		}},
		{Name: "username", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return gofakeit.Username(), nil
		}},
		{Name: "uuid", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return gofakeit.UUID(), nil
		}},
		{Name: "datetime", Affinities: []string{"TEXT", "INTEGER"}, Fn: genDatetime},
		{Name: "price", Affinities: []string{"REAL", "NUMERIC"}, Fn: func(_ string, opts map[string]any) (any, error) {
			min := optFloat(opts, "min", 1)
			max := optFloat(opts, "max", 1000)
			return gofakeit.Price(min, max), nil
		}},
		{Name: "url", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return gofakeit.URL(), nil
		}},
		{Name: "phone", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return gofakeit.Phone(), nil
		}},
		{Name: "bool", Affinities: []string{"INTEGER", "TEXT"}, Fn: genBool},
		{Name: "sentence", Affinities: []string{"TEXT"}, Fn: func(_ string, opts map[string]any) (any, error) {
			wordCount := optInt(opts, "wordCount", 8)
			return gofakeit.Sentence(wordCount), nil
		}},
		{Name: "word", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return gofakeit.Word(), nil
		}},
		{Name: "paragraph", Affinities: []string{"TEXT"}, Fn: func(_ string, opts map[string]any) (any, error) {
			sentences := optInt(opts, "sentences", 3)
			return gofakeit.LoremIpsumParagraph(1, sentences, 10, " "), nil
		}},
		{Name: "int", Affinities: []string{"INTEGER"}, Fn: func(_ string, opts map[string]any) (any, error) {
			min := optInt(opts, "min", 0)
			max := optInt(opts, "max", 10000)
			return gofakeit.IntRange(min, max), nil
		}},
		{Name: "float", Affinities: []string{"REAL", "NUMERIC"}, Fn: func(_ string, opts map[string]any) (any, error) {
			min := optFloat(opts, "min", 0)
			max := optFloat(opts, "max", 1000)
			return gofakeit.Float64Range(min, max), nil
		}},
		{Name: "company", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return gofakeit.Company(), nil
		}},
		{Name: "address", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return gofakeit.Address().Address, nil
		}},
		{Name: "city", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return gofakeit.City(), nil
		}},
		{Name: "country", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return gofakeit.Country(), nil
		}},
		{Name: "zipCode", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return gofakeit.Zip(), nil
		}},
		{Name: "ipv4", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return gofakeit.IPv4Address(), nil
		}},
		{Name: "bytes", Affinities: []string{"BLOB"}, Fn: func(_ string, opts map[string]any) (any, error) {
			length := optInt(opts, "length", 16)
			b := make([]byte, length)
			if _, err := rand.Read(b); err != nil {
				return nil, err
			}
			return b, nil
		}},
		{Name: ForeignKeyGeneratorName, Affinities: nil, Fn: nil},
	}

	m := make(map[string]GeneratorDef, len(defs))
	for _, d := range defs {
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
