package modules

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	vtabdriver "modernc.org/sqlite/vtab"
)

// CalendarModule implements the `calendar` module (VTABS.md #8): start=
// (required, YYYY-MM-DD), stop= (required), step= (default "1 day").
// Columns: day, dow, dow_name, iso_week, month, quarter, year, is_weekend.
type CalendarModule struct{}

func (m *CalendarModule) Create(ctx vtabdriver.Context, args []string) (vtabdriver.Table, error) {
	return m.connect(ctx, UsingArgs(args))
}

func (m *CalendarModule) Connect(ctx vtabdriver.Context, args []string) (vtabdriver.Table, error) {
	return m.connect(ctx, UsingArgs(args))
}

var calendarCols = []string{"day", "dow", "dow_name", "iso_week", "month", "quarter", "year", "is_weekend"}
var calendarTypes = []string{"TEXT", "INTEGER", "TEXT", "INTEGER", "INTEGER", "INTEGER", "INTEGER", "INTEGER"}

func (m *CalendarModule) connect(ctx vtabdriver.Context, rawArgs []string) (vtabdriver.Table, error) {
	a, err := ParseArgs(rawArgs)
	if err != nil {
		return nil, err
	}
	startStr, err := a.GetRequired("start")
	if err != nil {
		return nil, err
	}
	stopStr, err := a.GetRequired("stop")
	if err != nil {
		return nil, err
	}
	stepStr := a.Get("step", "1 day")

	start, err := time.Parse("2006-01-02", startStr)
	if err != nil {
		return nil, fmt.Errorf("calendar module: start must be YYYY-MM-DD: %w", err)
	}
	stop, err := time.Parse("2006-01-02", stopStr)
	if err != nil {
		return nil, fmt.Errorf("calendar module: stop must be YYYY-MM-DD: %w", err)
	}
	stepDays, err := parseStepDays(stepStr)
	if err != nil {
		return nil, fmt.Errorf("calendar module: %w", err)
	}

	if err := DeclareColumns(ctx, calendarCols, calendarTypes); err != nil {
		return nil, err
	}

	var rows [][]vtabdriver.Value
	for d := start; !d.After(stop); d = d.AddDate(0, 0, stepDays) {
		_, isoWeek := d.ISOWeek()
		dow := int(d.Weekday())
		isWeekend := 0
		if dow == 0 || dow == 6 {
			isWeekend = 1
		}
		quarter := (int(d.Month())-1)/3 + 1
		rows = append(rows, []vtabdriver.Value{
			d.Format("2006-01-02"),
			int64(dow),
			d.Weekday().String(),
			int64(isoWeek),
			int64(d.Month()),
			int64(quarter),
			int64(d.Year()),
			int64(isWeekend),
		})
	}

	return NewSimpleTable(rows), nil
}

// parseStepDays parses a "N day"/"N days" step spec (default unit: days) or
// a bare integer, returning the number of days to advance per row.
func parseStepDays(s string) (int, error) {
	s = strings.TrimSpace(s)
	fields := strings.Fields(s)
	switch len(fields) {
	case 1:
		n, err := strconv.Atoi(fields[0])
		if err != nil {
			return 0, fmt.Errorf("invalid step %q", s)
		}
		return n, nil
	case 2:
		n, err := strconv.Atoi(fields[0])
		if err != nil {
			return 0, fmt.Errorf("invalid step %q", s)
		}
		unit := strings.ToLower(strings.TrimSuffix(fields[1], "s"))
		if unit != "day" {
			return 0, fmt.Errorf("unsupported step unit %q (only day/days supported)", fields[1])
		}
		return n, nil
	default:
		return 0, fmt.Errorf("invalid step %q", s)
	}
}
