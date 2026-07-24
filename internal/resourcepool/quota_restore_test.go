package resourcepool

import (
	app_errors "api-load/internal/errors"
	"api-load/internal/models"
	"errors"
	"net/http"
	"testing"
	"time"
)

// RES020: 配额/账单类失败在池未配置自动恢复(默认)时直接标记 invalid,
// 不再进入限时全局冷却,也不会自动回到轮转。
func TestRES020QuotaFailureWithoutScheduleMarksResourceInvalid(t *testing.T) {
	provider, db, pool := newTestProvider(t)
	var resource models.UpstreamResource
	if err := db.Where("resource_pool_id = ?", pool.ID).Order("id asc").First(&resource).Error; err != nil {
		t.Fatalf("load resource: %v", err)
	}

	if err := provider.HandleFailure(&resource, "openai", http.StatusPaymentRequired, "insufficient_quota", nil); err != nil {
		t.Fatalf("handle quota failure: %v", err)
	}

	var updated models.UpstreamResource
	if err := db.First(&updated, resource.ID).Error; err != nil {
		t.Fatalf("reload resource: %v", err)
	}
	if updated.Status != models.ResourceStatusInvalid || updated.GlobalCooldownUntil != nil {
		t.Fatalf("expected invalid without cooldown, got status=%q cooldown=%v", updated.Status, updated.GlobalCooldownUntil)
	}
	if _, err := provider.SelectBoundResource(pool.ID, resource.ID, "openai"); !errors.Is(err, app_errors.ErrNoActiveKeys) {
		t.Fatalf("quota-exhausted resource must stay out of rotation, got err=%v", err)
	}
}

// RES021: 配置池级 auto_restore_schedule 后,配额失败进入按调度计算的全局冷却,
// 冷却点一过即由选择路径惰性放回,无需后台任务。
func TestRES021QuotaFailureWithScheduleCoolsDownAndLazilyRestores(t *testing.T) {
	provider, db, pool := newTestProvider(t)
	pool.AutoRestoreSchedule = "1h"
	if err := db.Save(pool).Error; err != nil {
		t.Fatalf("save pool schedule: %v", err)
	}
	if err := provider.SyncPoolToStore(pool); err != nil {
		t.Fatalf("sync pool config: %v", err)
	}

	var resource models.UpstreamResource
	if err := db.Where("resource_pool_id = ?", pool.ID).Order("id asc").First(&resource).Error; err != nil {
		t.Fatalf("load resource: %v", err)
	}
	before := time.Now()
	if err := provider.HandleFailure(&resource, "openai", http.StatusPaymentRequired, "insufficient_quota", nil); err != nil {
		t.Fatalf("handle quota failure: %v", err)
	}

	var cooled models.UpstreamResource
	if err := db.First(&cooled, resource.ID).Error; err != nil {
		t.Fatalf("reload resource: %v", err)
	}
	if cooled.Status != models.ResourceStatusActive || cooled.GlobalCooldownUntil == nil {
		t.Fatalf("expected active resource with cooldown, got status=%q cooldown=%v", cooled.Status, cooled.GlobalCooldownUntil)
	}
	if drift := cooled.GlobalCooldownUntil.Sub(before.Add(time.Hour)).Abs(); drift > 2*time.Minute {
		t.Fatalf("cooldown drifted from now+1h by %v", drift)
	}
	if _, err := provider.SelectBoundResource(pool.ID, resource.ID, "openai"); !errors.Is(err, app_errors.ErrNoActiveKeys) {
		t.Fatalf("cooling resource must be unselectable, got err=%v", err)
	}

	if err := provider.SetGlobalCooldown(resource.ID, time.Now().Add(-time.Minute), "cooldown expired"); err != nil {
		t.Fatalf("expire cooldown: %v", err)
	}
	restored, err := provider.SelectBoundResource(pool.ID, resource.ID, "openai")
	if err != nil || restored.ID != resource.ID {
		t.Fatalf("expected resource back in rotation after cooldown, got %#v %v", restored, err)
	}
}
