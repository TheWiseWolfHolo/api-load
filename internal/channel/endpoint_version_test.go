package channel

import (
	"encoding/json"
	"testing"
)

func TestResolveAcceptsBaseURLWithOrWithoutVersion(t *testing.T) {
	registry := NewRegistry()
	for _, test := range []struct {
		id      ID
		input   string
		want    string
		comment string
	}{
		{"newapi", "https://relay.example", "https://relay.example", "gateway root stays"},
		{"newapi", "https://relay.example/v1", "https://relay.example", "gateway drops /v1"},
		{"gpt_load", "https://relay.example/proxy/v1beta/", "https://relay.example/proxy", "gateway drops /v1beta"},
		{"sub2api", "https://relay.example/api", "https://relay.example/api", "gateway keeps other paths"},
		{"opencode_zen", "https://opencode.ai/zen/v1", "https://opencode.ai/zen", "fixed gateway override drops /v1"},
		{"openai_compatible", "https://api.example.com", "https://api.example.com/v1", "compatible origin gets /v1"},
		{"openai_compatible", "https://api.example.com/v1", "https://api.example.com/v1", "compatible prefix stays"},
		{"openai_compatible", "https://open.bigmodel.cn/api/paas/v4", "https://open.bigmodel.cn/api/paas/v4", "non-v1 prefix stays"},
		{"openai_compatible", "https://generativelanguage.googleapis.com/v1beta/openai", "https://generativelanguage.googleapis.com/v1beta/openai", "nested prefix stays"},
		{"siliconflow", "https://api.siliconflow.com", "https://api.siliconflow.com/v1", "preset origin gets preset path"},
		{"volcengine", "https://ark.ap-southeast.bytepluses.com", "https://ark.ap-southeast.bytepluses.com/api/v3", "preset keeps its own version path"},
		{"cohere", "https://api.cohere.ai/v2", "https://api.cohere.ai/v2", "preset prefix stays"},
		{"openrouter", "https://openrouter.ai/api/v1", "https://openrouter.ai/api", "SDK appends /v1 itself"},
		{"openrouter", "https://openrouter.ai/api", "https://openrouter.ai/api", "SDK root stays"},
		{"xai", "https://api.x.ai/v1", "https://api.x.ai", "xAI drops /v1"},
		{"openai", "https://api.openai.com/v1", "https://api.openai.com", "OpenAI SDK drops /v1"},
		{"anthropic", "https://api.anthropic.com/v1", "https://api.anthropic.com", "Anthropic drops /v1"},
		{"gemini", "https://generativelanguage.googleapis.com", "https://generativelanguage.googleapis.com/v1beta", "Gemini origin gets /v1beta"},
		{"gemini", "https://generativelanguage.googleapis.com/v1beta", "https://generativelanguage.googleapis.com/v1beta", "Gemini prefix stays"},
	} {
		t.Run(test.comment, func(t *testing.T) {
			raw, _ := json.Marshal(map[string]string{"base_url": test.input})
			target, err := registry.Resolve(test.id, raw)
			if err != nil {
				t.Fatalf("Resolve(%s, %s): %v", test.id, test.input, err)
			}
			var config struct {
				BaseURL string `json:"base_url"`
			}
			if err := json.Unmarshal(target.TargetConfig, &config); err != nil {
				t.Fatal(err)
			}
			if config.BaseURL != test.want {
				t.Fatalf("Resolve(%s, %s) base_url = %q, want %q", test.id, test.input, config.BaseURL, test.want)
			}
			again, err := registry.ResolveExecutionTarget(test.id, target.TargetConfig)
			if err != nil || string(again.TargetConfig) != string(target.TargetConfig) {
				t.Fatalf("frozen target is not stable: %s -> %s (%v)", target.TargetConfig, again.TargetConfig, err)
			}
		})
	}
}
