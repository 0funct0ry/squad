package seed

import (
	"math/rand"

	"github.com/brianvoe/gofakeit/v7"
)

func numericGenerators() []GeneratorDef {
	return []GeneratorDef{
		{Name: "int8", Group: "numeric", Description: "Random int8", Affinities: []string{"INTEGER"}, Fn: func(string, map[string]any) (any, error) {
			return int64(gofakeit.Int8()), nil
		}},
		{Name: "int16", Group: "numeric", Description: "Random int16", Affinities: []string{"INTEGER"}, Fn: func(string, map[string]any) (any, error) {
			return int64(gofakeit.Int16()), nil
		}},
		{Name: "int32", Group: "numeric", Description: "Random int32", Affinities: []string{"INTEGER"}, Fn: func(string, map[string]any) (any, error) {
			return int64(gofakeit.Int32()), nil
		}},
		{Name: "int64", Group: "numeric", Description: "Random int64", Affinities: []string{"INTEGER"}, Fn: func(string, map[string]any) (any, error) {
			return gofakeit.Int64(), nil
		}},
		{Name: "uint", Group: "numeric", Description: "Random uint", Affinities: []string{"INTEGER"}, Fn: func(string, map[string]any) (any, error) {
			return int64(gofakeit.Uint()), nil
		}},
		{Name: "uint8", Group: "numeric", Description: "Random uint8", Affinities: []string{"INTEGER"}, Fn: func(string, map[string]any) (any, error) {
			return int64(gofakeit.Uint8()), nil
		}},
		{Name: "uint16", Group: "numeric", Description: "Random uint16", Affinities: []string{"INTEGER"}, Fn: func(string, map[string]any) (any, error) {
			return int64(gofakeit.Uint16()), nil
		}},
		{Name: "uint32", Group: "numeric", Description: "Random uint32", Affinities: []string{"INTEGER"}, Fn: func(string, map[string]any) (any, error) {
			return int64(gofakeit.Uint32()), nil
		}},
		{Name: "uint64", Group: "numeric", Description: "Random uint64", Affinities: []string{"INTEGER"}, Fn: func(string, map[string]any) (any, error) {
			return int64(gofakeit.Uint64()), nil
		}},
		{Name: "intN", Group: "numeric", Description: "Random int in [0, n)", Affinities: []string{"INTEGER"}, OptionsSchema: []OptionField{
			{Key: "n", Label: "N", Kind: OptKindInt, Default: 100},
		}, Fn: func(_ string, opts map[string]any) (any, error) {
			n := optInt(opts, "n", 100)
			if n <= 0 {
				n = 1
			}
			return int64(gofakeit.IntN(n)), nil
		}},
		{Name: "uintN", Group: "numeric", Description: "Random uint in [0, n)", Affinities: []string{"INTEGER"}, OptionsSchema: []OptionField{
			{Key: "n", Label: "N", Kind: OptKindInt, Default: 100},
		}, Fn: func(_ string, opts map[string]any) (any, error) {
			n := optInt(opts, "n", 100)
			if n <= 0 {
				n = 1
			}
			return int64(gofakeit.UintN(uint(n))), nil
		}},
		{Name: "intRange", Group: "numeric", Description: "Int within a range", Affinities: []string{"INTEGER"}, OptionsSchema: []OptionField{
			{Key: "min", Label: "Min", Kind: OptKindInt, Default: 0},
			{Key: "max", Label: "Max", Kind: OptKindInt, Default: 100},
		}, Fn: func(_ string, opts map[string]any) (any, error) {
			min := optInt(opts, "min", 0)
			max := optInt(opts, "max", 100)
			return int64(gofakeit.IntRange(min, max)), nil
		}},
		{Name: "uintRange", Group: "numeric", Description: "Uint within a range", Affinities: []string{"INTEGER"}, OptionsSchema: []OptionField{
			{Key: "min", Label: "Min", Kind: OptKindInt, Default: 0},
			{Key: "max", Label: "Max", Kind: OptKindInt, Default: 100},
		}, Fn: func(_ string, opts map[string]any) (any, error) {
			min := optInt(opts, "min", 0)
			max := optInt(opts, "max", 100)
			if min < 0 {
				min = 0
			}
			if max < min {
				max = min
			}
			return int64(gofakeit.UintRange(uint(min), uint(max))), nil
		}},
		{Name: "float32", Group: "numeric", Description: "Random float32", Affinities: []string{"REAL"}, Fn: func(string, map[string]any) (any, error) {
			return float64(gofakeit.Float32()), nil
		}},
		{Name: "float32Range", Group: "numeric", Description: "Float32 within a range", Affinities: []string{"REAL"}, OptionsSchema: []OptionField{
			{Key: "min", Label: "Min", Kind: OptKindFloat, Default: 0.0},
			{Key: "max", Label: "Max", Kind: OptKindFloat, Default: 1000.0},
		}, Fn: func(_ string, opts map[string]any) (any, error) {
			min := optFloat(opts, "min", 0)
			max := optFloat(opts, "max", 1000)
			return float64(gofakeit.Float32Range(float32(min), float32(max))), nil
		}},
		{Name: "float64Range", Group: "numeric", Description: "Float64 within a range", Affinities: []string{"REAL"}, OptionsSchema: []OptionField{
			{Key: "min", Label: "Min", Kind: OptKindFloat, Default: 0.0},
			{Key: "max", Label: "Max", Kind: OptKindFloat, Default: 1000.0},
		}, Fn: func(_ string, opts map[string]any) (any, error) {
			min := optFloat(opts, "min", 0)
			max := optFloat(opts, "max", 1000)
			return gofakeit.Float64Range(min, max), nil
		}},
		{Name: "hexUint", Group: "numeric", Description: "Random hex-encoded uint", Affinities: []string{"TEXT"}, OptionsSchema: []OptionField{
			{Key: "bitSize", Label: "Bit size", Kind: OptKindInt, Default: 32},
		}, Fn: func(_ string, opts map[string]any) (any, error) {
			bitSize := optInt(opts, "bitSize", 32)
			return gofakeit.HexUint(bitSize), nil
		}},
		{Name: "randomInt", Group: "numeric", Description: "Random pick from a small generated pool", Affinities: []string{"INTEGER"}, Fn: func(string, map[string]any) (any, error) {
			pool := []int{1, 2, 3, 5, 8, 13, 21, 34}
			return int64(gofakeit.RandomInt(pool)), nil
		}},
		{Name: "randomUint", Group: "numeric", Description: "Random pick from a small generated pool", Affinities: []string{"INTEGER"}, Fn: func(string, map[string]any) (any, error) {
			pool := []int{10, 20, 30, 40, 50, 60}
			return int64(gofakeit.RandomInt(pool)), nil
		}},
		{Name: "shuffleInts", Group: "numeric", Description: "Value from a shuffled small pool", Affinities: []string{"INTEGER"}, Fn: func(string, map[string]any) (any, error) {
			pool := []int{1, 2, 3, 4, 5, 6, 7, 8}
			gofakeit.ShuffleInts(pool)
			return int64(pool[rand.Intn(len(pool))]), nil
		}},
	}
}
