package resourcebinding

import (
	"context"
	"testing"
	"time"

	"gpt-load/internal/state"
	"gpt-load/internal/storage"
	"gpt-load/internal/storage/models"
)

func openStore(t *testing.T) *Store {
	t.Helper()
	db, err := storage.Open(":memory:")
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := storage.AutoMigrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return NewStore(db)
}

func testBinding(key string, expires time.Time) state.ResponseBinding {
	return state.ResponseBinding{AccessKeyID: 7, ResponseID: key, GroupID: 3, CredentialID: 11, IdentityGeneration: 2, ExpiresAt: expires}
}

func TestStoreRoundTripKeepsFirstOwner(t *testing.T) {
	store := openStore(t)
	ctx := context.Background()
	key := "native/mistral/files/file-abc"
	expires := time.Now().Add(time.Hour).Truncate(time.Millisecond)

	if err := store.Save(ctx, testBinding(key, expires)); err != nil {
		t.Fatalf("save: %v", err)
	}
	other := testBinding(key, expires)
	other.CredentialID = 12
	if err := store.Save(ctx, other); err != nil {
		t.Fatalf("conflicting save: %v", err)
	}

	got, found, err := store.Load(ctx, 7, key)
	if err != nil || !found {
		t.Fatalf("load = %v, %v", found, err)
	}
	if got != testBinding(key, expires) {
		t.Fatalf("load = %+v, want first owner", got)
	}
	if _, found, _ := store.Load(ctx, 8, key); found {
		t.Fatal("another access key read the resource owner")
	}
}

func TestStoreIgnoresExpiredAndPurges(t *testing.T) {
	store := openStore(t)
	ctx := context.Background()
	now := time.Now()
	store.now = func() time.Time { return now }
	expired := "native/mistral/files/old"
	if err := store.Save(ctx, testBinding(expired, now.Add(time.Minute))); err != nil {
		t.Fatal(err)
	}

	now = now.Add(2 * time.Minute)
	if _, found, err := store.Load(ctx, 7, expired); err != nil || found {
		t.Fatalf("expired binding load = %v, %v", found, err)
	}
	store.lastPurge = time.Time{}
	if err := store.Save(ctx, testBinding("native/mistral/files/new", now.Add(time.Hour))); err != nil {
		t.Fatal(err)
	}
	var count int64
	if err := store.db.Model(&models.ResourceBinding{}).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("rows after purge = %d, %v; want 1", count, err)
	}
}

func TestStoreRejectsNonResourceKeys(t *testing.T) {
	store := openStore(t)
	if err := store.Save(context.Background(), testBinding("resp_123", time.Now().Add(time.Hour))); err == nil {
		t.Fatal("Responses continuation ID was persisted as a resource")
	}
	if _, found, err := store.Load(context.Background(), 7, "resp_123"); err != nil || found {
		t.Fatalf("load = %v, %v", found, err)
	}
}
