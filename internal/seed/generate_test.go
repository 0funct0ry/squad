package seed

import (
	"testing"

	"github.com/0funct0ry/squad/internal/db"
)

func simpleSchema(cols ...string) *db.TableSchema {
	colInfos := make([]db.ColumnInfo, len(cols))
	for i, name := range cols {
		colInfos[i] = db.ColumnInfo{Name: name, Type: "TEXT"}
	}
	return &db.TableSchema{Name: "t", Type: "table", Columns: colInfos}
}

func TestStatefulSequence_IncrementsAndResetsPerRequest(t *testing.T) {
	schema := simpleSchema("seq")
	specs := map[string]ColumnSpec{
		"seq": {Generator: "sequence", Options: map[string]any{"start": 10, "step": 2}},
	}

	gen, err := NewRowGenerator(nil, schema, specs, 0)
	if err != nil {
		t.Fatal(err)
	}
	var got []int64
	for i := 0; i < 3; i++ {
		row, err := gen.GenerateRow()
		if err != nil {
			t.Fatal(err)
		}
		v, ok := row["seq"].(int64)
		if !ok {
			t.Fatalf("expected int64, got %T (%v)", row["seq"], row["seq"])
		}
		got = append(got, v)
	}
	want := []int64{10, 12, 14}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("row %d: expected %d, got %d", i, want[i], got[i])
		}
	}

	// A fresh RowGenerator must reset the counter (verifies reset-per-request
	// semantics: dry-run and insert both construct their own generator, or in
	// the case of seed_handler.go, the same generator is reused only within
	// a single request/response cycle).
	gen2, err := NewRowGenerator(nil, schema, specs, 0)
	if err != nil {
		t.Fatal(err)
	}
	row, err := gen2.GenerateRow()
	if err != nil {
		t.Fatal(err)
	}
	if row["seq"].(int64) != 10 {
		t.Errorf("expected fresh generator to restart at 10, got %v", row["seq"])
	}
}

func TestStatefulRowNumber_DefaultsToOneAndFormats(t *testing.T) {
	schema := simpleSchema("rn")
	specs := map[string]ColumnSpec{
		"rn": {Generator: "rowNumber", Options: map[string]any{"format": "ROW-%04d"}},
	}
	gen, err := NewRowGenerator(nil, schema, specs, 0)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"ROW-0001", "ROW-0002", "ROW-0003"}
	for i, w := range want {
		row, err := gen.GenerateRow()
		if err != nil {
			t.Fatal(err)
		}
		if row["rn"] != w {
			t.Errorf("row %d: expected %q, got %v", i, w, row["rn"])
		}
	}
}

func TestStatefulCharacterSequence_Base26Labeling(t *testing.T) {
	schema := simpleSchema("cs")
	specs := map[string]ColumnSpec{
		"cs": {Generator: "characterSequence", Options: map[string]any{}},
	}
	gen, err := NewRowGenerator(nil, schema, specs, 0)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"A", "B", "C"}
	for i, w := range want {
		row, err := gen.GenerateRow()
		if err != nil {
			t.Fatal(err)
		}
		if row["cs"] != w {
			t.Errorf("row %d: expected %q, got %v", i, w, row["cs"])
		}
	}

	// Verify wraparound past Z into AA.
	schema2 := simpleSchema("cs")
	specs2 := map[string]ColumnSpec{
		"cs": {Generator: "characterSequence", Options: map[string]any{"start": 25}},
	}
	gen2, err := NewRowGenerator(nil, schema2, specs2, 0)
	if err != nil {
		t.Fatal(err)
	}
	wantWrap := []string{"Z", "AA", "AB"}
	for i, w := range wantWrap {
		row, err := gen2.GenerateRow()
		if err != nil {
			t.Fatal(err)
		}
		if row["cs"] != w {
			t.Errorf("wrap row %d: expected %q, got %v", i, w, row["cs"])
		}
	}
}

func TestStatefulDigitSequence_ZeroPadded(t *testing.T) {
	schema := simpleSchema("ds")
	specs := map[string]ColumnSpec{
		"ds": {Generator: "digitSequence", Options: map[string]any{"width": 4}},
	}
	gen, err := NewRowGenerator(nil, schema, specs, 0)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"0000", "0001", "0002"}
	for i, w := range want {
		row, err := gen.GenerateRow()
		if err != nil {
			t.Fatal(err)
		}
		if row["ds"] != w {
			t.Errorf("row %d: expected %q, got %v", i, w, row["ds"])
		}
	}
}

func TestFormula_HappyPath(t *testing.T) {
	schema := simpleSchema("price", "qty", "total")
	specs := map[string]ColumnSpec{
		"price": {Generator: "float", Options: map[string]any{"min": 5, "max": 5}},
		"qty":   {Generator: "int", Options: map[string]any{"min": 3, "max": 3}},
		"total": {Generator: "formula", Options: map[string]any{
			"columns":    []any{"price", "qty"},
			"expression": "price * qty",
		}},
	}
	gen, err := NewRowGenerator(nil, schema, specs, 0)
	if err != nil {
		t.Fatal(err)
	}
	row, err := gen.GenerateRow()
	if err != nil {
		t.Fatal(err)
	}
	total, ok := row["total"].(float64)
	if !ok {
		t.Fatalf("expected float64 total, got %T", row["total"])
	}
	if total != 15 {
		t.Errorf("expected total=15, got %v", total)
	}
}

func TestFormula_SelfReferenceRejected(t *testing.T) {
	columns := map[string]ColumnSpec{
		"a": {Generator: "formula", Options: map[string]any{
			"columns":    []any{"a"},
			"expression": "a",
		}},
	}
	if err := ValidateFormulaDependencies(columns); err == nil {
		t.Fatal("expected error for self-referencing formula column")
	}
}

func TestFormula_CycleRejected(t *testing.T) {
	columns := map[string]ColumnSpec{
		"a": {Generator: "formula", Options: map[string]any{
			"columns":    []any{"b"},
			"expression": "b",
		}},
		"b": {Generator: "formula", Options: map[string]any{
			"columns":    []any{"a"},
			"expression": "a",
		}},
	}
	if err := ValidateFormulaDependencies(columns); err == nil {
		t.Fatal("expected error for 2-node formula cycle")
	}
}

func TestFormula_UnknownColumnRejected(t *testing.T) {
	columns := map[string]ColumnSpec{
		"a": {Generator: "formula", Options: map[string]any{
			"columns":    []any{"nonexistent"},
			"expression": "nonexistent",
		}},
	}
	if err := ValidateFormulaDependencies(columns); err == nil {
		t.Fatal("expected error for unknown column reference")
	}
}
