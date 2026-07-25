package seed

import (
	"time"

	"github.com/brianvoe/gofakeit/v7"
)

func datetimeGenerators() []GeneratorDef {
	return []GeneratorDef{
		{Name: "dateRange", Group: "datetime", Description: "Date/time within a range", Affinities: []string{"TEXT", "INTEGER"}, OptionsSchema: []OptionField{
			{Key: "from", Label: "From", Kind: OptKindDateTime},
			{Key: "to", Label: "To", Kind: OptKindDateTime},
		}, Fn: func(affinity string, opts map[string]any) (any, error) {
			from := optTime(opts, "from", time.Now().AddDate(-5, 0, 0))
			to := optTime(opts, "to", time.Now())
			t := gofakeit.DateRange(from, to)
			if affinity == "INTEGER" {
				return t.Unix(), nil
			}
			return t.Format(time.RFC3339), nil
		}},
		{Name: "day", Group: "datetime", Description: "Day of month (1-31)", Affinities: []string{"INTEGER"}, Fn: func(string, map[string]any) (any, error) {
			return gofakeit.Day(), nil
		}},
		{Name: "futureDate", Group: "datetime", Description: "Future date/time", Affinities: []string{"TEXT", "INTEGER"}, Fn: func(affinity string, _ map[string]any) (any, error) {
			t := gofakeit.FutureDate()
			if affinity == "INTEGER" {
				return t.Unix(), nil
			}
			return t.Format(time.RFC3339), nil
		}},
		{Name: "hour", Group: "datetime", Description: "Hour (0-23)", Affinities: []string{"INTEGER"}, Fn: func(string, map[string]any) (any, error) {
			return gofakeit.Hour(), nil
		}},
		{Name: "minute", Group: "datetime", Description: "Minute (0-59)", Affinities: []string{"INTEGER"}, Fn: func(string, map[string]any) (any, error) {
			return gofakeit.Minute(), nil
		}},
		{Name: "month", Group: "datetime", Description: "Month number (1-12)", Affinities: []string{"INTEGER"}, Fn: func(string, map[string]any) (any, error) {
			return gofakeit.Month(), nil
		}},
		{Name: "monthString", Group: "datetime", Description: "Month name", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return gofakeit.MonthString(), nil
		}},
		{Name: "nanosecond", Group: "datetime", Description: "Nanosecond component", Affinities: []string{"INTEGER"}, Fn: func(string, map[string]any) (any, error) {
			return gofakeit.NanoSecond(), nil
		}},
		{Name: "pastDate", Group: "datetime", Description: "Past date/time", Affinities: []string{"TEXT", "INTEGER"}, Fn: func(affinity string, _ map[string]any) (any, error) {
			t := gofakeit.PastDate()
			if affinity == "INTEGER" {
				return t.Unix(), nil
			}
			return t.Format(time.RFC3339), nil
		}},
		{Name: "second", Group: "datetime", Description: "Second (0-59)", Affinities: []string{"INTEGER"}, Fn: func(string, map[string]any) (any, error) {
			return gofakeit.Second(), nil
		}},
		{Name: "time", Group: "datetime", Description: "Time of day (HH:MM:SS)", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			t := gofakeit.DateRange(time.Now().AddDate(-1, 0, 0), time.Now())
			return t.Format("15:04:05"), nil
		}},
		{Name: "timeRange", Group: "datetime", Description: "Time of day within a range", Affinities: []string{"TEXT"}, OptionsSchema: []OptionField{
			{Key: "from", Label: "From", Kind: OptKindDateTime},
			{Key: "to", Label: "To", Kind: OptKindDateTime},
		}, Fn: func(_ string, opts map[string]any) (any, error) {
			from := optTime(opts, "from", time.Now().AddDate(-5, 0, 0))
			to := optTime(opts, "to", time.Now())
			t := gofakeit.DateRange(from, to)
			return t.Format("15:04:05"), nil
		}},
		{Name: "timezone", Group: "datetime", Description: "Timezone name", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return gofakeit.TimeZone(), nil
		}},
		{Name: "timezoneAbv", Group: "datetime", Description: "Timezone abbreviation", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return gofakeit.TimeZoneAbv(), nil
		}},
		{Name: "timezoneFull", Group: "datetime", Description: "Full timezone name", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return gofakeit.TimeZoneFull(), nil
		}},
		{Name: "timezoneOffset", Group: "datetime", Description: "Timezone UTC offset", Affinities: []string{"REAL"}, Fn: func(string, map[string]any) (any, error) {
			return float64(gofakeit.TimeZoneOffset()), nil
		}},
		{Name: "timezoneRegion", Group: "datetime", Description: "Timezone region", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return gofakeit.TimeZoneRegion(), nil
		}},
		{Name: "weekday", Group: "datetime", Description: "Day of week name", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return gofakeit.WeekDay(), nil
		}},
		{Name: "year", Group: "datetime", Description: "Year", Affinities: []string{"INTEGER"}, Fn: func(string, map[string]any) (any, error) {
			return gofakeit.Year(), nil
		}},
	}
}
