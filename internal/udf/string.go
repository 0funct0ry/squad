package udf

import (
	"database/sql/driver"
	"fmt"
	"regexp"
	"regexp/syntax"
	"strings"
	"unicode"

	"modernc.org/sqlite"
)

const catString = "String manipulation"

func compileRegexp(pattern string) (*regexp.Regexp, error) {
	if _, err := syntax.Parse(pattern, syntax.Perl); err != nil {
		return nil, fmt.Errorf("invalid regular expression: %w", err)
	}
	return regexp.Compile(pattern)
}

func registerString() error {
	if err := sqlite.RegisterDeterministicScalarFunction("REGEXP_MATCH", 2,
		func(ctx *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			re, err := compileRegexp(argString(args[1]))
			if err != nil {
				return nil, err
			}
			return boolResult(re.MatchString(argString(args[0]))), nil
		}); err != nil {
		return err
	}
	// SQLite invokes a function named "regexp(pattern, expr)" under the hood
	// for the `expr REGEXP pattern` operator (reversed arg order vs.
	// REGEXP_MATCH(str, pattern)).
	if err := sqlite.RegisterDeterministicScalarFunction("REGEXP", 2,
		func(ctx *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			re, err := compileRegexp(argString(args[0]))
			if err != nil {
				return nil, err
			}
			return boolResult(re.MatchString(argString(args[1]))), nil
		}); err != nil {
		return err
	}
	add(Descriptor{
		Name: "REGEXP_MATCH", Category: catString, Signature: "REGEXP_MATCH(str, pattern) -> bool",
		Description: "Returns 1 if str matches the regular expression pattern, else 0. Also enables the REGEXP operator.",
		ExampleCall: `REGEXP_MATCH('hello123', '[0-9]+')`, ExampleResult: "1",
		Deterministic: true,
	})

	if err := sqlite.RegisterDeterministicScalarFunction("REGEXP_REPLACE", 3,
		func(ctx *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			re, err := compileRegexp(argString(args[1]))
			if err != nil {
				return nil, err
			}
			return re.ReplaceAllString(argString(args[0]), argString(args[2])), nil
		}); err != nil {
		return err
	}
	add(Descriptor{
		Name: "REGEXP_REPLACE", Category: catString, Signature: "REGEXP_REPLACE(str, pattern, repl) -> str",
		Description: "Replaces every regex match of pattern in str with repl.",
		ExampleCall: `REGEXP_REPLACE('hello world', '[aeiou]', '*')`, ExampleResult: "h*ll* w*rld",
		Deterministic: true,
	})

	if err := sqlite.RegisterDeterministicScalarFunction("REGEXP_EXTRACT", 3,
		func(ctx *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			re, err := compileRegexp(argString(args[1]))
			if err != nil {
				return nil, err
			}
			group, err := argInt(args[2])
			if err != nil {
				return nil, err
			}
			m := re.FindStringSubmatch(argString(args[0]))
			if m == nil || int(group) >= len(m) || group < 0 {
				return nil, nil
			}
			return m[group], nil
		}); err != nil {
		return err
	}
	add(Descriptor{
		Name: "REGEXP_EXTRACT", Category: catString, Signature: "REGEXP_EXTRACT(str, pattern, group) -> str",
		Description: "Extracts the given capture group number from the first regex match, or NULL if no match.",
		ExampleCall: `REGEXP_EXTRACT('order-2024-0091', '(\d{4})-(\d+)', 2)`, ExampleResult: "0091",
		Deterministic: true,
	})

	if err := sqlite.RegisterDeterministicScalarFunction("SPLIT_PART", 3,
		func(ctx *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			delim := argString(args[1])
			n, err := argInt(args[2])
			if err != nil {
				return nil, err
			}
			if n < 1 {
				return nil, fmt.Errorf("SPLIT_PART: n must be >= 1")
			}
			var parts []string
			if delim == "" {
				parts = []string{argString(args[0])}
			} else {
				parts = strings.Split(argString(args[0]), delim)
			}
			if int(n) > len(parts) {
				return "", nil
			}
			return parts[n-1], nil
		}); err != nil {
		return err
	}
	add(Descriptor{
		Name: "SPLIT_PART", Category: catString, Signature: "SPLIT_PART(str, delim, n) -> str",
		Description: "Splits str on delim and returns the nth part (1-indexed).",
		ExampleCall: `SPLIT_PART('a,b,c', ',', 2)`, ExampleResult: "b",
		Deterministic: true,
	})

	if err := sqlite.RegisterDeterministicScalarFunction("LEVENSHTEIN", 2,
		func(ctx *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			return int64(levenshtein(argString(args[0]), argString(args[1]))), nil
		}); err != nil {
		return err
	}
	add(Descriptor{
		Name: "LEVENSHTEIN", Category: catString, Signature: "LEVENSHTEIN(a, b) -> int",
		Description: "Edit distance between a and b.",
		ExampleCall: `LEVENSHTEIN('kitten', 'sitting')`, ExampleResult: "3",
		Deterministic: true,
	})

	if err := sqlite.RegisterDeterministicScalarFunction("SOUNDEX", 1,
		func(ctx *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			return soundex(argString(args[0])), nil
		}); err != nil {
		return err
	}
	add(Descriptor{
		Name: "SOUNDEX", Category: catString, Signature: "SOUNDEX(str) -> str",
		Description: "Phonetic encoding of str, useful for fuzzy name matching.",
		ExampleCall: `SOUNDEX('Robert')`, ExampleResult: "R163",
		Deterministic: true,
	})

	if err := sqlite.RegisterDeterministicScalarFunction("SLUGIFY", 1,
		func(ctx *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			return slugify(argString(args[0])), nil
		}); err != nil {
		return err
	}
	add(Descriptor{
		Name: "SLUGIFY", Category: catString, Signature: "SLUGIFY(str) -> str",
		Description: "Lower-cases, strips punctuation, and hyphenates str into a URL-safe slug.",
		ExampleCall: `SLUGIFY('Hello, World!')`, ExampleResult: "hello-world",
		Deterministic: true,
	})

	if err := sqlite.RegisterDeterministicScalarFunction("TITLE_CASE", 1,
		func(ctx *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			return strings.Title(strings.ToLower(argString(args[0]))), nil
		}); err != nil {
		return err
	}
	add(Descriptor{
		Name: "TITLE_CASE", Category: catString, Signature: "TITLE_CASE(str) -> str",
		Description: "Capitalizes the first letter of each word.",
		ExampleCall: `TITLE_CASE('the great gatsby')`, ExampleResult: "The Great Gatsby",
		Deterministic: true,
	})

	if err := sqlite.RegisterDeterministicScalarFunction("REVERSE", 1,
		func(ctx *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			r := []rune(argString(args[0]))
			for i, j := 0, len(r)-1; i < j; i, j = i+1, j-1 {
				r[i], r[j] = r[j], r[i]
			}
			return string(r), nil
		}); err != nil {
		return err
	}
	add(Descriptor{
		Name: "REVERSE", Category: catString, Signature: "REVERSE(str) -> str",
		Description: "Reverses the characters of str.",
		ExampleCall: `REVERSE('squad')`, ExampleResult: "dauqs",
		Deterministic: true,
	})

	padFn := func(left bool) func(ctx *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
		return func(ctx *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			str := argString(args[0])
			length, err := argInt(args[1])
			if err != nil {
				return nil, err
			}
			pad := argString(args[2])
			if pad == "" {
				pad = " "
			}
			for len([]rune(str)) < int(length) {
				if left {
					str = pad + str
				} else {
					str = str + pad
				}
			}
			r := []rune(str)
			if len(r) > int(length) {
				if left {
					r = r[len(r)-int(length):]
				} else {
					r = r[:length]
				}
			}
			return string(r), nil
		}
	}
	if err := sqlite.RegisterDeterministicScalarFunction("PAD_LEFT", 3, padFn(true)); err != nil {
		return err
	}
	add(Descriptor{
		Name: "PAD_LEFT", Category: catString, Signature: "PAD_LEFT(str, len, char) -> str",
		Description: "Pads str to length len with char, on the left.",
		ExampleCall: `PAD_LEFT('7', 3, '0')`, ExampleResult: "007",
		Deterministic: true,
	})
	if err := sqlite.RegisterDeterministicScalarFunction("PAD_RIGHT", 3, padFn(false)); err != nil {
		return err
	}
	add(Descriptor{
		Name: "PAD_RIGHT", Category: catString, Signature: "PAD_RIGHT(str, len, char) -> str",
		Description: "Pads str to length len with char, on the right.",
		ExampleCall: `PAD_RIGHT('7', 3, '0')`, ExampleResult: "700",
		Deterministic: true,
	})

	htmlTagRe := regexp.MustCompile(`<[^>]*>`)
	if err := sqlite.RegisterDeterministicScalarFunction("STRIP_HTML", 1,
		func(ctx *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			return htmlTagRe.ReplaceAllString(argString(args[0]), ""), nil
		}); err != nil {
		return err
	}
	add(Descriptor{
		Name: "STRIP_HTML", Category: catString, Signature: "STRIP_HTML(str) -> str",
		Description: "Removes HTML tags, leaving only text content.",
		ExampleCall: `STRIP_HTML('<b>Hi</b> there')`, ExampleResult: "Hi there",
		Deterministic: true,
	})

	if err := sqlite.RegisterDeterministicScalarFunction("NORMALIZE_WHITESPACE", 1,
		func(ctx *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			return strings.Join(strings.Fields(argString(args[0])), " "), nil
		}); err != nil {
		return err
	}
	add(Descriptor{
		Name: "NORMALIZE_WHITESPACE", Category: catString, Signature: "NORMALIZE_WHITESPACE(str) -> str",
		Description: "Collapses runs of whitespace into single spaces and trims the ends.",
		ExampleCall: `NORMALIZE_WHITESPACE('  a   b\t\tc  ')`, ExampleResult: "a b c",
		Deterministic: true,
	})

	return nil
}

// levenshtein computes edit distance with a two-row DP table.
func levenshtein(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	prev := make([]int, len(rb)+1)
	curr := make([]int, len(rb)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		curr[0] = i
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			del := prev[j] + 1
			ins := curr[j-1] + 1
			sub := prev[j-1] + cost
			m := del
			if ins < m {
				m = ins
			}
			if sub < m {
				m = sub
			}
			curr[j] = m
		}
		prev, curr = curr, prev
	}
	return prev[len(rb)]
}

var soundexCodes = map[rune]byte{
	'B': '1', 'F': '1', 'P': '1', 'V': '1',
	'C': '2', 'G': '2', 'J': '2', 'K': '2', 'Q': '2', 'S': '2', 'X': '2', 'Z': '2',
	'D': '3', 'T': '3',
	'L': '4',
	'M': '5', 'N': '5',
	'R': '6',
}

func soundex(s string) string {
	s = strings.ToUpper(strings.TrimSpace(s))
	var letters []rune
	for _, r := range s {
		if unicode.IsLetter(r) {
			letters = append(letters, r)
		}
	}
	if len(letters) == 0 {
		return ""
	}
	out := []byte{byte(letters[0])}
	lastCode := soundexCodes[letters[0]]
	for _, r := range letters[1:] {
		code, ok := soundexCodes[r]
		if ok && code != lastCode {
			out = append(out, code)
		}
		if !ok && r != 'H' && r != 'W' {
			lastCode = 0
		} else {
			lastCode = code
		}
		if len(out) >= 4 {
			break
		}
	}
	for len(out) < 4 {
		out = append(out, '0')
	}
	return string(out[:4])
}

var slugNonAlnumRe = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = slugNonAlnumRe.ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}
