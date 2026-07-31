package modules

import (
	"fmt"

	vtabdriver "modernc.org/sqlite/vtab"
)

// SeriesModule implements the `series` module (VTABS.md #7): start= (default
// 0), stop= (required), step= (default 1; may be fractional). Column: value.
type SeriesModule struct{}

func (m *SeriesModule) Create(ctx vtabdriver.Context, args []string) (vtabdriver.Table, error) {
	return m.connect(ctx, UsingArgs(args))
}

func (m *SeriesModule) Connect(ctx vtabdriver.Context, args []string) (vtabdriver.Table, error) {
	return m.connect(ctx, UsingArgs(args))
}

func (m *SeriesModule) connect(ctx vtabdriver.Context, rawArgs []string) (vtabdriver.Table, error) {
	a, err := ParseArgs(rawArgs)
	if err != nil {
		return nil, err
	}
	start, err := a.GetFloat("start", 0)
	if err != nil {
		return nil, err
	}
	stopStr, err := a.GetRequired("stop")
	if err != nil {
		return nil, err
	}
	stop, err := a.GetFloat("stop", 0)
	if err != nil {
		return nil, err
	}
	_ = stopStr
	step, err := a.GetFloat("step", 1)
	if err != nil {
		return nil, err
	}
	if step == 0 {
		return nil, fmt.Errorf("series module: step must be non-zero")
	}

	if err := DeclareColumns(ctx, []string{"value"}, []string{"REAL"}); err != nil {
		return nil, err
	}

	var rows [][]vtabdriver.Value
	if step > 0 {
		for v := start; v < stop; v += step {
			rows = append(rows, []vtabdriver.Value{v})
		}
	} else {
		for v := start; v > stop; v += step {
			rows = append(rows, []vtabdriver.Value{v})
		}
	}

	return NewSimpleTable(rows), nil
}
