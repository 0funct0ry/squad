// Package examples provides the canned example data-model library exposed
// behind the --examples flag: a registry of DDL scripts the user can insert
// into the SQL Editor and run against the open database.
package examples

// Example is one canned data model: a named, described DDL script.
type Example struct {
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Schema      string `json:"schema"`
}

// Meta is the JSON-serializable projection of an Example used by the list
// endpoint, omitting the (potentially large) Schema body.
type Meta struct {
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	Description string `json:"description"`
}
