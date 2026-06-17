package proxy

import "testing"

func TestPRX001ProxyPolicyPrecedenceIsKeyGroupSystemEnv(t *testing.T) {
	resolver := NewProxyPolicyResolver()
	env := ProxyPolicy{Mode: ProxyModeFixed, URL: "http://env.example.invalid:8080"}
	system := ProxyPolicy{Mode: ProxyModeFixed, URL: "http://system.example.invalid:8080"}
	group := ProxyPolicy{Mode: ProxyModeFixed, URL: "http://group.example.invalid:8080"}
	key := ProxyPolicy{Mode: ProxyModeDirect}

	resolved, err := resolver.Resolve(ProxyPolicyInputs{Key: &key, Group: &group, System: &system, Env: &env})
	if err != nil {
		t.Fatalf("resolve proxy policy: %v", err)
	}
	if resolved.Mode != ProxyModeDirect || resolved.URL != "" {
		t.Fatalf("expected key direct policy to win and bypass lower policies, got %#v", resolved)
	}

	resolved, err = resolver.Resolve(ProxyPolicyInputs{Group: &group, System: &system, Env: &env})
	if err != nil {
		t.Fatalf("resolve group policy: %v", err)
	}
	if resolved.URL != group.URL {
		t.Fatalf("expected group policy to win, got %#v", resolved)
	}
}

func TestPRX002FixedProxyRejectsDisabledProxyItemsAndMasksURL(t *testing.T) {
	resolver := NewProxyPolicyResolver()
	_, err := resolver.ResolveFixedProxyItem(ProxyPoolItem{
		ID:      "us-1",
		URL:     "http://user:pass@example.invalid:8080",
		Enabled: false,
	})
	if err == nil {
		t.Fatal("expected disabled proxy item to be rejected")
	}

	resolved, err := resolver.ResolveFixedProxyItem(ProxyPoolItem{
		ID:      "us-1",
		URL:     "http://user:pass@example.invalid:8080",
		Enabled: true,
	})
	if err != nil {
		t.Fatalf("resolve fixed proxy item: %v", err)
	}
	if resolved.MaskedURL != "http://***@example.invalid:8080" {
		t.Fatalf("proxy URL was not masked: %#v", resolved)
	}
	if resolved.URL != "http://user:pass@example.invalid:8080" {
		t.Fatalf("transport URL should retain original secret internally: %#v", resolved)
	}
}
