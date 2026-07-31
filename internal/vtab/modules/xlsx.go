package modules

import (
	"fmt"
	"regexp"
	"strconv"

	"github.com/xuri/excelize/v2"
	vtabdriver "modernc.org/sqlite/vtab"
)

// XLSXModule implements the `xlsx` module (VTABS.md #4): file=, sheet=
// (default: first sheet), range= (optional A1-style bound), header= (bool,
// default true).
type XLSXModule struct{}

func (m *XLSXModule) Create(ctx vtabdriver.Context, args []string) (vtabdriver.Table, error) {
	return m.connect(ctx, UsingArgs(args))
}

func (m *XLSXModule) Connect(ctx vtabdriver.Context, args []string) (vtabdriver.Table, error) {
	return m.connect(ctx, UsingArgs(args))
}

var a1RangeRe = regexp.MustCompile(`^([A-Za-z]+)(\d+):([A-Za-z]+)(\d+)$`)

func (m *XLSXModule) connect(ctx vtabdriver.Context, rawArgs []string) (vtabdriver.Table, error) {
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
	rangeArg := a.Get("range", "")

	path, err := ResolvePath(file)
	if err != nil {
		return nil, err
	}
	f, err := excelize.OpenFile(path)
	if err != nil {
		return nil, fmt.Errorf("xlsx module: %w", err)
	}
	defer f.Close()

	sheet := a.Get("sheet", "")
	if sheet == "" {
		sheets := f.GetSheetList()
		if len(sheets) == 0 {
			return nil, fmt.Errorf("xlsx module: file %q has no sheets", file)
		}
		sheet = sheets[0]
	}

	all, err := f.GetRows(sheet)
	if err != nil {
		return nil, fmt.Errorf("xlsx module: %w", err)
	}

	records := all
	if rangeArg != "" {
		records, err = sliceA1Range(all, rangeArg)
		if err != nil {
			return nil, fmt.Errorf("xlsx module: %w", err)
		}
	}

	var cols []string
	var dataRows [][]string
	if header {
		if len(records) == 0 {
			return nil, fmt.Errorf("xlsx module: sheet %q has no header row", sheet)
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

// sliceA1Range slices an already-fetched sheet (0-indexed rows/cols) down to
// an A1-style bound like "A1:C40".
func sliceA1Range(all [][]string, rangeArg string) ([][]string, error) {
	m := a1RangeRe.FindStringSubmatch(rangeArg)
	if m == nil {
		return nil, fmt.Errorf("invalid range %q, expected e.g. A1:C40", rangeArg)
	}
	c1, r1, err := excelize.CellNameToCoordinates(m[1] + m[2])
	if err != nil {
		return nil, err
	}
	c2, r2, err := excelize.CellNameToCoordinates(m[3] + m[4])
	if err != nil {
		return nil, err
	}
	if r2 < len(all) {
		all = all[:r2]
	}
	if r1-1 > len(all) {
		return nil, nil
	}
	all = all[r1-1:]

	out := make([][]string, len(all))
	for i, row := range all {
		lo, hi := c1-1, c2
		if hi > len(row) {
			hi = len(row)
		}
		if lo > len(row) {
			lo = len(row)
		}
		if lo > hi {
			lo = hi
		}
		out[i] = row[lo:hi]
	}
	return out, nil
}
