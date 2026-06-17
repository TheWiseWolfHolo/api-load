package utils

import (
	"strings"
	"testing"
)

func TestSEC001MaskAPIKeyConsistentlyHidesSecretMaterial(t *testing.T) {
	cases := []string{
		"sk-test-openai-secret-value",
		"AIzaSyDummyGeminiSecretValue",
		"sk-ant-api03-test-anthropic-secret",
		"unknown-provider-secret-value",
		"short",
	}

	for _, key := range cases {
		masked := MaskAPIKey(key)
		if masked == key {
			t.Fatalf("key was not masked: key=%q masked=%q", key, masked)
		}
		if len(key) > 8 && strings.Contains(masked, key[4:len(key)-4]) {
			t.Fatalf("masked key exposed middle secret material: key=%q masked=%q", key, masked)
		}
		if strings.Count(masked, "*") < 4 {
			t.Fatalf("masked key should use visible mask characters: %q", masked)
		}
	}
}
