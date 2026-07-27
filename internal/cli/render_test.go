package cli

import (
	"bytes"
	"strings"
	"testing"
)

func newTestState(mode OutputMode, headers bool) (*State, *bytes.Buffer) {
	var buf bytes.Buffer
	s := &State{Mode: mode, Headers: headers, Out: &buf, NullValue: ""}
	return s, &buf
}

func TestRenderCSV(t *testing.T) {
	s, buf := newTestState(ModeCSV, true)
	err := s.Render([]string{"id", "name"}, [][]any{{int64(1), "a,b"}, {int64(2), nil}})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	got := buf.String()
	if !strings.Contains(got, `"a,b"`) {
		t.Errorf("expected quoted comma field, got %q", got)
	}
	if !strings.HasPrefix(got, "id,name\n") {
		t.Errorf("expected header line, got %q", got)
	}
}

func TestRenderList(t *testing.T) {
	s, buf := newTestState(ModeList, false)
	if err := s.Render([]string{"id", "name"}, [][]any{{int64(1), "x"}}); err != nil {
		t.Fatalf("Render: %v", err)
	}
	if got := buf.String(); got != "1|x\n" {
		t.Errorf("got %q", got)
	}
}

func TestRenderJSON(t *testing.T) {
	s, buf := newTestState(ModeJSON, true)
	if err := s.Render([]string{"id"}, [][]any{{int64(1)}}); err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(buf.String(), `"id": "1"`) {
		t.Errorf("got %q", buf.String())
	}
}

func TestRenderNullValue(t *testing.T) {
	s, buf := newTestState(ModeList, false)
	s.NullValue = "<NULL>"
	if err := s.Render([]string{"v"}, [][]any{{nil}}); err != nil {
		t.Fatalf("Render: %v", err)
	}
	if got := buf.String(); got != "<NULL>\n" {
		t.Errorf("got %q", got)
	}
}

func TestSQLLiteral(t *testing.T) {
	cases := []struct {
		in   any
		want string
	}{
		{nil, "NULL"},
		{"it's", "'it''s'"},
		{int64(42), "42"},
		{3.5, "3.5"},
		{true, "1"},
		{false, "0"},
		{[]byte{0xde, 0xad}, "X'dead'"},
	}
	for _, c := range cases {
		got, err := sqlLiteral(c.in)
		if err != nil {
			t.Fatalf("sqlLiteral(%v): %v", c.in, err)
		}
		if got != c.want {
			t.Errorf("sqlLiteral(%v) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestTemplateRawValue(t *testing.T) {
	cases := []struct {
		in   any
		want string
	}{
		{nil, "NULL"},
		{"it's", "it''s"},
		{int64(42), "42"},
		{3.5, "3.5"},
		{true, "1"},
		{false, "0"},
		{[]byte{0xde, 0xad}, "X'dead'"},
	}
	for _, c := range cases {
		got, err := templateRawValue(c.in)
		if err != nil {
			t.Fatalf("templateRawValue(%v): %v", c.in, err)
		}
		if got != c.want {
			t.Errorf("templateRawValue(%v) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestIsValidMode(t *testing.T) {
	if !IsValidMode("column") {
		t.Error("expected column to be valid")
	}
	if IsValidMode("bogus") {
		t.Error("expected bogus to be invalid")
	}
}
