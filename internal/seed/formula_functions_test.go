package seed

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

// evalFormulaExpr evaluates expression directly via evalFormula/evalNode
// against row -- no RowGenerator construction or column generation needed,
// since formula's dependency columns are supplied as already-generated
// values in rowSoFar.
func evalFormulaExpr(t *testing.T, expression string, row map[string]any) any {
	t.Helper()
	gen := &RowGenerator{}
	v, err := gen.evalFormula("result", ColumnSpec{Generator: "formula", Options: map[string]any{
		"columns":    []any{},
		"expression": expression,
	}}, row)
	if err != nil {
		t.Fatalf("expression %q: unexpected error: %v", expression, err)
	}
	return v
}

func evalFormulaExprError(t *testing.T, expression string, row map[string]any) error {
	t.Helper()
	gen := &RowGenerator{}
	_, err := gen.evalFormula("result", ColumnSpec{Generator: "formula", Options: map[string]any{
		"columns":    []any{},
		"expression": expression,
	}}, row)
	return err
}

func TestFormulaFunctions_String(t *testing.T) {
	row := map[string]any{"name": "Ada Lovelace", "first": "ada", "last": "lovelace"}

	if v := evalFormulaExpr(t, `upper(name)`, row); v != "ADA LOVELACE" {
		t.Errorf("upper: got %v", v)
	}
	if v := evalFormulaExpr(t, `lower(name)`, row); v != "ada lovelace" {
		t.Errorf("lower: got %v", v)
	}
	if v := evalFormulaExpr(t, `concat(first, last)`, row); v != "adalovelace" {
		t.Errorf("concat: got %v", v)
	}
	if v := evalFormulaExpr(t, `trim(padded)`, map[string]any{"padded": "  hi  "}); v != "hi" {
		t.Errorf("trim: got %v", v)
	}
	if v := evalFormulaExpr(t, `len(name)`, row); v != int64(12) {
		t.Errorf("len: got %v (%T)", v, v)
	}
	if v := evalFormulaExpr(t, `capitalize(first)`, row); v != "Ada" {
		t.Errorf("capitalize: got %v", v)
	}
	if v := evalFormulaExpr(t, `capitalize(empty)`, map[string]any{"empty": ""}); v != "" {
		t.Errorf("capitalize empty: got %v", v)
	}
}

func TestFormulaFunctions_KebabAndCamelCase(t *testing.T) {
	cases := map[string]string{
		"myVariableName":  "my-variable-name",
		"Hello World Foo": "hello-world-foo",
		"snake_case_val":  "snake-case-val",
		"already-kebab":   "already-kebab",
	}
	for in, want := range cases {
		got := evalFormulaExpr(t, `kebabCase(s)`, map[string]any{"s": in})
		if got != want {
			t.Errorf("kebabCase(%q): got %q, want %q", in, got, want)
		}
	}

	camelCases := map[string]string{
		"my-variable-name": "myVariableName",
		"snake_case_val":   "snakeCaseVal",
		"Hello World":      "helloWorld",
	}
	for in, want := range camelCases {
		got := evalFormulaExpr(t, `camelCase(s)`, map[string]any{"s": in})
		if got != want {
			t.Errorf("camelCase(%q): got %q, want %q", in, got, want)
		}
	}
}

func TestFormulaFunctions_Encoding(t *testing.T) {
	row := map[string]any{"s": "hi"}
	if v := evalFormulaExpr(t, `hex(s)`, row); v != "6869" {
		t.Errorf("hex: got %v", v)
	}
	if v := evalFormulaExpr(t, `base64(s)`, row); v != "aGk=" {
		t.Errorf("base64: got %v", v)
	}
	if v := evalFormulaExpr(t, `base32(s)`, row); v != "NBUQ====" {
		t.Errorf("base32: got %v", v)
	}
}

func TestFormulaFunctions_Crypto(t *testing.T) {
	row := map[string]any{"s": "hello"}
	want := sha256.Sum256([]byte("hello"))
	if v := evalFormulaExpr(t, `sha256(s)`, row); v != hex.EncodeToString(want[:]) {
		t.Errorf("sha256: got %v", v)
	}
	if v := evalFormulaExpr(t, `md5(s)`, row); v != "5d41402abc4b2a76b9719d911017c592" {
		t.Errorf("md5: got %v", v)
	}
	if v := evalFormulaExpr(t, `sha1(s)`, row); v != "aaf4c61ddcc5e8a2dabede0f3b482cd9aea9434d" {
		t.Errorf("sha1: got %v", v)
	}
	sha512Want := "9b71d224bd62f3785d96d46ad3ea3d73319bfbc2890caadae2dff72519673ca72323c3d99ba5c11d7c7acc6e14b8c5da0c4663475c2e5c3adef46f73bcdec043"
	if v := evalFormulaExpr(t, `sha512(s)`, row); v != sha512Want {
		t.Errorf("sha512: got %v", v)
	}
}

func TestFormulaFunctions_Math(t *testing.T) {
	row := map[string]any{"a": -5.5, "b": 2.0, "c": 3.0}
	if v := evalFormulaExpr(t, `abs(a)`, row); v != 5.5 {
		t.Errorf("abs: got %v", v)
	}
	if v := evalFormulaExpr(t, `round(a)`, row); v != -6.0 {
		t.Errorf("round: got %v", v)
	}
	if v := evalFormulaExpr(t, `floor(a)`, row); v != -6.0 {
		t.Errorf("floor: got %v", v)
	}
	if v := evalFormulaExpr(t, `ceil(a)`, row); v != -5.0 {
		t.Errorf("ceil: got %v", v)
	}
	if v := evalFormulaExpr(t, `min(b, c)`, row); v != 2.0 {
		t.Errorf("min: got %v", v)
	}
	if v := evalFormulaExpr(t, `max(b, c)`, row); v != 3.0 {
		t.Errorf("max: got %v", v)
	}
	if v := evalFormulaExpr(t, `pow(b, c)`, row); v != 8.0 {
		t.Errorf("pow: got %v", v)
	}
	if v := evalFormulaExpr(t, `mod(c, b)`, row); v != 1.0 {
		t.Errorf("mod: got %v", v)
	}
	if err := evalFormulaExprError(t, `mod(c, zero)`, map[string]any{"c": 3.0, "zero": 0.0}); err == nil {
		t.Error("expected an error for mod by zero")
	}
}

func TestFormulaFunctions_NestedAndCombinedWithArithmetic(t *testing.T) {
	row := map[string]any{"price": 19.999, "qty": 3.0, "first": "ada", "last": "lovelace"}
	if v := evalFormulaExpr(t, `round(price * qty)`, row); v != 60.0 {
		t.Errorf("round(price*qty): got %v", v)
	}
	if v := evalFormulaExpr(t, `upper(concat(first, last))`, row); v != "ADALOVELACE" {
		t.Errorf("upper(concat(...)): got %v", v)
	}
}

func TestFormulaFunctions_UnknownFunctionRejected(t *testing.T) {
	if err := evalFormulaExprError(t, `bogus(price)`, map[string]any{"price": 1.0}); err == nil {
		t.Error("expected an error for an unknown function name")
	}
}

func TestFormulaFunctions_WrongArgCountRejected(t *testing.T) {
	if err := evalFormulaExprError(t, `upper(a, b)`, map[string]any{"a": "x", "b": "y"}); err == nil {
		t.Error("expected an error for upper() called with 2 arguments")
	}
	if err := evalFormulaExprError(t, `pow(a)`, map[string]any{"a": 1.0}); err == nil {
		t.Error("expected an error for pow() called with 1 argument")
	}
}

func TestFormulaFunctions_NonIdentCallTargetRejected(t *testing.T) {
	// A call through a selector expression (e.g. a package-qualified or
	// method-style call) must be rejected, not just unregistered names.
	if err := evalFormulaExprError(t, `strings.ToUpper(a)`, map[string]any{"a": "x"}); err == nil {
		t.Error("expected an error for a non-identifier call target")
	}
}

func TestFormulaFunctions_WrongArgTypeRejected(t *testing.T) {
	if err := evalFormulaExprError(t, `abs(name)`, map[string]any{"name": "not-a-number"}); err == nil {
		t.Error("expected an error for abs() called with a non-numeric argument")
	}
}
