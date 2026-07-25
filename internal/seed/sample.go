package seed

import "fmt"

// GenerateSample produces a single one-off preview value for the named
// generator, for use by the table-independent sample endpoint. It is not
// valid for foreignKey or formula (both need live table/row context) --
// callers should reject those before calling GenerateSample.
func GenerateSample(name, affinity string, opts map[string]any) (any, error) {
	def, ok := registry[name]
	if !ok {
		return nil, fmt.Errorf("unknown generator: %s", name)
	}
	if name == ForeignKeyGeneratorName || name == "formula" {
		return nil, fmt.Errorf("generator %s cannot be previewed without table/row context", name)
	}
	if def.Stateful {
		st := &sequenceState{}
		defaultStart := 0
		if name == "rowNumber" {
			defaultStart = 1
		}
		start := optInt(opts, "start", defaultStart)
		st.next = int64(start)
		g := &RowGenerator{}
		return g.generateStateful(ColumnSpec{Generator: name, Options: opts}, st)
	}
	if def.Fn == nil {
		return nil, fmt.Errorf("generator %s cannot be previewed without table/row context", name)
	}
	return def.Fn(affinity, opts)
}
