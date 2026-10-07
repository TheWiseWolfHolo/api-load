package control

import (
	"fmt"
	"net/http"
	"testing"

	"gpt-load/internal/pricing"
	"gpt-load/internal/storage/models"
	"gpt-load/internal/usage"
)

func TestModelPriceHTTPFixedRequestPriceAndReset(t *testing.T) {
	fixture, engine, row := newModelPriceHTTPFixture(t, true)
	path := fmt.Sprintf("/api/model-prices/%d", row.ID)
	for _, value := range []string{"0.0025", "0"} {
		body := fmt.Sprintf(`{"billing_unit":"request","request_price":%q,"input":null,"output":null,"cache_read":null,"cache_write":null,"context_tiers":[],"mode_schedules":{},"confirm_unpriced":false}`, value)
		response := serveModelPriceHTTPRequest(engine, http.MethodPut, path, body, authTestKey)
		if response.Code != http.StatusOK {
			t.Fatalf("fixed update = %d %s", response.Code, response.Body.String())
		}
		data := decodeModelPriceHTTPData(t, response)
		if data["billing_unit"] != "request" || data["request_price"] != value {
			t.Fatalf("fixed response = %#v", data)
		}
		var persisted models.ModelPrice
		if err := fixture.db.First(&persisted, row.ID).Error; err != nil {
			t.Fatal(err)
		}
		if persisted.RequestPriceNanoUSD == nil || persisted.InputPriceNanoUSDPerMillionTokens != nil {
			t.Fatalf("stored fixed price = %#v", persisted)
		}
		quote, receipt := fixture.priceRuntime.Load().QuoteWithReceipt(pricing.Identity{ChannelID: row.ChannelID, ModelID: row.ModelID}, usage.Result{State: usage.StateNotApplicable})
		if quote.State != pricing.CostStatePriced || int64(quote.EstimatedCostNanoUSD) != *persisted.RequestPriceNanoUSD || receipt == nil {
			t.Fatalf("published fixed quote = %#v", quote)
		}
	}
	for _, body := range []string{
		`{"billing_unit":"request","request_price":"1","input":"1","output":null,"cache_read":null,"cache_write":null,"context_tiers":[],"mode_schedules":{}}`,
		`{"billing_unit":"request","request_price":"-1","input":null,"output":null,"cache_read":null,"cache_write":null,"context_tiers":[],"mode_schedules":{}}`,
		`{"billing_unit":"request","input":null,"output":null,"cache_read":null,"cache_write":null,"context_tiers":[],"mode_schedules":{}}`,
	} {
		response := serveModelPriceHTTPRequest(engine, http.MethodPut, path, body, authTestKey)
		if response.Code == http.StatusOK {
			t.Fatalf("invalid fixed price accepted %s", body)
		}
	}
	reset := serveModelPriceHTTPRequest(engine, http.MethodPost, path+"/reset", `{}`, authTestKey)
	if reset.Code != http.StatusOK {
		t.Fatalf("reset = %d %s", reset.Code, reset.Body.String())
	}
	data := decodeModelPriceHTTPData(t, reset)
	if data["billing_unit"] != "token" || data["request_price"] != nil {
		t.Fatalf("reset fixed price = %#v", data)
	}
}
