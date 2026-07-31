package modules

import (
	"encoding/xml"
	"fmt"
	"os"
	"sort"
	"strings"

	vtabdriver "modernc.org/sqlite/vtab"
)

// XMLModule implements the `xml` module (VTABS.md #6): file=, path=
// (required, e.g. "/catalog/product"), attributes= (bool, default true —
// expose attributes as columns alongside child elements).
type XMLModule struct{}

func (m *XMLModule) Create(ctx vtabdriver.Context, args []string) (vtabdriver.Table, error) {
	return m.connect(ctx, UsingArgs(args))
}

func (m *XMLModule) Connect(ctx vtabdriver.Context, args []string) (vtabdriver.Table, error) {
	return m.connect(ctx, UsingArgs(args))
}

// xmlNode is a generic, order-preserving decode target for one element:
// its attributes, and its immediate child elements (by name, text content).
type xmlNode struct {
	XMLName  xml.Name
	Attrs    []xml.Attr `xml:",any,attr"`
	Children []xmlNode  `xml:",any"`
	Text     string     `xml:",chardata"`
}

func (m *XMLModule) connect(ctx vtabdriver.Context, rawArgs []string) (vtabdriver.Table, error) {
	a, err := ParseArgs(rawArgs)
	if err != nil {
		return nil, err
	}
	file, err := a.GetRequired("file")
	if err != nil {
		return nil, err
	}
	elemPath, err := a.GetRequired("path")
	if err != nil {
		return nil, err
	}
	withAttrs, err := a.GetBool("attributes", true)
	if err != nil {
		return nil, err
	}

	path, err := ResolvePath(file)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("xml module: %w", err)
	}
	defer f.Close()

	segments := strings.Split(strings.Trim(elemPath, "/"), "/")
	if len(segments) == 0 || segments[0] == "" {
		return nil, fmt.Errorf("xml module: path must name at least one element, got %q", elemPath)
	}

	var root xmlNode
	dec := xml.NewDecoder(f)
	if err := dec.Decode(&root); err != nil {
		return nil, fmt.Errorf("xml module: %w", err)
	}

	matches := findByPath(&root, segments)

	seen := map[string]bool{}
	var cols []string
	addCol := func(name string) {
		if !seen[name] {
			seen[name] = true
			cols = append(cols, name)
		}
	}
	type rowFields struct {
		attrs    map[string]string
		children map[string]string
	}
	rowData := make([]rowFields, 0, len(matches))
	for _, node := range matches {
		rf := rowFields{attrs: map[string]string{}, children: map[string]string{}}
		if withAttrs {
			for _, at := range node.Attrs {
				rf.attrs[at.Name.Local] = at.Value
				addCol(at.Name.Local)
			}
		}
		for _, ch := range node.Children {
			rf.children[ch.XMLName.Local] = strings.TrimSpace(ch.Text)
			addCol(ch.XMLName.Local)
		}
		rowData = append(rowData, rf)
	}
	sort.Strings(cols)

	if err := DeclareColumns(ctx, cols, nil); err != nil {
		return nil, err
	}

	rows := make([][]vtabdriver.Value, 0, len(rowData))
	for _, rf := range rowData {
		row := make([]vtabdriver.Value, len(cols))
		for i, c := range cols {
			if v, ok := rf.attrs[c]; ok {
				row[i] = sniffValue(v)
			} else if v, ok := rf.children[c]; ok {
				row[i] = sniffValue(v)
			}
		}
		rows = append(rows, row)
	}

	return NewSimpleTable(rows), nil
}

// findByPath walks segments (element-name path components, matching by
// local name) starting from root, returning every node matching the full
// path.
func findByPath(root *xmlNode, segments []string) []*xmlNode {
	current := []*xmlNode{root}
	// The first segment must match the document root's own element name.
	if root.XMLName.Local != segments[0] {
		return nil
	}
	current = []*xmlNode{root}
	for _, seg := range segments[1:] {
		var next []*xmlNode
		for _, node := range current {
			for i := range node.Children {
				if node.Children[i].XMLName.Local == seg {
					next = append(next, &node.Children[i])
				}
			}
		}
		current = next
	}
	if len(segments) == 1 {
		return current
	}
	return current
}
