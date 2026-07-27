package seed

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base32"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"
	"unicode"
)

// formulaFunc implements one whitelisted function callable from a formula
// expression (see evalNode's *ast.CallExpr case). Errors are returned
// without a "formula: " prefix -- evalNode adds that uniformly.
type formulaFunc func(args []any) (any, error)

// formulaFuncs is the complete whitelist of functions callable from a
// formula expression. A call to any name not in this map is rejected by
// evalNode before formulaFunc is ever invoked -- this map IS the surface
// area, so every entry must be deliberately safe (pure, deterministic given
// its arguments, no I/O).
var formulaFuncs = map[string]formulaFunc{
	// string
	"upper":      fnUpper,
	"lower":      fnLower,
	"concat":     fnConcat,
	"trim":       fnTrim,
	"len":        fnLen,
	"capitalize": fnCapitalize,
	"kebabCase":  fnKebabCase,
	"camelCase":  fnCamelCase,

	// encoding
	"hex":    fnHex,
	"base32": fnBase32,
	"base64": fnBase64,

	// crypto (hex-digest of the string form of the argument)
	"sha1":   fnSha1,
	"md5":    fnMd5,
	"sha256": fnSha256,
	"sha512": fnSha512,

	// math
	"abs":   fnAbs,
	"round": fnRound,
	"floor": fnFloor,
	"ceil":  fnCeil,
	"min":   fnMin,
	"max":   fnMax,
	"pow":   fnPow,
	"mod":   fnMod,
}

// FormulaFuncNames returns the sorted names of the whitelisted formula
// functions, for exposing them (e.g. as CLI template functions) without
// duplicating the formulaFuncs whitelist itself.
func FormulaFuncNames() []string {
	names := make([]string, 0, len(formulaFuncs))
	for name := range formulaFuncs {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// CallFormulaFunc invokes the named whitelisted formula function with the
// given arguments. ok is false if name is not in the formulaFuncs whitelist.
func CallFormulaFunc(name string, args []any) (result any, err error, ok bool) {
	fn, ok := formulaFuncs[name]
	if !ok {
		return nil, nil, false
	}
	result, err = fn(args)
	return result, err, true
}

func requireArgCount(name string, args []any, n int) error {
	if len(args) != n {
		return fmt.Errorf("%s expects %d argument(s), got %d", name, n, len(args))
	}
	return nil
}

func requireMinArgCount(name string, args []any, n int) error {
	if len(args) < n {
		return fmt.Errorf("%s expects at least %d argument(s), got %d", name, n, len(args))
	}
	return nil
}

func argAsString(name string, args []any, i int) string {
	return fmt.Sprintf("%v", args[i])
}

func argAsFloat(name string, args []any, i int) (float64, error) {
	f, ok := toFloat(args[i])
	if !ok {
		return 0, fmt.Errorf("%s: argument %d must be numeric, got %T", name, i+1, args[i])
	}
	return f, nil
}

// ---------------------------------------------------------------------
// string
// ---------------------------------------------------------------------

func fnUpper(args []any) (any, error) {
	if err := requireArgCount("upper", args, 1); err != nil {
		return nil, err
	}
	return strings.ToUpper(argAsString("upper", args, 0)), nil
}

func fnLower(args []any) (any, error) {
	if err := requireArgCount("lower", args, 1); err != nil {
		return nil, err
	}
	return strings.ToLower(argAsString("lower", args, 0)), nil
}

func fnConcat(args []any) (any, error) {
	if err := requireMinArgCount("concat", args, 2); err != nil {
		return nil, err
	}
	var b strings.Builder
	for i := range args {
		b.WriteString(argAsString("concat", args, i))
	}
	return b.String(), nil
}

func fnTrim(args []any) (any, error) {
	if err := requireArgCount("trim", args, 1); err != nil {
		return nil, err
	}
	return strings.TrimSpace(argAsString("trim", args, 0)), nil
}

func fnLen(args []any) (any, error) {
	if err := requireArgCount("len", args, 1); err != nil {
		return nil, err
	}
	return int64(len([]rune(argAsString("len", args, 0)))), nil
}

func fnCapitalize(args []any) (any, error) {
	if err := requireArgCount("capitalize", args, 1); err != nil {
		return nil, err
	}
	s := argAsString("capitalize", args, 0)
	if s == "" {
		return s, nil
	}
	r := []rune(s)
	r[0] = unicode.ToUpper(r[0])
	return string(r), nil
}

var kebabNonAlnumRun = regexp.MustCompile(`[^a-z0-9]+`)

// fnKebabCase lowercases the input, inserts a hyphen at camelCase word
// boundaries (a lowercase/digit followed by an uppercase letter), then
// collapses every remaining run of non-alphanumeric characters (spaces,
// underscores, punctuation) into a single hyphen and trims leading/trailing
// hyphens.
func fnKebabCase(args []any) (any, error) {
	if err := requireArgCount("kebabCase", args, 1); err != nil {
		return nil, err
	}
	s := argAsString("kebabCase", args, 0)
	runes := []rune(s)
	var b strings.Builder
	for i, r := range runes {
		if unicode.IsUpper(r) && i > 0 {
			prev := runes[i-1]
			if unicode.IsLower(prev) || unicode.IsDigit(prev) {
				b.WriteByte('-')
			}
		}
		b.WriteRune(unicode.ToLower(r))
	}
	out := kebabNonAlnumRun.ReplaceAllString(b.String(), "-")
	return strings.Trim(out, "-"), nil
}

var camelWordSplit = regexp.MustCompile(`[^a-zA-Z0-9]+`)

// fnCamelCase splits the input on runs of non-alphanumeric separators
// (spaces, hyphens, underscores) and joins the words back together with the
// first word lowercased and every subsequent word capitalized. An input
// with no separators (e.g. already-camelCase or a single word) is treated
// as one word and lowercased -- this function converts *into* camelCase
// from a separator-delimited form, it does not re-split existing camelCase.
func fnCamelCase(args []any) (any, error) {
	if err := requireArgCount("camelCase", args, 1); err != nil {
		return nil, err
	}
	s := argAsString("camelCase", args, 0)
	words := camelWordSplit.Split(s, -1)
	var b strings.Builder
	first := true
	for _, w := range words {
		if w == "" {
			continue
		}
		lw := strings.ToLower(w)
		if first {
			b.WriteString(lw)
			first = false
			continue
		}
		r := []rune(lw)
		r[0] = unicode.ToUpper(r[0])
		b.WriteString(string(r))
	}
	return b.String(), nil
}

// ---------------------------------------------------------------------
// encoding
// ---------------------------------------------------------------------

func fnHex(args []any) (any, error) {
	if err := requireArgCount("hex", args, 1); err != nil {
		return nil, err
	}
	return hex.EncodeToString([]byte(argAsString("hex", args, 0))), nil
}

func fnBase32(args []any) (any, error) {
	if err := requireArgCount("base32", args, 1); err != nil {
		return nil, err
	}
	return base32.StdEncoding.EncodeToString([]byte(argAsString("base32", args, 0))), nil
}

func fnBase64(args []any) (any, error) {
	if err := requireArgCount("base64", args, 1); err != nil {
		return nil, err
	}
	return base64.StdEncoding.EncodeToString([]byte(argAsString("base64", args, 0))), nil
}

// ---------------------------------------------------------------------
// crypto
// ---------------------------------------------------------------------

func fnSha1(args []any) (any, error) {
	if err := requireArgCount("sha1", args, 1); err != nil {
		return nil, err
	}
	sum := sha1.Sum([]byte(argAsString("sha1", args, 0)))
	return hex.EncodeToString(sum[:]), nil
}

func fnMd5(args []any) (any, error) {
	if err := requireArgCount("md5", args, 1); err != nil {
		return nil, err
	}
	sum := md5.Sum([]byte(argAsString("md5", args, 0)))
	return hex.EncodeToString(sum[:]), nil
}

func fnSha256(args []any) (any, error) {
	if err := requireArgCount("sha256", args, 1); err != nil {
		return nil, err
	}
	sum := sha256.Sum256([]byte(argAsString("sha256", args, 0)))
	return hex.EncodeToString(sum[:]), nil
}

func fnSha512(args []any) (any, error) {
	if err := requireArgCount("sha512", args, 1); err != nil {
		return nil, err
	}
	sum := sha512.Sum512([]byte(argAsString("sha512", args, 0)))
	return hex.EncodeToString(sum[:]), nil
}

// ---------------------------------------------------------------------
// math
// ---------------------------------------------------------------------

func fnAbs(args []any) (any, error) {
	if err := requireArgCount("abs", args, 1); err != nil {
		return nil, err
	}
	x, err := argAsFloat("abs", args, 0)
	if err != nil {
		return nil, err
	}
	return math.Abs(x), nil
}

func fnRound(args []any) (any, error) {
	if err := requireArgCount("round", args, 1); err != nil {
		return nil, err
	}
	x, err := argAsFloat("round", args, 0)
	if err != nil {
		return nil, err
	}
	return math.Round(x), nil
}

func fnFloor(args []any) (any, error) {
	if err := requireArgCount("floor", args, 1); err != nil {
		return nil, err
	}
	x, err := argAsFloat("floor", args, 0)
	if err != nil {
		return nil, err
	}
	return math.Floor(x), nil
}

func fnCeil(args []any) (any, error) {
	if err := requireArgCount("ceil", args, 1); err != nil {
		return nil, err
	}
	x, err := argAsFloat("ceil", args, 0)
	if err != nil {
		return nil, err
	}
	return math.Ceil(x), nil
}

func fnMin(args []any) (any, error) {
	if err := requireMinArgCount("min", args, 2); err != nil {
		return nil, err
	}
	m, err := argAsFloat("min", args, 0)
	if err != nil {
		return nil, err
	}
	for i := 1; i < len(args); i++ {
		v, err := argAsFloat("min", args, i)
		if err != nil {
			return nil, err
		}
		if v < m {
			m = v
		}
	}
	return m, nil
}

func fnMax(args []any) (any, error) {
	if err := requireMinArgCount("max", args, 2); err != nil {
		return nil, err
	}
	m, err := argAsFloat("max", args, 0)
	if err != nil {
		return nil, err
	}
	for i := 1; i < len(args); i++ {
		v, err := argAsFloat("max", args, i)
		if err != nil {
			return nil, err
		}
		if v > m {
			m = v
		}
	}
	return m, nil
}

func fnPow(args []any) (any, error) {
	if err := requireArgCount("pow", args, 2); err != nil {
		return nil, err
	}
	x, err := argAsFloat("pow", args, 0)
	if err != nil {
		return nil, err
	}
	y, err := argAsFloat("pow", args, 1)
	if err != nil {
		return nil, err
	}
	return math.Pow(x, y), nil
}

func fnMod(args []any) (any, error) {
	if err := requireArgCount("mod", args, 2); err != nil {
		return nil, err
	}
	x, err := argAsFloat("mod", args, 0)
	if err != nil {
		return nil, err
	}
	y, err := argAsFloat("mod", args, 1)
	if err != nil {
		return nil, err
	}
	if y == 0 {
		return nil, fmt.Errorf("mod: division by zero")
	}
	return math.Mod(x, y), nil
}
