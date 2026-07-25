package seed

import (
	"fmt"
	"math/rand"
	"regexp"
	"strconv"
	"strings"

	"github.com/brianvoe/gofakeit/v7"
)

// customListGenerators registers the user-typed-value-list generators
// (oneOf, weightedOneOf, regexEnum, incrementalEnum). Unlike most Fn-based
// generators, their value set comes from a string the user types into the
// Generator Picker's options form rather than a fixed registry-defined set.
func customListGenerators() []GeneratorDef {
	return []GeneratorDef{
		{Name: "oneOf", Group: "custom-list", Description: "Uniformly-random pick from a user-typed list of values", Affinities: []string{"TEXT", "INTEGER", "REAL"}, OptionsSchema: []OptionField{
			{Key: "values", Label: "Values", Kind: OptKindTextarea, Required: true, Description: "Comma- or newline-separated list of literal values"},
		}, Fn: func(_ string, opts map[string]any) (any, error) {
			values := parseValuesList(optString(opts, "values", ""))
			if len(values) < 2 {
				return nil, fmt.Errorf("oneOf: requires at least 2 values, got %d", len(values))
			}
			return values[rand.Intn(len(values))], nil
		}},
		{Name: "weightedOneOf", Group: "custom-list", Description: "Random pick from a user-typed list, skewed by relative weight", Affinities: []string{"TEXT", "INTEGER", "REAL"}, OptionsSchema: []OptionField{
			{Key: "values", Label: "Values", Kind: OptKindTextarea, Required: true, Description: "value:weight pairs, comma/newline-separated, e.g. PAID:70, PENDING:20, REFUNDED:10"},
		}, Fn: func(_ string, opts map[string]any) (any, error) {
			wvs, err := parseWeightedValues(optString(opts, "values", ""))
			if err != nil {
				return nil, err
			}
			var total float64
			for _, wv := range wvs {
				total += wv.weight
			}
			if total <= 0 {
				return nil, fmt.Errorf("weightedOneOf: all weights are zero")
			}
			r := rand.Float64() * total
			var cum float64
			for _, wv := range wvs {
				cum += wv.weight
				if r < cum {
					return wv.value, nil
				}
			}
			return wvs[len(wvs)-1].value, nil
		}},
		{Name: "regexEnum", Group: "custom-list", Description: "Pick one of several user-typed regex patterns per row, then expand it", Affinities: []string{"TEXT"}, OptionsSchema: []OptionField{
			{Key: "patterns", Label: "Patterns", Kind: OptKindTextarea, Required: true, Description: "One regex per line"},
		}, Fn: func(_ string, opts map[string]any) (any, error) {
			patterns, err := parseRegexPatterns(optString(opts, "patterns", ""))
			if err != nil {
				return nil, err
			}
			pattern := patterns[rand.Intn(len(patterns))]
			return gofakeit.Regex(pattern), nil
		}},
		{Name: "incrementalEnum", Group: "custom-list", Description: "Cycles through a user-typed list in order, one step per row", Affinities: []string{"TEXT", "INTEGER", "REAL"}, Stateful: true, OptionsSchema: []OptionField{
			{Key: "values", Label: "Values", Kind: OptKindTextarea, Required: true, Description: "Comma- or newline-separated list of literal values"},
			{Key: "start", Label: "Start index", Kind: OptKindInt, Default: 0},
			{Key: "step", Label: "Step", Kind: OptKindInt, Default: 1},
		}, Fn: nil},
	}
}

// parseValuesList splits a comma- or newline-separated string into trimmed,
// non-empty entries. Shared by every custom-list generator's "values"-shaped
// option.
func parseValuesList(raw string) []string {
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	parts := strings.FieldsFunc(raw, func(r rune) bool { return r == ',' || r == '\n' })
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

type weightedValue struct {
	value  string
	weight float64
}

// parseWeightedValues parses "value:weight" pairs (bare value -> weight 1).
func parseWeightedValues(raw string) ([]weightedValue, error) {
	parts := parseValuesList(raw)
	if len(parts) == 0 {
		return nil, fmt.Errorf("weightedOneOf: requires at least 1 value")
	}
	out := make([]weightedValue, 0, len(parts))
	for _, p := range parts {
		idx := strings.LastIndex(p, ":")
		if idx == -1 {
			out = append(out, weightedValue{value: p, weight: 1})
			continue
		}
		val := strings.TrimSpace(p[:idx])
		wStr := strings.TrimSpace(p[idx+1:])
		w, err := strconv.ParseFloat(wStr, 64)
		if err != nil || w < 0 {
			return nil, fmt.Errorf("weightedOneOf: invalid weight in %q", p)
		}
		out = append(out, weightedValue{value: val, weight: w})
	}
	return out, nil
}

// parseRegexPatterns parses one regex per line, validating each compiles.
func parseRegexPatterns(raw string) ([]string, error) {
	lines := strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n")
	var out []string
	for i, l := range lines {
		l = strings.TrimSpace(l)
		if l == "" {
			continue
		}
		if _, err := regexp.Compile(l); err != nil {
			return nil, fmt.Errorf("regexEnum: invalid pattern at line %d: %w", i+1, err)
		}
		out = append(out, l)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("regexEnum: requires at least 1 pattern")
	}
	return out, nil
}

// nextIncrementalEnumValue returns the value at idx (wrapped into range) and
// the next index to use, given the configured step.
func nextIncrementalEnumValue(values []string, idx, step int) (string, int) {
	n := len(values)
	i := ((idx % n) + n) % n
	return values[i], idx + step
}
