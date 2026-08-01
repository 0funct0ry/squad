package udf

import (
	"crypto/rand"
	"database/sql/driver"
	"fmt"

	"modernc.org/sqlite"
)

const catMisc = "Misc / utility"

const randomStringAlphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"

func randomString(n int) (string, error) {
	if n < 0 {
		return "", fmt.Errorf("RANDOM_STRING: length must be >= 0")
	}
	out := make([]byte, n)
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	for i, b := range buf {
		out[i] = randomStringAlphabet[int(b)%len(randomStringAlphabet)]
	}
	return string(out), nil
}

func registerMisc() error {
	if err := sqlite.RegisterScalarFunction("RANDOM_STRING", 1,
		func(ctx *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			n, err := argInt(args[0])
			if err != nil {
				return nil, err
			}
			return randomString(int(n))
		}); err != nil {
		return err
	}
	add(Descriptor{Name: "RANDOM_STRING", Category: catMisc, Signature: "RANDOM_STRING(len) -> str",
		Description: "Generates a random alphanumeric string of length len. Non-deterministic by design.",
		ExampleCall: `RANDOM_STRING(8)`, ExampleResult: "e.g. 'aZ3kQ9pL'",
		Deterministic: false})

	return nil
}
