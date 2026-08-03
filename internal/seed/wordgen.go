package seed

import (
	"crypto/rand"
	"strings"

	"github.com/brianvoe/gofakeit/v7"
)

// hexString returns n lowercase hex characters, suitable for SHA-like values.
func hexString(n int) string {
	const digits = "0123456789abcdef"
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand.Read on the standard reader does not fail in practice;
		// fall back to gofakeit rather than propagating an error from a
		// generator signature that only some callers can return.
		for i := range b {
			b[i] = digits[gofakeit.Number(0, len(digits)-1)]
		}
		return string(b)
	}
	out := make([]byte, n)
	for i, v := range b {
		out[i] = digits[int(v)%len(digits)]
	}
	return string(out)
}

// alnumString returns n random alphanumeric characters (0-9A-Za-z), matching
// the shape of Stripe-style object IDs.
func alnumString(n int) string {
	const digits = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		for i := range b {
			b[i] = digits[gofakeit.Number(0, len(digits)-1)]
		}
		return string(b)
	}
	out := make([]byte, n)
	for i, v := range b {
		out[i] = digits[int(v)%len(digits)]
	}
	return string(out)
}

// weightedPick picks one of items using the parallel weights slice (higher
// weight = more likely). Panics if the slices differ in length or are empty,
// which indicates a programmer error in a generator definition.
func weightedPick(items []string, weights []int) string {
	total := 0
	for _, w := range weights {
		total += w
	}
	r := gofakeit.Number(0, total-1)
	for i, w := range weights {
		if r < w {
			return items[i]
		}
		r -= w
	}
	return items[len(items)-1]
}

// kebabSlug joins between minWords and maxWords words picked uniformly from
// pool into a kebab-cased slug.
func kebabSlug(pool []string, minWords, maxWords int) string {
	n := minWords
	if maxWords > minWords {
		n = gofakeit.Number(minWords, maxWords)
	}
	words := make([]string, n)
	for i := range words {
		words[i] = pickFrom(pool)
	}
	return strings.Join(words, "-")
}
