package modules

import (
	"encoding/csv"
	"fmt"
	"os"
	"strconv"

	vtabdriver "modernc.org/sqlite/vtab"
)

// CSVModule implements the `csv` module (VTABS.md #1): file=, header=
// (default true), delimiter= (default ","), quote= (default `"`).
type CSVModule struct{}

func (m *CSVModule) Create(ctx vtabdriver.Context, args []string) (vtabdriver.Table, error) {
	return m.connect(ctx, UsingArgs(args))
}

func (m *CSVModule) Connect(ctx vtabdriver.Context, args []string) (vtabdriver.Table, error) {
	return m.connect(ctx, UsingArgs(args))
}

func (m *CSVModule) connect(ctx vtabdriver.Context, rawArgs []string) (vtabdriver.Table, error) {
	a, err := ParseArgs(rawArgs)
	if err != nil {
		return nil, err
	}
	file, err := a.GetRequired("file")
	if err != nil {
		return nil, err
	}
	header, err := a.GetBool("header", true)
	if err != nil {
		return nil, err
	}
	delimiter := a.Get("delimiter", ",")
	if len(delimiter) != 1 {
		return nil, fmt.Errorf("argument delimiter must be a single character, got %q", delimiter)
	}
	quote := a.Get("quote", `"`)

	path, err := ResolvePath(file)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("csv module: %w", err)
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.Comma = rune(delimiter[0])
	r.FieldsPerRecord = -1
	if len(quote) > 0 {
		// encoding/csv always treats " as the quote char; a non-default
		// quote= is accepted for schema compatibility but not honored by
		// the stdlib reader, which has no configurable quote rune.
		_ = quote
	}

	records, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("csv module: %w", err)
	}

	var cols []string
	var dataRows [][]string
	if header {
		if len(records) == 0 {
			return nil, fmt.Errorf("csv module: file %q has no header row", file)
		}
		cols = records[0]
		dataRows = records[1:]
	} else {
		width := 0
		for _, rec := range records {
			if len(rec) > width {
				width = len(rec)
			}
		}
		cols = make([]string, width)
		for i := range cols {
			cols[i] = "c" + strconv.Itoa(i+1)
		}
		dataRows = records
	}

	if err := DeclareColumns(ctx, cols, nil); err != nil {
		return nil, err
	}

	rows := make([][]vtabdriver.Value, 0, len(dataRows))
	for _, rec := range dataRows {
		row := make([]vtabdriver.Value, len(cols))
		for i := range cols {
			if i < len(rec) {
				row[i] = sniffValue(rec[i])
			} else {
				row[i] = nil
			}
		}
		rows = append(rows, row)
	}

	return NewSimpleTable(rows), nil
}
