package seed

import (
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/oklog/ulid/v2"
)

func identifierGenerators() []GeneratorDef {
	return []GeneratorDef{
		{Name: "digit", Group: "identifier", Description: "Single digit", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return gofakeit.Digit(), nil
		}},
		{Name: "digitN", Group: "identifier", Description: "N-digit string", Affinities: []string{"TEXT"}, OptionsSchema: []OptionField{
			{Key: "n", Label: "N", Kind: OptKindInt, Default: 6},
		}, Fn: func(_ string, opts map[string]any) (any, error) {
			n := optInt(opts, "n", 6)
			if n < 0 {
				n = 0
			}
			return gofakeit.DigitN(uint(n)), nil
		}},
		{Name: "letter", Group: "identifier", Description: "Single letter", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return gofakeit.Letter(), nil
		}},
		{Name: "letterN", Group: "identifier", Description: "N-letter string", Affinities: []string{"TEXT"}, OptionsSchema: []OptionField{
			{Key: "n", Label: "N", Kind: OptKindInt, Default: 6},
		}, Fn: func(_ string, opts map[string]any) (any, error) {
			n := optInt(opts, "n", 6)
			if n < 0 {
				n = 0
			}
			return gofakeit.LetterN(uint(n)), nil
		}},
		{Name: "vowel", Group: "identifier", Description: "Single vowel", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			vowels := []string{"a", "e", "i", "o", "u"}
			return vowels[gofakeit.Number(0, len(vowels)-1)], nil
		}},
		{Name: "lexify", Group: "identifier", Description: "Fill ? placeholders with letters", Affinities: []string{"TEXT"}, OptionsSchema: []OptionField{
			{Key: "pattern", Label: "Pattern", Kind: OptKindString, Default: "????????"},
		}, Fn: func(_ string, opts map[string]any) (any, error) {
			pattern := optString(opts, "pattern", "????????")
			return gofakeit.Lexify(pattern), nil
		}},
		{Name: "numerify", Group: "identifier", Description: "Fill # placeholders with digits", Affinities: []string{"TEXT"}, OptionsSchema: []OptionField{
			{Key: "pattern", Label: "Pattern", Kind: OptKindString, Default: "#####"},
		}, Fn: func(_ string, opts map[string]any) (any, error) {
			pattern := optString(opts, "pattern", "#####")
			return gofakeit.Numerify(pattern), nil
		}},
		{Name: "randomString", Group: "identifier", Description: "Random pick from a small pool", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			pool := []string{"alpha", "beta", "gamma", "delta", "epsilon"}
			return gofakeit.RandomString(pool), nil
		}},
		{Name: "regex", Group: "identifier", Description: "String matching a regex pattern", Affinities: []string{"TEXT"}, OptionsSchema: []OptionField{
			{Key: "pattern", Label: "Pattern", Kind: OptKindString, Required: true},
		}, Fn: func(_ string, opts map[string]any) (any, error) {
			pattern := optString(opts, "pattern", "[a-z0-9]{8}")
			return gofakeit.Regex(pattern), nil
		}},
		{Name: "guid", Group: "identifier", Description: "UUID v4 (alias)", Affinities: []string{"TEXT"}, Aliases: []string{"uuid"}, Fn: func(string, map[string]any) (any, error) {
			return gofakeit.UUID(), nil
		}},
		{Name: "ulid", Group: "identifier", Description: "ULID", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return ulid.MustNewDefault(time.Now()).String(), nil
		}},
		// mongoObjectId: hand-rolled 12 random bytes hex-encoded to a 24-char
		// string. This is a simplification of the real Mongo ObjectID format
		// (4-byte timestamp + 5-byte random + 3-byte counter) but is
		// acceptable for cosmetic seed data.
		{Name: "mongoObjectId", Group: "identifier", Description: "MongoDB-style ObjectId (hand-rolled)", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			b := make([]byte, 12)
			if _, err := rand.Read(b); err != nil {
				return nil, err
			}
			return hex.EncodeToString(b), nil
		}},
	}
}
