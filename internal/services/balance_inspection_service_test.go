package services

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestINS001DetectsOnlyKnownOfficialBalanceHosts(t *testing.T) {
	tests := []struct {
		baseURL  string
		provider string
	}{
		{"https://api.deepseek.com", "deepseek"},
		{"https://openrouter.ai/api", "openrouter"},
		{"https://api.siliconflow.com/v1", "siliconflow"},
		{"https://api.siliconflow.cn", "siliconflow"},
		{"https://api.moonshot.cn/v1", "moonshot"},
		{"https://api.moonshot.ai", "moonshot"},
		{"https://proxy.example.com/deepseek", ""},
	}
	for _, tc := range tests {
		t.Run(tc.baseURL, func(t *testing.T) {
			adapter, _ := balanceAdapterForBaseURL(tc.baseURL)
			got := ""
			if adapter != nil {
				got = adapter.provider
			}
			if got != tc.provider {
				t.Fatalf("provider = %q, want %q", got, tc.provider)
			}
		})
	}
}

func TestINS002BalanceAdaptersNormalizeProviderResponses(t *testing.T) {
	tests := []struct {
		name     string
		adapter  balanceAdapter
		body     string
		expected BalanceSnapshot
	}{
		{
			name: "deepseek", adapter: deepSeekBalanceAdapter(),
			body: "{\"is_available\":true,\"balance_infos\":[{\"currency\":\"USD\",\"total_balance\":\"12.50\",\"granted_balance\":\"2.50\",\"topped_up_balance\":\"10.00\"}]}",
			expected: BalanceSnapshot{Provider: "deepseek", Available: boolPointer(true), Balances: []BalanceAmount{
				{Kind: "total", Amount: "12.50", Currency: "USD"},
				{Kind: "granted", Amount: "2.50", Currency: "USD"},
				{Kind: "topped_up", Amount: "10.00", Currency: "USD"},
			}},
		},
		{
			name: "openrouter", adapter: openRouterBalanceAdapter(),
			body: "{\"data\":{\"limit\":50,\"limit_remaining\":17.5,\"usage\":32.5,\"usage_daily\":1.25}}",
			expected: BalanceSnapshot{Provider: "openrouter", Balances: []BalanceAmount{
				{Kind: "limit_remaining", Amount: "17.5", Currency: "credits"},
				{Kind: "limit", Amount: "50", Currency: "credits"},
			}, Metrics: []BalanceMetric{{Kind: "usage", Value: "32.5"}, {Kind: "usage_daily", Value: "1.25"}}},
		},
		{
			name: "siliconflow", adapter: siliconFlowBalanceAdapter(),
			body: "{\"code\":20000,\"status\":true,\"data\":{\"balance\":\"0.88\",\"chargeBalance\":\"88.00\",\"totalBalance\":\"88.88\"}}",
			expected: BalanceSnapshot{Provider: "siliconflow", Available: boolPointer(true), Balances: []BalanceAmount{
				{Kind: "balance", Amount: "0.88", Currency: "credits"},
				{Kind: "charge", Amount: "88.00", Currency: "credits"},
				{Kind: "total", Amount: "88.88", Currency: "credits"},
			}},
		},
		{
			name: "moonshot", adapter: moonshotBalanceAdapter("CNY"),
			body: "{\"code\":0,\"status\":true,\"data\":{\"available_balance\":49.5,\"voucher_balance\":46.5,\"cash_balance\":3}}",
			expected: BalanceSnapshot{Provider: "moonshot", Available: boolPointer(true), Balances: []BalanceAmount{
				{Kind: "available", Amount: "49.5", Currency: "CNY"},
				{Kind: "voucher", Amount: "46.5", Currency: "CNY"},
				{Kind: "cash", Amount: "3", Currency: "CNY"},
			}},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != "Bearer dummy-key" {
					t.Fatalf("missing bearer auth")
				}
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()
			got, err := queryBalanceWithAdapter(context.Background(), server.Client(), server.URL, "dummy-key", tc.adapter)
			if err != nil {
				t.Fatalf("query balance: %v", err)
			}
			if !reflect.DeepEqual(got, tc.expected) {
				t.Fatalf("snapshot = %#v, want %#v", got, tc.expected)
			}
		})
	}
}
