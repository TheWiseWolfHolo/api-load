package ratelimit

import (
	"maps"
	"sync"
	"time"
)

// Concurrency 只记录进程内尚未完成的业务，独立于配置快照。
type Concurrency struct {
	mu             sync.Mutex
	global         int64
	accessKeys     map[uint]int64
	groups         map[uint]int64
	accounts       map[string]int64
	accountWindows map[string]timestampDeque
	lastSweep      time.Time
}

type ConcurrencySnapshot struct {
	Global     int64
	AccessKeys map[uint]int64
	Groups     map[uint]int64
}

func NewConcurrency() *Concurrency {
	return &Concurrency{accessKeys: make(map[uint]int64), groups: make(map[uint]int64), accounts: make(map[string]int64), accountWindows: make(map[string]timestampDeque)}
}

// TryAcquireRequest 同时取得全局和访问密钥名额；调用方在业务收尾后释放。
func (c *Concurrency) TryAcquireRequest(key uint, globalLimit, keyLimit int64) (func(), bool) {
	c.mu.Lock()
	if (globalLimit > 0 && c.global >= globalLimit) || (keyLimit > 0 && c.accessKeys[key] >= keyLimit) {
		c.mu.Unlock()
		return nil, false
	}
	c.global++
	c.accessKeys[key]++
	c.mu.Unlock()
	return sync.OnceFunc(func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		c.global--
		c.accessKeys[key]--
		if c.accessKeys[key] == 0 {
			delete(c.accessKeys, key)
		}
	}), true
}

func (c *Concurrency) TryAcquireGroup(group uint, limit int64) (func(), bool) {
	release, reason := c.TryAcquireGroupAccount(group, limit, "", 0, 0)
	return release, reason == ""
}

// TryAcquireGroupAccount 复用分组准入锁和滑动分钟窗口；不按 key 数增加额度。
func (c *Concurrency) TryAcquireGroupAccount(group uint, limit int64, account string, accountLimit, accountRPM int64) (func(), string) {
	c.mu.Lock()
	if limit > 0 && c.groups[group] >= limit {
		c.mu.Unlock()
		return nil, "group_concurrency"
	}
	if account != "" {
		if accountLimit > 0 && c.accounts[account] >= accountLimit {
			c.mu.Unlock()
			return nil, "account_concurrency"
		}
		now := time.Now()
		c.sweepAccountWindows(now)
		window := c.accountWindows[account]
		window.dropThrough(now.Add(-time.Minute))
		if accountRPM > 0 && int64(window.len()) >= accountRPM {
			c.accountWindows[account] = window
			c.mu.Unlock()
			return nil, "account_rpm"
		}
		if accountRPM > 0 {
			window.push(now)
		}
		if window.len() > 0 {
			c.accountWindows[account] = window
		} else {
			delete(c.accountWindows, account)
		}
		c.accounts[account]++
	}
	c.groups[group]++
	c.mu.Unlock()
	return sync.OnceFunc(func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		if account != "" {
			c.accounts[account]--
			if c.accounts[account] == 0 {
				delete(c.accounts, account)
			}
		}
		c.groups[group]--
		if c.groups[group] == 0 {
			delete(c.groups, group)
		}
	}), ""
}

// sweepAccountWindows 每分钟最多一次删除已经清空的账号窗口，账号改名或删除后不会一直占内存。
// 调用方持有 c.mu。
func (c *Concurrency) sweepAccountWindows(now time.Time) {
	if now.Sub(c.lastSweep) < time.Minute {
		return
	}
	c.lastSweep = now
	cutoff := now.Add(-time.Minute)
	for account, window := range c.accountWindows {
		window.dropThrough(cutoff)
		if window.len() == 0 {
			delete(c.accountWindows, account)
		} else {
			c.accountWindows[account] = window
		}
	}
}

func (c *Concurrency) Snapshot() ConcurrencySnapshot {
	c.mu.Lock()
	defer c.mu.Unlock()
	return ConcurrencySnapshot{Global: c.global, AccessKeys: maps.Clone(c.accessKeys), Groups: maps.Clone(c.groups)}
}

func (c *Concurrency) AccessKeyCurrent(id uint) int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.accessKeys[id]
}
