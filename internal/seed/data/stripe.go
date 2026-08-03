package data

// StripeCardBrands and StripeCardBrandWeights drive weighted card brand
// selection.
var StripeCardBrands = []string{"visa", "mastercard", "amex", "discover"}
var StripeCardBrandWeights = []int{40, 30, 20, 10}

// StripeWebhookEventTypes and StripeWebhookEventWeights drive weighted
// selection of real Stripe webhook event names.
var StripeWebhookEventTypes = []string{
	"charge.succeeded", "invoice.paid", "customer.subscription.deleted",
	"payment_intent.payment_failed", "payment_intent.succeeded",
	"customer.subscription.created", "charge.refunded", "invoice.payment_failed",
}
var StripeWebhookEventWeights = []int{20, 15, 10, 15, 15, 10, 10, 5}

// StripeCurrencyCodes and StripeCurrencyWeights drive weighted ISO 4217
// currency code selection.
var StripeCurrencyCodes = []string{"usd", "eur", "gbp", "cad", "aud"}
var StripeCurrencyWeights = []int{50, 20, 15, 10, 5}

// StripeDisputeReasons and StripeDisputeReasonWeights drive weighted
// selection of real Stripe dispute reason codes.
var StripeDisputeReasons = []string{
	"fraudulent", "duplicate", "product_not_received",
	"subscription_canceled", "credit_not_processed",
}
var StripeDisputeReasonWeights = []int{30, 20, 20, 15, 15}

// StripeAPIKeyPrefixes and StripeAPIKeyPrefixWeights drive weighted selection
// of Stripe's real API key prefix conventions. Only the prefix (plus a short
// random suffix) is ever generated — never a full key.
var StripeAPIKeyPrefixes = []string{"pk_test_", "sk_live_", "pk_live_", "sk_test_"}
var StripeAPIKeyPrefixWeights = []int{30, 20, 20, 30}

// StripePlanTiers is a curated pool of subscription plan tier names.
var StripePlanTiers = []string{"Basic", "Pro", "Team", "Enterprise"}

// StripeProductWords is a curated SaaS-vocab pool used to build product and
// plan names.
var StripeProductWords = []string{
	"Cloud", "Analytics", "Dashboard", "Insights", "Workspace", "Sync",
	"Studio", "Pipeline", "Monitor", "Suite",
}

// StripeMetadataKeys is a curated pool of common Stripe object metadata keys.
var StripeMetadataKeys = []string{
	"order_id", "user_id", "source", "campaign", "referral_code", "plan_id",
}

// StripeCouponWords is a curated pool of readable coupon-code words, combined
// with a trailing number by the generator (e.g. "SAVE20").
var StripeCouponWords = []string{"SAVE", "WELCOME", "LAUNCH", "SUMMER", "LOYAL", "BONUS"}
