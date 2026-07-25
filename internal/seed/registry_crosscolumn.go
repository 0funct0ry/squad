package seed

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/rand"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/brianvoe/gofakeit/v7"
)

// crossColumnGeneratorNames lists the generators (besides "formula") that
// read already-generated sibling column values via their "columns" option,
// dispatched by RowGenerator.generateValue and ordered by topoSortColumns
// alongside formula (see formula.go's buildCrossColumnDeps).
var crossColumnGeneratorNames = map[string]bool{
	"formula":             true,
	"dependentOneOf":      true,
	"customDateSequence":  true,
	"statusTransitionLog": true,
	"checksumOfColumns":   true,
	"slugFromColumn":      true,
	"jsonTemplate":        true,
}

// crossColumnExtraGenerators registers the 6 new cross-column generators
// (formula itself is registered separately by formulaGenerators). All are
// Fn: nil and special-cased in generate.go's generateValue.
func crossColumnExtraGenerators() []GeneratorDef {
	return []GeneratorDef{
		{Name: "dependentOneOf", Group: "cross-column", Description: "Random pick from a value list chosen by another column's actual value", Affinities: []string{"TEXT", "INTEGER", "REAL"}, OptionsSchema: []OptionField{
			{Key: "columns", Label: "Dependency column", Kind: OptKindColumns, Required: true, Description: "Exactly one sibling column this reads"},
			{Key: "cases", Label: "Cases", Kind: OptKindTextarea, Required: true, Description: "whenValue => v1|v2|... per line; optional trailing 'default => ...'"},
		}, Fn: nil},
		{Name: "customDateSequence", Group: "cross-column", Description: "Coherent multi-milestone datetime timeline across several columns", Affinities: []string{"TEXT", "INTEGER"}, OptionsSchema: []OptionField{
			{Key: "columns", Label: "Milestone columns (ordered)", Kind: OptKindColumns, Required: true, Description: "This column's own name must be included at its position in the timeline"},
			{Key: "gaps", Label: "Gaps", Kind: OptKindTextarea, Required: true, Description: "One minMinutes-maxMinutes range per milestone step, e.g. 10-120,60-1440"},
			{Key: "skipProbability", Label: "Skip probability", Kind: OptKindFloat, Default: 0, Min: floatPtr(0), Max: floatPtr(1)},
		}, Fn: nil},
		{Name: "statusTransitionLog", Group: "cross-column", Description: "Ordered history string consistent with the row's real status column", Affinities: []string{"TEXT"}, OptionsSchema: []OptionField{
			{Key: "columns", Label: "Status column", Kind: OptKindColumns, Required: true, Description: "Exactly one sibling column holding the row's real status"},
			{Key: "transitions", Label: "Transitions", Kind: OptKindTextarea, Required: true, Description: "fromStatus => to1,to2 per line"},
		}, Fn: nil},
		{Name: "checksumOfColumns", Group: "cross-column", Description: "Real hash computed over other already-generated columns' values", Affinities: []string{"TEXT"}, OptionsSchema: []OptionField{
			{Key: "columns", Label: "Source columns", Kind: OptKindColumns, Required: true},
			{Key: "algorithm", Label: "Algorithm", Kind: OptKindSelect, Default: "sha256", Choices: []string{"md5", "sha1", "sha256"}},
			{Key: "separator", Label: "Separator", Kind: OptKindString, Default: "|"},
		}, Fn: nil},
		{Name: "slugFromColumn", Group: "cross-column", Description: "URL-safe slug derived from another already-generated column's value", Affinities: []string{"TEXT"}, OptionsSchema: []OptionField{
			{Key: "columns", Label: "Source column", Kind: OptKindColumns, Required: true, Description: "Exactly one sibling column"},
			{Key: "suffixLength", Label: "Suffix length", Kind: OptKindInt, Default: 0, Description: "0 = no suffix; >0 appends that many random alphanumeric chars"},
		}, Fn: nil},
		{Name: "jsonTemplate", Group: "cross-column", Description: "Fills a JSON template with sibling-column and nested-generator tokens", Affinities: []string{"TEXT", "BLOB"}, OptionsSchema: []OptionField{
			{Key: "columns", Label: "Referenced columns", Kind: OptKindColumns, Description: "Sibling columns referenced by {{column:name}} tokens"},
			{Key: "template", Label: "Template", Kind: OptKindTextarea, Required: true, Description: "JSON text with {{column:name}} and {{generator:name(options)}} tokens"},
		}, Fn: nil},
	}
}

// ---------------------------------------------------------------------
// dependentOneOf
// ---------------------------------------------------------------------

type dependentCase struct {
	when      string
	isDefault bool
	values    []string
}

func parseDependentCases(raw string) ([]dependentCase, error) {
	lines := strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n")
	var out []dependentCase
	for i, l := range lines {
		l = strings.TrimSpace(l)
		if l == "" {
			continue
		}
		idx := strings.Index(l, "=>")
		if idx == -1 {
			return nil, fmt.Errorf("dependentOneOf: invalid case at line %d: missing '=>'", i+1)
		}
		key := strings.TrimSpace(l[:idx])
		valsRaw := strings.TrimSpace(l[idx+2:])
		var vals []string
		for _, v := range strings.Split(valsRaw, "|") {
			v = strings.TrimSpace(v)
			if v != "" {
				vals = append(vals, v)
			}
		}
		if len(vals) == 0 {
			return nil, fmt.Errorf("dependentOneOf: case at line %d has no values", i+1)
		}
		out = append(out, dependentCase{when: key, isDefault: key == "default", values: vals})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("dependentOneOf: requires at least 1 case")
	}
	return out, nil
}

func (g *RowGenerator) evalDependentOneOf(colName string, spec ColumnSpec, rowSoFar map[string]any) (any, error) {
	depCols := optStringSlice(spec.Options, "columns")
	if len(depCols) != 1 {
		return nil, fmt.Errorf("dependentOneOf column %q: requires exactly 1 dependency column, got %d", colName, len(depCols))
	}
	depVal := fmt.Sprintf("%v", rowSoFar[depCols[0]])
	cases, err := parseDependentCases(optString(spec.Options, "cases", ""))
	if err != nil {
		return nil, err
	}
	var def *dependentCase
	for i := range cases {
		c := &cases[i]
		if c.isDefault {
			def = c
			continue
		}
		if c.when == depVal {
			return c.values[rand.Intn(len(c.values))], nil
		}
	}
	if def != nil {
		return def.values[rand.Intn(len(def.values))], nil
	}
	return nil, fmt.Errorf("dependentOneOf column %q: no case matches value %q and no default defined", colName, depVal)
}

// ---------------------------------------------------------------------
// customDateSequence
// ---------------------------------------------------------------------

func parseGaps(raw string) ([][2]time.Duration, error) {
	parts := parseValuesList(raw)
	if len(parts) == 0 {
		return nil, fmt.Errorf("customDateSequence: requires at least 1 gap")
	}
	out := make([][2]time.Duration, 0, len(parts))
	for _, p := range parts {
		idx := strings.Index(p, "-")
		if idx <= 0 {
			return nil, fmt.Errorf("customDateSequence: invalid gap %q", p)
		}
		minStr := strings.TrimSpace(p[:idx])
		maxStr := strings.TrimSpace(p[idx+1:])
		minM, err1 := strconv.Atoi(minStr)
		maxM, err2 := strconv.Atoi(maxStr)
		if err1 != nil || err2 != nil || minM < 0 || maxM < minM {
			return nil, fmt.Errorf("customDateSequence: invalid gap %q", p)
		}
		out = append(out, [2]time.Duration{time.Duration(minM) * time.Minute, time.Duration(maxM) * time.Minute})
	}
	return out, nil
}

func formatDateForAffinity(t time.Time, affinity string) any {
	if affinity == "INTEGER" {
		return t.Unix()
	}
	return t.Format(time.RFC3339)
}

func parseAnyTime(v any) (time.Time, error) {
	switch t := v.(type) {
	case string:
		return time.Parse(time.RFC3339, t)
	case int64:
		return time.Unix(t, 0), nil
	case int:
		return time.Unix(int64(t), 0), nil
	default:
		return time.Time{}, fmt.Errorf("unrecognized time value type %T", v)
	}
}

func (g *RowGenerator) evalCustomDateSequence(colName string, spec ColumnSpec, rowSoFar map[string]any) (any, error) {
	milestones := optStringSlice(spec.Options, "columns")
	idx := -1
	for i, m := range milestones {
		if m == colName {
			idx = i
			break
		}
	}
	if idx == -1 {
		return nil, fmt.Errorf("customDateSequence column %q: not found in its own columns list", colName)
	}
	gaps, err := parseGaps(optString(spec.Options, "gaps", ""))
	if err != nil {
		return nil, err
	}
	if len(gaps) < len(milestones)-1 {
		return nil, fmt.Errorf("customDateSequence column %q: expected at least %d gaps for %d milestones, got %d", colName, len(milestones)-1, len(milestones), len(gaps))
	}
	affinity := g.colAffinity[colName]

	if idx == 0 {
		return formatDateForAffinity(time.Now(), affinity), nil
	}

	skipProb := optFloat(spec.Options, "skipProbability", 0)
	if skipProb > 0 && rand.Float64() < skipProb {
		return nil, nil
	}

	anchor := time.Now()
	for i := idx - 1; i >= 0; i-- {
		raw, ok := rowSoFar[milestones[i]]
		if !ok || raw == nil {
			continue
		}
		t, perr := parseAnyTime(raw)
		if perr == nil {
			anchor = t
			break
		}
	}

	gap := gaps[idx-1]
	delta := gap[0]
	if gap[1] > gap[0] {
		delta += time.Duration(rand.Int63n(int64(gap[1] - gap[0])))
	}
	return formatDateForAffinity(anchor.Add(delta), affinity), nil
}

// ---------------------------------------------------------------------
// statusTransitionLog
// ---------------------------------------------------------------------

func parseTransitions(raw string) (map[string][]string, error) {
	lines := strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n")
	adj := make(map[string][]string)
	for i, l := range lines {
		l = strings.TrimSpace(l)
		if l == "" {
			continue
		}
		idx := strings.Index(l, "=>")
		if idx == -1 {
			return nil, fmt.Errorf("statusTransitionLog: invalid rule at line %d: missing '=>'", i+1)
		}
		from := strings.TrimSpace(l[:idx])
		toRaw := strings.TrimSpace(l[idx+2:])
		var tos []string
		for _, t := range strings.Split(toRaw, ",") {
			t = strings.TrimSpace(t)
			if t != "" {
				tos = append(tos, t)
			}
		}
		if from == "" || len(tos) == 0 {
			return nil, fmt.Errorf("statusTransitionLog: invalid rule at line %d", i+1)
		}
		adj[from] = append(adj[from], tos...)
	}
	if len(adj) == 0 {
		return nil, fmt.Errorf("statusTransitionLog: requires at least 1 transition rule")
	}
	return adj, nil
}

func bfsPathTo(adj map[string][]string, start, target string) []string {
	if start == target {
		return []string{start}
	}
	visited := map[string]bool{start: true}
	type node struct {
		name string
		path []string
	}
	queue := []node{{start, []string{start}}}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, next := range adj[cur.name] {
			if visited[next] {
				continue
			}
			newPath := append(append([]string{}, cur.path...), next)
			if next == target {
				return newPath
			}
			visited[next] = true
			queue = append(queue, node{next, newPath})
		}
	}
	return nil
}

func (g *RowGenerator) evalStatusTransitionLog(colName string, spec ColumnSpec, rowSoFar map[string]any) (any, error) {
	depCols := optStringSlice(spec.Options, "columns")
	if len(depCols) != 1 {
		return nil, fmt.Errorf("statusTransitionLog column %q: requires exactly 1 dependency column, got %d", colName, len(depCols))
	}
	target := fmt.Sprintf("%v", rowSoFar[depCols[0]])
	adj, err := parseTransitions(optString(spec.Options, "transitions", ""))
	if err != nil {
		return nil, err
	}

	allNodes := map[string]bool{}
	isTarget := map[string]bool{}
	for from, tos := range adj {
		allNodes[from] = true
		for _, to := range tos {
			allNodes[to] = true
			isTarget[to] = true
		}
	}
	var starts []string
	for n := range allNodes {
		if !isTarget[n] {
			starts = append(starts, n)
		}
	}
	if len(starts) == 0 {
		for n := range allNodes {
			starts = append(starts, n)
		}
	}
	sortStrings(starts)

	var best []string
	for _, i := range rand.Perm(len(starts)) {
		if path := bfsPathTo(adj, starts[i], target); path != nil {
			best = path
			break
		}
	}
	if best == nil {
		return nil, fmt.Errorf("statusTransitionLog column %q: no valid transition path to status %q", colName, target)
	}
	return strings.Join(best, "→"), nil
}

// ---------------------------------------------------------------------
// checksumOfColumns
// ---------------------------------------------------------------------

func (g *RowGenerator) evalChecksumOfColumns(colName string, spec ColumnSpec, rowSoFar map[string]any) (any, error) {
	cols := optStringSlice(spec.Options, "columns")
	if len(cols) == 0 {
		return nil, fmt.Errorf("checksumOfColumns column %q: requires at least 1 source column", colName)
	}
	sep := optString(spec.Options, "separator", "|")
	algorithm := optString(spec.Options, "algorithm", "sha256")
	parts := make([]string, len(cols))
	for i, c := range cols {
		parts[i] = fmt.Sprintf("%v", rowSoFar[c])
	}
	data := []byte(strings.Join(parts, sep))
	switch algorithm {
	case "md5":
		sum := md5.Sum(data)
		return hex.EncodeToString(sum[:]), nil
	case "sha1":
		sum := sha1.Sum(data)
		return hex.EncodeToString(sum[:]), nil
	case "sha256":
		sum := sha256.Sum256(data)
		return hex.EncodeToString(sum[:]), nil
	default:
		return nil, fmt.Errorf("checksumOfColumns column %q: unsupported algorithm %q", colName, algorithm)
	}
}

// ---------------------------------------------------------------------
// slugFromColumn
// ---------------------------------------------------------------------

var slugNonAlnum = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(s string) string {
	s = strings.ToLower(s)
	s = slugNonAlnum.ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}

func (g *RowGenerator) evalSlugFromColumn(colName string, spec ColumnSpec, rowSoFar map[string]any) (any, error) {
	cols := optStringSlice(spec.Options, "columns")
	if len(cols) != 1 {
		return nil, fmt.Errorf("slugFromColumn column %q: requires exactly 1 source column, got %d", colName, len(cols))
	}
	source := fmt.Sprintf("%v", rowSoFar[cols[0]])
	slug := slugify(source)
	suffixLen := optInt(spec.Options, "suffixLength", 0)
	if suffixLen > 0 {
		slug += "-" + strings.ToLower(gofakeit.LetterN(uint(suffixLen)))
	}
	return slug, nil
}

// ---------------------------------------------------------------------
// jsonTemplate
// ---------------------------------------------------------------------

var jsonTemplateTokenRe = regexp.MustCompile(`\{\{(column|generator):([a-zA-Z0-9_]+)(\([^)]*\))?\}\}`)

func (g *RowGenerator) evalJSONTemplate(colName string, spec ColumnSpec, rowSoFar map[string]any) (any, error) {
	tmpl := optString(spec.Options, "template", "")
	if tmpl == "" {
		return nil, fmt.Errorf("jsonTemplate column %q: missing options.template", colName)
	}

	var innerErr error
	result := jsonTemplateTokenRe.ReplaceAllStringFunc(tmpl, func(match string) string {
		if innerErr != nil {
			return match
		}
		parts := jsonTemplateTokenRe.FindStringSubmatch(match)
		kind, name, argsRaw := parts[1], parts[2], parts[3]

		if kind == "column" {
			v, ok := rowSoFar[name]
			if !ok {
				innerErr = fmt.Errorf("jsonTemplate column %q: unknown column reference %q", colName, name)
				return match
			}
			return fmt.Sprintf("%v", v)
		}

		// generator token: only plain Fn-based generators can be called here,
		// since this string-substitution path has no RowGenerator/row context
		// for FK/formula/stateful/cross-column/nullWithProbability/enumFromColumn.
		if !Exists(name) || name == ForeignKeyGeneratorName || name == "enumFromColumn" ||
			name == "nullWithProbability" || crossColumnGeneratorNames[name] || statefulGeneratorNames[name] {
			innerErr = fmt.Errorf("jsonTemplate column %q: generator %q cannot be used as a nested token (requires row/table context)", colName, name)
			return match
		}

		var opts map[string]any
		if argsRaw != "" {
			argsJSON := strings.TrimSuffix(strings.TrimPrefix(argsRaw, "("), ")")
			if argsJSON != "" {
				if err := json.Unmarshal([]byte(argsJSON), &opts); err != nil {
					innerErr = fmt.Errorf("jsonTemplate column %q: invalid options for generator %q: %w", colName, name, err)
					return match
				}
			}
		}
		v, err := Generate(name, "TEXT", opts)
		if err != nil {
			innerErr = fmt.Errorf("jsonTemplate column %q: generator %q: %w", colName, name, err)
			return match
		}
		return fmt.Sprintf("%v", v)
	})
	if innerErr != nil {
		return nil, innerErr
	}
	if !json.Valid([]byte(result)) {
		return nil, fmt.Errorf("jsonTemplate column %q: substituted output is not valid JSON: %s", colName, result)
	}
	return result, nil
}
