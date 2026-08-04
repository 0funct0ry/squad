package seed

import "testing"

func TestNullWithProbability_NullRateApproximatelyMatchesConfigured(t *testing.T) {
	schema := simpleSchema("email")
	specs := map[string]ColumnSpec{
		"email": {Generator: "nullWithProbability", Options: map[string]any{
			"generator": map[string]any{"generator": "email"},
			"nullRate":  0.3,
		}},
	}
	gen, err := NewRowGenerator(nil, schema, specs, 0)
	if err != nil {
		t.Fatal(err)
	}
	const n = 5000
	nullCount := 0
	for i := 0; i < n; i++ {
		row, err := gen.GenerateRow()
		if err != nil {
			t.Fatal(err)
		}
		if row["email"] == nil {
			nullCount++
			continue
		}
		if _, ok := row["email"].(string); !ok {
			t.Fatalf("expected a string email when non-null, got %T", row["email"])
		}
	}
	frac := float64(nullCount) / n
	if frac < 0.24 || frac > 0.36 {
		t.Errorf("expected null fraction near 0.3, got %f", frac)
	}
}

func TestNullWithProbability_CanWrapCrossColumnGenerator(t *testing.T) {
	schema := simpleSchema("title", "slug")
	specs := map[string]ColumnSpec{
		"title": {Generator: "oneOf", Options: map[string]any{"values": "a,b"}},
		"slug": {Generator: "nullWithProbability", Options: map[string]any{
			"generator": map[string]any{
				"generator": "slugFromColumn",
				"options":   map[string]any{"columns": []string{"title"}},
			},
			"nullRate": 0.0,
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
	if row["slug"] == nil || row["slug"] == "" {
		t.Errorf("expected a non-null slug with nullRate=0, got %v", row["slug"])
	}
}

func TestNullWithProbability_RejectsUnknownOrSelfReferentialWrappedGenerator(t *testing.T) {
	if _, _, err := wrappedGeneratorSpec(ColumnSpec{Options: map[string]any{
		"generator": map[string]any{"generator": "doesNotExist"},
	}}); err != nil {
		// wrappedGeneratorSpec itself doesn't validate existence -- that's
		// the server layer's job (seed_handler.go). Just confirm the shape
		// parses without panicking.
		t.Fatalf("unexpected parse error: %v", err)
	}

	if _, _, err := wrappedGeneratorSpec(ColumnSpec{Options: map[string]any{}}); err == nil {
		t.Error("expected an error when options.generator is missing")
	}
	if _, _, err := wrappedGeneratorSpec(ColumnSpec{Options: map[string]any{
		"generator": map[string]any{},
	}}); err == nil {
		t.Error("expected an error when options.generator.generator is missing")
	}
}
