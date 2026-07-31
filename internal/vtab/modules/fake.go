package modules

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/0funct0ry/squad/internal/seed"
	"github.com/brianvoe/gofakeit/v7"
	vtabdriver "modernc.org/sqlite/vtab"
)

// FakeModule implements the `fake` module (VTABS.md #9): rows= (int,
// required), then one <column>=<generator> pair per column (generator names
// from internal/seed's registry), plus optional seed= for reproducible
// output. Order-sensitive: column order in the schema follows argument
// order, so this module parses the raw USING(...) argument list itself
// rather than going through the unordered ModuleArgs map.
//
// A generator that itself takes options (e.g. oneOf's required `values`,
// digitSequence's `width`) is written as <column>=<generator>:<json>, the
// json object being exactly the generator's OptionsSchema keys — e.g.
// status=oneOf:{"values":"SHIPPED,PENDING"}. Generators needing no options
// (email, firstName, country, ...) are written bare, unchanged.
type FakeModule struct{}

func (m *FakeModule) Create(ctx vtabdriver.Context, args []string) (vtabdriver.Table, error) {
	return m.connect(ctx, UsingArgs(args))
}

func (m *FakeModule) Connect(ctx vtabdriver.Context, args []string) (vtabdriver.Table, error) {
	return m.connect(ctx, UsingArgs(args))
}

func (m *FakeModule) connect(ctx vtabdriver.Context, rawArgs []string) (vtabdriver.Table, error) {
	var rowCount int
	var haveRows bool
	var seedVal int64
	var haveSeed bool
	var cols []string
	var generators []string
	var genOpts []map[string]any

	for _, item := range rawArgs {
		parsed, err := ParseArgs([]string{item})
		if err != nil {
			return nil, err
		}
		for k, v := range parsed {
			switch k {
			case "rows":
				n, err := strconv.Atoi(v)
				if err != nil {
					return nil, fmt.Errorf("fake module: rows must be an integer: %w", err)
				}
				rowCount = n
				haveRows = true
			case "seed":
				n, err := strconv.ParseInt(v, 10, 64)
				if err != nil {
					return nil, fmt.Errorf("fake module: seed must be an integer: %w", err)
				}
				seedVal = n
				haveSeed = true
			default:
				gen, opts, err := parseGeneratorSpec(v)
				if err != nil {
					return nil, fmt.Errorf("fake module: column %q: %w", k, err)
				}
				cols = append(cols, k)
				generators = append(generators, gen)
				genOpts = append(genOpts, opts)
			}
		}
	}

	if !haveRows {
		return nil, fmt.Errorf("fake module: missing required argument: rows")
	}
	if len(cols) == 0 {
		return nil, fmt.Errorf("fake module: at least one <column>=<generator> pair is required")
	}
	for _, g := range generators {
		if !seed.Exists(g) {
			return nil, fmt.Errorf("fake module: unknown generator %q", g)
		}
	}

	if haveSeed {
		gofakeit.Seed(seedVal)
	}

	if err := DeclareColumns(ctx, cols, nil); err != nil {
		return nil, err
	}

	rows := make([][]vtabdriver.Value, 0, rowCount)
	for i := 0; i < rowCount; i++ {
		row := make([]vtabdriver.Value, len(cols))
		for j, gen := range generators {
			v, err := seed.Generate(gen, "TEXT", genOpts[j])
			if err != nil {
				return nil, fmt.Errorf("fake module: generator %q: %w", gen, err)
			}
			row[j] = fakeScalar(v)
		}
		rows = append(rows, row)
	}

	return NewSimpleTable(rows), nil
}

// parseGeneratorSpec splits a <generator> or <generator>:<json-options>
// column value. The json suffix is optional; a bare generator name (the
// common case: email, firstName, country, ...) returns nil options.
func parseGeneratorSpec(spec string) (string, map[string]any, error) {
	name, rawOpts, hasOpts := strings.Cut(spec, ":")
	if !hasOpts {
		return spec, nil, nil
	}
	var opts map[string]any
	if err := json.Unmarshal([]byte(rawOpts), &opts); err != nil {
		return "", nil, fmt.Errorf("invalid options JSON after generator %q: %w", name, err)
	}
	return name, opts, nil
}

func fakeScalar(v any) vtabdriver.Value {
	switch t := v.(type) {
	case nil, string, int64, float64, bool, []byte:
		return t
	case int:
		return int64(t)
	default:
		return fmt.Sprintf("%v", t)
	}
}
