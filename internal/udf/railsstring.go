package udf

import (
	"database/sql/driver"
	"strings"

	humanize "github.com/dustin/go-humanize"
	"github.com/gertd/go-pluralize"
	"github.com/gosimple/slug"
	"github.com/iancoleman/strcase"
	"modernc.org/sqlite"
)

const catRailsString = "Ruby/Rails-like string functions"

var pluralizeClient = pluralize.NewClient()

func humanizeWords(str string) string {
	snake := strcase.ToSnake(str)
	words := strings.Split(snake, "_")
	joined := strings.Join(words, " ")
	if joined == "" {
		return ""
	}
	return strings.ToUpper(joined[:1]) + joined[1:]
}

func registerRailsString() error {
	if err := sqlite.RegisterDeterministicScalarFunction("PLURALIZE", 1,
		func(ctx *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			return pluralizeClient.Plural(argString(args[0])), nil
		}); err != nil {
		return err
	}
	add(Descriptor{Name: "PLURALIZE", Category: catRailsString, Signature: "PLURALIZE(str) -> str",
		Description: "Pluralizes an English word.", ExampleCall: `PLURALIZE('box')`, ExampleResult: "boxes",
		Deterministic: true})

	if err := sqlite.RegisterDeterministicScalarFunction("SINGULARIZE", 1,
		func(ctx *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			return pluralizeClient.Singular(argString(args[0])), nil
		}); err != nil {
		return err
	}
	add(Descriptor{Name: "SINGULARIZE", Category: catRailsString, Signature: "SINGULARIZE(str) -> str",
		Description: "Singularizes an English word.", ExampleCall: `SINGULARIZE('boxes')`, ExampleResult: "box",
		Deterministic: true})

	if err := sqlite.RegisterDeterministicScalarFunction("CAMELIZE", 1,
		func(ctx *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			return strcase.ToCamel(argString(args[0])), nil
		}); err != nil {
		return err
	}
	add(Descriptor{Name: "CAMELIZE", Category: catRailsString, Signature: "CAMELIZE(str) -> str",
		Description: "Converts str to PascalCase.", ExampleCall: `CAMELIZE('user_name')`, ExampleResult: "UserName",
		Deterministic: true})

	if err := sqlite.RegisterDeterministicScalarFunction("UNDERSCORE", 1,
		func(ctx *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			return strcase.ToSnake(argString(args[0])), nil
		}); err != nil {
		return err
	}
	add(Descriptor{Name: "UNDERSCORE", Category: catRailsString, Signature: "UNDERSCORE(str) -> str",
		Description: "Converts str to snake_case.", ExampleCall: `UNDERSCORE('UserName')`, ExampleResult: "user_name",
		Deterministic: true})

	if err := sqlite.RegisterDeterministicScalarFunction("DASHERIZE", 1,
		func(ctx *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			return strcase.ToKebab(argString(args[0])), nil
		}); err != nil {
		return err
	}
	add(Descriptor{Name: "DASHERIZE", Category: catRailsString, Signature: "DASHERIZE(str) -> str",
		Description: "Converts str to kebab-case.", ExampleCall: `DASHERIZE('user_name')`, ExampleResult: "user-name",
		Deterministic: true})

	if err := sqlite.RegisterDeterministicScalarFunction("HUMANIZE", 1,
		func(ctx *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			return humanizeWords(argString(args[0])), nil
		}); err != nil {
		return err
	}
	add(Descriptor{Name: "HUMANIZE", Category: catRailsString, Signature: "HUMANIZE(str) -> str",
		Description: "Converts a snake_case/camelCase identifier into a human-readable phrase.",
		ExampleCall: `HUMANIZE('user_name')`, ExampleResult: "User name", Deterministic: true})

	if err := sqlite.RegisterDeterministicScalarFunction("PARAMETERIZE", 1,
		func(ctx *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			return slug.Make(argString(args[0])), nil
		}); err != nil {
		return err
	}
	add(Descriptor{Name: "PARAMETERIZE", Category: catRailsString, Signature: "PARAMETERIZE(str) -> str",
		Description: "Rails-style URL slug.", ExampleCall: `PARAMETERIZE('My Blog Post!')`,
		ExampleResult: "my-blog-post", Deterministic: true})

	if err := sqlite.RegisterDeterministicScalarFunction("ORDINALIZE", 1,
		func(ctx *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			n, err := argInt(args[0])
			if err != nil {
				return nil, err
			}
			return humanize.Ordinal(int(n)), nil
		}); err != nil {
		return err
	}
	add(Descriptor{Name: "ORDINALIZE", Category: catRailsString, Signature: "ORDINALIZE(n) -> str",
		Description: "Converts n to its ordinal string form.", ExampleCall: `ORDINALIZE(3)`, ExampleResult: "3rd",
		Deterministic: true})

	return nil
}
