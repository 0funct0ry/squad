package seed

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestDependentOneOf_BranchesOnDependencyValue(t *testing.T) {
	schema := simpleSchema("status", "tracking_number")
	specs := map[string]ColumnSpec{
		"status": {Generator: "oneOf", Options: map[string]any{"values": "SHIPPED,PENDING"}},
		"tracking_number": {Generator: "dependentOneOf", Options: map[string]any{
			"columns": []string{"status"},
			"cases":   "SHIPPED => TRACK-1|TRACK-2\ndefault => NONE",
		}},
	}
	gen, err := NewRowGenerator(nil, schema, specs)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 50; i++ {
		row, err := gen.GenerateRow()
		if err != nil {
			t.Fatal(err)
		}
		status := row["status"]
		tn := row["tracking_number"]
		if status == "SHIPPED" {
			if tn != "TRACK-1" && tn != "TRACK-2" {
				t.Errorf("expected TRACK-1/2 for SHIPPED, got %v", tn)
			}
		} else if tn != "NONE" {
			t.Errorf("expected default tracking number NONE, got %v", tn)
		}
	}
}

func TestDependentOneOf_NoMatchNoDefaultErrors(t *testing.T) {
	schema := simpleSchema("status", "tracking_number")
	specs := map[string]ColumnSpec{
		"status":          {Generator: "oneOf", Options: map[string]any{"values": "A,B"}},
		"tracking_number": {Generator: "dependentOneOf", Options: map[string]any{"columns": []string{"status"}, "cases": "C => x"}},
	}
	gen, err := NewRowGenerator(nil, schema, specs)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := gen.GenerateRow(); err == nil {
		t.Error("expected error when no case matches and no default is defined")
	}
}

func TestCustomDateSequence_MonotonicOrderingAndSkip(t *testing.T) {
	schema := simpleSchema("created_at", "paid_at", "shipped_at")
	milestones := []string{"created_at", "paid_at", "shipped_at"}
	specs := map[string]ColumnSpec{
		"created_at": {Generator: "customDateSequence", Options: map[string]any{"columns": milestones, "gaps": "10-20,10-20"}},
		"paid_at":    {Generator: "customDateSequence", Options: map[string]any{"columns": milestones, "gaps": "10-20,10-20"}},
		"shipped_at": {Generator: "customDateSequence", Options: map[string]any{"columns": milestones, "gaps": "10-20,10-20"}},
	}
	gen, err := NewRowGenerator(nil, schema, specs)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 20; i++ {
		row, err := gen.GenerateRow()
		if err != nil {
			t.Fatal(err)
		}
		created, err := time.Parse(time.RFC3339, row["created_at"].(string))
		if err != nil {
			t.Fatal(err)
		}
		paid, err := time.Parse(time.RFC3339, row["paid_at"].(string))
		if err != nil {
			t.Fatal(err)
		}
		shipped, err := time.Parse(time.RFC3339, row["shipped_at"].(string))
		if err != nil {
			t.Fatal(err)
		}
		if !paid.After(created) {
			t.Errorf("expected paid_at after created_at, got %v vs %v", paid, created)
		}
		if !shipped.After(paid) {
			t.Errorf("expected shipped_at after paid_at, got %v vs %v", shipped, paid)
		}
	}
}

func TestCustomDateSequence_SkipProbabilityLeavesLaterMilestoneNull(t *testing.T) {
	schema := simpleSchema("created_at", "delivered_at")
	milestones := []string{"created_at", "delivered_at"}
	specs := map[string]ColumnSpec{
		"created_at":   {Generator: "customDateSequence", Options: map[string]any{"columns": milestones, "gaps": "10-20"}},
		"delivered_at": {Generator: "customDateSequence", Options: map[string]any{"columns": milestones, "gaps": "10-20", "skipProbability": 1.0}},
	}
	gen, err := NewRowGenerator(nil, schema, specs)
	if err != nil {
		t.Fatal(err)
	}
	row, err := gen.GenerateRow()
	if err != nil {
		t.Fatal(err)
	}
	if row["delivered_at"] != nil {
		t.Errorf("expected delivered_at to be nil with skipProbability=1.0, got %v", row["delivered_at"])
	}
}

func TestStatusTransitionLog_ValidWalkEndingAtRealStatus(t *testing.T) {
	schema := simpleSchema("status", "history")
	transitions := "CREATED => PENDING\nPENDING => CHARGED\nCHARGED => CAPTURED"
	specs := map[string]ColumnSpec{
		"status":  {Generator: "oneOf", Options: map[string]any{"values": "CAPTURED,PENDING"}},
		"history": {Generator: "statusTransitionLog", Options: map[string]any{"columns": []string{"status"}, "transitions": transitions}},
	}
	gen, err := NewRowGenerator(nil, schema, specs)
	if err != nil {
		t.Fatal(err)
	}
	validNext := map[string]string{"CREATED": "PENDING", "PENDING": "CHARGED", "CHARGED": "CAPTURED"}
	for i := 0; i < 30; i++ {
		row, err := gen.GenerateRow()
		if err != nil {
			t.Fatal(err)
		}
		path := strings.Split(row["history"].(string), "→")
		status := row["status"].(string)
		if path[len(path)-1] != status {
			t.Errorf("expected path to end at status %q, got path %v", status, path)
		}
		for j := 0; j < len(path)-1; j++ {
			if validNext[path[j]] != path[j+1] {
				t.Errorf("invalid transition %q -> %q in path %v", path[j], path[j+1], path)
			}
		}
	}
}

func TestChecksumOfColumns_MatchesManualRecompute(t *testing.T) {
	schema := simpleSchema("a", "b", "hash")
	specs := map[string]ColumnSpec{
		"a":    {Generator: "oneOf", Options: map[string]any{"values": "x,y"}},
		"b":    {Generator: "oneOf", Options: map[string]any{"values": "1,2"}},
		"hash": {Generator: "checksumOfColumns", Options: map[string]any{"columns": []string{"a", "b"}, "algorithm": "sha256", "separator": "|"}},
	}
	gen, err := NewRowGenerator(nil, schema, specs)
	if err != nil {
		t.Fatal(err)
	}
	row, err := gen.GenerateRow()
	if err != nil {
		t.Fatal(err)
	}
	expected := sha256.Sum256([]byte(row["a"].(string) + "|" + row["b"].(string)))
	if row["hash"] != hex.EncodeToString(expected[:]) {
		t.Errorf("checksum mismatch: got %v", row["hash"])
	}
}

func TestSlugFromColumn_ProducesValidSlug(t *testing.T) {
	schema := simpleSchema("title", "slug")
	specs := map[string]ColumnSpec{
		"title": {Generator: "oneOf", Options: map[string]any{"values": "My Blog Post Title!,Another One??"}},
		"slug":  {Generator: "slugFromColumn", Options: map[string]any{"columns": []string{"title"}}},
	}
	gen, err := NewRowGenerator(nil, schema, specs)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 20; i++ {
		row, err := gen.GenerateRow()
		if err != nil {
			t.Fatal(err)
		}
		slug := row["slug"].(string)
		if slug == "" {
			t.Fatal("expected non-empty slug")
		}
		for _, r := range slug {
			if !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-') {
				t.Errorf("slug %q contains invalid character %q", slug, r)
			}
		}
		if strings.HasPrefix(slug, "-") || strings.HasSuffix(slug, "-") {
			t.Errorf("slug %q should not start/end with a hyphen", slug)
		}
	}
}

func TestSlugFromColumn_AppendsSuffix(t *testing.T) {
	schema := simpleSchema("title", "slug")
	specs := map[string]ColumnSpec{
		"title": {Generator: "oneOf", Options: map[string]any{"values": "same,same2"}},
		"slug":  {Generator: "slugFromColumn", Options: map[string]any{"columns": []string{"title"}, "suffixLength": 4}},
	}
	gen, err := NewRowGenerator(nil, schema, specs)
	if err != nil {
		t.Fatal(err)
	}
	row, err := gen.GenerateRow()
	if err != nil {
		t.Fatal(err)
	}
	slug := row["slug"].(string)
	parts := strings.Split(slug, "-")
	if len(parts[len(parts)-1]) != 4 {
		t.Errorf("expected a 4-char suffix, got slug %q", slug)
	}
}

func TestJSONTemplate_SubstitutesColumnsAndProducesValidJSON(t *testing.T) {
	schema := simpleSchema("username", "payload")
	specs := map[string]ColumnSpec{
		"username": {Generator: "oneOf", Options: map[string]any{"values": "alice,bob"}},
		"payload":  {Generator: "jsonTemplate", Options: map[string]any{"columns": []string{"username"}, "template": `{"user": "{{column:username}}", "active": true}`}},
	}
	gen, err := NewRowGenerator(nil, schema, specs)
	if err != nil {
		t.Fatal(err)
	}
	row, err := gen.GenerateRow()
	if err != nil {
		t.Fatal(err)
	}
	raw := row["payload"].(string)
	if !json.Valid([]byte(raw)) {
		t.Fatalf("expected valid JSON, got %q", raw)
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed["user"] != row["username"] {
		t.Errorf("expected user field to equal generated username %v, got %v", row["username"], parsed["user"])
	}
}

func TestJSONTemplate_MalformedAfterSubstitutionErrors(t *testing.T) {
	schema := simpleSchema("name", "payload")
	specs := map[string]ColumnSpec{
		"name":    {Generator: "oneOf", Options: map[string]any{"values": "he said \"hi\"\nshe said \"bye\""}},
		"payload": {Generator: "jsonTemplate", Options: map[string]any{"columns": []string{"name"}, "template": `{"name": "{{column:name}}"}`}},
	}
	gen, err := NewRowGenerator(nil, schema, specs)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := gen.GenerateRow(); err == nil {
		t.Error("expected an error for malformed JSON after substitution (unescaped quote)")
	}
}

func TestJSONTemplate_RejectsContextDependentNestedGenerator(t *testing.T) {
	schema := simpleSchema("payload")
	specs := map[string]ColumnSpec{
		"payload": {Generator: "jsonTemplate", Options: map[string]any{"template": `{"id": "{{generator:formula}}"}`}},
	}
	gen, err := NewRowGenerator(nil, schema, specs)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := gen.GenerateRow(); err == nil {
		t.Error("expected an error for a nested generator that requires row/table context")
	}
}

func TestValidateFormulaDependencies_MixedFormulaAndCrossColumn(t *testing.T) {
	columns := map[string]ColumnSpec{
		"a": {Generator: "formula", Options: map[string]any{"columns": []string{"b"}, "expression": "b"}},
		"b": {Generator: "slugFromColumn", Options: map[string]any{"columns": []string{"a"}}},
	}
	if err := ValidateFormulaDependencies(columns); err == nil {
		t.Error("expected a cycle error for a formula/cross-column mutual dependency")
	}

	valid := map[string]ColumnSpec{
		"title": {Generator: "oneOf", Options: map[string]any{"values": "a,b"}},
		"slug":  {Generator: "slugFromColumn", Options: map[string]any{"columns": []string{"title"}}},
		"price": {Generator: "int"},
		"total": {Generator: "formula", Options: map[string]any{"columns": []string{"price"}, "expression": "price"}},
	}
	if err := ValidateFormulaDependencies(valid); err != nil {
		t.Errorf("expected no error for a valid mixed dependency graph, got %v", err)
	}
}

func TestValidateFormulaDependencies_SelfReferenceAcrossCrossColumn(t *testing.T) {
	columns := map[string]ColumnSpec{
		"slug": {Generator: "slugFromColumn", Options: map[string]any{"columns": []string{"slug"}}},
	}
	if err := ValidateFormulaDependencies(columns); err == nil {
		t.Error("expected a self-reference error")
	}
}
