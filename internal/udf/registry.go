// Package udf implements squad's curated, always-on SQL user-defined
// function library (internal-docs/UDFS.md / M10b). Every category file
// (string.go, hashing.go, ...) exports a Register() that registers that
// category's functions with modernc.org/sqlite and appends their
// Descriptor to the package registry, so the registry can never drift from
// what's actually registered.
package udf

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

// Descriptor describes one registered UDF for the catalog API/CLI/editor
// autocomplete.
type Descriptor struct {
	Name          string `json:"name"`
	Category      string `json:"category"`
	Signature     string `json:"signature"`
	Description   string `json:"description"`
	ExampleCall   string `json:"-"`
	ExampleResult string `json:"-"`
	Aggregate     bool   `json:"aggregate"`
	Deterministic bool   `json:"deterministic"`
}

// Example is the JSON-facing shape of a Descriptor's worked example.
type Example struct {
	Call   string `json:"call"`
	Result string `json:"result"`
}

// FunctionJSON is the JSON-facing shape of a Descriptor.
type FunctionJSON struct {
	Name          string  `json:"name"`
	Signature     string  `json:"signature"`
	Description   string  `json:"description"`
	Example       Example `json:"example"`
	Aggregate     bool    `json:"aggregate"`
	Deterministic bool    `json:"deterministic"`
}

// CategoryGroup is a named group of functions, as returned by Catalog().
type CategoryGroup struct {
	Name      string         `json:"name"`
	Functions []FunctionJSON `json:"functions"`
}

var (
	mu       sync.Mutex
	byName   = map[string]Descriptor{}
	registry []Descriptor

	registerOnce sync.Once
	registerErr  error
)

// add registers a Descriptor into the package-level catalog. It is called by
// each category's Register() alongside the actual SQLite registration call,
// so the two can never go out of sync.
func add(d Descriptor) {
	mu.Lock()
	defer mu.Unlock()
	key := strings.ToUpper(d.Name)
	if _, exists := byName[key]; exists {
		panic(fmt.Sprintf("udf: descriptor %q registered twice", d.Name))
	}
	byName[key] = d
	registry = append(registry, d)
}

// categoryRegistrars lists every category's Register function. Order here is
// cosmetic (registration itself doesn't depend on order); Catalog() sorts
// its own output.
var categoryRegistrars = []func() error{
	registerString,
	registerHashing,
	registerDatetime,
	registerNumeric,
	registerJSON,
	registerGeo,
	registerValidation,
	registerMisc,
	registerRailsString,
	registerCompression,
	registerColor,
	registerAdvDatetime,
	registerPhysics,
	registerInternet,
}

// RegisterAll registers every category's functions with modernc.org/sqlite.
// Registration is process-global (not per-connection), so this must run
// exactly once per process, before the first sql.Open. It is safe to call
// RegisterAll multiple times; only the first call does any work.
func RegisterAll() error {
	registerOnce.Do(func() {
		for _, fn := range categoryRegistrars {
			if err := fn(); err != nil {
				registerErr = err
				return
			}
		}
	})
	return registerErr
}

// Catalog returns every registered function grouped by category, sorted by
// category name then function name, for the HTTP catalog API and the CLI's
// `.functions`/`.functions <category>` output.
func Catalog() []CategoryGroup {
	mu.Lock()
	defer mu.Unlock()

	groups := map[string][]Descriptor{}
	for _, d := range registry {
		groups[d.Category] = append(groups[d.Category], d)
	}

	var names []string
	for name := range groups {
		names = append(names, name)
	}
	sort.Strings(names)

	out := make([]CategoryGroup, 0, len(names))
	for _, name := range names {
		fns := groups[name]
		sort.Slice(fns, func(i, j int) bool { return fns[i].Name < fns[j].Name })
		jfns := make([]FunctionJSON, 0, len(fns))
		for _, d := range fns {
			jfns = append(jfns, toJSON(d))
		}
		out = append(out, CategoryGroup{Name: name, Functions: jfns})
	}
	return out
}

func toJSON(d Descriptor) FunctionJSON {
	return FunctionJSON{
		Name:          d.Name,
		Signature:     d.Signature,
		Description:   d.Description,
		Example:       Example{Call: d.ExampleCall, Result: d.ExampleResult},
		Aggregate:     d.Aggregate,
		Deterministic: d.Deterministic,
	}
}

// Find looks up a function by name, case-insensitive exact match.
func Find(name string) (Descriptor, bool) {
	mu.Lock()
	defer mu.Unlock()
	d, ok := byName[strings.ToUpper(name)]
	return d, ok
}

// All returns every registered Descriptor, sorted by name, mainly for tests.
func All() []Descriptor {
	mu.Lock()
	defer mu.Unlock()
	out := make([]Descriptor, len(registry))
	copy(out, registry)
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}
