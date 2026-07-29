package export

import (
	"bufio"
	"bytes"
	"testing"

	"google.golang.org/protobuf/encoding/protodelim"
	"google.golang.org/protobuf/types/known/structpb"
)

func decodeProtobufStream(t *testing.T, data []byte) []map[string]any {
	t.Helper()
	r := bufio.NewReader(bytes.NewReader(data))
	var out []map[string]any
	for {
		var st structpb.Struct
		if err := protodelim.UnmarshalFrom(r, &st); err != nil {
			break
		}
		out = append(out, st.AsMap())
	}
	return out
}

func TestExportProtobuf(t *testing.T) {
	columns := []string{"id", "name", "null_col"}
	rows := [][]any{
		{int64(1), "Alice", nil},
		{int64(2), "Bob", "note"},
	}

	var buf bytes.Buffer
	if err := ExportProtobuf(columns, newRowSource(rows), &buf); err != nil {
		t.Fatalf("ExportProtobuf failed: %v", err)
	}

	docs := decodeProtobufStream(t, buf.Bytes())
	if len(docs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(docs))
	}
	if docs[0]["name"] != "Alice" {
		t.Errorf("unexpected doc 0 name: %v", docs[0]["name"])
	}
	if docs[0]["id"] != float64(1) {
		t.Errorf("unexpected doc 0 id: %v", docs[0]["id"])
	}
	if docs[0]["null_col"] != nil {
		t.Errorf("expected null_col to decode as nil, got %v", docs[0]["null_col"])
	}
	if docs[1]["null_col"] != "note" {
		t.Errorf("unexpected doc 1 null_col: %v", docs[1]["null_col"])
	}
}

func TestExportProtobufColumnSubset(t *testing.T) {
	columns := []string{"name"}
	rows := [][]any{{"Alice"}, {"Bob"}}

	var buf bytes.Buffer
	if err := ExportProtobuf(columns, newRowSource(rows), &buf); err != nil {
		t.Fatalf("ExportProtobuf failed: %v", err)
	}
	docs := decodeProtobufStream(t, buf.Bytes())
	if len(docs) != 2 || len(docs[0]) != 1 {
		t.Fatalf("unexpected structure: %+v", docs)
	}
	if docs[0]["name"] != "Alice" {
		t.Errorf("unexpected value: %v", docs[0]["name"])
	}
}
