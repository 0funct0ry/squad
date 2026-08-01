package udf

import (
	"database/sql/driver"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"strings"

	"modernc.org/sqlite"
)

const catJSON = "JSON / structured data"

func jsonMerge(a, b map[string]any) map[string]any {
	out := make(map[string]any, len(a))
	for k, v := range a {
		out[k] = v
	}
	for k, v := range b {
		if bv, ok := v.(map[string]any); ok {
			if av, ok := out[k].(map[string]any); ok {
				out[k] = jsonMerge(av, bv)
				continue
			}
		}
		out[k] = v
	}
	return out
}

func jsonFlatten(prefix string, in map[string]any, out map[string]any) {
	for k, v := range in {
		key := k
		if prefix != "" {
			key = prefix + "." + k
		}
		if m, ok := v.(map[string]any); ok {
			jsonFlatten(key, m, out)
		} else {
			out[key] = v
		}
	}
}

func jsonPathExists(v any, path string) bool {
	path = strings.TrimPrefix(path, "$")
	path = strings.TrimPrefix(path, ".")
	if path == "" {
		return true
	}
	parts := strings.Split(path, ".")
	cur := v
	for _, p := range parts {
		m, ok := cur.(map[string]any)
		if !ok {
			return false
		}
		val, ok := m[p]
		if !ok {
			return false
		}
		cur = val
	}
	return true
}

func registerJSON() error {
	if err := sqlite.RegisterDeterministicScalarFunction("JSON_MERGE", 2,
		func(ctx *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			var a, b map[string]any
			if err := json.Unmarshal(argBytes(args[0]), &a); err != nil {
				return nil, fmt.Errorf("JSON_MERGE: invalid json1: %w", err)
			}
			if err := json.Unmarshal(argBytes(args[1]), &b); err != nil {
				return nil, fmt.Errorf("JSON_MERGE: invalid json2: %w", err)
			}
			out, err := json.Marshal(jsonMerge(a, b))
			if err != nil {
				return nil, err
			}
			return string(out), nil
		}); err != nil {
		return err
	}
	add(Descriptor{Name: "JSON_MERGE", Category: catJSON, Signature: "JSON_MERGE(json1, json2) -> json",
		Description:   "Deep-merges two JSON objects (json2 wins on key conflicts).",
		ExampleCall:   `JSON_MERGE('{"a":1,"c":{"x":1}}', '{"b":2,"c":{"y":2}}')`,
		ExampleResult: `{"a":1,"b":2,"c":{"x":1,"y":2}}`, Deterministic: true})

	if err := sqlite.RegisterDeterministicScalarFunction("JSON_FLATTEN", 1,
		func(ctx *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			var m map[string]any
			if err := json.Unmarshal(argBytes(args[0]), &m); err != nil {
				return nil, fmt.Errorf("JSON_FLATTEN: %w", err)
			}
			flat := map[string]any{}
			jsonFlatten("", m, flat)
			out, err := json.Marshal(flat)
			if err != nil {
				return nil, err
			}
			return string(out), nil
		}); err != nil {
		return err
	}
	add(Descriptor{Name: "JSON_FLATTEN", Category: catJSON, Signature: "JSON_FLATTEN(json) -> json",
		Description: "Flattens nested JSON into a single-level object with dotted keys.",
		ExampleCall: `JSON_FLATTEN('{"a":{"b":1,"c":2}}')`, ExampleResult: `{"a.b":1,"a.c":2}`,
		Deterministic: true})

	if err := sqlite.RegisterDeterministicScalarFunction("JSON_PATH_EXISTS", 2,
		func(ctx *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			var v any
			if err := json.Unmarshal(argBytes(args[0]), &v); err != nil {
				return nil, fmt.Errorf("JSON_PATH_EXISTS: %w", err)
			}
			return boolResult(jsonPathExists(v, argString(args[1]))), nil
		}); err != nil {
		return err
	}
	add(Descriptor{Name: "JSON_PATH_EXISTS", Category: catJSON, Signature: "JSON_PATH_EXISTS(json, path) -> bool",
		Description: "1 if the given JSON path exists in json, else 0.",
		ExampleCall: `JSON_PATH_EXISTS('{"a":{"b":1}}', '$.a.b')`, ExampleResult: "1",
		Deterministic: true})

	if err := sqlite.RegisterDeterministicScalarFunction("CSV_TO_JSON", 1,
		func(ctx *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			r := csv.NewReader(strings.NewReader(argString(args[0])))
			record, err := r.Read()
			if err != nil {
				return nil, fmt.Errorf("CSV_TO_JSON: %w", err)
			}
			out, err := json.Marshal(record)
			if err != nil {
				return nil, err
			}
			return string(out), nil
		}); err != nil {
		return err
	}
	add(Descriptor{Name: "CSV_TO_JSON", Category: catJSON, Signature: "CSV_TO_JSON(str) -> json",
		Description: "Converts a single CSV row string into a JSON array of fields.",
		ExampleCall: `CSV_TO_JSON('a,b,"c,d"')`, ExampleResult: `["a","b","c,d"]`,
		Deterministic: true})

	return nil
}
