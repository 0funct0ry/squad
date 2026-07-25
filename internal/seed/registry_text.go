package seed

import "github.com/brianvoe/gofakeit/v7"

func textGenerators() []GeneratorDef {
	return []GeneratorDef{
		{Name: "comment", Group: "text", Description: "Comment sentence", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return gofakeit.Comment(), nil
		}},
		{Name: "loremIpsumParagraph", Group: "text", Description: "Lorem ipsum paragraph", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return gofakeit.LoremIpsumParagraph(1, 3, 10, " "), nil
		}},
		{Name: "loremIpsumSentence", Group: "text", Description: "Lorem ipsum sentence", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return gofakeit.LoremIpsumSentence(8), nil
		}},
		{Name: "loremIpsumWord", Group: "text", Description: "Lorem ipsum word", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return gofakeit.LoremIpsumWord(), nil
		}},
		{Name: "phrase", Group: "text", Description: "Common phrase", Affinities: []string{"TEXT"}, OptionsSchema: []OptionField{
			{Key: "variant", Label: "Variant", Kind: OptKindString, Description: "reserved for future use"},
		}, Fn: func(string, map[string]any) (any, error) {
			return gofakeit.Phrase(), nil
		}},
		{Name: "question", Group: "text", Description: "Question sentence", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return gofakeit.Question(), nil
		}},
		{Name: "quote", Group: "text", Description: "Famous quote", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return gofakeit.Quote(), nil
		}},
	}
}
