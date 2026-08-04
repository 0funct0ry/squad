package seed

import (
	"database/sql"
	"fmt"
	"math/rand"
	"strings"

	"github.com/0funct0ry/squad/internal/db"
)

// sequenceState holds the mutable per-column counter state for the 4
// stateful generators (sequence, rowNumber, characterSequence, digitSequence).
type sequenceState struct{ next int64 }

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
	// unique marks a pool that must be sampled without replacement, because
	// at least one column drawing from it is itself unique-constrained (e.g.
	// a PK that's also a foreignKey, a genuine 1:1 relationship). pos is the
	// next unused index into values for that case.
	unique bool
	pos    int
}

// UniqueForeignKeyExhaustedError indicates a foreignKey generator's column is
// unique-constrained (e.g. it's also the table's primary key) but the
// referenced table doesn't have enough distinct rows to supply that many
// unique values — a 1:1 relationship that can't be seeded past the smaller
// side's row count.
type UniqueForeignKeyExhaustedError struct {
	Column    string
	Table     string
	RefColumn string
	Available int
	Requested int
}

func (e *UniqueForeignKeyExhaustedError) Error() string {
	return fmt.Sprintf(
		"column %q: foreign key to %s.%s must be unique, but %s only has %d row(s) — cannot generate %d unique values",
		e.Column, e.Table, e.RefColumn, e.Table, e.Available, e.Requested,
	)
}

// RowGenerator generates rows one at a time for a fixed set of column specs,
// reusing FK sample pools and an in-memory uniqueness pre-check across the
// whole request. It's shared by the dry-run preview (batch generation) and
// the real insert path (row-by-row, with DB-constraint-failure retries).
type RowGenerator struct {
	schema         *db.TableSchema
	colAffinity    map[string]string
	fkPools        map[string]*fkPool
	enumPools      map[string]*fkPool
	columns        map[string]ColumnSpec
	relevantGroups [][]string
	seen           map[string]map[string]bool
	orderedColumns []string
	formulaDeps    map[string][]string
	seqState       map[string]*sequenceState
	enumSeqState   map[string]*enumSequenceState
}

// enumSequenceState holds the mutable per-column cursor state for the
// incrementalEnum generator, which cycles through a user-typed value list in
// order rather than counting a bare int64 like sequenceState.
type enumSequenceState struct {
	values []string
	next   int
	step   int
}

// NewRowGenerator prepares a generator for the given column specs: it samples
// FK pools up front (once per referenced table+column) and returns
// *EmptyReferenceError if any FK-backed column's referenced table has zero
// rows, or *UniqueForeignKeyExhaustedError if a unique-constrained FK column
// (e.g. a PK that's also a foreignKey) doesn't have enough distinct
// referenced rows to supply `count` unique values. count is the number of
// rows the caller intends to generate with this generator; pass 0 if unknown
// (e.g. a caller that only wants to peek at a handful of rows without a
// unique-FK guarantee).
func NewRowGenerator(sqlDB *sql.DB, schema *db.TableSchema, columns map[string]ColumnSpec, count int) (*RowGenerator, error) {
	colAffinity := make(map[string]string, len(schema.Columns))
	for _, c := range schema.Columns {
		colAffinity[c.Name] = Affinity(c.Type)
	}

	uniqueGroups := computeUniqueGroups(schema)
	isSoloUnique := func(colName string) bool {
		group, ok := uniqueGroups[colName]
		return ok && len(group) == 1 && group[0] == colName
	}

	fkPools := make(map[string]*fkPool)
	for colName, spec := range columns {
		if spec.Generator != ForeignKeyGeneratorName {
			continue
		}
		table, _ := spec.Options["table"].(string)
		column, _ := spec.Options["column"].(string)
		key := table + "\x00" + column
		pool, ok := fkPools[key]
		if !ok {
			var err error
			pool, err = sampleForeignKeyPool(sqlDB, table, column)
			if err != nil {
				return nil, err
			}
			if len(pool.values) == 0 {
				return nil, &EmptyReferenceError{Table: table}
			}
			fkPools[key] = pool
		}
		wantUnique := isSoloUnique(colName)
		if v, ok := spec.Options["unique"].(bool); ok {
			// Explicit user override (surfaced as a UI toggle): sample without
			// replacement even if the column isn't itself unique-constrained,
			// or allow replacement (and the collision risk that comes with
			// it) even for a column that is, if the user's opted into that.
			wantUnique = v
		}
		if wantUnique {
			pool.unique = true
			if count > len(pool.values) {
				return nil, &UniqueForeignKeyExhaustedError{
					Column: colName, Table: table, RefColumn: column,
					Available: len(pool.values), Requested: count,
				}
			}
		}
	}

	enumPools := make(map[string]*fkPool)
	for _, spec := range columns {
		if spec.Generator != "enumFromColumn" {
			continue
		}
		table, _ := spec.Options["table"].(string)
		column, _ := spec.Options["column"].(string)
		key := table + "\x00" + column
		if _, ok := enumPools[key]; ok {
			continue
		}
		pool, err := sampleDistinctColumnPool(sqlDB, table, column, 500)
		if err != nil {
			return nil, err
		}
		if len(pool.values) == 0 {
			return nil, &EmptyReferenceError{Table: table}
		}
		enumPools[key] = pool
	}

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

	formulaDeps := buildCrossColumnDeps(columns)
	orderedColumns, err := topoSortColumns(columns, formulaDeps)
	if err != nil {
		return nil, err
	}

	seqState := make(map[string]*sequenceState)
	for colName, spec := range columns {
		if !statefulGeneratorNames[spec.Generator] {
			continue
		}
		var defaultStart int64
		switch spec.Generator {
		case "rowNumber":
			defaultStart = 1
		default:
			defaultStart = 0
		}
		start := optInt(spec.Options, "start", int(defaultStart))
		seqState[colName] = &sequenceState{next: int64(start)}
	}

	enumSeqState := make(map[string]*enumSequenceState)
	for colName, spec := range columns {
		if spec.Generator != "incrementalEnum" {
			continue
		}
		values := parseValuesList(optString(spec.Options, "values", ""))
		if len(values) == 0 {
			return nil, fmt.Errorf("incrementalEnum column %q: requires at least 1 value", colName)
		}
		start := optInt(spec.Options, "start", 0)
		step := optInt(spec.Options, "step", 1)
		enumSeqState[colName] = &enumSequenceState{values: values, next: start, step: step}
	}

	return &RowGenerator{
		schema:         schema,
		colAffinity:    colAffinity,
		fkPools:        fkPools,
		enumPools:      enumPools,
		columns:        columns,
		relevantGroups: relevantGroups,
		seen:           make(map[string]map[string]bool),
		orderedColumns: orderedColumns,
		formulaDeps:    formulaDeps,
		seqState:       seqState,
		enumSeqState:   enumSeqState,
	}, nil
}

// UniqueGroups returns the distinct unique-flagged column groups relevant to
// this request's column set (used by the caller to know what to regenerate
// after a DB-level constraint failure).
func (g *RowGenerator) UniqueGroups() [][]string {
	return g.relevantGroups
}

func (g *RowGenerator) generateValue(colName string, spec ColumnSpec, rowSoFar map[string]any) (any, error) {
	if spec.Generator == ForeignKeyGeneratorName {
		table, _ := spec.Options["table"].(string)
		column, _ := spec.Options["column"].(string)
		pool := g.fkPools[table+"\x00"+column]
		if pool.unique {
			if pool.pos >= len(pool.values) {
				return nil, &UniqueForeignKeyExhaustedError{
					Column: colName, Table: table, RefColumn: column,
					Available: len(pool.values), Requested: pool.pos + 1,
				}
			}
			v := pool.values[pool.pos]
			pool.pos++
			return v, nil
		}
		return pool.values[rand.Intn(len(pool.values))], nil
	}
	if spec.Generator == "enumFromColumn" {
		table, _ := spec.Options["table"].(string)
		column, _ := spec.Options["column"].(string)
		pool := g.enumPools[table+"\x00"+column]
		return pool.values[rand.Intn(len(pool.values))], nil
	}
	if spec.Generator == "formula" {
		return g.evalFormula(colName, spec, rowSoFar)
	}
	if spec.Generator == "dependentOneOf" {
		return g.evalDependentOneOf(colName, spec, rowSoFar)
	}
	if spec.Generator == "customDateSequence" {
		return g.evalCustomDateSequence(colName, spec, rowSoFar)
	}
	if spec.Generator == "statusTransitionLog" {
		return g.evalStatusTransitionLog(colName, spec, rowSoFar)
	}
	if spec.Generator == "checksumOfColumns" {
		return g.evalChecksumOfColumns(colName, spec, rowSoFar)
	}
	if spec.Generator == "slugFromColumn" {
		return g.evalSlugFromColumn(colName, spec, rowSoFar)
	}
	if spec.Generator == "jsonTemplate" {
		return g.evalJSONTemplate(colName, spec, rowSoFar)
	}
	if spec.Generator == "template" {
		return g.evalTemplate(colName, spec, rowSoFar)
	}
	if spec.Generator == "geohash" {
		return g.evalGeohash(spec, rowSoFar)
	}
	if spec.Generator == "nullWithProbability" {
		return g.evalNullWithProbability(colName, spec, rowSoFar)
	}
	if st, ok := g.enumSeqState[colName]; ok {
		val, next := nextIncrementalEnumValue(st.values, st.next, st.step)
		st.next = next
		return val, nil
	}
	if st, ok := g.seqState[colName]; ok {
		return g.generateStateful(spec, st)
	}
	affinity := g.colAffinity[colName]
	return Generate(spec.Generator, affinity, spec.Options)
}

// generateStateful produces the next value for one of the 4 stateful
// generators and advances its counter by the configured step.
func (g *RowGenerator) generateStateful(spec ColumnSpec, st *sequenceState) (any, error) {
	step := int64(optInt(spec.Options, "step", 1))
	cur := st.next
	st.next += step

	switch spec.Generator {
	case "sequence", "rowNumber":
		format := optString(spec.Options, "format", "")
		if format != "" {
			return fmt.Sprintf(format, cur), nil
		}
		return cur, nil
	case "characterSequence":
		return characterSequenceLabel(cur), nil
	case "digitSequence":
		width := optInt(spec.Options, "width", 6)
		s := fmt.Sprintf("%d", cur)
		if len(s) < width {
			s = strings.Repeat("0", width-len(s)) + s
		}
		return s, nil
	default:
		return nil, fmt.Errorf("unknown stateful generator: %s", spec.Generator)
	}
}

// characterSequenceLabel converts a 0-based index into a base-26 letter
// label: 0->"A", 25->"Z", 26->"AA", 27->"AB", ... (spreadsheet-column style).
func characterSequenceLabel(n int64) string {
	if n < 0 {
		n = 0
	}
	var b []byte
	for {
		rem := n % 26
		b = append([]byte{byte('A' + rem)}, b...)
		n = n/26 - 1
		if n < 0 {
			break
		}
	}
	return string(b)
}

// GenerateRow produces one row, applying the in-memory unique pre-check
// (regenerate up to 20x on a collision against values generated earlier in
// this request).
func (g *RowGenerator) GenerateRow() (map[string]any, error) {
	row := make(map[string]any, len(g.columns))
	for _, colName := range g.orderedColumns {
		spec := g.columns[colName]
		v, err := g.generateValue(colName, spec, row)
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
		v, err := g.generateValue(colName, spec, row)
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
	gen, err := NewRowGenerator(sqlDB, schema, columns, count)
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

// sampleDistinctColumnPool samples up to limit distinct, non-NULL values
// actually present in table.column, for the enumFromColumn generator.
func sampleDistinctColumnPool(sqlDB *sql.DB, table, column string, limit int) (*fkPool, error) {
	query := fmt.Sprintf("SELECT DISTINCT %s FROM %s WHERE %s IS NOT NULL ORDER BY RANDOM() LIMIT %d",
		db.QuoteIdentifier(column), db.QuoteIdentifier(table), db.QuoteIdentifier(column), limit)
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
