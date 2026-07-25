package seed

import (
	"fmt"

	"github.com/brianvoe/gofakeit/v7"
)

func geoGenerators() []GeneratorDef {
	return []GeneratorDef{
		{Name: "countryAbr", Group: "geo", Description: "Country abbreviation", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return gofakeit.CountryAbr(), nil
		}},
		{Name: "latitude", Group: "geo", Description: "Latitude", Affinities: []string{"REAL"}, Fn: func(string, map[string]any) (any, error) {
			return gofakeit.Latitude(), nil
		}},
		{Name: "latitudeRange", Group: "geo", Description: "Latitude within a range", Affinities: []string{"REAL"}, OptionsSchema: []OptionField{
			{Key: "min", Label: "Min", Kind: OptKindFloat, Default: -90.0},
			{Key: "max", Label: "Max", Kind: OptKindFloat, Default: 90.0},
		}, Fn: func(_ string, opts map[string]any) (any, error) {
			min := optFloat(opts, "min", -90)
			max := optFloat(opts, "max", 90)
			return gofakeit.LatitudeInRange(min, max)
		}},
		{Name: "longitude", Group: "geo", Description: "Longitude", Affinities: []string{"REAL"}, Fn: func(string, map[string]any) (any, error) {
			return gofakeit.Longitude(), nil
		}},
		{Name: "longitudeRange", Group: "geo", Description: "Longitude within a range", Affinities: []string{"REAL"}, OptionsSchema: []OptionField{
			{Key: "min", Label: "Min", Kind: OptKindFloat, Default: -180.0},
			{Key: "max", Label: "Max", Kind: OptKindFloat, Default: 180.0},
		}, Fn: func(_ string, opts map[string]any) (any, error) {
			min := optFloat(opts, "min", -180)
			max := optFloat(opts, "max", 180)
			return gofakeit.LongitudeInRange(min, max)
		}},
		{Name: "state", Group: "geo", Description: "US state", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return gofakeit.State(), nil
		}},
		{Name: "stateAbr", Group: "geo", Description: "US state abbreviation", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return gofakeit.StateAbr(), nil
		}},
		{Name: "street", Group: "geo", Description: "Street address", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return gofakeit.Street(), nil
		}},
		{Name: "streetName", Group: "geo", Description: "Street name", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return gofakeit.StreetName(), nil
		}},
		{Name: "streetNumber", Group: "geo", Description: "Street number", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return gofakeit.StreetNumber(), nil
		}},
		{Name: "streetPrefix", Group: "geo", Description: "Street prefix", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return gofakeit.StreetPrefix(), nil
		}},
		{Name: "streetSuffix", Group: "geo", Description: "Street suffix", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return gofakeit.StreetSuffix(), nil
		}},
		{Name: "unit", Group: "geo", Description: "Unit / apartment number", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return fmt.Sprintf("Unit %s", gofakeit.Numerify("##")), nil
		}},
		{Name: "addressLine2", Group: "geo", Description: "Apartment/suite/floor line", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			switch gofakeit.Number(0, 2) {
			case 0:
				return fmt.Sprintf("Apt %s", gofakeit.Numerify("###")), nil
			case 1:
				return fmt.Sprintf("Suite %s", gofakeit.Numerify("##")), nil
			default:
				return fmt.Sprintf("Floor %s", gofakeit.Numerify("##")), nil
			}
		}},
	}
}
