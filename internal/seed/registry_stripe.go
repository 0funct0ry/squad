package seed

// Stripe-category generators produce object IDs and key-prefix strings that
// mimic Stripe's real formatting conventions for realism, but every value is
// synthetic test-fixture data. In particular stripe.apiKeyPrefix never emits
// anything long or structured enough to function as a usable secret — it is
// a short prefix plus a few random characters, not a real API key.

import (
	"fmt"
	"strings"

	"github.com/0funct0ry/squad/internal/seed/data"
	"github.com/brianvoe/gofakeit/v7"
)

// genStripeInvoiceNumber builds a "<COMPANY>-<5digit>" invoice number in
// Stripe's default style.
func genStripeInvoiceNumber() string {
	prefix := strings.ToUpper(pickFrom(data.StripeProductWords))
	if len(prefix) > 4 {
		prefix = prefix[:4]
	}
	return fmt.Sprintf("%s-%05d", prefix, gofakeit.Number(0, 99999))
}

// genStripePlanName builds a "<Product> <Tier>" plan name.
func genStripePlanName() string {
	return pickFrom(data.StripeProductWords) + " " + pickFrom(data.StripePlanTiers)
}

// genStripeCouponCode builds either a readable word+digits coupon (e.g.
// "SAVE20") or a random uppercase alnum code, 6-10 chars.
func genStripeCouponCode() string {
	if gofakeit.Bool() {
		return pickFrom(data.StripeCouponWords) + fmt.Sprintf("%d", gofakeit.Number(5, 50))
	}
	n := gofakeit.Number(6, 10)
	return strings.ToUpper(alnumString(n))
}

// genStripeAPIKeyPrefix returns a Stripe-style key prefix plus a short
// non-secret suffix. This deliberately never resembles a full, usable key.
func genStripeAPIKeyPrefix() string {
	prefix := weightedPick(data.StripeAPIKeyPrefixes, data.StripeAPIKeyPrefixWeights)
	if gofakeit.Bool() {
		return prefix + alnumString(4)
	}
	return prefix
}

// genStripeCardLast4 returns 4 random digits, matching Stripe's own
// representation of a card's last 4 digits.
func genStripeCardLast4() string {
	s := ""
	for i := 0; i < 4; i++ {
		s += fmt.Sprintf("%d", gofakeit.Number(0, 9))
	}
	return s
}

func stripeGenerators() []GeneratorDef {
	return []GeneratorDef{
		{Name: "stripe.customerID", Group: "stripe", Description: "Stripe customer ID", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return "cus_" + alnumString(14), nil
		}},

		{Name: "stripe.chargeID", Group: "stripe", Description: "Stripe charge ID", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return "ch_" + alnumString(24), nil
		}},

		{Name: "stripe.paymentIntentID", Group: "stripe", Description: "Stripe payment intent ID", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return "pi_" + alnumString(24), nil
		}},

		{Name: "stripe.cardLast4", Group: "stripe", Description: "Last 4 digits of a card number", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return genStripeCardLast4(), nil
		}},

		{Name: "stripe.cardBrand", Group: "stripe", Description: "Card brand", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return weightedPick(data.StripeCardBrands, data.StripeCardBrandWeights), nil
		}},

		{Name: "stripe.invoiceNumber", Group: "stripe", Description: "Stripe-style invoice number", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return genStripeInvoiceNumber(), nil
		}},

		{Name: "stripe.subscriptionID", Group: "stripe", Description: "Stripe subscription ID", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return "sub_" + alnumString(14), nil
		}},

		{Name: "stripe.planName", Group: "stripe", Description: "Subscription plan name", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return genStripePlanName(), nil
		}},

		{Name: "stripe.couponCode", Group: "stripe", Description: "Coupon code", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return genStripeCouponCode(), nil
		}},

		{Name: "stripe.webhookEventType", Group: "stripe", Description: "Stripe webhook event type", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return weightedPick(data.StripeWebhookEventTypes, data.StripeWebhookEventWeights), nil
		}},

		{Name: "stripe.payoutID", Group: "stripe", Description: "Stripe payout ID", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return "po_" + alnumString(24), nil
		}},

		{Name: "stripe.currencyCode", Group: "stripe", Description: "ISO 4217 currency code", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return weightedPick(data.StripeCurrencyCodes, data.StripeCurrencyWeights), nil
		}},

		{Name: "stripe.productName", Group: "stripe", Description: "Product name", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return pickFrom(data.StripeProductWords), nil
		}},

		{Name: "stripe.priceID", Group: "stripe", Description: "Stripe price ID", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return "price_" + alnumString(14), nil
		}},

		{Name: "stripe.refundID", Group: "stripe", Description: "Stripe refund ID", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return "re_" + alnumString(24), nil
		}},

		{Name: "stripe.disputeReason", Group: "stripe", Description: "Stripe dispute reason", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return weightedPick(data.StripeDisputeReasons, data.StripeDisputeReasonWeights), nil
		}},

		{Name: "stripe.apiKeyPrefix", Group: "stripe", Description: "Stripe API key prefix (never a full/usable key)", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return genStripeAPIKeyPrefix(), nil
		}},

		{Name: "stripe.balanceTransactionID", Group: "stripe", Description: "Stripe balance transaction ID", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return "txn_" + alnumString(24), nil
		}},

		{Name: "stripe.metadataKey", Group: "stripe", Description: "Common Stripe object metadata key", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return pickFrom(data.StripeMetadataKeys), nil
		}},

		{Name: "stripe.taxRateID", Group: "stripe", Description: "Stripe tax rate ID", Affinities: []string{"TEXT"}, Fn: func(string, map[string]any) (any, error) {
			return "txr_" + alnumString(14), nil
		}},
	}
}
