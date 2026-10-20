package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"gpt-load/internal/channel"
	"gpt-load/internal/dialect"
	"gpt-load/internal/health"
	"gpt-load/internal/platform/config"
	"gpt-load/internal/state"
	"gpt-load/internal/testutil/encryptiontest"
)

type fakeLiveModels struct {
	byGroup map[uint][]string
	errs    map[uint]error
	calls   int
}

func (fake *fakeLiveModels) LiveGroupModels(_ context.Context, groupID uint) ([]string, error) {
	fake.calls++
	if err := fake.errs[groupID]; err != nil {
		return nil, err
	}
	return fake.byGroup[groupID], nil
}

func newLiveModelsEngine(t *testing.T, fake *fakeLiveModels, transparentOne bool) *gin.Engine {
	t.Helper()
	return newLiveModelsEngineWithCatalog(t, fake, []state.ModelConfig{{ID: "old-model"}}, transparentOne)
}

func newLiveModelsEngineWithCatalog(t *testing.T, fake *fakeLiveModels, catalog []state.ModelConfig, transparentOne ...bool) *gin.Engine {
	t.Helper()
	transparent := len(transparentOne) == 0 || transparentOne[0]
	keyService := encryptiontest.Service(t, "live-models-test-master-key")
	manager := state.NewManager()
	setting := func(on bool) config.Settings { return config.Settings{state.SettingTransparentModels: on} }
	if _, err := manager.Publish(state.CompileInput{
		ChannelRegistry: channel.NewRegistry(),
		Groups: []state.GroupConfig{
			{ConnectionType: "api_key", ID: 1, Name: "live", ChannelID: channel.OpenAI, Params: json.RawMessage(`{}`),
				Models: catalog, Enabled: true, Settings: setting(transparent),
			},
			{ConnectionType: "api_key", ID: 2, Name: "catalog", ChannelID: channel.OpenAI, Params: json.RawMessage(`{}`),
				Models: []state.ModelConfig{{ID: "pinned"}}, Enabled: true, Settings: setting(false),
			},
		},
		AccessKeys: []state.AccessKeyConfig{
			{ID: 1, Name: "live-only", KeyHash: keyService.Hash("gl-live"), Status: state.AccessKeyStatusActive,
				Filters: state.FilterSet{Groups: map[uint]struct{}{1: {}}}},
			{ID: 2, Name: "both", KeyHash: keyService.Hash("gl-both"), Status: state.AccessKeyStatusActive},
		},
	}); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	handler := NewHandler(
		manager, state.NewCredentialRegistry(), keyService, &scriptedForwarder{}, dialect.NewSet(), health.NewStatsStore(),
		health.NewMutationCoordinator(),
		nil, nil, nil,
	)
	handler.SetLiveModelLister(fake)
	engine := gin.New()
	bindGatewayRoutesForTest(t, engine, handler)
	return engine
}

func listModels(t *testing.T, engine *gin.Engine, key string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	request.Header.Set("Authorization", "Bearer "+key)
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, request)
	return recorder
}

func modelIDs(t *testing.T, body string) []string {
	t.Helper()
	var list struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(body), &list); err != nil {
		t.Fatalf("decode %q: %v", body, err)
	}
	ids := make([]string, 0, len(list.Data))
	for _, item := range list.Data {
		ids = append(ids, item.ID)
	}
	return ids
}

func TestTransparentModelListShowsOnlyWhatTheUpstreamReports(t *testing.T) {
	fake := &fakeLiveModels{byGroup: map[uint][]string{1: {"new-b", "new-a"}}}
	engine := newLiveModelsEngine(t, fake, true)

	recorder := listModels(t, engine, "gl-live")
	if recorder.Code != http.StatusOK {
		t.Fatalf("response = %d %s", recorder.Code, recorder.Body.String())
	}
	if got := strings.Join(modelIDs(t, recorder.Body.String()), ","); got != "new-a,new-b" {
		t.Fatalf("models = %s, want only the live list without the old catalog entry", got)
	}

	fake.byGroup[1] = []string{"new-a"}
	recorder = listModels(t, engine, "gl-live")
	if got := strings.Join(modelIDs(t, recorder.Body.String()), ","); got != "new-a" {
		t.Fatalf("models after the upstream dropped one = %s, want new-a", got)
	}
}

func TestModelListKeepsCatalogGroupsAndMergesWithLiveOnes(t *testing.T) {
	fake := &fakeLiveModels{byGroup: map[uint][]string{1: {"new-a"}}}
	engine := newLiveModelsEngine(t, fake, true)

	recorder := listModels(t, engine, "gl-both")
	if got := strings.Join(modelIDs(t, recorder.Body.String()), ","); got != "new-a,pinned" {
		t.Fatalf("models = %s, want live list plus the catalog group's model", got)
	}
}

func TestModelListOfTurnedOffGroupStaysTheCatalog(t *testing.T) {
	fake := &fakeLiveModels{byGroup: map[uint][]string{1: {"new-a"}}}
	engine := newLiveModelsEngine(t, fake, false)

	recorder := listModels(t, engine, "gl-live")
	if got := strings.Join(modelIDs(t, recorder.Body.String()), ","); got != "old-model" {
		t.Fatalf("models = %s, want the catalog", got)
	}
	if fake.calls != 0 {
		t.Fatalf("upstream was asked %d times for a group with transparent mode off", fake.calls)
	}
}

func TestModelListFallsBackToTheCatalogWhenTheUpstreamCannotList(t *testing.T) {
	fake := &fakeLiveModels{errs: map[uint]error{1: errors.New("upstream down")}}
	engine := newLiveModelsEngine(t, fake, true)

	recorder := listModels(t, engine, "gl-live")
	if recorder.Code != http.StatusOK {
		t.Fatalf("response = %d %s", recorder.Code, recorder.Body.String())
	}
	if got := strings.Join(modelIDs(t, recorder.Body.String()), ","); got != "old-model" {
		t.Fatalf("models = %s, want the group's catalog while its upstream cannot list", got)
	}
}

func TestModelListFailsWhenTheUpstreamCannotListAndThereIsNoCatalog(t *testing.T) {
	fake := &fakeLiveModels{errs: map[uint]error{1: errors.New("upstream down")}}
	engine := newLiveModelsEngineWithCatalog(t, fake, nil)

	recorder := listModels(t, engine, "gl-live")
	if recorder.Code != http.StatusBadGateway || !strings.Contains(recorder.Body.String(), `"code":"upstream_models_unavailable"`) {
		t.Fatalf("response = %d %s", recorder.Code, recorder.Body.String())
	}
}

func TestModelListMergesCatalogOfFailingGroupWithOthers(t *testing.T) {
	fake := &fakeLiveModels{errs: map[uint]error{1: errors.New("upstream down")}}
	engine := newLiveModelsEngine(t, fake, true)

	recorder := listModels(t, engine, "gl-both")
	if recorder.Code != http.StatusOK {
		t.Fatalf("response = %d %s", recorder.Code, recorder.Body.String())
	}
	if got := strings.Join(modelIDs(t, recorder.Body.String()), ","); got != "old-model,pinned" {
		t.Fatalf("models = %s, want both catalogs", got)
	}
}

func TestHandlerForwardsAModelNeverConfiguredToTheTransparentGroup(t *testing.T) {
	forwarder := &scriptedForwarder{results: []UpstreamResult{{
		StatusCode: http.StatusOK, Header: make(http.Header), Body: []byte(`{"ok":true}`), RequestWritten: true,
	}}}
	engine, _, _, _ := newStatsHandlerTestRuntime(t, forwarder, "sk-upstream")

	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions",
		strings.NewReader(`{"model":"brand-new-model","messages":[]}`))
	request.Header.Set("Authorization", "Bearer gl-client")
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("response = %d %s", recorder.Code, recorder.Body.String())
	}
	if len(forwarder.inputs) != 1 {
		t.Fatalf("forwarded %d requests, want 1", len(forwarder.inputs))
	}
	if got := forwarder.inputs[0]; got.ExternalModel != "brand-new-model" || got.UpstreamModelID != "brand-new-model" {
		t.Fatalf("external %q upstream %q, want the requested model sent unchanged", got.ExternalModel, got.UpstreamModelID)
	}
}
