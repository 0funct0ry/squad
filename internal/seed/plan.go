package seed

import (
	"database/sql"
	"regexp"
	"strconv"
	"strings"

	"github.com/0funct0ry/squad/internal/db"
)

// ColumnPlan describes the suggested (or overridden) seeding behavior for a
// single column, as returned by GET /api/tables/:name/seed/plan.
type ColumnPlan struct {
	Name        string         `json:"name"`
	Type        string         `json:"type"`
	Skip        bool           `json:"skip"`
	Reason      *string        `json:"reason"`
	Generator   *string        `json:"generator"`
	Options     map[string]any `json:"options"`
	UniqueGroup []string       `json:"uniqueGroup,omitempty"`
	// CheckClause is the raw `CHECK (...)` clause text for this column, if any
	// was found in the table's DDL, surfaced for display in the seed UI.
	CheckClause *string `json:"checkClause,omitempty"`
}

// Affinity classifies a SQLite declared column type into one of the five
// storage affinities per SQLite's type affinity rules (empty type -> BLOB).
func Affinity(declaredType string) string {
	t := strings.ToUpper(declaredType)
	switch {
	case strings.Contains(t, "INT"):
		return "INTEGER"
	case strings.Contains(t, "CHAR"), strings.Contains(t, "CLOB"), strings.Contains(t, "TEXT"):
		return "TEXT"
	case strings.Contains(t, "BLOB"), t == "":
		return "BLOB"
	case strings.Contains(t, "REAL"), strings.Contains(t, "FLOA"), strings.Contains(t, "DOUB"):
		return "REAL"
	default:
		return "NUMERIC"
	}
}

// BuildPlan computes the suggested seed plan for every insertable (non-generated)
// column of schema, applying the M6 heuristic priority order.
func BuildPlan(sqlDB *sql.DB, schema *db.TableSchema) ([]ColumnPlan, error) {
	uniqueGroups := computeUniqueGroups(schema)

	var plans []ColumnPlan
	for _, col := range schema.Columns {
		if col.Hidden == 2 || col.Hidden == 3 {
			// Generated columns can't be inserted into.
			continue
		}

		plan := ColumnPlan{
			Name:        col.Name,
			Type:        col.Type,
			UniqueGroup: uniqueGroups[col.Name],
		}

		if reason, ok := autoincrementSkipReason(schema, col); ok {
			plan.Skip = true
			plan.Reason = &reason
			plans = append(plans, plan)
			continue
		}

		if fk, ok := findForeignKey(schema, col.Name); ok {
			refColumn, err := resolveFKColumn(sqlDB, fk)
			if err != nil {
				return nil, err
			}
			gen := ForeignKeyGeneratorName
			plan.Generator = &gen
			// Default to sampling without replacement when this FK column is
			// itself unique-constrained (e.g. a PK that's also a foreignKey,
			// a genuine 1:1 relationship) — otherwise random-with-replacement
			// draws would eventually collide against the real constraint.
			// The user can still override this in the UI.
			isSoloUnique := len(plan.UniqueGroup) == 1 && plan.UniqueGroup[0] == col.Name
			plan.Options = map[string]any{"table": fk.Table, "column": refColumn, "unique": isSoloUnique}
			plans = append(plans, plan)
			continue
		}

		check := findCheckConstraint(schema.DDL, col.Name)
		if check.raw != "" {
			clause := check.raw
			plan.CheckClause = &clause
		}

		if len(check.enumValues) > 0 {
			gen := "oneOf"
			plan.Generator = &gen
			plan.Options = map[string]any{"values": strings.Join(check.enumValues, ", ")}
			plans = append(plans, plan)
			continue
		}

		gen, opts := nameHeuristic(col)
		if gen == "" {
			gen, opts = typeFallback(col)
		}
		if check.hasRange && (gen == "int" || gen == "float") {
			opts["min"] = check.rangeMin
			opts["max"] = check.rangeMax
		}
		plan.Generator = &gen
		plan.Options = opts
		plans = append(plans, plan)
	}

	return plans, nil
}

func autoincrementSkipReason(schema *db.TableSchema, col db.ColumnInfo) (string, bool) {
	if len(schema.PrimaryKey) == 1 && schema.PrimaryKey[0] == col.Name &&
		col.PK == 1 && strings.EqualFold(strings.TrimSpace(col.Type), "INTEGER") {
		return "auto-assigned rowid primary key", true
	}
	return "", false
}

func findForeignKey(schema *db.TableSchema, colName string) (db.ForeignKeyInfo, bool) {
	for _, fk := range schema.ForeignKeys {
		if fk.From == colName {
			return fk, true
		}
	}
	return db.ForeignKeyInfo{}, false
}

// resolveFKColumn returns fk.To if set, otherwise looks up the referenced
// table's own primary key column.
func resolveFKColumn(sqlDB *sql.DB, fk db.ForeignKeyInfo) (string, error) {
	if fk.To != "" {
		return fk.To, nil
	}
	refSchema, err := db.GetTableSchema(sqlDB, fk.Table)
	if err != nil {
		return "", err
	}
	if len(refSchema.PrimaryKey) > 0 {
		return refSchema.PrimaryKey[0], nil
	}
	return "rowid", nil
}

// computeUniqueGroups maps column name -> the group of column names it must
// be jointly unique with (itself for a solo unique/PK column, the full tuple
// for a composite PK).
func computeUniqueGroups(schema *db.TableSchema) map[string][]string {
	groups := make(map[string][]string)

	if len(schema.PrimaryKey) > 1 {
		for _, name := range schema.PrimaryKey {
			groups[name] = schema.PrimaryKey
		}
	} else if len(schema.PrimaryKey) == 1 {
		name := schema.PrimaryKey[0]
		// Only flag as a "uniqueGroup" if it's not the auto-assigned rowid
		// alias case (that column is skipped and never generated anyway).
		var col *db.ColumnInfo
		for i := range schema.Columns {
			if schema.Columns[i].Name == name {
				col = &schema.Columns[i]
				break
			}
		}
		if col != nil {
			if _, isAutoincrement := autoincrementSkipReason(schema, *col); !isAutoincrement {
				groups[name] = []string{name}
			}
		}
	}

	for _, idx := range schema.Indexes {
		if idx.Unique && len(idx.Columns) == 1 {
			name := idx.Columns[0]
			if _, exists := groups[name]; !exists {
				groups[name] = []string{name}
			}
		}
	}

	return groups
}

func nameHeuristic(col db.ColumnInfo) (string, map[string]any) {
	lower := strings.ToLower(col.Name)
	upperType := strings.ToUpper(strings.TrimSpace(col.Type))
	affinity := Affinity(col.Type)

	switch {
	case strings.Contains(lower, "email"):
		return "email", map[string]any{}
	case strings.Contains(lower, "first_name"), strings.Contains(lower, "firstname"):
		return "firstName", map[string]any{}
	case strings.Contains(lower, "last_name"), strings.Contains(lower, "lastname"):
		return "lastName", map[string]any{}
	case lower == "name", strings.HasSuffix(lower, "_name"):
		return "name", map[string]any{}
	case lower == "username":
		return "username", map[string]any{}
	case strings.HasSuffix(lower, "_at"), strings.HasSuffix(lower, "_on"),
		lower == "created_at", lower == "updated_at", lower == "deleted_at",
		strings.Contains(upperType, "DATE"), strings.Contains(upperType, "DATETIME"), strings.Contains(upperType, "TIMESTAMP"):
		opts := map[string]any{}
		if upperType == "DATE" {
			opts["onlyDate"] = true
		}
		return "datetime", opts
	case strings.Contains(lower, "price"), strings.Contains(lower, "amount"), strings.Contains(lower, "cost"), strings.Contains(lower, "total"):
		return "price", map[string]any{}
	case lower == "url", strings.HasSuffix(lower, "_url"):
		return "url", map[string]any{}
	case strings.Contains(lower, "phone"):
		return "phone", map[string]any{}
	case strings.Contains(lower, "uuid"), (lower == "id" || strings.HasSuffix(lower, "_id")) && affinity == "TEXT":
		return "uuid", map[string]any{}
	case strings.HasPrefix(lower, "is_"), strings.HasPrefix(lower, "has_"), upperType == "BOOLEAN":
		return "bool", map[string]any{}
	case strings.Contains(lower, "company"):
		return "company", map[string]any{}
	// ethnicity/ipv6 are checked before the generic "address"/"city"/"ip"
	// substring rules below, since "ethnicity" contains "city" and
	// "ip6_address"/"ipv6_addr" contain "address" -- without this ordering
	// the broader rules would shadow the more specific ones.
	case strings.Contains(lower, "ethnicity"):
		return "ethnicity", map[string]any{}
	case strings.Contains(lower, "ip6"), strings.Contains(lower, "ipv6"):
		return "ipv6", map[string]any{}
	case strings.Contains(lower, "address"):
		return "address", map[string]any{}
	case strings.Contains(lower, "city"):
		return "city", map[string]any{}
	case strings.Contains(lower, "country"):
		return "country", map[string]any{}
	case strings.Contains(lower, "zip"), strings.Contains(lower, "postal"):
		return "zipCode", map[string]any{}
	case lower == "ip", strings.Contains(lower, "ip_address"):
		return "ipv4", map[string]any{}
	case strings.Contains(lower, "ssn"):
		return "ssn", map[string]any{}
	case strings.Contains(lower, "gender"):
		return "gender", map[string]any{}
	case strings.Contains(lower, "lat"):
		return "latitude", map[string]any{}
	case strings.Contains(lower, "lng"), strings.Contains(lower, "lon"):
		return "longitude", map[string]any{}
	case strings.Contains(lower, "credit_card"), strings.Contains(lower, "creditcard"), strings.Contains(lower, "card_number"):
		return "creditCardNumber", map[string]any{}
	case strings.Contains(lower, "age"):
		return "age", map[string]any{}
	case strings.Contains(lower, "state_abr"), strings.Contains(lower, "stateabr"), strings.Contains(lower, "state_abbr"):
		return "stateAbr", map[string]any{}
	case strings.Contains(lower, "state"):
		return "state", map[string]any{}
	}

	// BLOB-only heuristics for the media generators (M6b). These run only
	// for BLOB-affinity columns, after all the checks above and before the
	// typeFallback default of "bytes" for BLOB.
	if affinity == "BLOB" {
		switch {
		case hasWholeToken(lower, "qr"):
			return "qrCode", map[string]any{}
		case strings.Contains(lower, "barcode"), strings.Contains(lower, "upc"), strings.Contains(lower, "ean"):
			return "barcode", map[string]any{}
		case strings.Contains(lower, "avatar"), strings.Contains(lower, "photo"), strings.Contains(lower, "profile_pic"),
			strings.Contains(lower, "profilepicture"), strings.Contains(lower, "headshot"):
			return "profilePicture", map[string]any{}
		case strings.Contains(lower, "svg"):
			return "svgImage", map[string]any{}
		case strings.Contains(lower, "icon"):
			return "icon", map[string]any{}
		case strings.Contains(lower, "audio"), strings.Contains(lower, "sound"), strings.Contains(lower, "clip"):
			return "soundData", map[string]any{}
		}
	}

	return "", nil
}

// hasWholeToken reports whether tok appears in lower as a standalone token,
// i.e. surrounded by non-alphanumeric separators (or the string bounds).
// This avoids e.g. matching "qr" inside an unrelated word.
func hasWholeToken(lower, tok string) bool {
	isSep := func(b byte) bool {
		return !(b >= 'a' && b <= 'z' || b >= '0' && b <= '9')
	}
	for i := 0; i+len(tok) <= len(lower); i++ {
		if lower[i:i+len(tok)] != tok {
			continue
		}
		beforeOK := i == 0 || isSep(lower[i-1])
		afterOK := i+len(tok) == len(lower) || isSep(lower[i+len(tok)])
		if beforeOK && afterOK {
			return true
		}
	}
	return false
}

type checkConstraint struct {
	raw        string
	enumValues []string
	hasRange   bool
	rangeMin   float64
	rangeMax   float64
}

var (
	checkInRe      = regexp.MustCompile(`(?is)CHECK\s*\(\s*([A-Za-z0-9_"'\x60\[\]]+)\s+IN\s*\(([^)]*)\)\s*\)`)
	checkBetweenRe = regexp.MustCompile(`(?is)CHECK\s*\(\s*([A-Za-z0-9_"'\x60\[\]]+)\s+BETWEEN\s+(-?[\d.]+)\s+AND\s+(-?[\d.]+)\s*\)`)
)

// findCheckConstraint scans a table's DDL for a `CHECK (col IN (...))` or
// `CHECK (col BETWEEN x AND y)` clause naming colName, and extracts the enum
// values or numeric range. Returns a zero-value checkConstraint if the DDL has
// no such constraint, or none naming this column, or a shape it doesn't
// recognize (e.g. multi-column or compound CHECK expressions) — those are
// simply left for nameHeuristic/typeFallback to handle as before.
func findCheckConstraint(ddl string, colName string) checkConstraint {
	if ddl == "" {
		return checkConstraint{}
	}
	stripIdent := func(s string) string {
		s = strings.TrimSpace(s)
		return strings.Trim(s, `"'`+"`[]")
	}

	for _, m := range checkInRe.FindAllStringSubmatch(ddl, -1) {
		if !strings.EqualFold(stripIdent(m[1]), colName) {
			continue
		}
		var values []string
		for _, v := range strings.Split(m[2], ",") {
			v = stripIdent(v)
			if v != "" {
				values = append(values, v)
			}
		}
		if len(values) > 0 {
			return checkConstraint{raw: strings.TrimSpace(m[0]), enumValues: values}
		}
	}

	for _, m := range checkBetweenRe.FindAllStringSubmatch(ddl, -1) {
		if !strings.EqualFold(stripIdent(m[1]), colName) {
			continue
		}
		min, errMin := strconv.ParseFloat(m[2], 64)
		max, errMax := strconv.ParseFloat(m[3], 64)
		if errMin == nil && errMax == nil {
			return checkConstraint{raw: strings.TrimSpace(m[0]), hasRange: true, rangeMin: min, rangeMax: max}
		}
	}

	return checkConstraint{}
}

func typeFallback(col db.ColumnInfo) (string, map[string]any) {
	if strings.TrimSpace(col.Type) == "" {
		return "sentence", map[string]any{}
	}
	switch Affinity(col.Type) {
	case "TEXT":
		return "sentence", map[string]any{}
	case "INTEGER":
		return "int", map[string]any{}
	case "REAL", "NUMERIC":
		return "float", map[string]any{}
	case "BLOB":
		return "bytes", map[string]any{}
	default:
		return "sentence", map[string]any{}
	}
}
