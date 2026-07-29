package importer

import "github.com/0funct0ry/squad/internal/db"

// ApplyFieldMapping renames/prunes each parsed row's keys according to
// mapping (file column -> target column; "" or SkipMapping means skip),
// then verifies every NOT NULL, no-default target column in schema is
// covered by the mapping. Returns a *ValidationError (never a partial
// result) if any required column is uncovered.
func ApplyFieldMapping(pf ParsedFile, mapping map[string]string, schema *db.TableSchema) ([]map[string]any, error) {
	mappedTargets := make(map[string]bool)
	for _, target := range mapping {
		if target != "" && target != SkipMapping {
			mappedTargets[target] = true
		}
	}

	var missing []string
	for _, col := range schema.Columns {
		if col.NotNull && col.DefaultVal == nil && !mappedTargets[col.Name] {
			missing = append(missing, col.Name)
		}
	}
	if len(missing) > 0 {
		return nil, &ValidationError{MissingColumns: missing}
	}

	rows := make([]map[string]any, len(pf.Rows))
	for i, r := range pf.Rows {
		out := make(map[string]any, len(mapping))
		for fileCol, target := range mapping {
			if target == "" || target == SkipMapping {
				continue
			}
			out[target] = r[fileCol]
		}
		rows[i] = out
	}
	return rows, nil
}
