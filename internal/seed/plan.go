package seed

import (
	"database/sql"
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
			plan.Options = map[string]any{"table": fk.Table, "column": refColumn}
			plans = append(plans, plan)
			continue
		}

		gen, opts := nameHeuristic(col)
		if gen == "" {
			gen, opts = typeFallback(col)
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
	default:
		return "", nil
	}
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
