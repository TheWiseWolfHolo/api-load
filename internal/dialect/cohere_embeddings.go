package dialect

import (
	"encoding/json"
	"fmt"
	"mime"
	"net/http"
	"strings"

	"gpt-load/internal/execution"
	"gpt-load/internal/protocol"
	"gpt-load/internal/usage"
)

// CohereEmbeddings 保留 Cohere v2 原生输入和输出，不借用文字 Embeddings 转换。
type CohereEmbeddings struct{}

func NewCohereEmbeddings() *CohereEmbeddings          { return &CohereEmbeddings{} }
func (*CohereEmbeddings) Protocol() protocol.Protocol { return protocol.CohereEmbeddings }

func (d *CohereEmbeddings) InspectRequest(request *ParsedRequest) (RequestMetadata, error) {
	if request == nil || request.Method != http.MethodPost || request.Path != "/v2/embed" {
		return RequestMetadata{}, fmt.Errorf("Cohere Embed requires POST /v2/embed")
	}
	if value := request.Header.Get("Content-Type"); value != "" {
		mediaType, _, err := mime.ParseMediaType(value)
		if err != nil || !strings.EqualFold(mediaType, "application/json") {
			return RequestMetadata{}, &RequestValidationError{Field: "content_type", Rule: "application_json_required"}
		}
	}
	metadata, err := inspectJSONRequestFields(request.Body, true, false)
	if err != nil {
		return RequestMetadata{}, err
	}
	if metadata.Stream {
		return RequestMetadata{}, &RequestValidationError{Field: "stream", Rule: "streaming_unsupported"}
	}
	object, err := decodeJSONObject(request.Body)
	if err != nil {
		return RequestMetadata{}, err
	}
	if inputType, ok := jsonStringField(object, "input_type"); !ok || strings.TrimSpace(inputType) == "" {
		return RequestMetadata{}, &RequestValidationError{Field: "input_type", Rule: "required_string"}
	}
	if raw, exists := object["output_dimension"]; exists {
		var dimension int64
		// Match the integer range used by management JSON; model-specific dimensions remain upstream-owned.
		if json.Unmarshal(raw, &dimension) != nil || dimension <= 0 || dimension > 1<<53-1 {
			return RequestMetadata{}, &RequestValidationError{Field: "output_dimension", Rule: "positive_safe_integer_required"}
		}
	}
	found := 0
	for _, field := range []string{"texts", "images", "inputs"} {
		raw, exists := object[field]
		if !exists {
			continue
		}
		found++
		var values []json.RawMessage
		if json.Unmarshal(raw, &values) != nil || len(values) == 0 {
			return RequestMetadata{}, &RequestValidationError{Field: field, Rule: "nonempty_array_required"}
		}
		for _, value := range values {
			if field == "inputs" {
				var input struct {
					Content []json.RawMessage `json:"content"`
				}
				if json.Unmarshal(value, &input) != nil || len(input.Content) == 0 {
					return RequestMetadata{}, &RequestValidationError{Field: field, Rule: "content_array_required"}
				}
			} else if !jsonEmbeddingString(value) {
				return RequestMetadata{}, &RequestValidationError{Field: field, Rule: "string_array_required"}
			}
		}
	}
	if found != 1 {
		return RequestMetadata{}, &RequestValidationError{Field: "texts_images_inputs", Rule: "exactly_one_required"}
	}
	metadata.Operation = execution.OperationEmbeddingsCreate
	metadata.RouteRequirement = execution.RouteRequirementNative
	metadata.ObserveUsage = true
	return metadata, nil
}

func (d *CohereEmbeddings) RewriteRequestModel(request *ParsedRequest, model string) (*ParsedRequest, error) {
	if _, err := d.InspectRequest(request); err != nil {
		return nil, err
	}
	return rewriteJSONRequestModel(request, model, string(d.Protocol()))
}

func (*CohereEmbeddings) RewriteResponseModel(body []byte, model string) ([]byte, error) {
	return rewriteOptionalJSONField(body, "model", model)
}

func (*CohereEmbeddings) ExtractUsage(body []byte) (usage.Result, error) {
	root, err := decodeJSONObject(body)
	if err != nil {
		return usage.Result{}, fmt.Errorf("decode Cohere usage response")
	}
	meta, diagnostics := usageOptionalObject(root, "meta")
	billed, detail := usageOptionalObject(meta, "billed_units")
	diagnostics.Merge(detail)
	input, detail := usageInteger(billed, "input_tokens", false)
	diagnostics.Merge(detail)
	if !usageIntegerUsable(detail) {
		input = nil
	}
	// 图片等独立计价单位不能伪装成文字 token 或免费请求。
	for _, field := range []string{"images", "image_tokens", "search_units", "pages"} {
		value, diag := usageInteger(billed, field, false)
		diagnostics.Merge(diag)
		if value != nil && *value > 0 {
			diagnostics.Add(usage.DiagnosticUnsupportedBillableDetail)
		}
	}
	var accumulator usage.Accumulator
	if err := accumulator.ReplaceSnapshot(usage.Patch{UncachedInput: input, Final: true, Diagnostics: diagnostics}); err != nil {
		return usage.Result{}, err
	}
	result, _ := accumulator.Finalize(true)
	result.MeasuredUnits.Images, _ = usageInteger(billed, "images", false)
	result.MeasuredUnits.ImageTokens, _ = usageInteger(billed, "image_tokens", false)
	result.MeasuredUnits.Pages, _ = usageInteger(billed, "pages", false)
	result.MeasuredUnits.SearchUnits, _ = usageInteger(billed, "search_units", false)
	if result.MeasuredUnits.HasPositive() {
		result.State = usage.StatePartial
	}
	return result, nil
}

func (*CohereEmbeddings) NewUsageStreamExtractor() UsageStreamExtractor {
	return &openAIEmbeddingsUnsupportedStreamUsage{}
}
