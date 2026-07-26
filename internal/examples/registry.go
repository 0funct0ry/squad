package examples

import (
	"sort"

	"github.com/0funct0ry/squad/internal/examples/placeholder1"
	"github.com/0funct0ry/squad/internal/examples/placeholder2"
)

// registry is the manually maintained list of canned example data models.
// Adding a new one is: create a subpackage with a Schema constant, then add
// one line here. No other code changes are required.
var registry = []Example{
	{Slug: "placeholder1", Name: "Placeholder: Blog", Description: "Minimal blog schema (authors, posts, comments) — scaffolding only", Schema: placeholder1.Schema},
	{Slug: "placeholder2", Name: "Placeholder: Inventory", Description: "Minimal inventory schema (products, warehouses, stock) — scaffolding only", Schema: placeholder2.Schema},
}

// All returns every registered example, sorted by Name.
func All() []Example {
	out := make([]Example, len(registry))
	copy(out, registry)
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// ByName returns the example with the given slug, if any.
func ByName(slug string) (Example, bool) {
	for _, e := range registry {
		if e.Slug == slug {
			return e, true
		}
	}
	return Example{}, false
}
