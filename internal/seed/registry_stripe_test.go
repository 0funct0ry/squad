package seed

import (
	"regexp"
	"testing"
)

func TestStripeGenerators(t *testing.T) {
	cases := []struct {
		name string
		re   *regexp.Regexp
	}{
		{"stripe.customerID", regexp.MustCompile(`^cus_[a-zA-Z0-9]{14}$`)},
		{"stripe.chargeID", regexp.MustCompile(`^ch_[a-zA-Z0-9]{24}$`)},
		{"stripe.paymentIntentID", regexp.MustCompile(`^pi_[a-zA-Z0-9]{24}$`)},
		{"stripe.cardLast4", regexp.MustCompile(`^\d{4}$`)},
		{"stripe.cardBrand", nil},
		{"stripe.invoiceNumber", regexp.MustCompile(`^[A-Z]+-\d{5}$`)},
		{"stripe.subscriptionID", regexp.MustCompile(`^sub_[a-zA-Z0-9]{14}$`)},
		{"stripe.planName", regexp.MustCompile(`^[A-Za-z]+ [A-Za-z]+$`)},
		{"stripe.couponCode", regexp.MustCompile(`^[A-Z0-9]+$`)},
		{"stripe.webhookEventType", nil},
		{"stripe.payoutID", regexp.MustCompile(`^po_[a-zA-Z0-9]{24}$`)},
		{"stripe.currencyCode", nil},
		{"stripe.productName", nil},
		{"stripe.priceID", regexp.MustCompile(`^price_[a-zA-Z0-9]{14}$`)},
		{"stripe.refundID", regexp.MustCompile(`^re_[a-zA-Z0-9]{24}$`)},
		{"stripe.disputeReason", nil},
		{"stripe.balanceTransactionID", regexp.MustCompile(`^txn_[a-zA-Z0-9]{24}$`)},
		{"stripe.metadataKey", nil},
		{"stripe.taxRateID", regexp.MustCompile(`^txr_[a-zA-Z0-9]{14}$`)},
	}

	if !Exists("stripe.customerID") {
		t.Fatal("expected stripe.customerID to be registered")
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if !Exists(tc.name) {
				t.Fatalf("generator %s not registered", tc.name)
			}
			for i := 0; i < 20; i++ {
				v, err := Generate(tc.name, "TEXT", nil)
				if err != nil {
					t.Fatalf("Generate(%s) error: %v", tc.name, err)
				}
				s, ok := v.(string)
				if !ok || s == "" {
					t.Fatalf("Generate(%s) returned empty/non-string value: %#v", tc.name, v)
				}
				if tc.re != nil && !tc.re.MatchString(s) {
					t.Fatalf("Generate(%s) = %q does not match %s", tc.name, s, tc.re.String())
				}
			}
		})
	}
}

func TestStripeAPIKeyPrefixNeverLooksReal(t *testing.T) {
	if !Exists("stripe.apiKeyPrefix") {
		t.Fatal("expected stripe.apiKeyPrefix to be registered")
	}
	// A real Stripe secret key is a prefix followed by ~24+ random chars.
	// Assert our output never gets anywhere close to that length.
	for i := 0; i < 50; i++ {
		v, err := Generate("stripe.apiKeyPrefix", "TEXT", nil)
		if err != nil {
			t.Fatalf("Generate error: %v", err)
		}
		s, ok := v.(string)
		if !ok || s == "" {
			t.Fatalf("expected non-empty string, got %#v", v)
		}
		if len(s) > 12 {
			t.Fatalf("stripe.apiKeyPrefix output too long to be obviously fake: %q (len %d)", s, len(s))
		}
	}
}
