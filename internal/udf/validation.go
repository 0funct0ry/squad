package udf

import (
	"database/sql/driver"
	"encoding/json"
	"net"
	"net/url"
	"regexp"

	"github.com/yl2chen/cidranger"
	"golang.org/x/net/idna"
	"modernc.org/sqlite"
)

const catValidation = "Validation"

var emailRe = regexp.MustCompile(`^[a-zA-Z0-9.!#$%&'*+/=?^_` + "`" + `{|}~-]+@[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(?:\.[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)+$`)
var domainLabelRe = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?$`)

func luhnCheck(s string) bool {
	sum := 0
	alt := false
	for i := len(s) - 1; i >= 0; i-- {
		c := s[i]
		if c < '0' || c > '9' {
			return false
		}
		d := int(c - '0')
		if alt {
			d *= 2
			if d > 9 {
				d -= 9
			}
		}
		sum += d
		alt = !alt
	}
	return len(s) > 0 && sum%10 == 0
}

func registerValidation() error {
	if err := sqlite.RegisterDeterministicScalarFunction("IS_VALID_EMAIL", 1,
		func(ctx *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			return boolResult(emailRe.MatchString(argString(args[0]))), nil
		}); err != nil {
		return err
	}
	add(Descriptor{Name: "IS_VALID_EMAIL", Category: catValidation, Signature: "IS_VALID_EMAIL(str) -> bool",
		Description: "Validates a string looks like an email address.",
		ExampleCall: "SELECT email FROM users WHERE NOT IS_VALID_EMAIL(email)", ExampleResult: "(bad rows)",
		Deterministic: true})

	if err := sqlite.RegisterDeterministicScalarFunction("IS_VALID_JSON", 1,
		func(ctx *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			return boolResult(json.Valid(argBytes(args[0]))), nil
		}); err != nil {
		return err
	}
	add(Descriptor{Name: "IS_VALID_JSON", Category: catValidation, Signature: "IS_VALID_JSON(str) -> bool",
		Description: "1 if str is valid JSON.", ExampleCall: `IS_VALID_JSON('{"a":1}')`, ExampleResult: "1",
		Deterministic: true})

	if err := sqlite.RegisterDeterministicScalarFunction("IS_VALID_URL", 1,
		func(ctx *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			u, err := url.ParseRequestURI(argString(args[0]))
			return boolResult(err == nil && u.Scheme != "" && u.Host != ""), nil
		}); err != nil {
		return err
	}
	add(Descriptor{Name: "IS_VALID_URL", Category: catValidation, Signature: "IS_VALID_URL(str) -> bool",
		Description: "1 if str is a valid, absolute URL.", ExampleCall: `IS_VALID_URL('https://squad.dev')`,
		ExampleResult: "1", Deterministic: true})

	if err := sqlite.RegisterDeterministicScalarFunction("LUHN_CHECK", 1,
		func(ctx *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			return boolResult(luhnCheck(argString(args[0]))), nil
		}); err != nil {
		return err
	}
	add(Descriptor{Name: "LUHN_CHECK", Category: catValidation, Signature: "LUHN_CHECK(str) -> bool",
		Description: "Validates credit-card-style Luhn checksums.", ExampleCall: `LUHN_CHECK('4111111111111111')`,
		ExampleResult: "1", Deterministic: true})

	if err := sqlite.RegisterDeterministicScalarFunction("IS_VALID_IP", 1,
		func(ctx *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			return boolResult(net.ParseIP(argString(args[0])) != nil), nil
		}); err != nil {
		return err
	}
	add(Descriptor{Name: "IS_VALID_IP", Category: catValidation, Signature: "IS_VALID_IP(str) -> bool",
		Description: "1 if str is a valid IPv4/IPv6 address.", ExampleCall: `IS_VALID_IP('192.168.1.1')`,
		ExampleResult: "1", Deterministic: true})

	if err := sqlite.RegisterDeterministicScalarFunction("IP_IN_CIDR", 2,
		func(ctx *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			ip := net.ParseIP(argString(args[0]))
			if ip == nil {
				return int64(0), nil
			}
			_, network, err := net.ParseCIDR(argString(args[1]))
			if err != nil {
				return nil, err
			}
			ranger := cidranger.NewPCTrieRanger()
			if err := ranger.Insert(cidranger.NewBasicRangerEntry(*network)); err != nil {
				return nil, err
			}
			ok, err := ranger.Contains(ip)
			if err != nil {
				return nil, err
			}
			return boolResult(ok), nil
		}); err != nil {
		return err
	}
	add(Descriptor{Name: "IP_IN_CIDR", Category: catValidation, Signature: "IP_IN_CIDR(ip, cidr) -> bool",
		Description: "1 if ip falls within cidr.", ExampleCall: `IP_IN_CIDR('192.168.1.5', '192.168.1.0/24')`,
		ExampleResult: "1", Deterministic: true})

	if err := sqlite.RegisterDeterministicScalarFunction("IS_VALID_DOMAIN", 1,
		func(ctx *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			ascii, err := idna.ToASCII(argString(args[0]))
			if err != nil {
				return int64(0), nil
			}
			labels := regexp.MustCompile(`\.`).Split(ascii, -1)
			if len(labels) < 2 {
				return int64(0), nil
			}
			for _, l := range labels {
				if !domainLabelRe.MatchString(l) {
					return int64(0), nil
				}
			}
			return int64(1), nil
		}); err != nil {
		return err
	}
	add(Descriptor{Name: "IS_VALID_DOMAIN", Category: catValidation, Signature: "IS_VALID_DOMAIN(str) -> bool",
		Description: "1 if str is a syntactically valid domain name.", ExampleCall: `IS_VALID_DOMAIN('example.com')`,
		ExampleResult: "1", Deterministic: true})

	return nil
}
