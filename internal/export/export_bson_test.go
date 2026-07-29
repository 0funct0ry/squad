package export

import (
	"bytes"
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// decodeBSONStream reads consecutive top-level BSON documents from data,
// mirroring how a consumer of ExportBSON's output would: each document's
// own leading little-endian int32 length prefix (which includes itself)
// says how many bytes to consume before the next document starts.
func decodeBSONStream(t *testing.T, data []byte) []bson.M {
	t.Helper()
	var docs []bson.M
	for len(data) > 0 {
		if len(data) < 4 {
			t.Fatalf("truncated BSON stream: %d trailing bytes", len(data))
		}
		length := int32(data[0]) | int32(data[1])<<8 | int32(data[2])<<16 | int32(data[3])<<24
		if int(length) > len(data) {
			t.Fatalf("BSON document length %d exceeds remaining %d bytes", length, len(data))
		}

		var doc bson.M
		if err := bson.Unmarshal(data[:length], &doc); err != nil {
			t.Fatalf("failed to decode BSON document: %v", err)
		}
		docs = append(docs, doc)
		data = data[length:]
	}
	return docs
}

func TestExportBSON(t *testing.T) {
	columns := []string{"id", "name", "null_col"}
	rows := [][]any{
		{int64(1), "Alice", nil},
		{int64(2), "Bob", "note"},
	}

	var buf bytes.Buffer
	if err := ExportBSON(columns, newRowSource(rows), &buf); err != nil {
		t.Fatalf("ExportBSON failed: %v", err)
	}

	docs := decodeBSONStream(t, buf.Bytes())
	if len(docs) != 2 {
		t.Fatalf("expected 2 BSON documents, got %d", len(docs))
	}
	if docs[0]["name"] != "Alice" {
		t.Errorf("unexpected doc 0 name: %v", docs[0]["name"])
	}
	if docs[0]["null_col"] != nil {
		t.Errorf("expected doc 0 null_col to be nil, got %v", docs[0]["null_col"])
	}
	if docs[1]["null_col"] != "note" {
		t.Errorf("unexpected doc 1 null_col: %v", docs[1]["null_col"])
	}
}

func TestExportBSONColumnSubset(t *testing.T) {
	columns := []string{"name"}
	rows := [][]any{{"Alice"}, {"Bob"}}

	var buf bytes.Buffer
	if err := ExportBSON(columns, newRowSource(rows), &buf); err != nil {
		t.Fatalf("ExportBSON failed: %v", err)
	}
	docs := decodeBSONStream(t, buf.Bytes())
	if len(docs) != 2 || len(docs[0]) != 1 {
		t.Fatalf("unexpected structure: %+v", docs)
	}
	if docs[0]["name"] != "Alice" {
		t.Errorf("unexpected value: %v", docs[0]["name"])
	}
}
