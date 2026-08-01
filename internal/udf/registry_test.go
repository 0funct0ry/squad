package udf

import (
	"database/sql"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	if err := RegisterAll(); err != nil {
		t.Fatalf("RegisterAll: %v", err)
	}
	db, err := sql.Open("sqlite", "file::memory:?cache=shared")
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// sqliteBuiltins is a representative set of SQLite's own scalar/aggregate
// function names, used to assert none of our registrations shadow a
// built-in.
var sqliteBuiltins = []string{
	"abs", "avg", "coalesce", "count", "date", "datetime", "glob",
	"group_concat", "hex", "ifnull", "instr", "json", "json_extract",
	"julianday", "length", "like", "lower", "ltrim", "max", "min", "printf",
	"quote", "random", "replace", "round", "rtrim", "strftime", "substr",
	"sum", "time", "total", "trim", "typeof", "unicode", "upper", "zeroblob",
}

func TestRegisterAllNoBuiltinCollisions(t *testing.T) {
	openTestDB(t)
	builtins := map[string]bool{}
	for _, b := range sqliteBuiltins {
		builtins[strings.ToUpper(b)] = true
	}
	for _, d := range All() {
		if builtins[strings.ToUpper(d.Name)] {
			t.Errorf("registered function %q shadows a SQLite built-in", d.Name)
		}
	}
}

func TestRegisterAllCallable(t *testing.T) {
	db := openTestDB(t)
	for _, d := range All() {
		d := d
		t.Run(d.Name, func(t *testing.T) {
			if d.Aggregate {
				var res any
				query := "SELECT " + d.Name + "(x) FROM (SELECT column1 AS x FROM (VALUES (1),(2),(3)))"
				if d.Name == "PERCENTILE" {
					query = "SELECT PERCENTILE(x, 0.5) FROM (SELECT column1 AS x FROM (VALUES (1),(2),(3)))"
				}
				if err := db.QueryRow(query).Scan(&res); err != nil {
					t.Fatalf("%s not callable as an aggregate: %v", d.Name, err)
				}
				return
			}
			// Every scalar function is callable via NULL args to confirm
			// registration succeeded, ignoring any resulting SQL-level error
			// from bad input (that's covered by the worked-example tests
			// below) — we only care that the function itself is registered.
			nargs := strings.Count(d.Signature, ",") + 1
			if strings.Contains(d.Signature, "()") {
				nargs = 0
			}
			placeholders := strings.TrimSuffix(strings.Repeat("NULL,", nargs), ",")
			var res any
			err := db.QueryRow("SELECT " + d.Name + "(" + placeholders + ")").Scan(&res)
			if err != nil && strings.Contains(err.Error(), "no such function") {
				t.Fatalf("%s not registered: %v", d.Name, err)
			}
		})
	}
}

type example struct {
	name  string
	query string
	check func(t *testing.T, v any)
}

func str(t *testing.T, v any) string {
	t.Helper()
	switch s := v.(type) {
	case string:
		return s
	case []byte:
		return string(s)
	default:
		t.Fatalf("expected string-ish, got %T (%v)", v, v)
		return ""
	}
}

func TestWorkedExamples(t *testing.T) {
	db := openTestDB(t)
	cases := []example{
		{"REGEXP_MATCH", `SELECT REGEXP_MATCH('hello123', '[0-9]+')`, func(t *testing.T, v any) {
			if v.(int64) != 1 {
				t.Fatalf("got %v", v)
			}
		}},
		{"REGEXP_REPLACE", `SELECT REGEXP_REPLACE('hello world', '[aeiou]', '*')`, func(t *testing.T, v any) {
			if str(t, v) != "h*ll* w*rld" {
				t.Fatalf("got %q", str(t, v))
			}
		}},
		{"REGEXP_EXTRACT", `SELECT REGEXP_EXTRACT('order-2024-0091', '(\d{4})-(\d+)', 2)`, func(t *testing.T, v any) {
			if str(t, v) != "0091" {
				t.Fatalf("got %q", str(t, v))
			}
		}},
		{"SPLIT_PART", `SELECT SPLIT_PART('a,b,c', ',', 2)`, func(t *testing.T, v any) {
			if str(t, v) != "b" {
				t.Fatalf("got %q", str(t, v))
			}
		}},
		{"LEVENSHTEIN", `SELECT LEVENSHTEIN('kitten', 'sitting')`, func(t *testing.T, v any) {
			if v.(int64) != 3 {
				t.Fatalf("got %v", v)
			}
		}},
		{"SLUGIFY", `SELECT SLUGIFY('Hello, World!')`, func(t *testing.T, v any) {
			if str(t, v) != "hello-world" {
				t.Fatalf("got %q", str(t, v))
			}
		}},
		{"TITLE_CASE", `SELECT TITLE_CASE('the great gatsby')`, func(t *testing.T, v any) {
			if str(t, v) != "The Great Gatsby" {
				t.Fatalf("got %q", str(t, v))
			}
		}},
		{"REVERSE", `SELECT REVERSE('squad')`, func(t *testing.T, v any) {
			if str(t, v) != "dauqs" {
				t.Fatalf("got %q", str(t, v))
			}
		}},
		{"PAD_LEFT", `SELECT PAD_LEFT('7', 3, '0')`, func(t *testing.T, v any) {
			if str(t, v) != "007" {
				t.Fatalf("got %q", str(t, v))
			}
		}},
		{"PAD_RIGHT", `SELECT PAD_RIGHT('7', 3, '0')`, func(t *testing.T, v any) {
			if str(t, v) != "700" {
				t.Fatalf("got %q", str(t, v))
			}
		}},
		{"STRIP_HTML", `SELECT STRIP_HTML('<b>Hi</b> there')`, func(t *testing.T, v any) {
			if str(t, v) != "Hi there" {
				t.Fatalf("got %q", str(t, v))
			}
		}},
		{"NORMALIZE_WHITESPACE", "SELECT NORMALIZE_WHITESPACE('  a   b\t\tc  ')", func(t *testing.T, v any) {
			if str(t, v) != "a b c" {
				t.Fatalf("got %q", str(t, v))
			}
		}},
		{"BASE64_ENCODE", `SELECT BASE64_ENCODE('squad')`, func(t *testing.T, v any) {
			if str(t, v) != "c3F1YWQ=" {
				t.Fatalf("got %q", str(t, v))
			}
		}},
		{"BASE64_DECODE", `SELECT BASE64_DECODE('c3F1YWQ=')`, func(t *testing.T, v any) {
			if str(t, v) != "squad" {
				t.Fatalf("got %q", str(t, v))
			}
		}},
		{"HEX_ENCODE", `SELECT HEX_ENCODE('AB')`, func(t *testing.T, v any) {
			if str(t, v) != "4142" {
				t.Fatalf("got %q", str(t, v))
			}
		}},
		{"HEX_DECODE", `SELECT HEX_DECODE('4142')`, func(t *testing.T, v any) {
			if str(t, v) != "AB" {
				t.Fatalf("got %q", str(t, v))
			}
		}},
		{"CRC32", `SELECT CRC32('squad')`, func(t *testing.T, v any) {
			if v.(int64) == 0 {
				t.Fatalf("got %v", v)
			}
		}},
		{"UUID", `SELECT UUID()`, func(t *testing.T, v any) {
			if len(str(t, v)) != 36 {
				t.Fatalf("got %q", str(t, v))
			}
		}},
		{"DATE_TRUNC", `SELECT DATE_TRUNC('month', '2026-07-31')`, func(t *testing.T, v any) {
			if str(t, v) != "2026-07-01" {
				t.Fatalf("got %q", str(t, v))
			}
		}},
		{"WEEKDAY_NAME", `SELECT WEEKDAY_NAME('2026-07-31')`, func(t *testing.T, v any) {
			if str(t, v) != "Friday" {
				t.Fatalf("got %q", str(t, v))
			}
		}},
		{"IS_WEEKEND", `SELECT IS_WEEKEND('2026-08-01')`, func(t *testing.T, v any) {
			if v.(int64) != 1 {
				t.Fatalf("got %v", v)
			}
		}},
		{"UNIX_TO_ISO", `SELECT UNIX_TO_ISO(1785456000)`, func(t *testing.T, v any) {
			if str(t, v) != "2026-07-31T00:00:00Z" {
				t.Fatalf("got %q", str(t, v))
			}
		}},
		{"ISO_TO_UNIX", `SELECT ISO_TO_UNIX('2026-07-31T00:00:00Z')`, func(t *testing.T, v any) {
			if v.(int64) != 1785456000 {
				t.Fatalf("got %v", v)
			}
		}},
		{"ROUND_TO", `SELECT ROUND_TO(3.14159, 2)`, func(t *testing.T, v any) {
			if v.(float64) != 3.14 {
				t.Fatalf("got %v", v)
			}
		}},
		{"CLAMP", `SELECT CLAMP(120, 0, 100)`, func(t *testing.T, v any) {
			if v.(float64) != 100 {
				t.Fatalf("got %v", v)
			}
		}},
		{"GCD", `SELECT GCD(12, 18)`, func(t *testing.T, v any) {
			if v.(int64) != 6 {
				t.Fatalf("got %v", v)
			}
		}},
		{"LCM", `SELECT LCM(4, 6)`, func(t *testing.T, v any) {
			if v.(int64) != 12 {
				t.Fatalf("got %v", v)
			}
		}},
		{"IS_PRIME", `SELECT IS_PRIME(17)`, func(t *testing.T, v any) {
			if v.(int64) != 1 {
				t.Fatalf("got %v", v)
			}
		}},
		{"SAFE_DIVIDE-ok", `SELECT SAFE_DIVIDE(10, 2)`, func(t *testing.T, v any) {
			if v.(float64) != 5 {
				t.Fatalf("got %v", v)
			}
		}},
		{"SAFE_DIVIDE-zero", `SELECT SAFE_DIVIDE(10, 0)`, func(t *testing.T, v any) {
			if v != nil {
				t.Fatalf("got %v, want NULL", v)
			}
		}},
		{"JSON_MERGE", `SELECT JSON_MERGE('{"a":1,"c":{"x":1}}', '{"b":2,"c":{"y":2}}')`, func(t *testing.T, v any) {
			got := str(t, v)
			if !strings.Contains(got, `"a":1`) || !strings.Contains(got, `"b":2`) {
				t.Fatalf("got %q", got)
			}
		}},
		{"JSON_PATH_EXISTS", `SELECT JSON_PATH_EXISTS('{"a":{"b":1}}', '$.a.b')`, func(t *testing.T, v any) {
			if v.(int64) != 1 {
				t.Fatalf("got %v", v)
			}
		}},
		{"CSV_TO_JSON", `SELECT CSV_TO_JSON('a,b,"c,d"')`, func(t *testing.T, v any) {
			if str(t, v) != `["a","b","c,d"]` {
				t.Fatalf("got %q", str(t, v))
			}
		}},
		{"HAVERSINE_DISTANCE", `SELECT HAVERSINE_DISTANCE(40.7128, -74.0060, 51.5074, -0.1278)`, func(t *testing.T, v any) {
			f := v.(float64)
			if f < 5500 || f > 5650 {
				t.Fatalf("got %v", f)
			}
		}},
		{"GEOHASH_ENCODE", `SELECT GEOHASH_ENCODE(40.7128, -74.0060, 7)`, func(t *testing.T, v any) {
			if str(t, v) != "dr5regw" {
				t.Fatalf("got %q", str(t, v))
			}
		}},
		{"GEOHASH_DECODE", `SELECT GEOHASH_DECODE('dr5regw')`, func(t *testing.T, v any) {
			got := str(t, v)
			if !strings.Contains(got, `"lat":40.7`) {
				t.Fatalf("got %q", got)
			}
		}},
		{"BOUNDING_BOX_CONTAINS", `SELECT BOUNDING_BOX_CONTAINS(40.7, -74.0, 40.0, -75.0, 41.0, -73.0)`, func(t *testing.T, v any) {
			if v.(int64) != 1 {
				t.Fatalf("got %v", v)
			}
		}},
		{"POINT_IN_POLYGON", `SELECT POINT_IN_POLYGON(1, 1, '{"type":"Polygon","coordinates":[[[0,0],[0,2],[2,2],[2,0],[0,0]]]}')`, func(t *testing.T, v any) {
			if v.(int64) != 1 {
				t.Fatalf("got %v", v)
			}
		}},
		{"IS_VALID_EMAIL", `SELECT IS_VALID_EMAIL('a@example.com')`, func(t *testing.T, v any) {
			if v.(int64) != 1 {
				t.Fatalf("got %v", v)
			}
		}},
		{"IS_VALID_JSON", `SELECT IS_VALID_JSON('{"a":1}')`, func(t *testing.T, v any) {
			if v.(int64) != 1 {
				t.Fatalf("got %v", v)
			}
		}},
		{"IS_VALID_URL", `SELECT IS_VALID_URL('https://squad.dev')`, func(t *testing.T, v any) {
			if v.(int64) != 1 {
				t.Fatalf("got %v", v)
			}
		}},
		{"LUHN_CHECK", `SELECT LUHN_CHECK('4111111111111111')`, func(t *testing.T, v any) {
			if v.(int64) != 1 {
				t.Fatalf("got %v", v)
			}
		}},
		{"IS_VALID_IP", `SELECT IS_VALID_IP('192.168.1.1')`, func(t *testing.T, v any) {
			if v.(int64) != 1 {
				t.Fatalf("got %v", v)
			}
		}},
		{"IP_IN_CIDR", `SELECT IP_IN_CIDR('192.168.1.5', '192.168.1.0/24')`, func(t *testing.T, v any) {
			if v.(int64) != 1 {
				t.Fatalf("got %v", v)
			}
		}},
		{"IS_VALID_DOMAIN", `SELECT IS_VALID_DOMAIN('example.com')`, func(t *testing.T, v any) {
			if v.(int64) != 1 {
				t.Fatalf("got %v", v)
			}
		}},
		{"RANDOM_STRING", `SELECT RANDOM_STRING(8)`, func(t *testing.T, v any) {
			if len(str(t, v)) != 8 {
				t.Fatalf("got %q", str(t, v))
			}
		}},
		{"PLURALIZE", `SELECT PLURALIZE('box')`, func(t *testing.T, v any) {
			if str(t, v) != "boxes" {
				t.Fatalf("got %q", str(t, v))
			}
		}},
		{"SINGULARIZE", `SELECT SINGULARIZE('boxes')`, func(t *testing.T, v any) {
			if str(t, v) != "box" {
				t.Fatalf("got %q", str(t, v))
			}
		}},
		{"CAMELIZE", `SELECT CAMELIZE('user_name')`, func(t *testing.T, v any) {
			if str(t, v) != "UserName" {
				t.Fatalf("got %q", str(t, v))
			}
		}},
		{"UNDERSCORE", `SELECT UNDERSCORE('UserName')`, func(t *testing.T, v any) {
			if str(t, v) != "user_name" {
				t.Fatalf("got %q", str(t, v))
			}
		}},
		{"DASHERIZE", `SELECT DASHERIZE('user_name')`, func(t *testing.T, v any) {
			if str(t, v) != "user-name" {
				t.Fatalf("got %q", str(t, v))
			}
		}},
		{"HUMANIZE", `SELECT HUMANIZE('user_name')`, func(t *testing.T, v any) {
			if str(t, v) != "User name" {
				t.Fatalf("got %q", str(t, v))
			}
		}},
		{"PARAMETERIZE", `SELECT PARAMETERIZE('My Blog Post!')`, func(t *testing.T, v any) {
			if str(t, v) != "my-blog-post" {
				t.Fatalf("got %q", str(t, v))
			}
		}},
		{"ORDINALIZE", `SELECT ORDINALIZE(3)`, func(t *testing.T, v any) {
			if str(t, v) != "3rd" {
				t.Fatalf("got %q", str(t, v))
			}
		}},
		{"COMPRESSION_RATIO", `SELECT COMPRESSION_RATIO(10, 5)`, func(t *testing.T, v any) {
			if v.(float64) != 2 {
				t.Fatalf("got %v", v)
			}
		}},
		{"GZIP_ROUNDTRIP", `SELECT GZIP_DECOMPRESS(GZIP_COMPRESS('squad'))`, func(t *testing.T, v any) {
			if str(t, v) != "squad" {
				t.Fatalf("got %q", str(t, v))
			}
		}},
		{"HEX_TO_RGB", `SELECT HEX_TO_RGB('#22d3ee')`, func(t *testing.T, v any) {
			got := str(t, v)
			if !strings.Contains(got, `"r":34`) {
				t.Fatalf("got %q", got)
			}
		}},
		{"RGB_TO_HEX", `SELECT RGB_TO_HEX(34, 211, 238)`, func(t *testing.T, v any) {
			if str(t, v) != "#22d3ee" {
				t.Fatalf("got %q", str(t, v))
			}
		}},
		{"COLOR_MIX", `SELECT COLOR_MIX('#ffffff', '#000000', 0.5)`, func(t *testing.T, v any) {
			got := str(t, v)
			if got != "#7f7f7f" && got != "#808080" {
				t.Fatalf("got %q", got)
			}
		}},
		{"COLOR_IS_LIGHT", `SELECT COLOR_IS_LIGHT('#ffffff')`, func(t *testing.T, v any) {
			if v.(int64) != 1 {
				t.Fatalf("got %v", v)
			}
		}},
		{"COLOR_CONTRAST_RATIO", `SELECT COLOR_CONTRAST_RATIO('#000000', '#ffffff')`, func(t *testing.T, v any) {
			f := v.(float64)
			if f < 20.9 || f > 21.1 {
				t.Fatalf("got %v", f)
			}
		}},
		{"CRON_NEXT_RUN", `SELECT CRON_NEXT_RUN('0 9 * * MON', '2026-07-31T00:00:00Z')`, func(t *testing.T, v any) {
			if str(t, v) != "2026-08-03T09:00:00Z" {
				t.Fatalf("got %q", str(t, v))
			}
		}},
		{"HOLIDAY_NAME", `SELECT HOLIDAY_NAME('2026-12-25', 'US')`, func(t *testing.T, v any) {
			if str(t, v) != "Christmas Day" {
				t.Fatalf("got %q", str(t, v))
			}
		}},
		{"IS_HOLIDAY", `SELECT IS_HOLIDAY('2026-12-25', 'US')`, func(t *testing.T, v any) {
			if v.(int64) != 1 {
				t.Fatalf("got %v", v)
			}
		}},
		{"ISO_WEEK", `SELECT ISO_WEEK('2026-07-31')`, func(t *testing.T, v any) {
			if v.(int64) != 31 {
				t.Fatalf("got %v", v)
			}
		}},
		{"UNIT_CONVERT", `SELECT UNIT_CONVERT(10, 'km', 'mi')`, func(t *testing.T, v any) {
			f := v.(float64)
			if f < 6.2 || f > 6.22 {
				t.Fatalf("got %v", f)
			}
		}},
		{"MIME_TYPE_FROM_EXT", `SELECT MIME_TYPE_FROM_EXT('report.pdf')`, func(t *testing.T, v any) {
			if str(t, v) != "application/pdf" {
				t.Fatalf("got %q", str(t, v))
			}
		}},
		{"EXTRACT_URLS", `SELECT EXTRACT_URLS('see https://squad.dev and http://x.io')`, func(t *testing.T, v any) {
			got := str(t, v)
			if !strings.Contains(got, "squad.dev") || !strings.Contains(got, "x.io") {
				t.Fatalf("got %q", got)
			}
		}},
		{"MEDIAN", `SELECT MEDIAN(x) FROM (SELECT column1 AS x FROM (VALUES (1),(2),(3)))`, func(t *testing.T, v any) {
			if v.(float64) != 2 {
				t.Fatalf("got %v", v)
			}
		}},
		{"MODE", `SELECT MODE(x) FROM (SELECT column1 AS x FROM (VALUES ('a'),('b'),('a')))`, func(t *testing.T, v any) {
			if str(t, v) != "a" {
				t.Fatalf("got %q", str(t, v))
			}
		}},
		{"STDDEV-VARIANCE", `SELECT VARIANCE(x) FROM (SELECT column1 AS x FROM (VALUES (2),(4),(4),(4),(5),(5),(7),(9)))`, func(t *testing.T, v any) {
			f := v.(float64)
			if f < 3.9 || f > 4.1 {
				t.Fatalf("got %v", f)
			}
		}},
		{"PERCENTILE", `SELECT PERCENTILE(x, 0.5) FROM (SELECT column1 AS x FROM (VALUES (1),(2),(3)))`, func(t *testing.T, v any) {
			if v.(float64) != 2 {
				t.Fatalf("got %v", v)
			}
		}},
		// Edge cases
		{"SLUGIFY-empty", `SELECT SLUGIFY('')`, func(t *testing.T, v any) {
			if str(t, v) != "" {
				t.Fatalf("got %q", str(t, v))
			}
		}},
		{"LEVENSHTEIN-empty", `SELECT LEVENSHTEIN('', 'abc')`, func(t *testing.T, v any) {
			if v.(int64) != 3 {
				t.Fatalf("got %v", v)
			}
		}},
		{"IS_PRIME-zero", `SELECT IS_PRIME(0)`, func(t *testing.T, v any) {
			if v.(int64) != 0 {
				t.Fatalf("got %v", v)
			}
		}},
		{"GCD-zero", `SELECT GCD(0, 5)`, func(t *testing.T, v any) {
			if v.(int64) != 5 {
				t.Fatalf("got %v", v)
			}
		}},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			var v any
			if err := db.QueryRow(c.query).Scan(&v); err != nil {
				t.Fatalf("query %q: %v", c.query, err)
			}
			c.check(t, v)
		})
	}
}
