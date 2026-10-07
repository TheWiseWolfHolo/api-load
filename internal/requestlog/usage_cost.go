package requestlog

import (
	"fmt"
	"gpt-load/internal/pricing"
	"gpt-load/internal/telemetry"
	"gpt-load/internal/usage"
)

func ValidateUsageCostState(
	usageState usage.State,
	costState pricing.CostState,
	completeness pricing.Completeness,
	estimatedCostNanoUSD int64,
) error {
	return validateFrozenPricingState(usageState, costState, completeness, estimatedCostNanoUSD)
}

func normalizedBillingUnit(unit string) string {
	if unit == "" {
		return string(pricing.BillingUnitToken)
	}
	return unit
}

// ValidateBillingUsageCostState keeps token observations strict while allowing
// successful fixed-price requests to have missing or inapplicable token usage.
func ValidateBillingUsageCostState(unit pricing.BillingUnit, status telemetry.RequestStatus, usageState usage.State, costState pricing.CostState, completeness pricing.Completeness, cost int64) error {
	if unit == "" || unit == pricing.BillingUnitToken {
		return validateFrozenPricingState(usageState, costState, completeness, cost)
	}
	if unit != pricing.BillingUnitRequest || cost < 0 {
		return fmt.Errorf("invalid billing unit or request cost")
	}
	switch usageState {
	case usage.StateComplete, usage.StatePartial, usage.StateMissing, usage.StateNotApplicable:
	default:
		return fmt.Errorf("invalid request usage state")
	}
	switch costState {
	case pricing.CostStatePriced:
		if status != telemetry.RequestStatusSuccess || completeness != pricing.CompletenessComplete {
			return fmt.Errorf("fixed charge requires a successful request and complete pricing")
		}
	case pricing.CostStateUnpriced:
		if completeness != pricing.CompletenessUnavailable || cost != 0 {
			return fmt.Errorf("invalid unpriced request cost")
		}
	case pricing.CostStateNotApplicable:
		if completeness != pricing.CompletenessNotApplicable || cost != 0 {
			return fmt.Errorf("invalid nonbillable request cost")
		}
	default:
		return fmt.Errorf("invalid request cost state")
	}
	return nil
}
