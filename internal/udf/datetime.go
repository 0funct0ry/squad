package udf

import (
	"database/sql/driver"
	"fmt"
	"strings"
	"time"

	"modernc.org/sqlite"
)

const catDatetime = "Date/time"

// parseFlexibleDate tries a handful of common ISO-ish layouts, since inputs
// from a SELECT may be date-only, datetime, or already RFC3339.
func parseFlexibleDate(s string) (time.Time, error) {
	layouts := []string{
		time.RFC3339, "2006-01-02T15:04:05", "2006-01-02 15:04:05",
		"2006-01-02", "2006-01-02T15:04:05Z07:00",
	}
	var lastErr error
	for _, l := range layouts {
		if t, err := time.Parse(l, s); err == nil {
			return t, nil
		} else {
			lastErr = err
		}
	}
	return time.Time{}, lastErr
}

func registerDatetime() error {
	if err := sqlite.RegisterDeterministicScalarFunction("PARSE_DATE", 2,
		func(ctx *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			t, err := time.Parse(argString(args[1]), argString(args[0]))
			if err != nil {
				return nil, fmt.Errorf("PARSE_DATE: %w", err)
			}
			if t.Hour() == 0 && t.Minute() == 0 && t.Second() == 0 {
				return t.Format("2006-01-02"), nil
			}
			return t.Format("2006-01-02T15:04:05"), nil
		}); err != nil {
		return err
	}
	add(Descriptor{Name: "PARSE_DATE", Category: catDatetime, Signature: "PARSE_DATE(str, layout) -> str",
		Description: "Parses str using a Go-style reference layout, returns an ISO-8601 date/time string.",
		ExampleCall: `PARSE_DATE('31/07/2026', '02/01/2006')`, ExampleResult: "2026-07-31",
		Deterministic: true})

	if err := sqlite.RegisterDeterministicScalarFunction("DATE_TRUNC", 2,
		func(ctx *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			t, err := parseFlexibleDate(argString(args[1]))
			if err != nil {
				return nil, fmt.Errorf("DATE_TRUNC: %w", err)
			}
			switch strings.ToLower(argString(args[0])) {
			case "year":
				t = time.Date(t.Year(), 1, 1, 0, 0, 0, 0, t.Location())
			case "month":
				t = time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location())
			case "day":
				t = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
			case "hour":
				t = t.Truncate(time.Hour)
			case "minute":
				t = t.Truncate(time.Minute)
			case "week":
				wd := int(t.Weekday())
				if wd == 0 {
					wd = 7
				}
				t = time.Date(t.Year(), t.Month(), t.Day()-(wd-1), 0, 0, 0, 0, t.Location())
			default:
				return nil, fmt.Errorf("DATE_TRUNC: unknown unit %q", argString(args[0]))
			}
			return t.Format("2006-01-02"), nil
		}); err != nil {
		return err
	}
	add(Descriptor{Name: "DATE_TRUNC", Category: catDatetime, Signature: "DATE_TRUNC(unit, date) -> str",
		Description: "Truncates date to the start of the given unit ('day', 'month', 'year', etc.).",
		ExampleCall: `DATE_TRUNC('month', '2026-07-31')`, ExampleResult: "2026-07-01",
		Deterministic: true})

	if err := sqlite.RegisterDeterministicScalarFunction("AGE", 2,
		func(ctx *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			t1, err := parseFlexibleDate(argString(args[0]))
			if err != nil {
				return nil, fmt.Errorf("AGE: %w", err)
			}
			t2, err := parseFlexibleDate(argString(args[1]))
			if err != nil {
				return nil, fmt.Errorf("AGE: %w", err)
			}
			if t2.After(t1) {
				t1, t2 = t2, t1
			}
			years, months, days := diffYMD(t2, t1)
			return fmt.Sprintf("%d years, %d months, %d days", years, months, days), nil
		}); err != nil {
		return err
	}
	add(Descriptor{Name: "AGE", Category: catDatetime, Signature: "AGE(date1, date2) -> str",
		Description: "Human-readable duration between two dates.",
		ExampleCall: `AGE('2026-07-31', '2000-01-15')`, ExampleResult: "26 years, 6 months, 16 days",
		Deterministic: true})

	if err := sqlite.RegisterDeterministicScalarFunction("WEEKDAY_NAME", 1,
		func(ctx *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			t, err := parseFlexibleDate(argString(args[0]))
			if err != nil {
				return nil, fmt.Errorf("WEEKDAY_NAME: %w", err)
			}
			return t.Weekday().String(), nil
		}); err != nil {
		return err
	}
	add(Descriptor{Name: "WEEKDAY_NAME", Category: catDatetime, Signature: "WEEKDAY_NAME(date) -> str",
		Description: "Full weekday name.", ExampleCall: `WEEKDAY_NAME('2026-07-31')`,
		ExampleResult: "Friday", Deterministic: true})

	if err := sqlite.RegisterDeterministicScalarFunction("IS_WEEKEND", 1,
		func(ctx *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			t, err := parseFlexibleDate(argString(args[0]))
			if err != nil {
				return nil, fmt.Errorf("IS_WEEKEND: %w", err)
			}
			wd := t.Weekday()
			return boolResult(wd == time.Saturday || wd == time.Sunday), nil
		}); err != nil {
		return err
	}
	add(Descriptor{Name: "IS_WEEKEND", Category: catDatetime, Signature: "IS_WEEKEND(date) -> bool",
		Description: "1 if date falls on Saturday/Sunday, else 0.", ExampleCall: `IS_WEEKEND('2026-08-01')`,
		ExampleResult: "1", Deterministic: true})

	if err := sqlite.RegisterDeterministicScalarFunction("ADD_BUSINESS_DAYS", 2,
		func(ctx *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			t, err := parseFlexibleDate(argString(args[0]))
			if err != nil {
				return nil, fmt.Errorf("ADD_BUSINESS_DAYS: %w", err)
			}
			n, err := argInt(args[1])
			if err != nil {
				return nil, err
			}
			step := 1
			if n < 0 {
				step = -1
				n = -n
			}
			for n > 0 {
				t = t.AddDate(0, 0, step)
				if t.Weekday() != time.Saturday && t.Weekday() != time.Sunday {
					n--
				}
			}
			return t.Format("2006-01-02"), nil
		}); err != nil {
		return err
	}
	add(Descriptor{Name: "ADD_BUSINESS_DAYS", Category: catDatetime, Signature: "ADD_BUSINESS_DAYS(date, n) -> str",
		Description: "Adds n business days (skipping Sat/Sun) to date.",
		ExampleCall: `ADD_BUSINESS_DAYS('2026-07-31', 5)`, ExampleResult: "2026-08-07",
		Deterministic: true})

	if err := sqlite.RegisterDeterministicScalarFunction("UNIX_TO_ISO", 1,
		func(ctx *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			ts, err := argInt(args[0])
			if err != nil {
				return nil, err
			}
			return time.Unix(ts, 0).UTC().Format("2006-01-02T15:04:05Z"), nil
		}); err != nil {
		return err
	}
	add(Descriptor{Name: "UNIX_TO_ISO", Category: catDatetime, Signature: "UNIX_TO_ISO(timestamp) -> str",
		Description: "Converts Unix epoch seconds to an ISO-8601 string.",
		ExampleCall: `UNIX_TO_ISO(1785456000)`, ExampleResult: "2026-07-31T00:00:00Z",
		Deterministic: true})

	if err := sqlite.RegisterDeterministicScalarFunction("ISO_TO_UNIX", 1,
		func(ctx *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			t, err := parseFlexibleDate(argString(args[0]))
			if err != nil {
				return nil, fmt.Errorf("ISO_TO_UNIX: %w", err)
			}
			return t.Unix(), nil
		}); err != nil {
		return err
	}
	add(Descriptor{Name: "ISO_TO_UNIX", Category: catDatetime, Signature: "ISO_TO_UNIX(str) -> int",
		Description: "Converts an ISO-8601 string to Unix epoch seconds.",
		ExampleCall: `ISO_TO_UNIX('2026-07-31T00:00:00Z')`, ExampleResult: "1785456000",
		Deterministic: true})

	if err := sqlite.RegisterDeterministicScalarFunction("TIMEZONE_CONVERT", 3,
		func(ctx *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			fromLoc, err := time.LoadLocation(argString(args[1]))
			if err != nil {
				return nil, fmt.Errorf("TIMEZONE_CONVERT: %w", err)
			}
			toLoc, err := time.LoadLocation(argString(args[2]))
			if err != nil {
				return nil, fmt.Errorf("TIMEZONE_CONVERT: %w", err)
			}
			layout := "2006-01-02 15:04:05"
			t, err := time.ParseInLocation(layout, argString(args[0]), fromLoc)
			if err != nil {
				return nil, fmt.Errorf("TIMEZONE_CONVERT: %w", err)
			}
			return t.In(toLoc).Format(layout), nil
		}); err != nil {
		return err
	}
	add(Descriptor{Name: "TIMEZONE_CONVERT", Category: catDatetime, Signature: "TIMEZONE_CONVERT(datetime, from_tz, to_tz) -> str",
		Description:   "Converts a datetime string between IANA time zones using embedded tzdata.",
		ExampleCall:   `TIMEZONE_CONVERT('2026-07-31 09:00:00', 'UTC', 'America/New_York')`,
		ExampleResult: "2026-07-31 05:00:00", Deterministic: true})

	return nil
}

// diffYMD computes the calendar years/months/days between from (earlier) and
// to (later).
func diffYMD(from, to time.Time) (years, months, days int) {
	y1, m1, d1 := from.Date()
	y2, m2, d2 := to.Date()
	years = y2 - y1
	months = int(m2 - m1)
	days = d2 - d1
	if days < 0 {
		months--
		daysInPrevMonth := time.Date(y2, m2, 0, 0, 0, 0, 0, time.UTC).Day()
		days += daysInPrevMonth
	}
	if months < 0 {
		years--
		months += 12
	}
	return years, months, days
}
