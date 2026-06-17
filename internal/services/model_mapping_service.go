package services

import (
	"errors"
	"fmt"
	"strings"
)

var ErrModelNotSupported = errors.New("model not supported")

type modelMappingRNG interface {
	Intn(n int) int
}

type ModelMappingService struct {
	rng modelMappingRNG
}

type ModelMappingRule struct {
	Alias   string               `json:"alias"`
	Targets []ModelMappingTarget `json:"targets"`
}

type ModelMappingTarget struct {
	SubGroupID uint   `json:"sub_group_id"`
	Model      string `json:"model"`
	Weight     int    `json:"weight"`
}

type ModelMappingDecision struct {
	SubGroupID uint   `json:"sub_group_id"`
	Model      string `json:"model"`
	Fallback   bool   `json:"fallback"`
}

func NewModelMappingService(rng modelMappingRNG) *ModelMappingService {
	return &ModelMappingService{rng: rng}
}

func (s *ModelMappingService) Resolve(requestedModel string, rules []ModelMappingRule, strict bool) (ModelMappingDecision, error) {
	if rule, ok := findExactMappingRule(requestedModel, rules); ok {
		return s.selectTarget(rule)
	}
	if rule, ok := findWildcardMappingRule(requestedModel, rules); ok {
		return s.selectTarget(rule)
	}
	if strict {
		return ModelMappingDecision{}, fmt.Errorf("%w: %s", ErrModelNotSupported, requestedModel)
	}
	return ModelMappingDecision{Model: requestedModel, Fallback: true}, nil
}

func (s *ModelMappingService) selectTarget(rule ModelMappingRule) (ModelMappingDecision, error) {
	targets := make([]ModelMappingTarget, 0, len(rule.Targets))
	totalWeight := 0
	for _, target := range rule.Targets {
		if target.Weight <= 0 || strings.TrimSpace(target.Model) == "" {
			continue
		}
		targets = append(targets, target)
		totalWeight += target.Weight
	}
	if len(targets) == 0 || totalWeight == 0 {
		return ModelMappingDecision{}, fmt.Errorf("%w: %s", ErrModelNotSupported, rule.Alias)
	}

	pick := 0
	if s.rng != nil {
		pick = s.rng.Intn(totalWeight)
	}
	cumulative := 0
	for _, target := range targets {
		cumulative += target.Weight
		if pick < cumulative {
			return ModelMappingDecision{SubGroupID: target.SubGroupID, Model: target.Model}, nil
		}
	}

	last := targets[len(targets)-1]
	return ModelMappingDecision{SubGroupID: last.SubGroupID, Model: last.Model}, nil
}

func findExactMappingRule(requestedModel string, rules []ModelMappingRule) (ModelMappingRule, bool) {
	for _, rule := range rules {
		if rule.Alias == requestedModel && !strings.Contains(rule.Alias, "*") {
			return rule, true
		}
	}
	return ModelMappingRule{}, false
}

func findWildcardMappingRule(requestedModel string, rules []ModelMappingRule) (ModelMappingRule, bool) {
	for _, rule := range rules {
		if strings.Contains(rule.Alias, "*") && wildcardModelMatch(rule.Alias, requestedModel) {
			return rule, true
		}
	}
	return ModelMappingRule{}, false
}

func wildcardModelMatch(pattern, value string) bool {
	if pattern == "*" {
		return true
	}
	parts := strings.Split(pattern, "*")
	if len(parts) == 1 {
		return pattern == value
	}
	if parts[0] != "" && !strings.HasPrefix(value, parts[0]) {
		return false
	}
	if parts[len(parts)-1] != "" && !strings.HasSuffix(value, parts[len(parts)-1]) {
		return false
	}
	position := 0
	for _, part := range parts {
		if part == "" {
			continue
		}
		index := strings.Index(value[position:], part)
		if index < 0 {
			return false
		}
		position += index + len(part)
	}
	return true
}

func IsModelNotSupportedError(err error) bool {
	return errors.Is(err, ErrModelNotSupported)
}
