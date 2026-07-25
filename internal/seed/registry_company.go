package seed

import "github.com/brianvoe/gofakeit/v7"

func companyGenerators() []GeneratorDef {
	return []GeneratorDef{
		{Name: "blurb", Group: "company", Description: "Company blurb", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return gofakeit.Blurb(), nil
		}},
		{Name: "bs", Group: "company", Description: "Business speak phrase", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return gofakeit.BS(), nil
		}},
		{Name: "buzzword", Group: "company", Description: "Business buzzword", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return gofakeit.BuzzWord(), nil
		}},
		{Name: "companySuffix", Group: "company", Description: "Company suffix (Inc, LLC, ...)", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return gofakeit.CompanySuffix(), nil
		}},
		{Name: "jobDescriptor", Group: "company", Description: "Job descriptor", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return gofakeit.JobDescriptor(), nil
		}},
		{Name: "jobLevel", Group: "company", Description: "Job level", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return gofakeit.JobLevel(), nil
		}},
		{Name: "jobTitle", Group: "company", Description: "Job title", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return gofakeit.JobTitle(), nil
		}},
		{Name: "slogan", Group: "company", Description: "Company slogan", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return gofakeit.Slogan(), nil
		}},
	}
}
