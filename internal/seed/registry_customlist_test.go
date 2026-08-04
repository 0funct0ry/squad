package seed

import "testing"

func TestOneOf_ParsesAndPicksFromList(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 200; i++ {
		v, err := Generate("oneOf", "TEXT", map[string]any{"values": "CREATED,PENDING,CHARGED,CAPTURED,PAID"})
		if err != nil {
			t.Fatal(err)
		}
		s, ok := v.(string)
		if !ok {
			t.Fatalf("expected string, got %T", v)
		}
		seen[s] = true
	}
	for _, want := range []string{"CREATED", "PENDING", "CHARGED", "CAPTURED", "PAID"} {
		if !seen[want] {
			t.Errorf("expected %q to appear among samples", want)
		}
	}
	for s := range seen {
		switch s {
		case "CREATED", "PENDING", "CHARGED", "CAPTURED", "PAID":
		default:
			t.Errorf("unexpected value %q", s)
		}
	}
}

func TestOneOf_NewlineSeparatedAndTrimmed(t *testing.T) {
	v, err := Generate("oneOf", "TEXT", map[string]any{"values": " a \n b \n"})
	if err != nil {
		t.Fatal(err)
	}
	s := v.(string)
	if s != "a" && s != "b" {
		t.Errorf("expected trimmed 'a' or 'b', got %q", s)
	}
}

func TestOneOf_RejectsEmptyOrSingleValue(t *testing.T) {
	if _, err := Generate("oneOf", "TEXT", map[string]any{"values": ""}); err == nil {
		t.Error("expected error for empty values")
	}
	if _, err := Generate("oneOf", "TEXT", map[string]any{"values": "only-one"}); err == nil {
		t.Error("expected error for a single value")
	}
}

func TestWeightedOneOf_MatchesConfiguredRatiosApproximately(t *testing.T) {
	counts := map[string]int{}
	const n = 20000
	for i := 0; i < n; i++ {
		v, err := Generate("weightedOneOf", "TEXT", map[string]any{"values": "PAID:70, PENDING:20, REFUNDED:10"})
		if err != nil {
			t.Fatal(err)
		}
		counts[v.(string)]++
	}
	paidFrac := float64(counts["PAID"]) / n
	if paidFrac < 0.6 || paidFrac > 0.8 {
		t.Errorf("expected PAID fraction near 0.7, got %f (counts=%v)", paidFrac, counts)
	}
	refundedFrac := float64(counts["REFUNDED"]) / n
	if refundedFrac < 0.05 || refundedFrac > 0.15 {
		t.Errorf("expected REFUNDED fraction near 0.1, got %f (counts=%v)", refundedFrac, counts)
	}
}

func TestWeightedOneOf_BareValueDefaultsToWeightOne(t *testing.T) {
	v, err := Generate("weightedOneOf", "TEXT", map[string]any{"values": "a:1,b"})
	if err != nil {
		t.Fatal(err)
	}
	s := v.(string)
	if s != "a" && s != "b" {
		t.Errorf("unexpected value %q", s)
	}
}

func TestWeightedOneOf_RejectsUnparseableOrAllZeroWeights(t *testing.T) {
	if _, err := Generate("weightedOneOf", "TEXT", map[string]any{"values": "a:notanumber"}); err == nil {
		t.Error("expected error for unparseable weight")
	}
	if _, err := Generate("weightedOneOf", "TEXT", map[string]any{"values": "a:0,b:0"}); err == nil {
		t.Error("expected error for all-zero weights")
	}
}

func TestRegexEnum_PicksAmongPatternsAndExpands(t *testing.T) {
	v, err := Generate("regexEnum", "TEXT", map[string]any{"patterns": "ORD-[0-9]{4}\nLEGACY-[A-Z]{3}"})
	if err != nil {
		t.Fatal(err)
	}
	s := v.(string)
	if len(s) == 0 {
		t.Error("expected non-empty expansion")
	}
}

func TestRegexEnum_RejectsInvalidPattern(t *testing.T) {
	if _, err := Generate("regexEnum", "TEXT", map[string]any{"patterns": "["}); err == nil {
		t.Error("expected error for invalid regex")
	}
}

func TestIncrementalEnum_CyclesDeterministicallyAndWraps(t *testing.T) {
	schema := simpleSchema("day")
	specs := map[string]ColumnSpec{
		"day": {Generator: "incrementalEnum", Options: map[string]any{"values": "Mon,Tue,Wed"}},
	}
	gen, err := NewRowGenerator(nil, schema, specs, 0)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"Mon", "Tue", "Wed", "Mon", "Tue", "Wed", "Mon"}
	for i, w := range want {
		row, err := gen.GenerateRow()
		if err != nil {
			t.Fatal(err)
		}
		if row["day"] != w {
			t.Errorf("row %d: expected %q, got %v", i, w, row["day"])
		}
	}
}

func TestIncrementalEnum_RespectsStartAndStep(t *testing.T) {
	schema := simpleSchema("day")
	specs := map[string]ColumnSpec{
		"day": {Generator: "incrementalEnum", Options: map[string]any{"values": "A,B,C,D", "start": 1, "step": 2}},
	}
	gen, err := NewRowGenerator(nil, schema, specs, 0)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"B", "D", "B", "D"}
	for i, w := range want {
		row, err := gen.GenerateRow()
		if err != nil {
			t.Fatal(err)
		}
		if row["day"] != w {
			t.Errorf("row %d: expected %q, got %v", i, w, row["day"])
		}
	}
}
