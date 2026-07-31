package modules

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/goccy/go-yaml"
	vtabdriver "modernc.org/sqlite/vtab"
)

// YAMLModule implements the `yaml` module (VTABS.md #5): file=, root=
// (path into the document, default ""), multidoc= (bool, default false —
// treat "---"-separated docs as rows).
type YAMLModule struct{}

func (m *YAMLModule) Create(ctx vtabdriver.Context, args []string) (vtabdriver.Table, error) {
	return m.connect(ctx, UsingArgs(args))
}

func (m *YAMLModule) Connect(ctx vtabdriver.Context, args []string) (vtabdriver.Table, error) {
	return m.connect(ctx, UsingArgs(args))
}

func (m *YAMLModule) connect(ctx vtabdriver.Context, rawArgs []string) (vtabdriver.Table, error) {
	a, err := ParseArgs(rawArgs)
	if err != nil {
		return nil, err
	}
	file, err := a.GetRequired("file")
	if err != nil {
		return nil, err
	}
	root := a.Get("root", "")
	multidoc, err := a.GetBool("multidoc", false)
	if err != nil {
		return nil, err
	}

	path, err := ResolvePath(file)
	if err != nil {
		return nil, err
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("yaml module: %w", err)
	}

	var records []any
	if multidoc {
		docs := strings.Split(string(content), "\n---")
		for _, d := range docs {
			var v any
			if err := yaml.Unmarshal([]byte(d), &v); err != nil {
				return nil, fmt.Errorf("yaml module: %w", err)
			}
			if v == nil {
				continue
			}
			records = append(records, jsonify(v))
		}
	} else {
		var doc any
		if err := yaml.Unmarshal(content, &doc); err != nil {
			return nil, fmt.Errorf("yaml module: %w", err)
		}
		navigated, err := jsonPointerNav(jsonify(doc), root)
		if err != nil {
			return nil, fmt.Errorf("yaml module: %w", err)
		}
		switch v := navigated.(type) {
		case []any:
			records = v
		case map[string]any:
			records = []any{v}
		default:
			return nil, fmt.Errorf("yaml module: root %q does not select a sequence or mapping", root)
		}
	}

	cols := unionKeys(records)
	if err := DeclareColumns(ctx, cols, nil); err != nil {
		return nil, err
	}

	rows := make([][]vtabdriver.Value, 0, len(records))
	for _, rec := range records {
		obj, _ := rec.(map[string]any)
		row := make([]vtabdriver.Value, len(cols))
		for i, c := range cols {
			row[i] = jsonScalar(obj[c])
		}
		rows = append(rows, row)
	}

	return NewSimpleTable(rows), nil
}

// jsonify normalizes a go-yaml decoded value (which may use
// map[string]interface{} already, but nested map keys can come back as
// map[interface{}]interface{} depending on decode path) into plain
// map[string]any/[]any/scalars by round-tripping through encoding/json.
func jsonify(v any) any {
	b, err := json.Marshal(v)
	if err != nil {
		return v
	}
	var out any
	if err := json.Unmarshal(b, &out); err != nil {
		return v
	}
	return out
}
