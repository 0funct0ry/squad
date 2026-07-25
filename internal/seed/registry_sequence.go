package seed

// sequenceGenerators registers the 4 stateful per-request counter generators.
// These never run through the normal Generate() dispatch (Fn: nil, mirroring
// the foreignKey sentinel) -- RowGenerator special-cases them by name.
func sequenceGenerators() []GeneratorDef {
	return []GeneratorDef{
		{Name: "sequence", Group: "identifier", Description: "Incrementing counter starting at a configurable value", Affinities: []string{"INTEGER", "TEXT"}, Stateful: true, OptionsSchema: []OptionField{
			{Key: "start", Label: "Start", Kind: OptKindInt, Default: 0},
			{Key: "step", Label: "Step", Kind: OptKindInt, Default: 1},
			{Key: "format", Label: "Format (e.g. ROW-%04d)", Kind: OptKindString},
		}, Fn: nil},
		{Name: "rowNumber", Group: "identifier", Description: "1-based row counter", Affinities: []string{"INTEGER", "TEXT"}, Stateful: true, OptionsSchema: []OptionField{
			{Key: "start", Label: "Start", Kind: OptKindInt, Default: 1},
			{Key: "step", Label: "Step", Kind: OptKindInt, Default: 1},
			{Key: "format", Label: "Format (e.g. ROW-%04d)", Kind: OptKindString},
		}, Fn: nil},
		{Name: "characterSequence", Group: "identifier", Description: "Base-26 letter labeling: A, B, ..., Z, AA, AB, ...", Affinities: []string{"TEXT"}, Stateful: true, OptionsSchema: []OptionField{
			{Key: "start", Label: "Start index (0 = A)", Kind: OptKindInt, Default: 0},
			{Key: "step", Label: "Step", Kind: OptKindInt, Default: 1},
		}, Fn: nil},
		{Name: "digitSequence", Group: "identifier", Description: "Zero-padded incrementing digit string", Affinities: []string{"TEXT"}, Stateful: true, OptionsSchema: []OptionField{
			{Key: "start", Label: "Start", Kind: OptKindInt, Default: 0},
			{Key: "step", Label: "Step", Kind: OptKindInt, Default: 1},
			{Key: "width", Label: "Width", Kind: OptKindInt, Default: 6},
		}, Fn: nil},
	}
}

// statefulGeneratorNames lists the 4 names special-cased by RowGenerator.
var statefulGeneratorNames = map[string]bool{
	"sequence":          true,
	"rowNumber":         true,
	"characterSequence": true,
	"digitSequence":     true,
}
