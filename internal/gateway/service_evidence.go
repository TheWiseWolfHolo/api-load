package gateway

import (
	"encoding/json"
	"slices"

	"gpt-load/internal/dialect"
	"gpt-load/internal/protocol"
	"gpt-load/internal/telemetry"
)

func requestServiceEvidence(p protocol.Protocol, request *dialect.ParsedRequest) *telemetry.ServiceEvidence {
	if p != protocol.CohereEmbeddings && p != protocol.OpenAIEmbeddings && p != protocol.Rerank {
		return nil
	}
	var body map[string]json.RawMessage
	if json.Unmarshal(request.Body, &body) != nil {
		return nil
	}
	e := &telemetry.ServiceEvidence{Format: string(p), Quality: "not_evaluated", ResultCheck: "not_checked"}
	if p == protocol.CohereEmbeddings {
		if _, ok := body["texts"]; ok {
			e.Input = []string{"text"}
		}
		if _, ok := body["images"]; ok {
			e.Input = []string{"image"}
		}
		if raw, ok := body["inputs"]; ok {
			e.Input = retrievalModalities(raw)
		}
		var inputType string
		_ = json.Unmarshal(body["input_type"], &inputType)
		if slices.Contains([]string{"search_document", "search_query", "classification", "clustering", "image"}, inputType) {
			e.InputType = inputType
		}
		_ = json.Unmarshal(body["output_dimension"], &e.RequestedDimensions)
	} else if p == protocol.Rerank {
		e.Query = retrievalModalities(body["query"])
		e.Documents = retrievalModalities(body["documents"])
	} else {
		e.Input = retrievalModalities(body["input"])
		_ = json.Unmarshal(body["dimensions"], &e.RequestedDimensions)
	}
	return e
}

// Only fixed modality labels leave this function. URLs, text and file names never do.
func retrievalModalities(raw json.RawMessage) []string {
	var value any
	if json.Unmarshal(raw, &value) != nil {
		return nil
	}
	found := map[string]bool{}
	var visit func(any)
	visit = func(item any) {
		switch item := item.(type) {
		case string:
			found["text"] = true
		case []any:
			for _, child := range item {
				visit(child)
			}
		case map[string]any:
			if _, ok := item["text"]; ok {
				found["text"] = true
			}
			if _, ok := item["image"]; ok {
				found["image"] = true
			}
			if _, ok := item["image_url"]; ok {
				found["image"] = true
			}
			if content, ok := item["content"]; ok {
				visit(content)
			}
		}
	}
	visit(value)
	var result []string
	for _, modality := range []string{"text", "image"} {
		if found[modality] {
			result = append(result, modality)
		}
	}
	return result
}
