package udf

import (
	"database/sql/driver"
	"fmt"
	"strings"
	"time"

	humanize "github.com/dustin/go-humanize"
	"github.com/olebedev/when"
	"github.com/olebedev/when/rules/common"
	"github.com/olebedev/when/rules/en"
	"github.com/rickar/cal/v2"
	"github.com/rickar/cal/v2/gb"
	"github.com/rickar/cal/v2/us"
	"github.com/robfig/cron/v3"
	"modernc.org/sqlite"
)

const catAdvDatetime = "Advanced Date/Time"

var whenParser = func() *when.Parser {
	p := when.New(nil)
	p.Add(en.All...)
	p.Add(common.All...)
	return p
}()

var holidayCalendars = map[string][]*cal.Holiday{
	"US": us.Holidays,
	"GB": gb.Holidays,
}

func holidayFor(date time.Time, country string) (*cal.Holiday, error) {
	holidays, ok := holidayCalendars[strings.ToUpper(country)]
	if !ok {
		return nil, fmt.Errorf("unsupported country %q (supported: US, GB)", country)
	}
	c := &cal.Calendar{Holidays: holidays}
	_, observed, h := c.IsHoliday(date)
	if !observed {
		return nil, nil
	}
	return h, nil
}

func registerAdvDatetime() error {
	if err := sqlite.RegisterScalarFunction("NATURAL_TIME_AGO", 1,
		func(ctx *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			t, err := parseFlexibleDate(argString(args[0]))
			if err != nil {
				return nil, fmt.Errorf("NATURAL_TIME_AGO: %w", err)
			}
			return humanize.Time(t), nil
		}); err != nil {
		return err
	}
	add(Descriptor{Name: "NATURAL_TIME_AGO", Category: catAdvDatetime, Signature: "NATURAL_TIME_AGO(datetime) -> str",
		Description: "Renders datetime relative to the current time (e.g. '3 hours ago'). Depends on wall-clock time, so registered non-deterministic.",
		ExampleCall: "SELECT NATURAL_TIME_AGO(created_at) FROM events", ExampleResult: "e.g. '3 hours ago'",
		Deterministic: false})

	if err := sqlite.RegisterScalarFunction("PARSE_NATURAL_DATE", 1,
		func(ctx *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			r, err := whenParser.Parse(argString(args[0]), time.Now())
			if err != nil {
				return nil, fmt.Errorf("PARSE_NATURAL_DATE: %w", err)
			}
			if r == nil {
				return nil, fmt.Errorf("PARSE_NATURAL_DATE: could not parse %q", argString(args[0]))
			}
			return r.Time.Format("2006-01-02"), nil
		}); err != nil {
		return err
	}
	add(Descriptor{Name: "PARSE_NATURAL_DATE", Category: catAdvDatetime, Signature: "PARSE_NATURAL_DATE(str) -> str",
		Description: "Parses a natural-language date phrase relative to now. Depends on wall-clock time, so registered non-deterministic.",
		ExampleCall: `PARSE_NATURAL_DATE('next friday')`, ExampleResult: "2026-08-07", Deterministic: false})

	if err := sqlite.RegisterDeterministicScalarFunction("CRON_NEXT_RUN", 2,
		func(ctx *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			sched, err := cron.ParseStandard(argString(args[0]))
			if err != nil {
				return nil, fmt.Errorf("CRON_NEXT_RUN: %w", err)
			}
			from, err := parseFlexibleDate(argString(args[1]))
			if err != nil {
				return nil, fmt.Errorf("CRON_NEXT_RUN: %w", err)
			}
			return sched.Next(from).UTC().Format("2006-01-02T15:04:05Z"), nil
		}); err != nil {
		return err
	}
	add(Descriptor{Name: "CRON_NEXT_RUN", Category: catAdvDatetime, Signature: "CRON_NEXT_RUN(cron_expr, from) -> str",
		Description: "Next run time of cron_expr after from.",
		ExampleCall: `CRON_NEXT_RUN('0 9 * * MON', '2026-07-31T00:00:00Z')`, ExampleResult: "2026-08-03T09:00:00Z",
		Deterministic: true})

	if err := sqlite.RegisterDeterministicScalarFunction("HOLIDAY_NAME", 2,
		func(ctx *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			t, err := parseFlexibleDate(argString(args[0]))
			if err != nil {
				return nil, fmt.Errorf("HOLIDAY_NAME: %w", err)
			}
			h, err := holidayFor(t, argString(args[1]))
			if err != nil {
				return nil, fmt.Errorf("HOLIDAY_NAME: %w", err)
			}
			if h == nil {
				return nil, nil
			}
			return h.Name, nil
		}); err != nil {
		return err
	}
	add(Descriptor{Name: "HOLIDAY_NAME", Category: catAdvDatetime, Signature: "HOLIDAY_NAME(date, country) -> str",
		Description: "Name of the public holiday on date for country ('US'/'GB'), or NULL.",
		ExampleCall: `HOLIDAY_NAME('2026-12-25', 'US')`, ExampleResult: "Christmas Day",
		Deterministic: true})

	if err := sqlite.RegisterDeterministicScalarFunction("IS_HOLIDAY", 2,
		func(ctx *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			t, err := parseFlexibleDate(argString(args[0]))
			if err != nil {
				return nil, fmt.Errorf("IS_HOLIDAY: %w", err)
			}
			h, err := holidayFor(t, argString(args[1]))
			if err != nil {
				return nil, fmt.Errorf("IS_HOLIDAY: %w", err)
			}
			return boolResult(h != nil), nil
		}); err != nil {
		return err
	}
	add(Descriptor{Name: "IS_HOLIDAY", Category: catAdvDatetime, Signature: "IS_HOLIDAY(date, country) -> bool",
		Description: "1 if date is a public holiday for country ('US'/'GB').",
		ExampleCall: `IS_HOLIDAY('2026-12-25', 'US')`, ExampleResult: "1",
		Deterministic: true})

	if err := sqlite.RegisterDeterministicScalarFunction("ISO_WEEK", 1,
		func(ctx *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			t, err := parseFlexibleDate(argString(args[0]))
			if err != nil {
				return nil, fmt.Errorf("ISO_WEEK: %w", err)
			}
			_, week := t.ISOWeek()
			return int64(week), nil
		}); err != nil {
		return err
	}
	add(Descriptor{Name: "ISO_WEEK", Category: catAdvDatetime, Signature: "ISO_WEEK(date) -> int",
		Description: "ISO-8601 week number of date.",
		ExampleCall: `ISO_WEEK('2026-07-31')`, ExampleResult: "31",
		Deterministic: true})

	return nil
}
