package pricing

import (
	"encoding/json"
	"testing"

	"gpt-load/internal/usage"
)

func TestRequestPriceIsIndependentOfTokenUsage(t *testing.T) {
	identity := Identity{ChannelID: "openai-compatible", ModelID: "rerank-test"}
	for _, price := range []NanoUSD{0, 2_500_000} {
		table, err := NewTable([]Rule{{Identity: identity, BillingUnit: BillingUnitRequest, RequestPrice: &price}})
		if err != nil {
			t.Fatal(err)
		}
		for _, state := range []usage.State{usage.StateComplete, usage.StatePartial, usage.StateMissing, usage.StateNotApplicable} {
			quote, receipt := table.QuoteForModeWithMultipliers(identity, usage.Result{State: state, Tokens: usage.Tokens{UncachedInput: 9_000_000, Output: 3_000_000}}, ModeFast, PriceMultipliers{Group: 1_500_000, AccessKey: DefaultPriceMultiplier})
			if quote.State != CostStatePriced || quote.Completeness != CompletenessComplete || quote.EstimatedCostNanoUSD != price*3/2 {
				t.Fatalf("quote for %s = %#v", state, quote)
			}
			if receipt == nil || receipt.SchemaVersion != 7 || receipt.Method != ReceiptMethodPerRequest {
				t.Fatalf("receipt = %#v", receipt)
			}
			encoded, err := json.Marshal(receipt)
			if err != nil {
				t.Fatal(err)
			}
			var decoded Receipt
			if err := json.Unmarshal(encoded, &decoded); err != nil {
				t.Fatal(err)
			}
			if err := ValidateReceipt(decoded); err != nil {
				t.Fatal(err)
			}
			decoded.LineItems[0].Quantity = 2
			if ValidateReceipt(decoded) == nil {
				t.Fatal("tampered request quantity accepted")
			}
		}
	}
}

func TestRequestPriceRulesRejectMixedAndNegativeRates(t *testing.T) {
	identity := Identity{ChannelID: "openai-compatible", ModelID: "test"}
	negative, positive := NanoUSD(-1), NanoUSD(1)
	for _, rule := range []Rule{
		{Identity: identity, BillingUnit: BillingUnitRequest, RequestPrice: &negative},
		{Identity: identity, BillingUnit: BillingUnitToken, RequestPrice: &positive},
		{Identity: identity, BillingUnit: BillingUnitRequest, RequestPrice: &positive, Prices: Prices{Input: Price{Set: true}}},
		{Identity: identity, BillingUnit: "invalid"},
	} {
		if _, err := NewTable([]Rule{rule}); err == nil {
			t.Fatalf("invalid rule accepted %#v", rule)
		}
	}
	table, err := NewTable([]Rule{{Identity: identity, BillingUnit: BillingUnitRequest, RequestPrice: &positive}})
	if err != nil {
		t.Fatal(err)
	}
	positive = 99
	rule, _ := table.Lookup(identity)
	if *rule.RequestPrice != 1 {
		t.Fatal("table did not freeze request price")
	}
	*rule.RequestPrice = 123
	quote, _ := table.QuoteWithReceipt(identity, usage.Result{State: usage.StateMissing})
	if quote.EstimatedCostNanoUSD != 1 {
		t.Fatal("lookup mutated request price")
	}
}
