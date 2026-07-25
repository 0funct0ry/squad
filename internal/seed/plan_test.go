package seed

import (
	"testing"

	"github.com/0funct0ry/squad/internal/db"
)

func planFor(t *testing.T, plans []ColumnPlan, name string) ColumnPlan {
	t.Helper()
	for _, p := range plans {
		if p.Name == name {
			return p
		}
	}
	t.Fatalf("no plan for column %s", name)
	return ColumnPlan{}
}

func TestBuildPlan_AutoincrementSkip(t *testing.T) {
	database := openScratchExample(t, "blog")
	schema, err := db.GetTableSchema(database, "users")
	if err != nil {
		t.Fatal(err)
	}
	plans, err := BuildPlan(database, schema)
	if err != nil {
		t.Fatal(err)
	}

	p := planFor(t, plans, "id")
	if !p.Skip {
		t.Errorf("expected id to be skipped")
	}
	if p.Reason == nil || *p.Reason != "auto-assigned rowid primary key" {
		t.Errorf("unexpected reason: %v", p.Reason)
	}
	if p.Generator != nil {
		t.Errorf("expected no generator for skipped column, got %v", *p.Generator)
	}
}

func TestBuildPlan_ForeignKey(t *testing.T) {
	database := openScratchExample(t, "blog")
	schema, err := db.GetTableSchema(database, "posts")
	if err != nil {
		t.Fatal(err)
	}
	plans, err := BuildPlan(database, schema)
	if err != nil {
		t.Fatal(err)
	}

	p := planFor(t, plans, "author_id")
	if p.Generator == nil || *p.Generator != ForeignKeyGeneratorName {
		t.Fatalf("expected foreignKey generator, got %v", p.Generator)
	}
	if p.Options["table"] != "users" || p.Options["column"] != "id" {
		t.Errorf("unexpected fk options: %+v", p.Options)
	}
}

func TestBuildPlan_NameHeuristics(t *testing.T) {
	database := openScratchExample(t, "blog")
	schema, err := db.GetTableSchema(database, "users")
	if err != nil {
		t.Fatal(err)
	}
	plans, err := BuildPlan(database, schema)
	if err != nil {
		t.Fatal(err)
	}

	cases := map[string]string{
		"username":   "username",
		"email":      "email",
		"created_at": "datetime",
		"is_active":  "bool",
	}
	for col, wantGen := range cases {
		p := planFor(t, plans, col)
		if p.Generator == nil || *p.Generator != wantGen {
			t.Errorf("column %s: expected generator %s, got %v", col, wantGen, p.Generator)
		}
	}
}

func TestBuildPlan_TypeFallback(t *testing.T) {
	database := openScratchExample(t, "types_zoo")
	schema, err := db.GetTableSchema(database, "affinities")
	if err != nil {
		t.Fatal(err)
	}
	plans, err := BuildPlan(database, schema)
	if err != nil {
		t.Fatal(err)
	}

	cases := map[string]string{
		"c_int":     "int",
		"c_real":    "float",
		"c_text":    "sentence",
		"c_blob":    "bytes",
		"c_numeric": "float",
		"c_null":    "sentence",
	}
	for col, wantGen := range cases {
		p := planFor(t, plans, col)
		if p.Generator == nil || *p.Generator != wantGen {
			t.Errorf("column %s: expected generator %s, got %v", col, wantGen, p.Generator)
		}
	}

	// id is a single-column INTEGER PRIMARY KEY -> should be skipped.
	idPlan := planFor(t, plans, "id")
	if !idPlan.Skip {
		t.Errorf("expected affinities.id to be skipped")
	}
}

func TestBuildPlan_CompositePKUniqueGroup(t *testing.T) {
	database := openScratchExample(t, "blog")
	schema, err := db.GetTableSchema(database, "post_tags")
	if err != nil {
		t.Fatal(err)
	}
	plans, err := BuildPlan(database, schema)
	if err != nil {
		t.Fatal(err)
	}

	postID := planFor(t, plans, "post_id")
	tagID := planFor(t, plans, "tag_id")

	if postID.Generator == nil || *postID.Generator != ForeignKeyGeneratorName {
		t.Errorf("expected post_id to use foreignKey generator")
	}
	if tagID.Generator == nil || *tagID.Generator != ForeignKeyGeneratorName {
		t.Errorf("expected tag_id to use foreignKey generator")
	}

	if len(postID.UniqueGroup) != 2 || len(tagID.UniqueGroup) != 2 {
		t.Errorf("expected both composite-PK columns to share a 2-column uniqueGroup, got post_id=%v tag_id=%v",
			postID.UniqueGroup, tagID.UniqueGroup)
	}
}

func TestBuildPlan_ExcludesGeneratedColumns(t *testing.T) {
	database := openScratchExample(t, "types_zoo")
	schema, err := db.GetTableSchema(database, "measurements")
	if err != nil {
		t.Fatal(err)
	}
	plans, err := BuildPlan(database, schema)
	if err != nil {
		t.Fatal(err)
	}

	for _, p := range plans {
		if p.Name == "area_cm2" || p.Name == "ratio" {
			t.Errorf("generated column %s should not appear in the plan", p.Name)
		}
	}
	if len(plans) != 4 { // id, label, width_cm, height_cm
		t.Errorf("expected 4 insertable columns, got %d: %+v", len(plans), plans)
	}
}

func TestBuildPlan_SoloUniqueColumn(t *testing.T) {
	database := openScratchExample(t, "library")
	schema, err := db.GetTableSchema(database, "books")
	if err != nil {
		t.Fatal(err)
	}
	plans, err := BuildPlan(database, schema)
	if err != nil {
		t.Fatal(err)
	}

	isbn := planFor(t, plans, "isbn")
	if len(isbn.UniqueGroup) != 1 || isbn.UniqueGroup[0] != "isbn" {
		t.Errorf("expected isbn to have a solo uniqueGroup, got %v", isbn.UniqueGroup)
	}
}

func TestNameHeuristic_M6aNewRules(t *testing.T) {
	cases := []struct {
		colName string
		colType string
		wantGen string
	}{
		{"ssn", "TEXT", "ssn"},
		{"employee_ssn", "TEXT", "ssn"},
		{"ethnicity", "TEXT", "ethnicity"},
		{"gender", "TEXT", "gender"},
		{"latitude", "REAL", "latitude"},
		{"lat", "REAL", "latitude"},
		{"longitude", "REAL", "longitude"},
		{"lng", "REAL", "longitude"},
		{"lon", "REAL", "longitude"},
		{"ip6_address", "TEXT", "ipv6"},
		{"ipv6_addr", "TEXT", "ipv6"},
		{"credit_card_number", "TEXT", "creditCardNumber"},
		{"creditcard", "TEXT", "creditCardNumber"},
		{"card_number", "TEXT", "creditCardNumber"},
		{"age", "INTEGER", "age"},
		{"state", "TEXT", "state"},
		{"state_abr", "TEXT", "stateAbr"},
	}
	for _, tc := range cases {
		gen, _ := nameHeuristic(db.ColumnInfo{Name: tc.colName, Type: tc.colType})
		if gen != tc.wantGen {
			t.Errorf("nameHeuristic(%q): expected %q, got %q", tc.colName, tc.wantGen, gen)
		}
	}
}
