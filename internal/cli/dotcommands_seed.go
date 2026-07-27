package cli

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/0funct0ry/squad/internal/db"
	"github.com/0funct0ry/squad/internal/seed"
)

const seedMaxRetries = 20

func isUniqueConstraintError(err error) bool {
	return strings.Contains(err.Error(), "UNIQUE constraint failed")
}

// cmdSeed implements ".seed TABLE N": inserts N generated rows into TABLE
// using internal/seed's registry (seed.BuildPlan picks a generator per
// column via the same column-name/type heuristics the web UI's seed plan
// endpoint uses, already resolving FK target table/column into each plan's
// Options -- no separate FK-resolution step needed here). Write-gated like
// .import.
func (s *State) cmdSeed(args []string) {
	if !s.Write {
		s.shellError(fmt.Errorf(".seed is not allowed in read-only mode (READ_ONLY)"))
		return
	}
	if len(args) != 2 {
		s.shellError(fmt.Errorf("usage: .seed TABLE N"))
		return
	}
	table := args[0]
	n, err := strconv.Atoi(args[1])
	if err != nil || n < 1 {
		s.shellError(fmt.Errorf("invalid row count: %s", args[1]))
		return
	}

	schema, err := db.GetTableSchema(s.DB, table)
	if err != nil {
		s.shellError(err)
		return
	}
	if schema.Type == "view" {
		s.shellError(fmt.Errorf("cannot seed a view"))
		return
	}

	plans, err := seed.BuildPlan(s.DB, schema)
	if err != nil {
		s.shellError(err)
		return
	}

	specs := make(map[string]seed.ColumnSpec, len(plans))
	for _, p := range plans {
		if p.Skip || p.Generator == nil {
			continue
		}
		opts := p.Options
		if opts == nil {
			opts = map[string]any{}
		}
		specs[p.Name] = seed.ColumnSpec{Generator: *p.Generator, Options: opts}
	}
	if len(specs) == 0 {
		s.shellError(fmt.Errorf("no insertable columns found for %s", table))
		return
	}

	gen, err := seed.NewRowGenerator(s.DB, schema, specs)
	if err != nil {
		s.shellError(err)
		return
	}

	colNames := make([]string, 0, len(specs))
	for colName := range specs {
		colNames = append(colNames, colName)
	}
	sort.Strings(colNames)

	quotedCols := make([]string, len(colNames))
	placeholders := make([]string, len(colNames))
	for i, c := range colNames {
		quotedCols[i] = db.QuoteIdentifier(c)
		placeholders[i] = "?"
	}
	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
		db.QuoteIdentifier(table), strings.Join(quotedCols, ", "), strings.Join(placeholders, ", "))

	tx, err := s.DB.Begin()
	if err != nil {
		s.shellError(err)
		return
	}

	for i := 0; i < n; i++ {
		row, err := gen.GenerateRow()
		if err != nil {
			tx.Rollback()
			s.shellError(fmt.Errorf("row %d: %w", i+1, err))
			return
		}

		var insertErr error
		for attempt := 0; attempt <= seedMaxRetries; attempt++ {
			vals := make([]any, len(colNames))
			for j, c := range colNames {
				vals[j] = row[c]
			}
			_, insertErr = tx.Exec(query, vals...)
			if insertErr == nil {
				break
			}
			if !isUniqueConstraintError(insertErr) || attempt == seedMaxRetries {
				break
			}
			for _, group := range gen.UniqueGroups() {
				if err := gen.RegenerateGroup(row, group); err != nil {
					insertErr = err
					break
				}
			}
		}
		if insertErr != nil {
			tx.Rollback()
			s.shellError(fmt.Errorf("row %d: %w", i+1, insertErr))
			return
		}
	}

	if err := tx.Commit(); err != nil {
		s.shellError(err)
		return
	}
	if s.Interactive {
		fmt.Fprintf(s.Out, "seeded %d rows into %s\n", n, table)
	}
}
