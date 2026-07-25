package seed

import (
	"fmt"

	"github.com/brianvoe/gofakeit/v7"
)

func personGenerators() []GeneratorDef {
	return []GeneratorDef{
		{Name: "age", Group: "person", Description: "Age in years", Affinities: []string{"INTEGER"}, OptionsSchema: []OptionField{
			{Key: "min", Label: "Min", Kind: OptKindInt, Default: 1},
			{Key: "max", Label: "Max", Kind: OptKindInt, Default: 99},
		}, Fn: func(_ string, opts map[string]any) (any, error) {
			min := optInt(opts, "min", 1)
			max := optInt(opts, "max", 99)
			return gofakeit.Number(min, max), nil
		}},
		{Name: "bio", Group: "person", Description: "Short biography", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return gofakeit.Sentence(10), nil
		}},
		{Name: "ein", Group: "person", Description: "Employer identification number", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return gofakeit.Numerify("##-#######"), nil
		}},
		{Name: "ethnicity", Group: "person", Description: "Ethnicity", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return gofakeit.Ethnicity(), nil
		}},
		{Name: "gender", Group: "person", Description: "Gender", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return gofakeit.Gender(), nil
		}},
		{Name: "hobby", Group: "person", Description: "Hobby", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return gofakeit.Hobby(), nil
		}},
		{Name: "middleName", Group: "person", Description: "Middle name", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return gofakeit.FirstName(), nil
		}},
		{Name: "namePrefix", Group: "person", Description: "Name prefix (Mr., Mrs., ...)", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return gofakeit.NamePrefix(), nil
		}},
		{Name: "nameSuffix", Group: "person", Description: "Name suffix (Jr., Sr., ...)", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return gofakeit.NameSuffix(), nil
		}},
		{Name: "phoneFormatted", Group: "person", Description: "Formatted phone number", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return gofakeit.Phone(), nil
		}},
		{Name: "socialMedia", Group: "person", Description: "Social media handle", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return fmt.Sprintf("@%s", gofakeit.Username()), nil
		}},
		{Name: "ssn", Group: "person", Description: "Social security number", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return gofakeit.SSN(), nil
		}},
	}
}
