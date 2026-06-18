package utils

import (
	"encoding/hex"
	"testing"
)

func TestDeriveAESKeyStaysCompatible(t *testing.T) {
	key := DeriveAESKey("compatibility-test-key")
	got := hex.EncodeToString(key)
	want := "e22e0402262b37c577519476dc94405946e65a59e0c6bc04045e5159c1d1e7e3"

	if got != want {
		t.Fatalf("derived AES key changed: got %s want %s", got, want)
	}
}
