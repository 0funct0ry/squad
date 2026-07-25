package seed

import "github.com/brianvoe/gofakeit/v7"

func colorGenerators() []GeneratorDef {
	return []GeneratorDef{
		{Name: "color", Group: "color", Description: "Color name", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return gofakeit.Color(), nil
		}},
		{Name: "hexColor", Group: "color", Description: "Hex color code", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return gofakeit.HexColor(), nil
		}},
		{Name: "safeColor", Group: "color", Description: "Web-safe color name", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return gofakeit.SafeColor(), nil
		}},
		// shortHexColor: gofakeit only exposes 6-digit HexColor, so this
		// hand-rolls the 3-digit CSS shorthand variant.
		{Name: "shortHexColor", Group: "color", Description: "3-digit hex color code", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return "#" + gofakeit.Regex("[0-9a-f]{3}"), nil
		}},
	}
}
