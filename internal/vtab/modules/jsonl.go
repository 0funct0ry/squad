package modules

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	vtabdriver "modernc.org/sqlite/vtab"
)

// JSONLModule implements the `jsonl` module (VTABS.md #2): file=, root=
// (JSON Pointer, default "" — document root), columns= (optional explicit
// key list; otherwise inferred by unioning keys across all records).
type JSONLModule struct{}

func (m *JSONLModule) Create(ctx vtabdriver.Context, args []string) (vtabdriver.Table, error) {
	return m.connect(ctx, UsingArgs(args))
}

func (m *JSONLModule) Connect(ctx vtabdriver.Context, args []string) (vtabdriver.Table, error) {
	return m.connect(ctx, UsingArgs(args))
}

func (m *JSONLModule) connect(ctx vtabdriver.Context, rawArgs []string) (vtabdriver.Table, error) {
	a, err := ParseArgs(rawArgs)
	if err != nil {
		return nil, err
	}
	file, err := a.GetRequired("file")
	if err != nil {
		return nil, err
	}
	root := a.Get("root", "")
	explicitColumns := parseColumnsList(a.Get("columns", ""))

	path, err := ResolvePath(file)
	if err != nil {
		return nil, err
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("jsonl module: %w", err)
	}

	records, err := loadJSONRecords(content, root)
	if err != nil {
		return nil, err
	}

	cols := explicitColumns
	if len(cols) == 0 {
		cols = unionKeys(records)
	}

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

// loadJSONRecords parses content as either a single JSON document (array or
// object, navigated to root) or, when whole-document parsing fails, as
// NDJSON (one JSON value per line; root is not applicable there).
func loadJSONRecords(content []byte, root string) ([]any, error) {
	var doc any
	if err := json.Unmarshal(content, &doc); err == nil {
		navigated, err := jsonPointerNav(doc, root)
		if err != nil {
			return nil, fmt.Errorf("jsonl module: %w", err)
		}
		switch v := navigated.(type) {
		case []any:
			return v, nil
		case map[string]any:
			return []any{v}, nil
		default:
			return nil, fmt.Errorf("jsonl module: root %q does not select an array or object", root)
		}
	}

	var records []any
	scanner := bufio.NewScanner(bytes.NewReader(content))
	scanner.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		var v any
		if err := json.Unmarshal(line, &v); err != nil {
			return nil, fmt.Errorf("jsonl module: invalid NDJSON line: %w", err)
		}
		records = append(records, v)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("jsonl module: %w", err)
	}
	return records, nil
}

func unionKeys(records []any) []string {
	seen := map[string]bool{}
	var cols []string
	for _, rec := range records {
		obj, ok := rec.(map[string]any)
		if !ok {
			continue
		}
		for k := range obj {
			if !seen[k] {
				seen[k] = true
				cols = append(cols, k)
			}
		}
	}
	sort.Strings(cols)
	return cols
}

func parseColumnsList(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// jsonScalar converts a decoded JSON value into a driver value: scalars pass
// through (numbers as float64 per encoding/json), objects/arrays are
// re-serialized to JSON text so json_extract still works on them.
func jsonScalar(v any) vtabdriver.Value {
	switch t := v.(type) {
	case nil:
		return nil
	case string:
		return t
	case float64:
		return t
	case bool:
		return t
	default:
		b, err := json.Marshal(t)
		if err != nil {
			return nil
		}
		return string(b)
	}
}
