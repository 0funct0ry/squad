package seed

import (
	"database/sql"
	"fmt"
	"math/rand"
	"strings"

	"github.com/0funct0ry/squad/internal/db"
)

// ColumnSpec is the caller-supplied generator + options for one column,
// as sent in POST .../seed's `columns` map.
type ColumnSpec struct {
	Generator string
	Options   map[string]any
}

// EmptyReferenceError indicates a foreignKey generator's referenced table has
// zero rows, so no valid FK value can be sampled.
type EmptyReferenceError struct{ Table string }

func (e *EmptyReferenceError) Error() string {
	return fmt.Sprintf("referenced table %q has no rows", e.Table)
}

type fkPool struct {
	values []any
}

// RowGenerator generates rows one at a time for a fixed set of column specs,
// reusing FK sample pools and an in-memory uniqueness pre-check across the
// whole request. It's shared by the dry-run preview (batch generation) and
// the real insert path (row-by-row, with DB-constraint-failure retries).
type RowGenerator struct {
	schema         *db.TableSchema
	colAffinity    map[string]string
	fkPools        map[string]*fkPool
	columns        map[string]ColumnSpec
	relevantGroups [][]string
	seen           map[string]map[string]bool
}

// NewRowGenerator prepares a generator for the given column specs: it samples
// FK pools up front (once per referenced table+column) and returns
// *EmptyReferenceError if any FK-backed column's referenced table has zero rows.
func NewRowGenerator(sqlDB *sql.DB, schema *db.TableSchema, columns map[string]ColumnSpec) (*RowGenerator, error) {
	colAffinity := make(map[string]string, len(schema.Columns))
	for _, c := range schema.Columns {
		colAffinity[c.Name] = Affinity(c.Type)
	}

	fkPools := make(map[string]*fkPool)
	for _, spec := range columns {
		if spec.Generator != ForeignKeyGeneratorName {
			continue
		}
		table, _ := spec.Options["table"].(string)
		column, _ := spec.Options["column"].(string)
		key := table + "\x00" + column
		if _, ok := fkPools[key]; ok {
			continue
		}
		pool, err := sampleForeignKeyPool(sqlDB, table, column)
		if err != nil {
			return nil, err
		}
		if len(pool.values) == 0 {
			return nil, &EmptyReferenceError{Table: table}
		}
		fkPools[key] = pool
	}

	uniqueGroups := computeUniqueGroups(schema)
	var relevantGroups [][]string
	seenGroupKey := map[string]bool{}
	for colName := range columns {
		group, ok := uniqueGroups[colName]
		if !ok {
			continue
		}
		key := strings.Join(group, ",")
		if seenGroupKey[key] {
			continue
		}
		seenGroupKey[key] = true
		relevantGroups = append(relevantGroups, group)
	}

	return &RowGenerator{
		schema:         schema,
		colAffinity:    colAffinity,
		fkPools:        fkPools,
		columns:        columns,
		relevantGroups: relevantGroups,
		seen:           make(map[string]map[string]bool),
	}, nil
}

// UniqueGroups returns the distinct unique-flagged column groups relevant to
// this request's column set (used by the caller to know what to regenerate
// after a DB-level constraint failure).
func (g *RowGenerator) UniqueGroups() [][]string {
	return g.relevantGroups
}

func (g *RowGenerator) generateValue(colName string, spec ColumnSpec) (any, error) {
	if spec.Generator == ForeignKeyGeneratorName {
		table, _ := spec.Options["table"].(string)
		column, _ := spec.Options["column"].(string)
		pool := g.fkPools[table+"\x00"+column]
		return pool.values[rand.Intn(len(pool.values))], nil
	}
	affinity := g.colAffinity[colName]
	return Generate(spec.Generator, affinity, spec.Options)
}

// GenerateRow produces one row, applying the in-memory unique pre-check
// (regenerate up to 20x on a collision against values generated earlier in
// this request).
func (g *RowGenerator) GenerateRow() (map[string]any, error) {
	row := make(map[string]any, len(g.columns))
	for colName, spec := range g.columns {
		v, err := g.generateValue(colName, spec)
		if err != nil {
			return nil, err
		}
		row[colName] = v
	}

	for _, group := range g.relevantGroups {
		groupKey := strings.Join(group, ",")
		if g.seen[groupKey] == nil {
			g.seen[groupKey] = make(map[string]bool)
		}
		for attempt := 0; attempt < 20; attempt++ {
			tupleKey := tupleKeyFor(row, group)
			if !g.seen[groupKey][tupleKey] {
				g.seen[groupKey][tupleKey] = true
				break
			}
			if err := g.RegenerateGroup(row, group); err != nil {
				return nil, err
			}
			if attempt == 19 {
				g.seen[groupKey][tupleKeyFor(row, group)] = true
			}
		}
	}

	return row, nil
}

// RegenerateGroup regenerates the values of the given unique-flagged column
// group within row (columns not present in this request's spec are left
// untouched, since they weren't generated in the first place).
func (g *RowGenerator) RegenerateGroup(row map[string]any, group []string) error {
	for _, colName := range group {
		spec, ok := g.columns[colName]
		if !ok {
			continue
		}
		v, err := g.generateValue(colName, spec)
		if err != nil {
			return err
		}
		row[colName] = v
	}
	return nil
}

func tupleKeyFor(row map[string]any, group []string) string {
	parts := make([]string, len(group))
	for i, col := range group {
		parts[i] = fmt.Sprintf("%v", row[col])
	}
	return strings.Join(parts, "\x1f")
}

// GenerateRows generates `count` rows of fake data for the given column specs,
// without touching the target table. Used by the dry-run preview path.
func GenerateRows(sqlDB *sql.DB, schema *db.TableSchema, columns map[string]ColumnSpec, count int) ([]map[string]any, error) {
	gen, err := NewRowGenerator(sqlDB, schema, columns)
	if err != nil {
		return nil, err
	}

	rows := make([]map[string]any, 0, count)
	for i := 0; i < count; i++ {
		row, err := gen.GenerateRow()
		if err != nil {
			return nil, err
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func sampleForeignKeyPool(sqlDB *sql.DB, table, column string) (*fkPool, error) {
	query := fmt.Sprintf("SELECT %s FROM %s ORDER BY RANDOM() LIMIT 10000",
		db.QuoteIdentifier(column), db.QuoteIdentifier(table))
	rows, err := sqlDB.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var values []any
	for rows.Next() {
		var v any
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		values = append(values, v)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return &fkPool{values: values}, nil
}
