package proxy

import "testing"

type proxySequenceRNG struct {
	values []int
	next   int
}

func (r *proxySequenceRNG) Intn(n int) int {
	value := r.values[r.next%len(r.values)]
	r.next++
	if n == 0 {
		return 0
	}
	return value % n
}

func TestPRX003ProxyPoolStrategiesSkipDisabledItems(t *testing.T) {
	items := []ProxyPoolItem{
		{ID: "disabled", URL: "http://disabled.example.invalid:8080", Enabled: false},
		{ID: "a", URL: "http://a.example.invalid:8080", Enabled: true},
		{ID: "b", URL: "http://b.example.invalid:8080", Enabled: true},
	}

	roundRobin := NewProxyPoolSelector(ProxyPoolStrategyRoundRobin, nil)
	first, err := roundRobin.Select("pool-1", items, "client-a")
	if err != nil {
		t.Fatalf("round robin first select: %v", err)
	}
	second, err := roundRobin.Select("pool-1", items, "client-a")
	if err != nil {
		t.Fatalf("round robin second select: %v", err)
	}
	if first.ID == "disabled" || second.ID == "disabled" || first.ID == second.ID {
		t.Fatalf("round robin did not rotate enabled items only: first=%#v second=%#v", first, second)
	}

	random := NewProxyPoolSelector(ProxyPoolStrategyRandom, &proxySequenceRNG{values: []int{1}})
	randomItem, err := random.Select("pool-1", items, "client-a")
	if err != nil {
		t.Fatalf("random select: %v", err)
	}
	if randomItem.ID != "b" {
		t.Fatalf("expected deterministic random item b, got %#v", randomItem)
	}

	sticky := NewProxyPoolSelector(ProxyPoolStrategySticky, nil)
	stickyFirst, err := sticky.Select("pool-1", items, "client-a")
	if err != nil {
		t.Fatalf("sticky first select: %v", err)
	}
	stickySecond, err := sticky.Select("pool-1", items, "client-a")
	if err != nil {
		t.Fatalf("sticky second select: %v", err)
	}
	if stickyFirst.ID != stickySecond.ID {
		t.Fatalf("sticky did not keep affinity: first=%#v second=%#v", stickyFirst, stickySecond)
	}

	failover := NewProxyPoolSelector(ProxyPoolStrategyFailover, nil)
	failoverItem, err := failover.Select("pool-1", items, "client-a")
	if err != nil {
		t.Fatalf("failover select: %v", err)
	}
	if failoverItem.ID != "a" {
		t.Fatalf("expected failover to pick first enabled item, got %#v", failoverItem)
	}
}
