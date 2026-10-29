// Package resourcebinding 把原生资源的访问令牌归属写入数据库，进程重启或内存淘汰后仍能找回。
package resourcebinding

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"gpt-load/internal/state"
	"gpt-load/internal/storage/models"
)

// purgeInterval 限制清理过期归属的频率；清理顺带在写入时进行，不需要单独的后台任务。
const purgeInterval = time.Hour

var ErrOwnershipConflict = state.ErrResourceBindingConflict

// Store 只保存归属关系。凭据是否仍可用由调用方按当前配置和身份版本判断。
type Store struct {
	db        *gorm.DB
	now       func() time.Time
	mu        sync.Mutex
	lastPurge time.Time
}

func NewStore(db *gorm.DB) *Store {
	return &Store{db: db, now: time.Now}
}

func resourceHash(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])
}

// Save preserves the checkpoint import contract. Claim also returns the actual
// persisted expiry for callers that cache a binding after writing it.
func (store *Store) Save(ctx context.Context, binding state.ResponseBinding) error {
	_, err := store.Claim(ctx, binding)
	return err
}

// Claim atomically retains an active first owner or replaces an expired entry.
// The returned binding is canonical, including its original expiry.
func (store *Store) Claim(ctx context.Context, binding state.ResponseBinding) (state.ResponseBinding, error) {
	if store == nil || store.db == nil {
		return binding, nil
	}
	if !state.IsResourceBindingKey(binding.ResponseID) {
		return state.ResponseBinding{}, fmt.Errorf("resource binding key has an unexpected format")
	}
	if binding.AccessKeyID == 0 || binding.GroupID == 0 || binding.CredentialID == 0 || binding.IdentityGeneration == 0 {
		return state.ResponseBinding{}, fmt.Errorf("resource binding is incomplete")
	}
	now := store.now().UnixMilli()
	if binding.ExpiresAt.UnixMilli() <= now {
		return state.ResponseBinding{}, fmt.Errorf("resource binding has expired")
	}
	row := models.ResourceBinding{
		AccessKeyID:        binding.AccessKeyID,
		ResourceHash:       resourceHash(binding.ResponseID),
		ResourceKey:        binding.ResponseID,
		GroupID:            binding.GroupID,
		CredentialID:       binding.CredentialID,
		IdentityGeneration: binding.IdentityGeneration,
		ExpiresAtMS:        binding.ExpiresAt.UnixMilli(),
	}
	var saved models.ResourceBinding
	err := store.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("access_key_id = ? AND resource_hash = ? AND expires_at_ms <= ?", row.AccessKeyID, row.ResourceHash, now).Delete(&models.ResourceBinding{}).Error; err != nil {
			return err
		}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&row).Error; err != nil {
			return err
		}
		if err := tx.Where("access_key_id = ? AND resource_hash = ?", row.AccessKeyID, row.ResourceHash).Take(&saved).Error; err != nil {
			return err
		}
		if saved.ResourceKey != row.ResourceKey || saved.GroupID != row.GroupID || saved.CredentialID != row.CredentialID || saved.IdentityGeneration != row.IdentityGeneration {
			return ErrOwnershipConflict
		}
		return nil
	})
	if err != nil {
		return state.ResponseBinding{}, fmt.Errorf("save resource binding: %w", err)
	}
	store.purgeExpired(ctx)
	return bindingFromRow(saved), nil
}

// Load 读取未过期的归属；没有记录时 found 为 false，数据库错误单独返回。
func (store *Store) Load(ctx context.Context, accessKeyID uint, key string) (state.ResponseBinding, bool, error) {
	if store == nil || store.db == nil || accessKeyID == 0 || !state.IsResourceBindingKey(key) {
		return state.ResponseBinding{}, false, nil
	}
	var row models.ResourceBinding
	err := store.db.WithContext(ctx).
		Where("access_key_id = ? AND resource_hash = ? AND expires_at_ms > ?", accessKeyID, resourceHash(key), store.now().UnixMilli()).
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return state.ResponseBinding{}, false, nil
	}
	if err != nil {
		return state.ResponseBinding{}, false, fmt.Errorf("load resource binding: %w", err)
	}
	if row.ResourceKey != key {
		// 哈希碰撞或手工改动过的数据都不能借用别的资源归属。
		return state.ResponseBinding{}, false, nil
	}
	return bindingFromRow(row), true, nil
}

func bindingFromRow(row models.ResourceBinding) state.ResponseBinding {
	return state.ResponseBinding{
		AccessKeyID:        row.AccessKeyID,
		ResponseID:         row.ResourceKey,
		GroupID:            row.GroupID,
		CredentialID:       row.CredentialID,
		IdentityGeneration: row.IdentityGeneration,
		ExpiresAt:          time.UnixMilli(row.ExpiresAtMS),
	}
}

func (store *Store) purgeExpired(ctx context.Context) {
	now := store.now()
	store.mu.Lock()
	if now.Sub(store.lastPurge) < purgeInterval {
		store.mu.Unlock()
		return
	}
	store.lastPurge = now
	store.mu.Unlock()
	// 清理失败只会让过期行多留一会儿，Load 本身已经按到期时间过滤。
	_ = store.db.WithContext(ctx).Where("expires_at_ms <= ?", now.UnixMilli()).Delete(&models.ResourceBinding{}).Error
}
