package modules

import (
	"fmt"
	"strconv"
	"strings"
)

// jsonPointerNav navigates an RFC 6901 JSON Pointer ("" or "/a/b/0") into a
// decoded JSON document (the result of json.Unmarshal into `any`).
func jsonPointerNav(doc any, pointer string) (any, error) {
	if pointer == "" {
		return doc, nil
	}
	if !strings.HasPrefix(pointer, "/") {
		return nil, fmt.Errorf("root %q must start with '/' (e.g. root=/%s)", pointer, pointer)
	}
	cur := doc
	for _, tok := range strings.Split(pointer, "/")[1:] {
		tok = strings.ReplaceAll(tok, "~1", "/")
		tok = strings.ReplaceAll(tok, "~0", "~")
		switch v := cur.(type) {
		case map[string]any:
			next, ok := v[tok]
			if !ok {
				return nil, fmt.Errorf("root %q: key %q not found", pointer, tok)
			}
			cur = next
		case []any:
			idx, err := strconv.Atoi(tok)
			if err != nil || idx < 0 || idx >= len(v) {
				return nil, fmt.Errorf("root %q: %q is not a valid index into a %d-element list", pointer, tok, len(v))
			}
			cur = v[idx]
		default:
			return nil, fmt.Errorf("root %q: %q is a plain value, not a list or object to descend into", pointer, tok)
		}
	}
	return cur, nil
}
