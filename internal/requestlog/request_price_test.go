package requestlog

import (
	"encoding/json"
	"testing"

	"gpt-load/internal/platform/redact"
	"gpt-load/internal/pricing"
	"gpt-load/internal/storage/models"
	"gpt-load/internal/telemetry"
	"gpt-load/internal/usage"
)

func TestFixedRequestPricePersistsAndAggregatesWithoutTokens(t *testing.T) {
	db := openRequestLogQueryDB(t)
	price := pricing.NanoUSD(2_500_000)
	identity := pricing.Identity{ChannelID: "openai", ModelID: "upstream-model"}
	table, err := pricing.NewTable([]pricing.Rule{{Identity: identity, BillingUnit: pricing.BillingUnitRequest, RequestPrice: &price}})
	if err != nil {
		t.Fatal(err)
	}
	for index, state := range []usage.State{usage.StateMissing, usage.StateNotApplicable} {
		event := testEvent(aggregationRequestID(900 + index))
		event.Usage.Result = usage.Result{State: state}
		quote, receipt := table.QuoteForModeWithMultipliers(identity, event.Usage.Result, pricing.ModeStandard, pricing.PriceMultipliers{Group: 1_500_000, AccessKey: pricing.DefaultPriceMultiplier})
		encoded, err := json.Marshal(receipt)
		if err != nil {
			t.Fatal(err)
		}
		event.Usage.Pricing = telemetry.PricingObservation{BillingUnit: "request", UpstreamModel: identity.ModelID, CostState: string(quote.State), PricingCompleteness: string(quote.Completeness), EstimatedCostNanoUSD: int64(quote.EstimatedCostNanoUSD), ReceiptJSON: string(encoded)}
		row, err := mapEvent(redact.New(), event)
		if err != nil {
			t.Fatal(err)
		}
		if err := (&gormBatchWriter{db: db}).WriteBatch(t.Context(), []models.RequestLog{row}); err != nil {
			t.Fatal(err)
		}
		if err := (&gormBatchWriter{db: db}).WriteBatch(t.Context(), []models.RequestLog{row}); err != nil {
			t.Fatal(err)
		}
	}
	service := newRequestLogTestService(db)
	total, err := service.QueryAccessKeyTotalCost(t.Context(), 42)
	if err != nil || total != 7_500_000 {
		t.Fatalf("retained key total = %d, %v", total, err)
	}
	var stat models.UsageStat
	if err := db.Where("access_key_id = ?", 42).Take(&stat).Error; err != nil {
		t.Fatal(err)
	}
	if stat.RequestCount != 2 || stat.UncachedInputTokens != 0 || stat.OutputTokens != 0 || stat.UsageMissingCount != 1 || stat.EstimatedCostNanoUSD != total {
		t.Fatalf("fixed usage aggregate = %#v", stat)
	}
	page, err := service.List(t.Context(), ListQuery{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 2 {
		t.Fatalf("fixed log page = %#v", page)
	}
	for _, record := range page.Items {
		if record.EstimatedCostNanoUSD != 3_750_000 || record.BillingUnit != pricing.BillingUnitRequest {
			t.Fatalf("fixed log record = %#v", record)
		}
	}
	if ValidateBillingUsageCostState(pricing.BillingUnitRequest, telemetry.RequestStatusError, usage.StateMissing, pricing.CostStatePriced, pricing.CompletenessComplete, 100) == nil {
		t.Fatal("failed fixed request accepted a charge")
	}
}
