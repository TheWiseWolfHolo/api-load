package models

import (
	"encoding/json"
	"testing"
)

func TestKEY001DisabledIsPersistentKeyStatus(t *testing.T) {
	key := APIKey{Status: "disabled"}

	data, err := json.Marshal(key)
	if err != nil {
		t.Fatalf("marshal APIKey: %v", err)
	}

	var decoded APIKey
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal APIKey: %v", err)
	}

	if decoded.Status != "disabled" {
		t.Fatalf("expected disabled status to round-trip, got %q", decoded.Status)
	}
}
