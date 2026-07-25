package seed

import (
	"fmt"
	"math/rand"
)

// nullWithProbabilityGenerators registers the nullWithProbability
// meta-wrapper generator. Unlike every other generator, it doesn't produce a
// value itself -- it wraps another column's chosen generator and
// occasionally substitutes NULL instead of calling it. Its "generator"
// option holds a nested {generator, options} value (OptKindGenerator),
// rendered in the Generator Picker as a recursive mini generator-picker.
func nullWithProbabilityGenerators() []GeneratorDef {
	return []GeneratorDef{
		{Name: "nullWithProbability", Group: "misc", Description: "Wraps another generator; substitutes NULL some fraction of rows", Affinities: nil, OptionsSchema: []OptionField{
			{Key: "generator", Label: "Wrapped generator", Kind: OptKindGenerator, Required: true, Description: "The generator (and its options) to wrap"},
			{Key: "nullRate", Label: "Null rate", Kind: OptKindFloat, Default: 0.15, Min: floatPtr(0), Max: floatPtr(1)},
		}, Fn: nil},
	}
}

// wrappedGeneratorSpec extracts the wrapped {generator, options} value from
// a nullWithProbability spec's options.generator field.
func wrappedGeneratorSpec(spec ColumnSpec) (string, map[string]any, error) {
	wrapped, ok := spec.Options["generator"].(map[string]any)
	if !ok {
		return "", nil, fmt.Errorf("nullWithProbability: missing options.generator")
	}
	name, _ := wrapped["generator"].(string)
	if name == "" {
		return "", nil, fmt.Errorf("nullWithProbability: options.generator.generator is required")
	}
	opts, _ := wrapped["options"].(map[string]any)
	if opts == nil {
		opts = map[string]any{}
	}
	return name, opts, nil
}

func (g *RowGenerator) evalNullWithProbability(colName string, spec ColumnSpec, rowSoFar map[string]any) (any, error) {
	nullRate := optFloat(spec.Options, "nullRate", 0.15)
	if rand.Float64() < nullRate {
		return nil, nil
	}
	name, opts, err := wrappedGeneratorSpec(spec)
	if err != nil {
		return nil, err
	}
	return g.generateValue(colName, ColumnSpec{Generator: name, Options: opts}, rowSoFar)
}
