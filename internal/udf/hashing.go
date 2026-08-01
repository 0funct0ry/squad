package udf

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"database/sql/driver"
	"encoding/base64"
	"encoding/hex"
	"hash/crc32"

	"github.com/google/uuid"
	"modernc.org/sqlite"
)

const catHashing = "Hashing / encoding"

func registerHashing() error {
	type hashFn struct {
		name string
		fn   func([]byte) []byte
	}
	hashes := []hashFn{
		{"SHA256", func(b []byte) []byte { h := sha256.Sum256(b); return h[:] }},
		{"SHA1", func(b []byte) []byte { h := sha1.Sum(b); return h[:] }},
		{"MD5", func(b []byte) []byte { h := md5.Sum(b); return h[:] }},
	}
	for _, h := range hashes {
		h := h
		if err := sqlite.RegisterDeterministicScalarFunction(h.name, 1,
			func(ctx *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
				return hex.EncodeToString(h.fn(argBytes(args[0]))), nil
			}); err != nil {
			return err
		}
	}
	add(Descriptor{Name: "SHA256", Category: catHashing, Signature: "SHA256(str) -> str",
		Description: "Hex-encoded SHA-256 digest.", ExampleCall: `SHA256('squad')`,
		ExampleResult: "3b0e7...(64-char hex digest)", Deterministic: true})
	add(Descriptor{Name: "SHA1", Category: catHashing, Signature: "SHA1(str) -> str",
		Description: "Hex-encoded SHA-1 digest.", ExampleCall: `SHA1('squad')`,
		ExampleResult: "(40-char hex digest)", Deterministic: true})
	add(Descriptor{Name: "MD5", Category: catHashing, Signature: "MD5(str) -> str",
		Description: "Hex-encoded MD5 digest.", ExampleCall: `MD5('squad')`,
		ExampleResult: "(32-char hex digest)", Deterministic: true})

	if err := sqlite.RegisterDeterministicScalarFunction("BASE64_ENCODE", 1,
		func(ctx *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			return base64.StdEncoding.EncodeToString(argBytes(args[0])), nil
		}); err != nil {
		return err
	}
	add(Descriptor{Name: "BASE64_ENCODE", Category: catHashing, Signature: "BASE64_ENCODE(blob) -> str",
		Description: "Standard Base64 encode.", ExampleCall: `BASE64_ENCODE('squad')`,
		ExampleResult: "c3F1YWQ=", Deterministic: true})

	if err := sqlite.RegisterDeterministicScalarFunction("BASE64_DECODE", 1,
		func(ctx *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			b, err := base64.StdEncoding.DecodeString(argString(args[0]))
			if err != nil {
				return nil, err
			}
			return b, nil
		}); err != nil {
		return err
	}
	add(Descriptor{Name: "BASE64_DECODE", Category: catHashing, Signature: "BASE64_DECODE(str) -> blob",
		Description: "Standard Base64 decode.", ExampleCall: `BASE64_DECODE('c3F1YWQ=')`,
		ExampleResult: "squad", Deterministic: true})

	if err := sqlite.RegisterDeterministicScalarFunction("HEX_ENCODE", 1,
		func(ctx *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			return hex.EncodeToString(argBytes(args[0])), nil
		}); err != nil {
		return err
	}
	add(Descriptor{Name: "HEX_ENCODE", Category: catHashing, Signature: "HEX_ENCODE(blob) -> str",
		Description: "Hex encode.", ExampleCall: `HEX_ENCODE('AB')`,
		ExampleResult: "4142", Deterministic: true})

	if err := sqlite.RegisterDeterministicScalarFunction("HEX_DECODE", 1,
		func(ctx *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			b, err := hex.DecodeString(argString(args[0]))
			if err != nil {
				return nil, err
			}
			return b, nil
		}); err != nil {
		return err
	}
	add(Descriptor{Name: "HEX_DECODE", Category: catHashing, Signature: "HEX_DECODE(str) -> blob",
		Description: "Hex decode (SQLite ships hex() for encoding but has no built-in decode).",
		ExampleCall: `HEX_DECODE('4142')`, ExampleResult: "AB", Deterministic: true})

	if err := sqlite.RegisterScalarFunction("UUID", 0,
		func(ctx *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			return uuid.NewString(), nil
		}); err != nil {
		return err
	}
	add(Descriptor{Name: "UUID", Category: catHashing, Signature: "UUID() -> str",
		Description: "Generates a random UUIDv4. Non-deterministic by design, like random().",
		ExampleCall: `UUID()`, ExampleResult: "b3f2c9de-8a3f-4e11-9f2a-8f9d2b6b6b10", Deterministic: false})

	if err := sqlite.RegisterDeterministicScalarFunction("CRC32", 1,
		func(ctx *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			return int64(crc32.ChecksumIEEE(argBytes(args[0]))), nil
		}); err != nil {
		return err
	}
	add(Descriptor{Name: "CRC32", Category: catHashing, Signature: "CRC32(blob) -> int",
		Description: "32-bit CRC checksum, useful for cheap integrity/change checks.",
		ExampleCall: `CRC32('squad')`, ExampleResult: "(a 32-bit integer)", Deterministic: true})

	return nil
}
