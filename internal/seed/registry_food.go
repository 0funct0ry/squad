package seed

import "github.com/brianvoe/gofakeit/v7"

func foodGenerators() []GeneratorDef {
	return []GeneratorDef{
		{Name: "food", Group: "food", Description: "Food item by meal type", Affinities: []string{"TEXT"}, OptionsSchema: []OptionField{
			{Key: "mealType", Label: "Meal type", Kind: OptKindSelect, Default: "breakfast",
				Choices: []string{"breakfast", "dessert", "dinner", "drink", "fruit", "lunch", "snack", "vegetable"}},
		}, Fn: func(_ string, opts map[string]any) (any, error) {
			switch optString(opts, "mealType", "breakfast") {
			case "dessert":
				return gofakeit.Dessert(), nil
			case "dinner":
				return gofakeit.Dinner(), nil
			case "drink":
				return gofakeit.Drink(), nil
			case "fruit":
				return gofakeit.Fruit(), nil
			case "lunch":
				return gofakeit.Lunch(), nil
			case "snack":
				return gofakeit.Snack(), nil
			case "vegetable":
				return gofakeit.Vegetable(), nil
			default:
				return gofakeit.Breakfast(), nil
			}
		}},
	}
}
