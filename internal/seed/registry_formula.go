package seed

// formulaGenerators registers the cross-column "formula" generator. Like
// foreignKey and the stateful sequence generators, it is never invoked
// through the normal Generate() dispatch (Fn: nil) -- RowGenerator special-
// cases it via evalFormula, using topologically-ordered column generation so
// referenced sibling columns are already populated in rowSoFar.
func formulaGenerators() []GeneratorDef {
	return []GeneratorDef{
		{
			Name:        "formula",
			Group:       "cross-column",
			Description: "Compute a value from other columns in the same row using a simple expression",
			Affinities:  []string{"TEXT", "INTEGER", "REAL", "NUMERIC"},
			OptionsSchema: []OptionField{
				{Key: "columns", Label: "Referenced columns", Kind: OptKindColumns, Required: true, Description: "Sibling columns this formula reads"},
				{Key: "expression", Label: "Expression", Kind: OptKindString, Required: true, Description: "e.g. price * qty, or round(price * qty), upper(concat(first, last)), sha256(email)"},
			},
			Fn: nil,
		},
	}
}
