// Package keymanager 高性能密钥管理器
// @author OpenAI Proxy Team
// @version 2.0.0
package keymanager

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"openai-multi-key-proxy/internal/config"

	"github.com/sirupsen/logrus"
)

// KeyInfo 密钥信息
type KeyInfo struct {
	Key     string `json:"key"`
	Index   int    `json:"index"`
	Preview string `json:"preview"`
}

// Stats 统计信息
type Stats struct {
	CurrentIndex    int64       `json:"currentIndex"`
	TotalKeys       int         `json:"totalKeys"`
	HealthyKeys     int         `json:"healthyKeys"`
	BlacklistedKeys int         `json:"blacklistedKeys"`
	SuccessCount    int64       `json:"successCount"`
	FailureCount    int64       `json:"failureCount"`
	MemoryUsage     MemoryUsage `json:"memoryUsage"`
}

// MemoryUsage 内存使用情况
type MemoryUsage struct {
	FailureCountsSize int `json:"failureCountsSize"`
	BlacklistSize     int `json:"blacklistSize"`
}

// BlacklistDetail 黑名单详情
type BlacklistDetail struct {
	Index      int    `json:"index"`
	LineNumber int    `json:"lineNumber"`
	KeyPreview string `json:"keyPreview"`
	FullKey    string `json:"fullKey"`
}

// BlacklistInfo 黑名单信息
type BlacklistInfo struct {
	TotalBlacklisted int               `json:"totalBlacklisted"`
	TotalKeys        int               `json:"totalKeys"`
	HealthyKeys      int               `json:"healthyKeys"`
	BlacklistedKeys  []BlacklistDetail `json:"blacklistedKeys"`
}

// KeyManager 密钥管理器
type KeyManager struct {
	keysFilePath     string
	keys             []string
	keyPreviews      []string
	currentIndex     int64
	blacklistedKeys  sync.Map
	successCount     int64
	failureCount     int64
	keyFailureCounts sync.Map

	// 性能优化：预编译正则表达式
	permanentErrorPatterns []*regexp.Regexp

	// 内存管理
	cleanupTicker *time.Ticker
	stopCleanup   chan bool

	// 读写锁保护密钥列表
	keysMutex sync.RWMutex
}

// NewKeyManager 创建新的密钥管理器
func NewKeyManager(keysFilePath string) *KeyManager {
	if keysFilePath == "" {
		keysFilePath = config.AppConfig.Keys.FilePath
	}

	km := &KeyManager{
		keysFilePath: keysFilePath,
		currentIndex: int64(config.AppConfig.Keys.StartIndex),
		stopCleanup:  make(chan bool),

		// 预编译正则表达式
		permanentErrorPatterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)invalid api key`),
			regexp.MustCompile(`(?i)incorrect api key`),
			regexp.MustCompile(`(?i)api key not found`),
			regexp.MustCompile(`(?i)unauthorized`),
			regexp.MustCompile(`(?i)account deactivated`),
			regexp.MustCompile(`(?i)billing`),
		},
	}

	// 启动内存清理
	km.setupMemoryCleanup()

	return km
}

// LoadKeys 加载密钥文件
func (km *KeyManager) LoadKeys() error {
	file, err := os.Open(km.keysFilePath)
	if err != nil {
		return fmt.Errorf("无法打开密钥文件: %w", err)
	}
	defer file.Close()

	var keys []string
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			keys = append(keys, line)
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("读取密钥文件失败: %w", err)
	}

	if len(keys) == 0 {
		return fmt.Errorf("密钥文件中没有有效的API密钥")
	}

	km.keysMutex.Lock()
	km.keys = keys
	// 预生成密钥预览，避免运行时重复计算
	km.keyPreviews = make([]string, len(keys))
	for i, key := range keys {
		if len(key) > 20 {
			km.keyPreviews[i] = key[:20] + "..."
		} else {
			km.keyPreviews[i] = key
		}
	}
	km.keysMutex.Unlock()

	logrus.Infof("✅ 成功加载 %d 个 API 密钥", len(keys))
	return nil
}

// GetNextKey 获取下一个可用的密钥（高性能版本）
func (km *KeyManager) GetNextKey() (*KeyInfo, error) {
	km.keysMutex.RLock()
	keysLen := len(km.keys)
	if keysLen == 0 {
		km.keysMutex.RUnlock()
		return nil, fmt.Errorf("没有可用的 API 密钥")
	}

	// 快速路径：直接获取下一个密钥，避免黑名单检查的开销
	currentIdx := atomic.AddInt64(&km.currentIndex, 1) - 1
	keyIndex := int(currentIdx) % keysLen
	selectedKey := km.keys[keyIndex]
	keyPreview := km.keyPreviews[keyIndex]
	km.keysMutex.RUnlock()

	// 检查是否被拉黑
	if _, blacklisted := km.blacklistedKeys.Load(selectedKey); !blacklisted {
		return &KeyInfo{
			Key:     selectedKey,
			Index:   keyIndex,
			Preview: keyPreview,
		}, nil
	}

	// 慢速路径：寻找可用密钥
	attempts := 0
	maxAttempts := keysLen * 2 // 最多尝试两轮

	for attempts < maxAttempts {
		currentIdx = atomic.AddInt64(&km.currentIndex, 1) - 1
		keyIndex = int(currentIdx) % keysLen

		km.keysMutex.RLock()
		selectedKey = km.keys[keyIndex]
		keyPreview = km.keyPreviews[keyIndex]
		km.keysMutex.RUnlock()

		if _, blacklisted := km.blacklistedKeys.Load(selectedKey); !blacklisted {
			return &KeyInfo{
				Key:     selectedKey,
				Index:   keyIndex,
				Preview: keyPreview,
			}, nil
		}

		attempts++
	}

	// 检查是否所有密钥都被拉黑，如果是则重置
	blacklistedCount := 0
	km.blacklistedKeys.Range(func(key, value interface{}) bool {
		blacklistedCount++
		return blacklistedCount < keysLen // 提前退出优化
	})

	if blacklistedCount >= keysLen {
		logrus.Warn("⚠️ 所有密钥都被拉黑，重置黑名单")
		km.blacklistedKeys = sync.Map{}
		km.keyFailureCounts = sync.Map{}

		// 重置后返回第一个密钥
		km.keysMutex.RLock()
		firstKey := km.keys[0]
		firstPreview := km.keyPreviews[0]
		km.keysMutex.RUnlock()

		return &KeyInfo{
			Key:     firstKey,
			Index:   0,
			Preview: firstPreview,
		}, nil
	}

	return nil, fmt.Errorf("暂时没有可用的 API 密钥")
}

// RecordSuccess 记录密钥使用成功
func (km *KeyManager) RecordSuccess(key string) {
	atomic.AddInt64(&km.successCount, 1)
	// 成功时重置该密钥的失败计数
	km.keyFailureCounts.Delete(key)
}

// RecordFailure 记录密钥使用失败
func (km *KeyManager) RecordFailure(key string, err error) {
	atomic.AddInt64(&km.failureCount, 1)

	// 检查是否是永久性错误
	if km.isPermanentError(err) {
		km.blacklistedKeys.Store(key, true)
		km.keyFailureCounts.Delete(key) // 清理计数
		logrus.Warnf("🚫 密钥已被拉黑（永久性错误）: %s (%s)", key[:20]+"...", err.Error())
		return
	}

	// 临时性错误：增加失败计数
	currentFailures := 0
	if val, exists := km.keyFailureCounts.Load(key); exists {
		currentFailures = val.(int)
	}
	newFailures := currentFailures + 1
	km.keyFailureCounts.Store(key, newFailures)

	threshold := config.AppConfig.Keys.BlacklistThreshold
	if newFailures >= threshold {
		km.blacklistedKeys.Store(key, true)
		km.keyFailureCounts.Delete(key) // 清理计数
		logrus.Warnf("🚫 密钥已被拉黑（达到阈值）: %s (失败 %d 次: %s)", key[:20]+"...", newFailures, err.Error())
	} else {
		logrus.Warnf("⚠️ 密钥失败: %s (%d/%d 次: %s)", key[:20]+"...", newFailures, threshold, err.Error())
	}
}

// isPermanentError 判断是否是永久性错误
func (km *KeyManager) isPermanentError(err error) bool {
	errorMessage := err.Error()
	for _, pattern := range km.permanentErrorPatterns {
		if pattern.MatchString(errorMessage) {
			return true
		}
	}
	return false
}

// GetStats 获取密钥统计信息
func (km *KeyManager) GetStats() *Stats {
	km.keysMutex.RLock()
	totalKeys := len(km.keys)
	km.keysMutex.RUnlock()

	blacklistedCount := 0
	km.blacklistedKeys.Range(func(key, value interface{}) bool {
		blacklistedCount++
		return true
	})

	failureCountsSize := 0
	km.keyFailureCounts.Range(func(key, value interface{}) bool {
		failureCountsSize++
		return true
	})

	return &Stats{
		CurrentIndex:    atomic.LoadInt64(&km.currentIndex),
		TotalKeys:       totalKeys,
		HealthyKeys:     totalKeys - blacklistedCount,
		BlacklistedKeys: blacklistedCount,
		SuccessCount:    atomic.LoadInt64(&km.successCount),
		FailureCount:    atomic.LoadInt64(&km.failureCount),
		MemoryUsage: MemoryUsage{
			FailureCountsSize: failureCountsSize,
			BlacklistSize:     blacklistedCount,
		},
	}
}

// ResetKeys 重置密钥状态
func (km *KeyManager) ResetKeys() map[string]interface{} {
	beforeCount := 0
	km.blacklistedKeys.Range(func(key, value interface{}) bool {
		beforeCount++
		return true
	})

	km.blacklistedKeys = sync.Map{}
	km.keyFailureCounts = sync.Map{}

	logrus.Infof("🔄 密钥状态已重置，清除了 %d 个黑名单密钥", beforeCount)

	km.keysMutex.RLock()
	totalKeys := len(km.keys)
	km.keysMutex.RUnlock()

	return map[string]interface{}{
		"success":      true,
		"message":      fmt.Sprintf("已清除 %d 个黑名单密钥", beforeCount),
		"clearedCount": beforeCount,
		"totalKeys":    totalKeys,
	}
}

// GetBlacklistDetails 获取黑名单详情
func (km *KeyManager) GetBlacklistDetails() *BlacklistInfo {
	var blacklistDetails []BlacklistDetail

	km.keysMutex.RLock()
	keys := km.keys
	keyPreviews := km.keyPreviews
	km.keysMutex.RUnlock()

	for i, key := range keys {
		if _, blacklisted := km.blacklistedKeys.Load(key); blacklisted {
			blacklistDetails = append(blacklistDetails, BlacklistDetail{
				Index:      i,
				LineNumber: i + 1,
				KeyPreview: keyPreviews[i],
				FullKey:    key,
			})
		}
	}

	return &BlacklistInfo{
		TotalBlacklisted: len(blacklistDetails),
		TotalKeys:        len(keys),
		HealthyKeys:      len(keys) - len(blacklistDetails),
		BlacklistedKeys:  blacklistDetails,
	}
}

// setupMemoryCleanup 设置内存清理机制
func (km *KeyManager) setupMemoryCleanup() {
	km.cleanupTicker = time.NewTicker(10 * time.Minute)

	go func() {
		for {
			select {
			case <-km.cleanupTicker.C:
				km.performMemoryCleanup()
			case <-km.stopCleanup:
				km.cleanupTicker.Stop()
				return
			}
		}
	}()
}

// performMemoryCleanup 执行内存清理
func (km *KeyManager) performMemoryCleanup() {
	km.keysMutex.RLock()
	maxSize := len(km.keys) * 2
	if maxSize < 1000 {
		maxSize = 1000
	}
	km.keysMutex.RUnlock()

	currentSize := 0
	km.keyFailureCounts.Range(func(key, value interface{}) bool {
		currentSize++
		return true
	})

	if currentSize > maxSize {
		logrus.Infof("🧹 清理失败计数缓存 (%d -> %d)", currentSize, maxSize)

		// 简单策略：清理一半的失败计数
		cleared := 0
		target := currentSize - maxSize

		km.keyFailureCounts.Range(func(key, value interface{}) bool {
			if cleared < target {
				km.keyFailureCounts.Delete(key)
				cleared++
			}
			return cleared < target
		})
	}
}

// Close 关闭密钥管理器
func (km *KeyManager) Close() {
	close(km.stopCleanup)
}
