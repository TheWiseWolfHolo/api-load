package keypool

import (
	"fmt"
	"testing"
	"time"

	"api-load/internal/models"
)

func TestNextAutoRestoreTimeDuration(t *testing.T) {
	from := time.Date(2026, 7, 22, 10, 0, 0, 0, time.UTC)

	got, err := NextAutoRestoreTime("24h", from)
	if err != nil {
		t.Fatalf("parse duration schedule: %v", err)
	}
	if want := from.Add(24 * time.Hour); !got.Equal(want) {
		t.Fatalf("expected %v, got %v", want, got)
	}

	got, err = NextAutoRestoreTime("90m", from)
	if err != nil {
		t.Fatalf("parse duration schedule: %v", err)
	}
	if want := from.Add(90 * time.Minute); !got.Equal(want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
}

func TestNextAutoRestoreTimeRejectsInvalidSchedules(t *testing.T) {
	from := time.Date(2026, 7, 22, 10, 0, 0, 0, time.UTC)
	for _, schedule := range []string{"", "0h", "-1h", "banana", "25:99", "00:05 +99:00"} {
		if _, err := NextAutoRestoreTime(schedule, from); err == nil {
			t.Fatalf("expected error for schedule %q", schedule)
		}
	}
}

func TestNextAutoRestoreTimeDailyClockWithOffset(t *testing.T) {
	// 2026-07-22 10:00 UTC 在 +08:00 是 18:00,下一个 00:05 +08:00 是
	// 2026-07-23 00:05 +08:00,即 2026-07-22 16:05 UTC。
	from := time.Date(2026, 7, 22, 10, 0, 0, 0, time.UTC)
	got, err := NextAutoRestoreTime("00:05 +08:00", from)
	if err != nil {
		t.Fatalf("parse daily schedule: %v", err)
	}
	want := time.Date(2026, 7, 22, 16, 5, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("expected %v, got %v", want, got)
	}

	// from 恰好落在钟点上时必须滚动到下一天,而不是返回当下。
	got, err = NextAutoRestoreTime("00:05 +08:00", want)
	if err != nil {
		t.Fatalf("parse daily schedule at boundary: %v", err)
	}
	next := time.Date(2026, 7, 23, 16, 5, 0, 0, time.UTC)
	if !got.Equal(next) {
		t.Fatalf("expected roll-over to %v, got %v", next, got)
	}
}

func TestAutoRestoreCooldownUntilRespectsConfigAndStatusCode(t *testing.T) {
	now := time.Date(2026, 7, 22, 10, 0, 0, 0, time.UTC)

	if got := autoRestoreCooldownUntil(&models.Group{}, 429, now); got != nil {
		t.Fatalf("expected nil cooldown without schedule config, got %v", got)
	}

	enabled := &models.Group{Config: map[string]any{"auto_restore_schedule": "24h"}}
	got := autoRestoreCooldownUntil(enabled, 429, now)
	if got == nil {
		t.Fatal("expected cooldown for 429 with schedule enabled")
	}
	if want := now.Add(24 * time.Hour); !got.Equal(want) {
		t.Fatalf("expected %v, got %v", want, got)
	}

	if got := autoRestoreCooldownUntil(enabled, 401, now); got != nil {
		t.Fatalf("expected nil cooldown for non-matching 401, got %v", got)
	}

	custom := &models.Group{Config: map[string]any{
		"auto_restore_schedule":     "24h",
		"auto_restore_status_codes": "429,403",
	}}
	if got := autoRestoreCooldownUntil(custom, 403, now); got == nil {
		t.Fatal("expected cooldown for 403 with custom status codes")
	}
}

func TestKEY009HandleFailureSetsCooldownAndSweepRestores(t *testing.T) {
	provider, db, memStore := newTestProvider(t)
	group := createTestGroup(t, db)
	group.Config = map[string]any{"auto_restore_schedule": "24h"}
	group.EffectiveConfig.BlacklistThreshold = 1

	key := models.APIKey{GroupID: group.ID, KeyValue: "sk-test-quota", KeyHash: "hash-quota", Status: models.KeyStatusActive}
	if err := db.Create(&key).Error; err != nil {
		t.Fatalf("create key: %v", err)
	}
	keyHashKey := fmt.Sprintf("key:%d", key.ID)
	activeListKey := fmt.Sprintf("group:%d:active_keys", group.ID)
	if err := memStore.HSet(keyHashKey, provider.apiKeyToMap(&key)); err != nil {
		t.Fatalf("cache key: %v", err)
	}
	if err := memStore.LPush(activeListKey, key.ID); err != nil {
		t.Fatalf("seed active list: %v", err)
	}

	if err := provider.handleFailure(&key, &group, keyHashKey, activeListKey, 429); err != nil {
		t.Fatalf("handle failure: %v", err)
	}

	var blacklisted models.APIKey
	if err := db.First(&blacklisted, key.ID).Error; err != nil {
		t.Fatalf("reload key: %v", err)
	}
	if blacklisted.Status != models.KeyStatusInvalid {
		t.Fatalf("expected invalid status, got %q", blacklisted.Status)
	}
	if blacklisted.LastFailureStatusCode != 429 {
		t.Fatalf("expected last failure status code 429, got %d", blacklisted.LastFailureStatusCode)
	}
	if blacklisted.CooldownUntil == nil {
		t.Fatal("expected cooldown_until to be set for 429 blacklist with schedule enabled")
	}
	if drift := time.Until(blacklisted.CooldownUntil.Add(-24 * time.Hour)).Abs(); drift > 2*time.Minute {
		t.Fatalf("cooldown_until drifted from now+24h by %v", drift)
	}
	if length, err := memStore.LLen(activeListKey); err != nil || length != 0 {
		t.Fatalf("expected key removed from active list, length=%d err=%v", length, err)
	}

	// 冷却未到期:不恢复。
	restored, err := provider.RestoreCooldownExpiredKeys(time.Now())
	if err != nil {
		t.Fatalf("restore before expiry: %v", err)
	}
	if restored != 0 {
		t.Fatalf("expected no keys restored before expiry, got %d", restored)
	}

	// 冷却到期:直接恢复,清零计数与冷却时间,回到活跃池。
	restored, err = provider.RestoreCooldownExpiredKeys(time.Now().Add(25 * time.Hour))
	if err != nil {
		t.Fatalf("restore after expiry: %v", err)
	}
	if restored != 1 {
		t.Fatalf("expected one key restored, got %d", restored)
	}
	var recovered models.APIKey
	if err := db.First(&recovered, key.ID).Error; err != nil {
		t.Fatalf("reload restored key: %v", err)
	}
	if recovered.Status != models.KeyStatusActive || recovered.FailureCount != 0 || recovered.CooldownUntil != nil {
		t.Fatalf("unexpected restored key state: status=%q failures=%d cooldown=%v", recovered.Status, recovered.FailureCount, recovered.CooldownUntil)
	}
	if length, err := memStore.LLen(activeListKey); err != nil || length != 1 {
		t.Fatalf("expected restored key back in active list, length=%d err=%v", length, err)
	}
}

func TestKEY010HandleFailureWithoutMatchLeavesCooldownEmpty(t *testing.T) {
	provider, db, memStore := newTestProvider(t)
	group := createTestGroup(t, db)
	group.Config = map[string]any{"auto_restore_schedule": "24h"}
	group.EffectiveConfig.BlacklistThreshold = 1

	key := models.APIKey{GroupID: group.ID, KeyValue: "sk-test-dead", KeyHash: "hash-dead", Status: models.KeyStatusActive}
	if err := db.Create(&key).Error; err != nil {
		t.Fatalf("create key: %v", err)
	}
	keyHashKey := fmt.Sprintf("key:%d", key.ID)
	activeListKey := fmt.Sprintf("group:%d:active_keys", group.ID)
	if err := memStore.HSet(keyHashKey, provider.apiKeyToMap(&key)); err != nil {
		t.Fatalf("cache key: %v", err)
	}

	if err := provider.handleFailure(&key, &group, keyHashKey, activeListKey, 401); err != nil {
		t.Fatalf("handle failure: %v", err)
	}

	var blacklisted models.APIKey
	if err := db.First(&blacklisted, key.ID).Error; err != nil {
		t.Fatalf("reload key: %v", err)
	}
	if blacklisted.Status != models.KeyStatusInvalid {
		t.Fatalf("expected invalid status, got %q", blacklisted.Status)
	}
	if blacklisted.LastFailureStatusCode != 401 {
		t.Fatalf("expected last failure status code 401, got %d", blacklisted.LastFailureStatusCode)
	}
	if blacklisted.CooldownUntil != nil {
		t.Fatalf("expected no cooldown for non-matching 401, got %v", blacklisted.CooldownUntil)
	}

	if restored, err := provider.RestoreCooldownExpiredKeys(time.Now().Add(48 * time.Hour)); err != nil || restored != 0 {
		t.Fatalf("expected sweep to skip keys without cooldown, restored=%d err=%v", restored, err)
	}
}

func TestKEY011CronValidationSkipsCoolingKeys(t *testing.T) {
	provider, db, _ := newTestProvider(t)
	group := createTestGroup(t, db)

	future := time.Now().Add(6 * time.Hour)
	key := models.APIKey{
		GroupID:       group.ID,
		KeyValue:      "sk-test-cooling",
		KeyHash:       "hash-cooling",
		Status:        models.KeyStatusInvalid,
		CooldownUntil: &future,
	}
	if err := db.Create(&key).Error; err != nil {
		t.Fatalf("create cooling key: %v", err)
	}

	countingEncryption := &countingEncryptionService{base: provider.encryptionSvc}
	checker := &CronChecker{
		DB:            db,
		EncryptionSvc: countingEncryption,
		stopChan:      make(chan struct{}),
	}

	checker.validateGroupKeys(&group)

	if countingEncryption.decryptCount.Load() != 0 {
		t.Fatalf("cooling key was decrypted %d times; it must be exempt from validation probes", countingEncryption.decryptCount.Load())
	}
}
