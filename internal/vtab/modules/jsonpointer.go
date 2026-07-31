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
		return nil, fmt.Errorf("json pointer must start with '/': %q", pointer)
	}
	cur := doc
	for _, tok := range strings.Split(pointer, "/")[1:] {
		tok = strings.ReplaceAll(tok, "~1", "/")
		tok = strings.ReplaceAll(tok, "~0", "~")
		switch v := cur.(type) {
		case map[string]any:
			next, ok := v[tok]
			if !ok {
				return nil, fmt.Errorf("json pointer %q: no key %q", pointer, tok)
			}
			cur = next
		case []any:
			idx, err := strconv.Atoi(tok)
			if err != nil || idx < 0 || idx >= len(v) {
				return nil, fmt.Errorf("json pointer %q: invalid index %q", pointer, tok)
			}
			cur = v[idx]
		default:
			return nil, fmt.Errorf("json pointer %q: cannot descend into scalar at %q", pointer, tok)
		}
	}
	return cur, nil
}
