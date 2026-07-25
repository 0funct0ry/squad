package seed

import "github.com/brianvoe/gofakeit/v7"

func internetGenerators() []GeneratorDef {
	return []GeneratorDef{
		{Name: "apiUserAgent", Group: "internet", Description: "API-style user agent", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return gofakeit.UserAgent(), nil
		}},
		{Name: "domainName", Group: "internet", Description: "Domain name", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return gofakeit.DomainName(), nil
		}},
		{Name: "domainSuffix", Group: "internet", Description: "Domain suffix", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return gofakeit.DomainSuffix(), nil
		}},
		{Name: "httpMethod", Group: "internet", Description: "HTTP method", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return gofakeit.HTTPMethod(), nil
		}},
		{Name: "httpStatusCode", Group: "internet", Description: "HTTP status code", Affinities: []string{"INTEGER"}, Fn: func(string, map[string]any) (any, error) {
			return int64(gofakeit.HTTPStatusCode()), nil
		}},
		{Name: "httpStatusCodeSimple", Group: "internet", Description: "Common HTTP status code", Affinities: []string{"INTEGER"}, Fn: func(string, map[string]any) (any, error) {
			return int64(gofakeit.HTTPStatusCodeSimple()), nil
		}},
		{Name: "httpVersion", Group: "internet", Description: "HTTP version", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return gofakeit.HTTPVersion(), nil
		}},
		{Name: "ipv6", Group: "internet", Description: "IPv6 address", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return gofakeit.IPv6Address(), nil
		}},
		{Name: "logLevel", Group: "internet", Description: "Log level", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return gofakeit.LogLevel("general"), nil
		}},
		{Name: "macAddress", Group: "internet", Description: "MAC address", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return gofakeit.MacAddress(), nil
		}},
		{Name: "urlSlug", Group: "internet", Description: "URL slug", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return gofakeit.UrlSlug(3), nil
		}},
		{Name: "userAgent", Group: "internet", Description: "User agent string", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return gofakeit.UserAgent(), nil
		}},
	}
}
