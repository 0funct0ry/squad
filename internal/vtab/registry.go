// Package vtab implements squad's --modules feature (M10e):
// Go-implemented SQLite virtual table modules, mounted on demand into the
// temp schema so external sources (local files and in-process generators)
// can be queried and joined like ordinary tables without an import/export
// round trip. Nothing in this package registers anything unless the
// process was started with --modules. A network/host/interop module set
// was designed and prototyped as a follow-on and deliberately not shipped —
// see internal-docs/VTABS.md's "## Excluded" section for the security
// rationale; do not add such a module here without a real per-caller
// authorization model.
package vtab

import (
	"sort"

	"github.com/0funct0ry/squad/internal/seed"
	vtabdriver "modernc.org/sqlite/vtab"
)

// ModuleDef describes one registered virtual table module: its argument
// schema (reusing seed.OptionField so the Modules tab and .mount can render
// the same form/flag derivation the Seed feature already has), and the
// capabilities that gate it beyond the base --modules flag.
type ModuleDef struct {
	Name         string
	Group        string
	Description  string
	Args         []seed.OptionField
	RequiresNet  bool
	RequiresFile bool
	Writable     bool
	factory      func() vtabdriver.Module
}

// ModuleMeta is the JSON-serializable projection of a ModuleDef, used by the
// GET /api/modules catalog field and by `.modules`.
type ModuleMeta struct {
	Name         string             `json:"name"`
	Group        string             `json:"group"`
	Description  string             `json:"description"`
	Args         []seed.OptionField `json:"args"`
	RequiresNet  bool               `json:"requiresNet"`
	RequiresFile bool               `json:"requiresFile"`
	Writable     bool               `json:"writable"`
}

var registry = buildRegistry()

func buildRegistry() map[string]ModuleDef {
	defs := []ModuleDef{
		csvModuleDef(),
		jsonlModuleDef(),
		parquetModuleDef(),
		xlsxModuleDef(),
		yamlModuleDef(),
		xmlModuleDef(),
		seriesModuleDef(),
		calendarModuleDef(),
		fakeModuleDef(),
		tokensModuleDef(),
	}
	m := make(map[string]ModuleDef, len(defs))
	for _, d := range defs {
		m[d.Name] = d
	}
	return m
}

// Get returns the definition for a registered module name.
func Get(name string) (ModuleDef, bool) {
	d, ok := registry[name]
	return d, ok
}

// Exists reports whether name is a registered module.
func Exists(name string) bool {
	_, ok := registry[name]
	return ok
}

// Catalog returns every registered module's metadata, sorted by name — the
// one function feeding both the CLI's `.modules` and the web `/api/modules`
// endpoint, mirroring seed.GeneratorCatalog().
func Catalog() []ModuleMeta {
	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	sort.Strings(names)

	out := make([]ModuleMeta, 0, len(names))
	for _, name := range names {
		d := registry[name]
		out = append(out, ModuleMeta{
			Name:         d.Name,
			Group:        d.Group,
			Description:  d.Description,
			Args:         d.Args,
			RequiresNet:  d.RequiresNet,
			RequiresFile: d.RequiresFile,
			Writable:     d.Writable,
		})
	}
	return out
}
