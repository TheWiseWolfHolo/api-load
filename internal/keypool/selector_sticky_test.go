package keypool

import (
	"testing"

	"api-load/internal/models"

	"gorm.io/datatypes"
)

func TestSCH003StickyByGroupReusesKeyUntilInvalidated(t *testing.T) {
	provider, db, _ := newTestProvider(t)
	group := createTestGroup(t, db)
	group.Config = datatypes.JSONMap{
		"key_selection_strategy": "sticky",
		"key_affinity_scope":     "group",
	}

	keys := []models.APIKey{
		{GroupID: group.ID, KeyValue: "sk-test-sticky-a", KeyHash: "hash-sticky-a", Status: models.KeyStatusActive},
		{GroupID: group.ID, KeyValue: "sk-test-sticky-b", KeyHash: "hash-sticky-b", Status: models.KeyStatusActive},
	}
	if err := provider.AddKeys(group.ID, keys); err != nil {
		t.Fatalf("add keys: %v", err)
	}

	first, err := provider.SelectKeyForRequest(&group, SelectionRequest{})
	if err != nil {
		t.Fatalf("select first sticky key: %v", err)
	}
	second, err := provider.SelectKeyForRequest(&group, SelectionRequest{})
	if err != nil {
		t.Fatalf("select second sticky key: %v", err)
	}
	if first.ID != second.ID {
		t.Fatalf("expected sticky group affinity to reuse key %d, got %d", first.ID, second.ID)
	}

	var stored models.APIKey
	if err := db.First(&stored, first.ID).Error; err != nil {
		t.Fatalf("load first key: %v", err)
	}
	stored.Status = models.KeyStatusInvalid
	if err := db.Save(&stored).Error; err != nil {
		t.Fatalf("mark sticky key invalid: %v", err)
	}
	if err := provider.SyncKeyToStore(&stored); err != nil {
		t.Fatalf("sync invalid key: %v", err)
	}

	afterInvalidation, err := provider.SelectKeyForRequest(&group, SelectionRequest{})
	if err != nil {
		t.Fatalf("select after invalidation: %v", err)
	}
	if afterInvalidation.ID == first.ID {
		t.Fatalf("expected sticky affinity to switch after invalidation, still got %d", first.ID)
	}
}

func TestSCH004StickyByModelKeepsSeparateModelAffinities(t *testing.T) {
	provider, db, _ := newTestProvider(t)
	group := createTestGroup(t, db)
	group.Config = datatypes.JSONMap{
		"key_selection_strategy": "sticky",
		"key_affinity_scope":     "model",
	}

	keys := []models.APIKey{
		{GroupID: group.ID, KeyValue: "sk-test-sticky-model-a", KeyHash: "hash-sticky-model-a", Status: models.KeyStatusActive},
		{GroupID: group.ID, KeyValue: "sk-test-sticky-model-b", KeyHash: "hash-sticky-model-b", Status: models.KeyStatusActive},
	}
	if err := provider.AddKeys(group.ID, keys); err != nil {
		t.Fatalf("add keys: %v", err)
	}

	modelAFirst, err := provider.SelectKeyForRequest(&group, SelectionRequest{Model: "gpt-a"})
	if err != nil {
		t.Fatalf("select model a first: %v", err)
	}
	modelBFirst, err := provider.SelectKeyForRequest(&group, SelectionRequest{Model: "gpt-b"})
	if err != nil {
		t.Fatalf("select model b first: %v", err)
	}
	modelASecond, err := provider.SelectKeyForRequest(&group, SelectionRequest{Model: "gpt-a"})
	if err != nil {
		t.Fatalf("select model a second: %v", err)
	}
	modelBSecond, err := provider.SelectKeyForRequest(&group, SelectionRequest{Model: "gpt-b"})
	if err != nil {
		t.Fatalf("select model b second: %v", err)
	}

	if modelAFirst.ID != modelASecond.ID {
		t.Fatalf("expected model A affinity to be stable, got %d then %d", modelAFirst.ID, modelASecond.ID)
	}
	if modelBFirst.ID != modelBSecond.ID {
		t.Fatalf("expected model B affinity to be stable, got %d then %d", modelBFirst.ID, modelBSecond.ID)
	}
	if modelAFirst.ID == modelBFirst.ID {
		t.Fatalf("expected separate model affinity slots to be able to use different keys, both got %d", modelAFirst.ID)
	}
}

func TestSCH005StickyByModelAndProxyKeyIsolatesProxyEntrypoints(t *testing.T) {
	provider, db, memStore := newTestProvider(t)
	group := createTestGroup(t, db)
	group.Config = datatypes.JSONMap{
		"key_selection_strategy": "sticky",
		"key_affinity_scope":     "model+proxy_key",
	}

	keys := []models.APIKey{
		{GroupID: group.ID, KeyValue: "sk-test-sticky-proxy-a", KeyHash: "hash-sticky-proxy-a", Status: models.KeyStatusActive},
		{GroupID: group.ID, KeyValue: "sk-test-sticky-proxy-b", KeyHash: "hash-sticky-proxy-b", Status: models.KeyStatusActive},
	}
	if err := provider.AddKeys(group.ID, keys); err != nil {
		t.Fatalf("add keys: %v", err)
	}

	proxyA, err := provider.SelectKeyForRequest(&group, SelectionRequest{Model: "gpt-a", ProxyKey: "proxy-secret-a"})
	if err != nil {
		t.Fatalf("select proxy a: %v", err)
	}
	proxyB, err := provider.SelectKeyForRequest(&group, SelectionRequest{Model: "gpt-a", ProxyKey: "proxy-secret-b"})
	if err != nil {
		t.Fatalf("select proxy b: %v", err)
	}
	proxyAAgain, err := provider.SelectKeyForRequest(&group, SelectionRequest{Model: "gpt-a", ProxyKey: "proxy-secret-a"})
	if err != nil {
		t.Fatalf("select proxy a again: %v", err)
	}

	if proxyA.ID != proxyAAgain.ID {
		t.Fatalf("expected same proxy key affinity to be stable, got %d then %d", proxyA.ID, proxyAAgain.ID)
	}
	if proxyA.ID == proxyB.ID {
		t.Fatalf("expected different proxy key affinity slots to be isolated, both got %d", proxyA.ID)
	}

	rawKeyInStore, err := memStore.Exists("sticky:1:gpt-a:proxy-secret-a")
	if err != nil {
		t.Fatalf("check raw sticky key: %v", err)
	}
	if rawKeyInStore {
		t.Fatal("sticky store key included raw proxy key")
	}
}

func TestSCH009StickyRetryExcludesFailedKeyForSameRequest(t *testing.T) {
	provider, db, _ := newTestProvider(t)
	group := createTestGroup(t, db)
	group.Config = datatypes.JSONMap{
		"key_selection_strategy": "sticky",
		"key_affinity_scope":     "model+proxy_key",
	}

	keys := []models.APIKey{
		{GroupID: group.ID, KeyValue: "sk-test-sticky-retry-a", KeyHash: "hash-sticky-retry-a", Status: models.KeyStatusActive},
		{GroupID: group.ID, KeyValue: "sk-test-sticky-retry-b", KeyHash: "hash-sticky-retry-b", Status: models.KeyStatusActive},
	}
	if err := provider.AddKeys(group.ID, keys); err != nil {
		t.Fatalf("add keys: %v", err)
	}

	req := SelectionRequest{Model: "gpt-a", ProxyKey: "proxy-secret-a"}
	first, err := provider.SelectKeyForRequest(&group, req)
	if err != nil {
		t.Fatalf("select first sticky key: %v", err)
	}

	req.ExcludeKeyIDs = []uint{first.ID}
	retry, err := provider.SelectKeyForRequest(&group, req)
	if err != nil {
		t.Fatalf("select retry sticky key: %v", err)
	}
	if retry.ID == first.ID {
		t.Fatalf("expected retry selection to bypass failed sticky key %d", first.ID)
	}

	afterRetry, err := provider.SelectKeyForRequest(&group, SelectionRequest{Model: "gpt-a", ProxyKey: "proxy-secret-a"})
	if err != nil {
		t.Fatalf("select sticky key after retry: %v", err)
	}
	if afterRetry.ID != first.ID {
		t.Fatalf("expected request-local exclusion not to rewrite sticky affinity, got %d want %d", afterRetry.ID, first.ID)
	}
}
