package ratelimit

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestConcurrencyLimitsAndRelease(t *testing.T) {
	c := NewConcurrency()
	release, ok := c.TryAcquireRequest(1, 0, 0)
	if !ok {
		t.Fatal("unlimited rejected")
	}
	if extra, ok := c.TryAcquireRequest(1, 10, 1); ok {
		extra()
		t.Fatal("key capacity was exceeded")
	}
	if extra, ok := c.TryAcquireRequest(2, 1, 0); ok {
		extra()
		t.Fatal("global capacity was exceeded")
	}
	if s := c.Snapshot(); s.Global != 1 || s.AccessKeys[1] != 1 || len(s.AccessKeys) != 1 {
		t.Fatalf("rejection changed counts: %+v", s)
	}
	group, ok := c.TryAcquireGroup(7, 1)
	if !ok {
		t.Fatal("group rejected")
	}
	if extra, ok := c.TryAcquireGroup(7, 1); ok {
		extra()
		t.Fatal("group capacity was exceeded")
	}
	other, ok := c.TryAcquireGroup(8, 1)
	if !ok {
		t.Fatal("groups are not independent")
	}
	other()
	group()
	group()
	release()
	release()
	if s := c.Snapshot(); s.Global != 0 || len(s.AccessKeys) != 0 || len(s.Groups) != 0 {
		t.Fatalf("leaked counts: %+v", s)
	}
}

func TestConcurrencyAtomicAdmission(t *testing.T) {
	c := NewConcurrency()
	var ready, done sync.WaitGroup
	var admitted atomic.Int64
	finish := make(chan struct{})
	for i := range 64 {
		ready.Add(1)
		done.Add(1)
		go func(key uint) {
			defer done.Done()
			release, ok := c.TryAcquireRequest(key, 8, 2)
			if ok {
				admitted.Add(1)
			}
			ready.Done()
			<-finish
			if ok {
				release()
				release()
			}
		}(uint(i%4 + 1))
	}
	ready.Wait()
	current := c.Snapshot()
	close(finish)
	done.Wait()
	if admitted.Load() != 8 || current.Global != 8 {
		t.Fatalf("admitted %d, snapshot %+v", admitted.Load(), current)
	}
	for _, n := range current.AccessKeys {
		if n > 2 {
			t.Fatalf("key admitted %d", n)
		}
	}
	if c.Snapshot().Global != 0 {
		t.Fatal("requests not released")
	}
}

func TestSharedAccountLimitsAcrossGroups(t *testing.T) {
	c := NewConcurrency()
	release, reason := c.TryAcquireGroupAccount(1, 4, "provider-account", 1, 2)
	if reason != "" {
		t.Fatal(reason)
	}
	if _, reason = c.TryAcquireGroupAccount(2, 4, "provider-account", 1, 2); reason != "account_concurrency" {
		t.Fatal("shared concurrency not enforced", reason)
	}
	release()
	release, reason = c.TryAcquireGroupAccount(2, 4, "provider-account", 1, 2)
	if reason != "" {
		t.Fatal(reason)
	}
	release()
	if _, reason = c.TryAcquireGroupAccount(1, 4, "provider-account", 1, 2); reason != "account_rpm" {
		t.Fatal("shared rate not enforced", reason)
	}
	release, reason = c.TryAcquireGroupAccount(3, 4, "other-account", 1, 2)
	if reason != "" {
		t.Fatal(reason)
	}
	release()
}

func TestSharedAccountWindowsAreReleasedWhenIdle(t *testing.T) {
	c := NewConcurrency()
	release, blocked := c.TryAcquireGroupAccount(1, 0, "plain", 0, 0)
	if blocked != "" {
		t.Fatal(blocked)
	}
	release()
	if _, kept := c.accountWindows["plain"]; kept {
		t.Fatal("account without an RPM limit kept an empty window")
	}

	stale := timestampDeque{}
	stale.push(time.Now().Add(-2 * time.Minute))
	c.accountWindows["removed-account"] = stale
	c.lastSweep = time.Time{}
	release, blocked = c.TryAcquireGroupAccount(1, 0, "active", 0, 5)
	if blocked != "" {
		t.Fatal(blocked)
	}
	release()
	if _, kept := c.accountWindows["removed-account"]; kept {
		t.Fatal("stale window of a removed account was not swept")
	}
	if window := c.accountWindows["active"]; window.len() != 1 {
		t.Fatalf("active account window length = %d, want 1", window.len())
	}

	c.accountWindows["removed-account"] = stale
	c.lastSweep = time.Time{}
	release, allowed := c.TryAcquireGroup(2, 0)
	if !allowed {
		t.Fatal("ordinary group admission failed")
	}
	release()
	if _, kept := c.accountWindows["removed-account"]; kept {
		t.Fatal("ordinary group traffic retained an expired account window")
	}
	if window := c.accountWindows["active"]; window.len() != 1 {
		t.Fatalf("ordinary group sweep removed an active RPM window: %d", window.len())
	}
}
