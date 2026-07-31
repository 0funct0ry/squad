package modules

import (
	"fmt"
	"regexp"
	"strings"

	vtabdriver "modernc.org/sqlite/vtab"
)

// TokensModule implements the `tokens` module (VTABS.md #10): text=
// (required), delimiter= (default ",") or regex= (mutually exclusive),
// trim= (bool, default true). Columns: n (1-based position), token.
type TokensModule struct{}

func (m *TokensModule) Create(ctx vtabdriver.Context, args []string) (vtabdriver.Table, error) {
	return m.connect(ctx, UsingArgs(args))
}

func (m *TokensModule) Connect(ctx vtabdriver.Context, args []string) (vtabdriver.Table, error) {
	return m.connect(ctx, UsingArgs(args))
}

func (m *TokensModule) connect(ctx vtabdriver.Context, rawArgs []string) (vtabdriver.Table, error) {
	a, err := ParseArgs(rawArgs)
	if err != nil {
		return nil, err
	}
	text, err := a.GetRequired("text")
	if err != nil {
		return nil, err
	}
	_, hasDelim := a["delimiter"]
	_, hasRegex := a["regex"]
	if hasDelim && hasRegex {
		return nil, fmt.Errorf("tokens module: delimiter and regex are mutually exclusive")
	}
	trim, err := a.GetBool("trim", true)
	if err != nil {
		return nil, err
	}

	var parts []string
	if hasRegex {
		re, err := regexp.Compile(a["regex"])
		if err != nil {
			return nil, fmt.Errorf("tokens module: invalid regex: %w", err)
		}
		parts = re.Split(text, -1)
	} else {
		delimiter := a.Get("delimiter", ",")
		parts = strings.Split(text, delimiter)
	}

	if err := DeclareColumns(ctx, []string{"n", "token"}, []string{"INTEGER", "TEXT"}); err != nil {
		return nil, err
	}

	rows := make([][]vtabdriver.Value, 0, len(parts))
	for i, p := range parts {
		if trim {
			p = strings.TrimSpace(p)
		}
		rows = append(rows, []vtabdriver.Value{int64(i + 1), p})
	}

	return NewSimpleTable(rows), nil
}
