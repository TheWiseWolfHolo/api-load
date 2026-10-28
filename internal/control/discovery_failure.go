package control

import (
	app_errors "gpt-load/internal/platform/errors"
)

// catalogDiscoveryError 保留固定失败阶段与类别，不携带上游响应内容。
type catalogDiscoveryError struct {
	Kind   string
	Status int
}

func (e *catalogDiscoveryError) Error() string {
	return "model catalog discovery failed (" + e.Kind + ")"
}
func (e *catalogDiscoveryError) Unwrap() error { return app_errors.ErrBadGateway }
func (e *catalogDiscoveryError) As(target any) bool {
	value, ok := target.(**app_errors.APIError)
	if !ok {
		return false
	}
	*value = app_errors.NewAPIErrorWithData(app_errors.ErrBadGateway, map[string]any{"failure_stage": "catalog_request", "failure_kind": e.Kind, "upstream_status": e.Status})
	return true
}
func modelCatalogFailure(status int) *catalogDiscoveryError {
	kind := "upstream_error"
	switch status {
	case 0:
		kind = "network"
	case 400, 422:
		kind = "invalid_request"
	case 401:
		kind = "authentication"
	case 403:
		kind = "permission"
	case 404, 410:
		kind = "catalog_missing"
	case 429:
		kind = "rate_limit"
	}
	return &catalogDiscoveryError{Kind: kind, Status: status}
}
