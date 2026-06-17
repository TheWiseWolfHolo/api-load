package proxy

import (
	"strings"
	"testing"
)

func TestPRX004ProxyCredentialsAreMaskedOutsideTransportSetup(t *testing.T) {
	masked := MaskProxyURL("http://user:pass@example.invalid:8080/path")
	if strings.Contains(masked, "user") || strings.Contains(masked, "pass") {
		t.Fatalf("masked proxy URL exposed credentials: %q", masked)
	}
	if masked != "http://***@example.invalid:8080/path" {
		t.Fatalf("unexpected masked proxy URL: %q", masked)
	}
}
