package seed

import (
	"crypto/md5"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"

	"github.com/0funct0ry/squad/internal/seed/data"
	"github.com/brianvoe/gofakeit/v7"
	"golang.org/x/crypto/bcrypt"
)

func securityGenerators() []GeneratorDef {
	return []GeneratorDef{
		{Name: "md5", Group: "security", Description: "MD5 hash of random text", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			sum := md5.Sum([]byte(gofakeit.Word()))
			return hex.EncodeToString(sum[:]), nil
		}},
		{Name: "sha1", Group: "security", Description: "SHA-1 hash of random text", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			sum := sha1.Sum([]byte(gofakeit.Word()))
			return hex.EncodeToString(sum[:]), nil
		}},
		{Name: "sha256", Group: "security", Description: "SHA-256 hash of random text", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			sum := sha256.Sum256([]byte(gofakeit.Word()))
			return hex.EncodeToString(sum[:]), nil
		}},
		{Name: "passwordHash", Group: "security", Description: "Bcrypt hash of a random password", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			pw := gofakeit.Password(true, true, true, true, false, 12)
			hashed, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.DefaultCost)
			if err != nil {
				return nil, err
			}
			return string(hashed), nil
		}},
		// encrypt: NOT real encryption - this is just base64 of random bytes,
		// suitable only as cosmetic seed data.
		{Name: "encrypt", Group: "security", Description: "Base64 blob (not real encryption)", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			b := make([]byte, 24)
			if _, err := rand.Read(b); err != nil {
				return nil, err
			}
			return base64.StdEncoding.EncodeToString(b), nil
		}},
		{Name: "naughtyString", Group: "security", Description: "Classic edge-case string", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			pool := data.NaughtyStrings
			return pool[gofakeit.Number(0, len(pool)-1)], nil
		}},
		{Name: "password", Group: "security", Description: "Random password", Affinities: []string{"TEXT"}, OptionsSchema: []OptionField{
			{Key: "lower", Label: "Lowercase", Kind: OptKindBool, Default: true},
			{Key: "upper", Label: "Uppercase", Kind: OptKindBool, Default: true},
			{Key: "numeric", Label: "Numeric", Kind: OptKindBool, Default: true},
			{Key: "special", Label: "Special", Kind: OptKindBool, Default: true},
			{Key: "length", Label: "Length", Kind: OptKindInt, Default: 12},
		}, Fn: func(_ string, opts map[string]any) (any, error) {
			lower := optBool(opts, "lower", true)
			upper := optBool(opts, "upper", true)
			numeric := optBool(opts, "numeric", true)
			special := optBool(opts, "special", true)
			length := optInt(opts, "length", 12)
			return gofakeit.Password(lower, upper, numeric, special, false, length), nil
		}},
	}
}
