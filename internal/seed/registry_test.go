package seed

import (
	"regexp"
	"strings"
	"testing"
)

var uuidRegexp = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

func TestGuidAndUuid_ShapeParity(t *testing.T) {
	u, err := Generate("uuid", "TEXT", map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	g, err := Generate("guid", "TEXT", map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	us, ok := u.(string)
	if !ok || !uuidRegexp.MatchString(us) {
		t.Errorf("uuid did not match UUID shape: %v", u)
	}
	gs, ok := g.(string)
	if !ok || !uuidRegexp.MatchString(gs) {
		t.Errorf("guid did not match UUID shape: %v", g)
	}
}

// expectedGeneratorCount pins the total number of registered generators
// (including foreignKey) after the M6a registry expansion. Update this
// alongside any future registry_*.go additions.
const expectedGeneratorCount = 301

func TestAvailableGeneratorsIncludesForeignKey(t *testing.T) {
	names := AvailableGenerators()
	found := false
	for _, n := range names {
		if n == ForeignKeyGeneratorName {
			found = true
		}
	}
	if !found {
		t.Errorf("expected foreignKey in availableGenerators, got %v", names)
	}
}

func TestExists(t *testing.T) {
	if !Exists("email") {
		t.Errorf("expected email generator to exist")
	}
	if Exists("nonsense") {
		t.Errorf("expected nonsense generator to not exist")
	}
}

// typeMatchesAffinity reports whether v's Go type is a plausible binding for
// the given SQLite affinity, allowing the small set of numeric/string Go
// types our generators actually return (int, int64, float32, float64,
// string, []byte).
func typeMatchesAffinity(v any, affinity string) bool {
	switch affinity {
	case "TEXT":
		_, ok := v.(string)
		return ok
	case "INTEGER":
		switch v.(type) {
		case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
			return true
		}
		return false
	case "REAL", "NUMERIC":
		switch v.(type) {
		case float32, float64:
			return true
		}
		return false
	case "BLOB":
		_, ok := v.([]byte)
		return ok
	default:
		return true
	}
}

// TestGenerateAllRegisteredGenerators exercises every non-special generator
// returned by AvailableGenerators() (i.e. everything except foreignKey,
// which requires live DB access, and any Stateful generator, which requires
// a RowGenerator). It asserts no error, a non-nil result, and that the
// result's Go type is a plausible match for at least one of the generator's
// declared affinities.
func TestGenerateAllRegisteredGenerators(t *testing.T) {
	names := AvailableGenerators()
	if len(names) != expectedGeneratorCount {
		t.Fatalf("expected %d registered generators, got %d: %v", expectedGeneratorCount, len(names), names)
	}

	// Fn: nil generators that need live DB access, row context, or a
	// RowGenerator to produce a value -- exercised separately by their own
	// package tests instead of the generic Generate() smoke test here.
	needsContext := map[string]bool{
		ForeignKeyGeneratorName: true,
		"formula":               true,
		"enumFromColumn":        true,
		"dependentOneOf":        true,
		"customDateSequence":    true,
		"statusTransitionLog":   true,
		"checksumOfColumns":     true,
		"slugFromColumn":        true,
		"jsonTemplate":          true,
		"template":              true,
		"geohash":               true,
		"nullWithProbability":   true,
		// user-authored value-list generators with no sensible generic
		// default -- exercised by registry_customlist_test.go instead.
		"oneOf":         true,
		"weightedOneOf": true,
		"regexEnum":     true,
	}

	for _, name := range names {
		if needsContext[name] {
			continue
		}
		meta, ok := GeneratorMetaByName(name)
		if !ok {
			t.Errorf("%s: GeneratorMetaByName returned not-found for a name from AvailableGenerators", name)
			continue
		}
		if meta.Stateful {
			continue
		}
		if len(meta.Affinities) == 0 {
			t.Errorf("%s: expected at least one declared affinity", name)
			continue
		}
		for _, affinity := range meta.Affinities {
			v, err := Generate(name, affinity, map[string]any{})
			if err != nil {
				t.Errorf("%s/%s: unexpected error: %v", name, affinity, err)
				continue
			}
			if v == nil {
				t.Errorf("%s/%s: expected a non-nil value", name, affinity)
				continue
			}
			if !typeMatchesAffinity(v, affinity) {
				t.Errorf("%s/%s: value %v (%T) does not match declared affinity", name, affinity, v, v)
			}
		}
	}
}

func TestGenerateBoolAffinities(t *testing.T) {
	v, err := Generate("bool", "INTEGER", map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := v.(int64); !ok {
		t.Errorf("expected bool/INTEGER to produce int64, got %T", v)
	}

	v, err = Generate("bool", "TEXT", map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	s, ok := v.(string)
	if !ok || (s != "true" && s != "false") {
		t.Errorf("expected bool/TEXT to produce \"true\"/\"false\", got %v", v)
	}
}

func TestGenerateDatetimeAffinities(t *testing.T) {
	v, err := Generate("datetime", "INTEGER", map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := v.(int64); !ok {
		t.Errorf("expected datetime/INTEGER to produce int64 unix time, got %T", v)
	}

	v, err = Generate("datetime", "TEXT", map[string]any{"onlyDate": true})
	if err != nil {
		t.Fatal(err)
	}
	s, ok := v.(string)
	if !ok || len(s) != len("2006-01-02") {
		t.Errorf("expected onlyDate to produce a YYYY-MM-DD string, got %v", v)
	}
}

func TestGenerateUnknownGenerator(t *testing.T) {
	if _, err := Generate("does-not-exist", "TEXT", nil); err == nil {
		t.Errorf("expected an error for an unknown generator")
	}
}

func TestGenerateRangeOptions(t *testing.T) {
	v, err := Generate("int", "INTEGER", map[string]any{"min": 5, "max": 5})
	if err != nil {
		t.Fatal(err)
	}
	if v.(int) != 5 {
		t.Errorf("expected exact min=max=5, got %v", v)
	}
}

func TestGenerateSentenceRespectsWordCount(t *testing.T) {
	for _, wc := range []int{1, 2, 3, 8, 15} {
		v, err := Generate("sentence", "TEXT", map[string]any{"wordCount": wc})
		if err != nil {
			t.Fatal(err)
		}
		s := v.(string)
		words := strings.Fields(strings.TrimSuffix(s, "."))
		if len(words) != wc {
			t.Errorf("wordCount=%d: expected %d words, got %d in %q", wc, wc, len(words), s)
		}
	}
}
