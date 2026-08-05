package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

var ErrBalanceInspectionUnsupported = errors.New("balance inspection unsupported")

type BalanceAmount struct {
	Kind     string `json:"kind"`
	Amount   string `json:"amount"`
	Currency string `json:"currency"`
}

type BalanceMetric struct {
	Kind  string `json:"kind"`
	Value string `json:"value"`
}

type BalanceSnapshot struct {
	Provider   string          `json:"provider"`
	ResourceID uint            `json:"resource_id,omitempty"`
	Available  *bool           `json:"available,omitempty"`
	Balances   []BalanceAmount `json:"balances"`
	Metrics    []BalanceMetric `json:"metrics,omitempty"`
	CheckedAt  string          `json:"checked_at,omitempty"`
}

type balanceAdapter struct {
	provider string
	path     string
	parse    func([]byte) (BalanceSnapshot, error)
}

func balanceAdapterForBaseURL(baseURL string) (*balanceAdapter, string) {
	parsed, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return nil, ""
	}
	host := strings.ToLower(parsed.Hostname())
	origin := parsed.Scheme + "://" + parsed.Host
	var adapter balanceAdapter
	switch host {
	case "api.deepseek.com":
		adapter = deepSeekBalanceAdapter()
	case "openrouter.ai":
		adapter = openRouterBalanceAdapter()
	case "api.siliconflow.com", "api.siliconflow.cn":
		adapter = siliconFlowBalanceAdapter()
	case "api.moonshot.cn":
		adapter = moonshotBalanceAdapter("CNY")
	case "api.moonshot.ai":
		adapter = moonshotBalanceAdapter("USD")
	default:
		return nil, ""
	}
	return &adapter, origin
}

func queryBalanceWithAdapter(
	ctx context.Context,
	client *http.Client,
	origin string,
	key string,
	adapter balanceAdapter,
) (BalanceSnapshot, error) {
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		strings.TrimRight(origin, "/")+adapter.path,
		nil,
	)
	if err != nil {
		return BalanceSnapshot{}, err
	}
	req.Header.Set("Authorization", "Bearer "+key)
	resp, err := client.Do(req)
	if err != nil {
		return BalanceSnapshot{}, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return BalanceSnapshot{}, err
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return BalanceSnapshot{}, fmt.Errorf(
			"%s balance request failed with status %d",
			adapter.provider,
			resp.StatusCode,
		)
	}
	snapshot, err := adapter.parse(body)
	if err != nil {
		return BalanceSnapshot{}, fmt.Errorf("parse %s balance response: %w", adapter.provider, err)
	}
	snapshot.Provider = adapter.provider
	if snapshot.Balances == nil {
		snapshot.Balances = []BalanceAmount{}
	}
	return snapshot, nil
}

func formatBalanceNumber(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}

func deepSeekBalanceAdapter() balanceAdapter {
	return balanceAdapter{provider: "deepseek", path: "/user/balance", parse: func(body []byte) (BalanceSnapshot, error) {
		var response struct {
			Available bool `json:"is_available"`
			Infos     []struct {
				Currency string `json:"currency"`
				Total    string `json:"total_balance"`
				Granted  string `json:"granted_balance"`
				ToppedUp string `json:"topped_up_balance"`
			} `json:"balance_infos"`
		}
		if err := json.Unmarshal(body, &response); err != nil {
			return BalanceSnapshot{}, err
		}
		result := BalanceSnapshot{Available: boolPointer(response.Available)}
		for _, info := range response.Infos {
			result.Balances = append(result.Balances,
				BalanceAmount{Kind: "total", Amount: info.Total, Currency: info.Currency},
				BalanceAmount{Kind: "granted", Amount: info.Granted, Currency: info.Currency},
				BalanceAmount{Kind: "topped_up", Amount: info.ToppedUp, Currency: info.Currency},
			)
		}
		return result, nil
	}}
}

func openRouterBalanceAdapter() balanceAdapter {
	return balanceAdapter{provider: "openrouter", path: "/api/v1/key", parse: func(body []byte) (BalanceSnapshot, error) {
		var response struct {
			Data struct {
				Limit      *float64 `json:"limit"`
				Remaining  *float64 `json:"limit_remaining"`
				Usage      float64  `json:"usage"`
				UsageDaily float64  `json:"usage_daily"`
			} `json:"data"`
		}
		if err := json.Unmarshal(body, &response); err != nil {
			return BalanceSnapshot{}, err
		}
		result := BalanceSnapshot{}
		if response.Data.Remaining != nil {
			result.Balances = append(result.Balances, BalanceAmount{Kind: "limit_remaining", Amount: formatBalanceNumber(*response.Data.Remaining), Currency: "credits"})
		}
		if response.Data.Limit != nil {
			result.Balances = append(result.Balances, BalanceAmount{Kind: "limit", Amount: formatBalanceNumber(*response.Data.Limit), Currency: "credits"})
		}
		result.Metrics = []BalanceMetric{
			{Kind: "usage", Value: formatBalanceNumber(response.Data.Usage)},
			{Kind: "usage_daily", Value: formatBalanceNumber(response.Data.UsageDaily)},
		}
		return result, nil
	}}
}

func siliconFlowBalanceAdapter() balanceAdapter {
	return balanceAdapter{provider: "siliconflow", path: "/v1/user/info", parse: func(body []byte) (BalanceSnapshot, error) {
		var response struct {
			Status bool `json:"status"`
			Data   struct {
				Balance string `json:"balance"`
				Charge  string `json:"chargeBalance"`
				Total   string `json:"totalBalance"`
			} `json:"data"`
		}
		if err := json.Unmarshal(body, &response); err != nil {
			return BalanceSnapshot{}, err
		}
		return BalanceSnapshot{Available: boolPointer(response.Status), Balances: []BalanceAmount{
			{Kind: "balance", Amount: response.Data.Balance, Currency: "credits"},
			{Kind: "charge", Amount: response.Data.Charge, Currency: "credits"},
			{Kind: "total", Amount: response.Data.Total, Currency: "credits"},
		}}, nil
	}}
}

func moonshotBalanceAdapter(currency string) balanceAdapter {
	return balanceAdapter{provider: "moonshot", path: "/v1/users/me/balance", parse: func(body []byte) (BalanceSnapshot, error) {
		var response struct {
			Status bool `json:"status"`
			Data   struct {
				Available float64 `json:"available_balance"`
				Voucher   float64 `json:"voucher_balance"`
				Cash      float64 `json:"cash_balance"`
			} `json:"data"`
		}
		if err := json.Unmarshal(body, &response); err != nil {
			return BalanceSnapshot{}, err
		}
		return BalanceSnapshot{Available: boolPointer(response.Status), Balances: []BalanceAmount{
			{Kind: "available", Amount: formatBalanceNumber(response.Data.Available), Currency: currency},
			{Kind: "voucher", Amount: formatBalanceNumber(response.Data.Voucher), Currency: currency},
			{Kind: "cash", Amount: formatBalanceNumber(response.Data.Cash), Currency: currency},
		}}, nil
	}}
}

func boolPointer(value bool) *bool { return &value }
