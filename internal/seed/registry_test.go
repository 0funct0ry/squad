package seed

import (
	"testing"
)

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

func TestGenerateBasicGenerators(t *testing.T) {
	cases := []struct {
		name     string
		affinity string
	}{
		{"email", "TEXT"},
		{"firstName", "TEXT"},
		{"lastName", "TEXT"},
		{"name", "TEXT"},
		{"username", "TEXT"},
		{"uuid", "TEXT"},
		{"url", "TEXT"},
		{"phone", "TEXT"},
		{"sentence", "TEXT"},
		{"word", "TEXT"},
		{"paragraph", "TEXT"},
		{"company", "TEXT"},
		{"address", "TEXT"},
		{"city", "TEXT"},
		{"country", "TEXT"},
		{"zipCode", "TEXT"},
		{"ipv4", "TEXT"},
		{"int", "INTEGER"},
		{"float", "REAL"},
		{"price", "REAL"},
		{"bytes", "BLOB"},
	}
	for _, tc := range cases {
		v, err := Generate(tc.name, tc.affinity, map[string]any{})
		if err != nil {
			t.Errorf("%s: unexpected error: %v", tc.name, err)
		}
		if v == nil {
			t.Errorf("%s: expected a non-nil value", tc.name)
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
