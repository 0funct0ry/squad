package seed

import (
	"math"
	"math/rand"
)

func distributionGenerators() []GeneratorDef {
	return []GeneratorDef{
		// normal: Box-Muller transform over two uniform samples.
		{Name: "normal", Group: "distribution", Description: "Normal (Gaussian) distribution", Affinities: []string{"REAL"}, OptionsSchema: []OptionField{
			{Key: "mean", Label: "Mean", Kind: OptKindFloat, Default: 0.0},
			{Key: "stddev", Label: "Std dev", Kind: OptKindFloat, Default: 1.0},
		}, Fn: func(_ string, opts map[string]any) (any, error) {
			mean := optFloat(opts, "mean", 0)
			stddev := optFloat(opts, "stddev", 1)
			return mean + stddev*rand.NormFloat64(), nil
		}},
		// binomial: sum of `trials` independent Bernoulli(p) trials.
		{Name: "binomial", Group: "distribution", Description: "Binomial distribution", Affinities: []string{"REAL"}, OptionsSchema: []OptionField{
			{Key: "trials", Label: "Trials", Kind: OptKindInt, Default: 10},
			{Key: "p", Label: "P", Kind: OptKindFloat, Default: 0.5},
		}, Fn: func(_ string, opts map[string]any) (any, error) {
			trials := optInt(opts, "trials", 10)
			p := optFloat(opts, "p", 0.5)
			count := 0
			for i := 0; i < trials; i++ {
				if rand.Float64() < p {
					count++
				}
			}
			return float64(count), nil
		}},
		// exponential: inverse transform sampling, X = -ln(U)/lambda.
		{Name: "exponential", Group: "distribution", Description: "Exponential distribution", Affinities: []string{"REAL"}, OptionsSchema: []OptionField{
			{Key: "lambda", Label: "Lambda", Kind: OptKindFloat, Default: 1.0},
		}, Fn: func(_ string, opts map[string]any) (any, error) {
			lambda := optFloat(opts, "lambda", 1)
			if lambda <= 0 {
				lambda = 1
			}
			return rand.ExpFloat64() / lambda, nil
		}},
		// geometric: inverse CDF, X = ceil(ln(1-U)/ln(1-p)), number of trials
		// until first success.
		{Name: "geometric", Group: "distribution", Description: "Geometric distribution", Affinities: []string{"REAL"}, OptionsSchema: []OptionField{
			{Key: "p", Label: "P", Kind: OptKindFloat, Default: 0.5},
		}, Fn: func(_ string, opts map[string]any) (any, error) {
			p := optFloat(opts, "p", 0.5)
			if p <= 0 {
				p = 0.01
			}
			if p >= 1 {
				return float64(1), nil
			}
			u := rand.Float64()
			trials := int(1 + (math.Log(1-u) / math.Log(1-p)))
			if trials < 1 {
				trials = 1
			}
			return float64(trials), nil
		}},
		// poisson: Knuth's algorithm.
		{Name: "poisson", Group: "distribution", Description: "Poisson distribution", Affinities: []string{"REAL"}, OptionsSchema: []OptionField{
			{Key: "lambda", Label: "Lambda", Kind: OptKindFloat, Default: 4.0},
		}, Fn: func(_ string, opts map[string]any) (any, error) {
			lambda := optFloat(opts, "lambda", 4)
			if lambda <= 0 {
				lambda = 1
			}
			l := math.Exp(-lambda)
			k := 0
			p := 1.0
			for {
				k++
				p *= rand.Float64()
				if p <= l {
					break
				}
			}
			return float64(k - 1), nil
		}},
	}
}
