package resourcebinding

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"gpt-load/internal/state"
	"gpt-load/internal/storage"
	"gpt-load/internal/storage/models"
)

func openStore(t *testing.T) *Store {
	t.Helper()
	dsn := os.Getenv("GPT_LOAD_RESOURCE_BINDING_TEST_DSN")
	if dsn == "" {
		dsn = ":memory:"
	}
	db, err := storage.Open(dsn)
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
	// An explicit external DSN must point at a disposable test database.
	// Tests run serially and clear only this module's fixture table.
	if os.Getenv("GPT_LOAD_RESOURCE_BINDING_TEST_DSN") != "" {
		if err := db.Exec("DELETE FROM resource_bindings").Error; err != nil {
			t.Fatal(err)
		}
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
	if err := store.Save(ctx, other); !errors.Is(err, ErrOwnershipConflict) {
		t.Fatalf("conflicting save = %v, want ownership conflict", err)
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

func TestStoreReplacesExpiredBindingAndReturnsCanonicalExpiry(t *testing.T) {
	store := openStore(t)
	now := time.Now().Truncate(time.Millisecond)
	store.now = func() time.Time { return now }
	key := "native/mistral/files/reused"
	if err := store.Save(t.Context(), testBinding(key, now.Add(time.Minute))); err != nil {
		t.Fatal(err)
	}
	now = now.Add(2 * time.Minute)
	replacement := testBinding(key, now.Add(time.Hour))
	replacement.CredentialID++
	got, err := store.Claim(t.Context(), replacement)
	if err != nil || got != replacement {
		t.Fatalf("expired replacement = %#v, %v", got, err)
	}
	if got, found, err := store.Load(t.Context(), 7, key); err != nil || !found || got != replacement {
		t.Fatalf("replacement was not persisted: %#v, %t, %v", got, found, err)
	}
	later := replacement
	later.ExpiresAt = later.ExpiresAt.Add(time.Hour)
	got, err = store.Claim(t.Context(), later)
	if err != nil || !got.ExpiresAt.Equal(replacement.ExpiresAt) {
		t.Fatalf("claim extended first-owner expiry: %#v, %v", got, err)
	}
}

func TestStoreConcurrentClaimsHaveOneOwner(t *testing.T) {
	store := openStore(t)
	key := "native/mistral/files/concurrent"
	start := make(chan struct{})
	type result struct {
		binding state.ResponseBinding
		err     error
	}
	results := make(chan result, 2)
	for _, id := range []uint{11, 12} {
		go func(id uint) {
			binding := testBinding(key, time.Now().Add(time.Hour))
			binding.CredentialID = id
			<-start
			canonical, err := store.Claim(context.Background(), binding)
			results <- result{canonical, err}
		}(id)
	}
	close(start)
	var winner state.ResponseBinding
	conflicts := 0
	for range 2 {
		got := <-results
		if errors.Is(got.err, ErrOwnershipConflict) {
			conflicts++
		} else if got.err != nil {
			t.Fatal(got.err)
		} else {
			winner = got.binding
		}
	}
	stored, found, err := store.Load(t.Context(), 7, key)
	if err != nil || !found || conflicts != 1 || stored != winner {
		t.Fatalf("concurrent claims = stored:%#v winner:%#v conflicts:%d err:%v", stored, winner, conflicts, err)
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
