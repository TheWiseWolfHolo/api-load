package keypool

import "testing"

func TestSCH008QuotaErrorClassifierOnlyMatchesExplicitQuotaOrBillingFailures(t *testing.T) {
	quotaMessages := []string{
		`{"error":{"code":"insufficient_quota"}}`,
		"quota_exceeded: monthly limit reached",
		"billing_hard_limit reached",
		"Your credit balance is too low to access the Anthropic API",
		"You exceeded your current quota, please check your plan and billing details",
		"custom vendor says balance exhausted",
	}
	for _, message := range quotaMessages {
		if !IsQuotaOrBillingFailure(message, []string{"balance exhausted"}) {
			t.Fatalf("expected quota/billing failure for %q", message)
		}
	}

	transientMessages := []string{
		"rate limited",
		"HTTP 429 too many requests",
		"500 internal server error",
		"502 bad gateway",
		"503 service unavailable",
	}
	for _, message := range transientMessages {
		if IsQuotaOrBillingFailure(message, []string{"balance exhausted"}) {
			t.Fatalf("did not expect quota/billing failure for %q", message)
		}
	}
}
