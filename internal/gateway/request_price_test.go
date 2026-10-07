package gateway

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"gpt-load/internal/accessquota"
	"gpt-load/internal/channel"
	"gpt-load/internal/health"
	"gpt-load/internal/pricing"
	"gpt-load/internal/protocol"
	"gpt-load/internal/telemetry"
	"gpt-load/internal/usage"
)

func TestFixedRequestCostRequiresSuccessfulFinalAttempt(t *testing.T) {
	price := pricing.NanoUSD(2_500_000)
	model := "request-price-test"
	table, err := pricing.NewTable([]pricing.Rule{{Identity: pricing.Identity{ChannelID: string(channel.OpenAI), ModelID: model}, BillingUnit: pricing.BillingUnitRequest, RequestPrice: &price}})
	if err != nil {
		t.Fatal(err)
	}
	for _, status := range []int{http.StatusOK, http.StatusBadRequest, http.StatusBadGateway} {
		for _, state := range []usage.State{usage.StateComplete, usage.StateMissing, usage.StateNotApplicable} {
			recorder := newRequestRecorder(&recordingRequestLogSink{}, "fixed-request", time.Unix(100, 0), 1, protocol.OpenAICompletions, func() time.Time { return time.Unix(101, 0) })
			recorder.freezeNextAttemptPricing(frozenAttemptPricing{channelID: string(channel.OpenAI), groupID: 1, upstreamModel: model, table: table, applicable: state != usage.StateNotApplicable, priceMultipliers: pricing.PriceMultipliers{Group: 1_500_000, AccessKey: pricing.DefaultPriceMultiplier}})
			selection := requestLogSelection(1, 2, "group")
			selection.UpstreamModelID = &model
			index := recorder.appendAttempt(selection, UpstreamResult{StatusCode: status}, telemetry.FailureCategoryOK, telemetry.ActionTerminate, "", "", time.Unix(100, 0), time.Unix(101, 0))
			recorder.completeResponse(UpstreamResult{StatusCode: status, Usage: usage.Result{State: state}}, health.Decision{}, model, index)
			if status != http.StatusOK {
				if recorder.estimatedCostNanoUSD() != 0 || recorder.usage.Pricing.ReceiptJSON != "" {
					t.Fatal("failed request charged")
				}
				continue
			}
			if recorder.estimatedCostNanoUSD() != 3_750_000 {
				t.Fatalf("quota cost = %d", recorder.estimatedCostNanoUSD())
			}
			var receipt pricing.Receipt
			if err := json.Unmarshal([]byte(recorder.usage.Pricing.ReceiptJSON), &receipt); err != nil {
				t.Fatal(err)
			}
			if err := pricing.ValidateReceipt(receipt); err != nil {
				t.Fatal(err)
			}
		}
	}
	for _, success := range []bool{false, true} {
		observed := quoteFrozenAttempt(frozenAttemptPricing{channelID: string(channel.OpenAI), upstreamModel: model, table: table, priceMultipliers: pricing.PriceMultipliers{Group: pricing.DefaultPriceMultiplier, AccessKey: pricing.DefaultPriceMultiplier}}, usage.Result{State: usage.StatePartial}, pricing.ModeStandard, success)
		if !success && observed.EstimatedCostNanoUSD != 0 {
			t.Fatal("incomplete/canceled stream charged")
		}
	}
}

func TestHandlerFixedRequestPriceChargesQuotaWithoutTokenUsage(t *testing.T) {
	for _, status := range []int{http.StatusOK, http.StatusBadRequest} {
		forwarder := &scriptedForwarder{results: []UpstreamResult{{StatusCode: status, Header: make(http.Header), Body: []byte(`{"ok":true}`), RequestWritten: true, Usage: usage.Result{State: usage.StateMissing}}}}
		engine, handler, _, _ := newRequestLogHandlerTestRuntime(t, forwarder, &recordingAccessKeyRPMLimiter{}, &recordingRequestLogSink{}, "sk-first")
		price := pricing.NanoUSD(2_500_000)
		table, err := pricing.NewTable([]pricing.Rule{{Identity: pricing.Identity{ChannelID: string(channel.OpenAI), ModelID: "gpt-4o"}, BillingUnit: pricing.BillingUnitRequest, RequestPrice: &price}})
		if err != nil {
			t.Fatal(err)
		}
		runtime := accessquota.NewRuntime()
		if err := runtime.Reconcile(map[uint][]accessquota.Rule{1: {{ID: 501, Revision: 1, Kind: accessquota.KindTotal, LimitNanoUSD: int64(price)}}}); err != nil {
			t.Fatal(err)
		}
		handler.accessQuota = runtime
		handler.priceTables = &mutableGatewayPriceTableProvider{table: table}
		handler.now = func() time.Time { return time.Unix(4_000, 0) }
		request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-4o"}`))
		request.Header.Set("Authorization", "Bearer gl-client")
		response := httptest.NewRecorder()
		engine.ServeHTTP(response, request)
		if response.Code != status {
			t.Fatalf("response = %d, want %d", response.Code, status)
		}
		view := runtime.Snapshot(1, time.Unix(4_001, 0))
		want := int64(0)
		if status == http.StatusOK {
			want = int64(price)
		}
		if len(view.Rules) != 1 || view.Rules[0].UsedNanoUSD != want {
			t.Fatalf("quota = %#v, want %d", view, want)
		}
		if status == http.StatusOK {
			blocked := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
			blocked.Header.Set("Authorization", "Bearer gl-client")
			blockedResponse := httptest.NewRecorder()
			engine.ServeHTTP(blockedResponse, blocked)
			if blockedResponse.Code != http.StatusTooManyRequests {
				t.Fatalf("exhausted quota response = %d", blockedResponse.Code)
			}
		}
	}
}

func TestFixedRequestStreamCompletionChargesOnlyCleanEOF(t *testing.T) {
	price := pricing.NanoUSD(2_500_000)
	model := "request-price-stream"
	table, err := pricing.NewTable([]pricing.Rule{{Identity: pricing.Identity{ChannelID: string(channel.OpenAI), ModelID: model}, BillingUnit: pricing.BillingUnitRequest, RequestPrice: &price}})
	if err != nil {
		t.Fatal(err)
	}
	for _, reason := range []StreamEndReason{StreamEndCleanEOF, StreamEndSSEError, StreamEndClientCanceled, StreamEndServerShutdown} {
		recorder := newRequestRecorder(&recordingRequestLogSink{}, "fixed-stream", time.Unix(100, 0), 1, protocol.OpenAICompletions, func() time.Time { return time.Unix(101, 0) })
		recorder.freezeNextAttemptPricing(frozenAttemptPricing{channelID: string(channel.OpenAI), groupID: 1, upstreamModel: model, table: table, applicable: true, priceMultipliers: pricing.PriceMultipliers{Group: pricing.DefaultPriceMultiplier, AccessKey: pricing.DefaultPriceMultiplier}})
		selection := requestLogSelection(1, 2, "group")
		selection.UpstreamModelID = &model
		result := UpstreamResult{StatusCode: http.StatusOK, Usage: usage.Result{State: usage.StateMissing}, Stream: StreamObservation{EndReason: reason}}
		index := recorder.appendAttempt(selection, result, telemetry.FailureCategoryOK, telemetry.ActionTerminate, "", "", time.Unix(100, 0), time.Unix(101, 0))
		recorder.completeStream(result, model, index)
		want := int64(0)
		if reason == StreamEndCleanEOF {
			want = int64(price)
		}
		if recorder.estimatedCostNanoUSD() != want {
			t.Fatalf("stream %v cost = %d, want %d", reason, recorder.estimatedCostNanoUSD(), want)
		}
	}
}
