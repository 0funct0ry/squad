package seed

import "github.com/brianvoe/gofakeit/v7"

func productGenerators() []GeneratorDef {
	return []GeneratorDef{
		{Name: "productAudience", Group: "product", Description: "Product audience", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			audiences := gofakeit.ProductAudience()
			if len(audiences) == 0 {
				return "", nil
			}
			return audiences[0], nil
		}},
		{Name: "productBenefit", Group: "product", Description: "Product benefit", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return gofakeit.ProductBenefit(), nil
		}},
		{Name: "productCategory", Group: "product", Description: "Product category", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return gofakeit.ProductCategory(), nil
		}},
		{Name: "productDescription", Group: "product", Description: "Product description", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return gofakeit.ProductDescription(), nil
		}},
		{Name: "productDimension", Group: "product", Description: "Product dimension", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return gofakeit.ProductDimension(), nil
		}},
		{Name: "productFeature", Group: "product", Description: "Product feature", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return gofakeit.ProductFeature(), nil
		}},
		// productIsbn: gofakeit has no dedicated ISBN function, so this
		// hand-rolls an ISBN-13-shaped string (not checksum-validated).
		{Name: "productIsbn", Group: "product", Description: "ISBN-shaped string (not validated)", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return "978-" + gofakeit.Numerify("#-#####-###-#"), nil
		}},
		{Name: "productMaterial", Group: "product", Description: "Product material", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return gofakeit.ProductMaterial(), nil
		}},
		{Name: "productName", Group: "product", Description: "Product name", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return gofakeit.ProductName(), nil
		}},
		{Name: "productSuffix", Group: "product", Description: "Product suffix", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return gofakeit.ProductSuffix(), nil
		}},
		// productUpc: gofakeit has no dedicated UPC function, so this
		// hand-rolls a UPC-A-shaped 12-digit string (not checksum-validated).
		{Name: "productUpc", Group: "product", Description: "UPC-shaped string (not validated)", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return gofakeit.Numerify("############"), nil
		}},
		{Name: "productUseCase", Group: "product", Description: "Product use case", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return gofakeit.ProductUseCase(), nil
		}},
	}
}
