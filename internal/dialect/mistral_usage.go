package dialect

import (
	"bytes"
	"gpt-load/internal/usage"
)

func (*Mistral) ExtractUsage(body []byte) (usage.Result, error) {
	if !bytes.HasPrefix(bytes.TrimSpace(body), []byte("{")) {
		return usage.Result{State: usage.StateMissing}, nil
	}
	root, err := decodeJSONObject(body)
	if err != nil {
		return usage.Result{}, err
	}
	if _, ok := root["usage"]; ok {
		return NewOpenAI().ExtractUsage(body)
	}
	info, diagnostics := usageOptionalObject(root, "usage_info")
	pages, detail := usageInteger(info, "pages_processed", false)
	diagnostics.Merge(detail)
	result := usage.Result{State: usage.StateMissing, Diagnostics: diagnostics, MeasuredUnits: usage.Units{Pages: pages}}
	if pages != nil {
		result.State = usage.StatePartial
	}
	return result, nil
}
func (*Mistral) NewUsageStreamExtractor() UsageStreamExtractor {
	return NewOpenAI().NewUsageStreamExtractor()
}
