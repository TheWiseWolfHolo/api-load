package dialect

import (
	"encoding/json"
	"strings"
)

// StructuredRetrievalInput 区分原生内容对象与 OpenAI 的文字或 token 输入。
func StructuredRetrievalInput(body []byte) bool {
	var object map[string]json.RawMessage
	if json.Unmarshal(body, &object) != nil {
		return false
	}
	var input any
	if json.Unmarshal(object["input"], &input) != nil {
		return false
	}
	switch value := input.(type) {
	case map[string]any:
		return true
	case []any:
		for _, item := range value {
			if _, ok := item.(map[string]any); ok {
				return true
			}
		}
	}
	return false
}

// validRetrievalContent 只识别文档定义的内容形状，模型权限由原生上游判断。
func validRetrievalContent(raw json.RawMessage) bool {
	if jsonEmbeddingString(raw) {
		return true
	}
	var content map[string]json.RawMessage
	if json.Unmarshal(raw, &content) != nil || len(content) == 0 {
		return false
	}
	found := false
	for _, field := range []string{"text", "image"} {
		if value, exists := content[field]; exists {
			var text string
			if json.Unmarshal(value, &text) != nil || strings.TrimSpace(text) == "" {
				return false
			}
			found = true
		}
	}
	return found
}

func validStructuredRetrievalInput(raw json.RawMessage) bool {
	if validRetrievalContent(raw) {
		return true
	}
	var values []json.RawMessage
	if json.Unmarshal(raw, &values) != nil || len(values) == 0 {
		return false
	}
	for _, value := range values {
		if !validRetrievalContent(value) {
			return false
		}
	}
	return true
}
