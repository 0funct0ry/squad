package seed

import (
	"github.com/brianvoe/gofakeit/v7"
)

func financeGenerators() []GeneratorDef {
	return []GeneratorDef{
		{Name: "achAccount", Group: "finance", Description: "ACH account number", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return gofakeit.AchAccount(), nil
		}},
		{Name: "achRouting", Group: "finance", Description: "ACH routing number", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return gofakeit.AchRouting(), nil
		}},
		{Name: "bankName", Group: "finance", Description: "Bank name", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return gofakeit.BankName(), nil
		}},
		{Name: "bankType", Group: "finance", Description: "Bank account type", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return gofakeit.BankType(), nil
		}},
		{Name: "bitcoinAddress", Group: "finance", Description: "Bitcoin address", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return gofakeit.BitcoinAddress(), nil
		}},
		{Name: "bitcoinPrivateKey", Group: "finance", Description: "Bitcoin private key", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return gofakeit.BitcoinPrivateKey(), nil
		}},
		// ethereumAddress: gofakeit has no dedicated function, so we hand-roll
		// a plausible "0x" + 40 hex chars string via Regex.
		{Name: "ethereumAddress", Group: "finance", Description: "Ethereum address (hand-rolled)", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return "0x" + gofakeit.Regex("[a-f0-9]{40}"), nil
		}},
		{Name: "creditCardCvv", Group: "finance", Description: "Credit card CVV", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return gofakeit.CreditCardCvv(), nil
		}},
		{Name: "creditCardExp", Group: "finance", Description: "Credit card expiration", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return gofakeit.CreditCardExp(), nil
		}},
		{Name: "creditCardNumber", Group: "finance", Description: "Credit card number", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return gofakeit.CreditCardNumber(nil), nil
		}},
		{Name: "creditCardType", Group: "finance", Description: "Credit card type", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return gofakeit.CreditCardType(), nil
		}},
		{Name: "currencyLong", Group: "finance", Description: "Currency name", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return gofakeit.CurrencyLong(), nil
		}},
		{Name: "currencyShort", Group: "finance", Description: "Currency code", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return gofakeit.CurrencyShort(), nil
		}},
		// iban: hand-rolled IBAN-*shaped* string (2-letter country + 2 check
		// digits + alphanumeric). This is NOT mod-97 validated - cosmetic only.
		{Name: "iban", Group: "finance", Description: "IBAN-shaped string (not validated)", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			country := gofakeit.CountryAbr()
			check := gofakeit.Numerify("##")
			rest := gofakeit.Regex("[A-Z0-9]{16}")
			return country + check + rest, nil
		}},
		{Name: "cusip", Group: "finance", Description: "CUSIP identifier", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return gofakeit.Cusip(), nil
		}},
		{Name: "isin", Group: "finance", Description: "ISIN identifier", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return gofakeit.Isin(), nil
		}},
	}
}
