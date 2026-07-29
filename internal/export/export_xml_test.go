package export

import (
	"bytes"
	"encoding/xml"
	"strings"
	"testing"
)

// genericXMLDoc decodes an arbitrary root element containing arbitrary
// row elements, each with arbitrary column child elements, without
// depending on their exact tag names - used to assert on structure/content
// regardless of the configured tag-naming options.
type genericXMLDoc struct {
	XMLName xml.Name
	Rows    []genericXMLRow `xml:",any"`
}
type genericXMLRow struct {
	XMLName xml.Name
	Cols    []genericXMLCol `xml:",any"`
}
type genericXMLCol struct {
	XMLName xml.Name
	Value   string `xml:",chardata"`
}

func decodeGenericXML(t *testing.T, data []byte) genericXMLDoc {
	t.Helper()
	var doc genericXMLDoc
	if err := xml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("failed to decode XML: %v\n%s", err, string(data))
	}
	return doc
}

func TestExportXMLDefaults(t *testing.T) {
	columns := []string{"order_id", "customer_id", "notes"}
	rows := [][]any{
		{int64(1), int64(4716), nil},
		{int64(2), int64(1429), "gift wrap"},
	}

	var buf bytes.Buffer
	if err := ExportXML(columns, newRowSource(rows), &buf, DefaultXMLOptions("orders")); err != nil {
		t.Fatalf("ExportXML failed: %v", err)
	}

	out := buf.String()
	if !strings.HasPrefix(out, xml.Header) {
		t.Errorf("expected output to start with the XML declaration, got:\n%s", out)
	}
	if !strings.Contains(out, "<orders>") || !strings.Contains(out, "</orders>") {
		t.Errorf("expected root element <orders>, got:\n%s", out)
	}
	if !strings.Contains(out, "<order>") {
		t.Errorf("expected singularized row element <order>, got:\n%s", out)
	}
	if !strings.Contains(out, "<order_id>1</order_id>") {
		t.Errorf("expected snake_case column element <order_id>, got:\n%s", out)
	}
	if !strings.Contains(out, "<notes></notes>") {
		t.Errorf("expected NULL to render as an empty element by default, got:\n%s", out)
	}
	// Pretty-printed by default: real indentation/newlines present.
	if !strings.Contains(out, "\n    <order>") {
		t.Errorf("expected 4-space-indented pretty output, got:\n%s", out)
	}

	doc := decodeGenericXML(t, buf.Bytes())
	if doc.XMLName.Local != "orders" {
		t.Errorf("root element = %q, want orders", doc.XMLName.Local)
	}
	if len(doc.Rows) != 2 || doc.Rows[0].XMLName.Local != "order" {
		t.Fatalf("unexpected rows: %+v", doc.Rows)
	}
}

func TestExportXMLCaseStyles(t *testing.T) {
	columns := []string{"order_id", "customer_id"}
	rows := [][]any{{int64(1), int64(2)}}

	cases := []struct {
		style      XMLCaseStyle
		wantRoot   string
		wantRow    string
		wantColTag string
	}{
		{XMLCaseSnake, "orders", "order", "order_id"},
		{XMLCaseCamel, "orders", "order", "orderId"},
		{XMLCasePascal, "Orders", "Order", "OrderId"},
		{XMLCaseKebab, "orders", "order", "order-id"},
	}

	for _, tc := range cases {
		t.Run(string(tc.style), func(t *testing.T) {
			opts := DefaultXMLOptions("orders")
			opts.CaseStyle = tc.style

			var buf bytes.Buffer
			if err := ExportXML(columns, newRowSource(rows), &buf, opts); err != nil {
				t.Fatalf("ExportXML failed: %v", err)
			}
			doc := decodeGenericXML(t, buf.Bytes())
			if doc.XMLName.Local != tc.wantRoot {
				t.Errorf("root = %q, want %q", doc.XMLName.Local, tc.wantRoot)
			}
			if len(doc.Rows) != 1 || doc.Rows[0].XMLName.Local != tc.wantRow {
				t.Fatalf("row = %+v, want %q", doc.Rows, tc.wantRow)
			}
			if len(doc.Rows[0].Cols) == 0 || doc.Rows[0].Cols[0].XMLName.Local != tc.wantColTag {
				t.Errorf("first col tag = %+v, want %q", doc.Rows[0].Cols, tc.wantColTag)
			}
		})
	}
}

func TestExportXMLCustomTags(t *testing.T) {
	columns := []string{"id"}
	rows := [][]any{{int64(1)}}

	opts := DefaultXMLOptions("orders")
	opts.RootTag = "results"
	opts.RowTag = "item"

	var buf bytes.Buffer
	if err := ExportXML(columns, newRowSource(rows), &buf, opts); err != nil {
		t.Fatalf("ExportXML failed: %v", err)
	}
	doc := decodeGenericXML(t, buf.Bytes())
	if doc.XMLName.Local != "results" {
		t.Errorf("root = %q, want results", doc.XMLName.Local)
	}
	if len(doc.Rows) != 1 || doc.Rows[0].XMLName.Local != "item" {
		t.Fatalf("row = %+v, want item", doc.Rows)
	}
}

func TestExportXMLNullHandlingOmit(t *testing.T) {
	columns := []string{"id", "notes"}
	rows := [][]any{{int64(1), nil}}

	opts := DefaultXMLOptions("orders")
	opts.NullHandling = XMLNullOmit

	var buf bytes.Buffer
	if err := ExportXML(columns, newRowSource(rows), &buf, opts); err != nil {
		t.Fatalf("ExportXML failed: %v", err)
	}
	doc := decodeGenericXML(t, buf.Bytes())
	if len(doc.Rows) != 1 || len(doc.Rows[0].Cols) != 1 || doc.Rows[0].Cols[0].XMLName.Local != "id" {
		t.Errorf("expected only the non-null column present, got: %+v", doc.Rows[0].Cols)
	}
}

func TestExportXMLCompactNoDeclaration(t *testing.T) {
	columns := []string{"id"}
	rows := [][]any{{int64(1)}}

	opts := DefaultXMLOptions("orders")
	opts.Pretty = false
	opts.IncludeDeclaration = false

	var buf bytes.Buffer
	if err := ExportXML(columns, newRowSource(rows), &buf, opts); err != nil {
		t.Fatalf("ExportXML failed: %v", err)
	}
	out := buf.String()
	if strings.Contains(out, xml.Header) {
		t.Errorf("expected no XML declaration, got:\n%s", out)
	}
	if strings.Contains(out, "\n") {
		t.Errorf("expected compact (no-newline) output, got:\n%s", out)
	}
}

func TestExportXMLColumnSubset(t *testing.T) {
	columns := []string{"name"}
	rows := [][]any{{"Alice"}, {"Bob"}}

	var buf bytes.Buffer
	if err := ExportXML(columns, newRowSource(rows), &buf, DefaultXMLOptions("users")); err != nil {
		t.Fatalf("ExportXML failed: %v", err)
	}
	doc := decodeGenericXML(t, buf.Bytes())
	if len(doc.Rows) != 2 || len(doc.Rows[0].Cols) != 1 {
		t.Fatalf("unexpected structure: %+v", doc)
	}
	if doc.Rows[0].Cols[0].XMLName.Local != "name" || doc.Rows[0].Cols[0].Value != "Alice" {
		t.Errorf("unexpected col: %+v", doc.Rows[0].Cols[0])
	}
}

func TestSingularize(t *testing.T) {
	cases := map[string]string{
		"orders":     "order",
		"categories": "category",
		"boxes":      "box",
		"glasses":    "glass",
		"data":       "data",
		"status":     "statu", // best-effort heuristic; not linguistically perfect
	}
	for in, want := range cases {
		if got := singularize(in); got != want {
			t.Errorf("singularize(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSanitizeXMLName(t *testing.T) {
	cases := map[string]string{
		"order_id":  "order_id",
		"2024_data": "_2024_data",
		"":          "_",
		"a b":       "a_b",
	}
	for in, want := range cases {
		if got := sanitizeXMLName(in); got != want {
			t.Errorf("sanitizeXMLName(%q) = %q, want %q", in, got, want)
		}
	}
}
