package control

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"gpt-load/internal/channel"
	"gpt-load/internal/execution"
	"gpt-load/internal/platform/config"
	"gpt-load/internal/protocol"
	"gpt-load/internal/state"
	"gpt-load/internal/storage/models"
	"gpt-load/internal/telemetry"
)

func TestGetGroupSummaryUsesCollectionServiceStatusAndOnlyReturnsHeaderCounts(t *testing.T) {
	t.Parallel()
	fixture := newServiceFixture(t)
	available := createGroupCollectionGroup(t, fixture, "summary-available", true, nil)
	unavailable := createGroupCollectionGroup(t, fixture, "summary-unavailable", true, nil)
	openAIWithoutModels := createGroupCollectionGroup(t, fixture, "summary-openai-no-models", true, nil)
	disabled := createGroupCollectionGroup(t, fixture, "summary-disabled", false, nil)
	noCredentials := createGroupCollectionGroup(t, fixture, "summary-no-credentials", true, nil)
	setGroupCollectionChannel(t, fixture, available, channel.OpenAI, models.JSON(`{}`))
	setGroupCollectionChannel(t, fixture, unavailable, channel.Anthropic, models.JSON(`{}`))
	setGroupCollectionChannel(t, fixture, openAIWithoutModels, channel.OpenAI, models.JSON(`{}`))
	setGroupCollectionRoute(t, fixture, available, `["openai-responses"]`, `[{"id":"gpt-test"}]`)
	setGroupCollectionRoute(t, fixture, unavailable, `["openai-completions"]`, `[]`)
	setGroupCollectionRoute(t, fixture, openAIWithoutModels, `["openai-responses"]`, `[]`)
	setGroupCollectionRoute(t, fixture, disabled, `["openai-responses"]`, `[]`)
	setGroupCollectionChannel(t, fixture, noCredentials, channel.Anthropic, models.JSON(`{}`))
	setGroupCollectionRoute(t, fixture, noCredentials, `["openai-completions"]`, `[{"id":"model"}]`)
	publishGroupCollectionRuntime(t, fixture, []state.CredentialEntry{
		createGroupCollectionKey(t, fixture, available.ID, models.CredentialStatusActive, nil),
		createGroupCollectionKey(t, fixture, unavailable.ID, models.CredentialStatusActive, nil),
		createGroupCollectionKey(t, fixture, openAIWithoutModels.ID, models.CredentialStatusActive, nil),
		createGroupCollectionKey(t, fixture, disabled.ID, models.CredentialStatusActive, nil),
	})

	for _, test := range []struct {
		name       string
		groupID    uint
		wantStatus GroupCollectionStatus
		wantReason GroupUnavailableReason
		wantKeys   int64
	}{
		{name: "available", groupID: available.ID, wantStatus: GroupCollectionStatusAvailable, wantKeys: 1},
		{name: "unavailable without models", groupID: unavailable.ID, wantStatus: GroupCollectionStatusUnavailable, wantReason: GroupUnavailableReasonNoModels, wantKeys: 1},
		{name: "OpenAI unavailable without models", groupID: openAIWithoutModels.ID, wantStatus: GroupCollectionStatusUnavailable, wantReason: GroupUnavailableReasonNoModels, wantKeys: 1},
		{name: "disabled", groupID: disabled.ID, wantStatus: GroupCollectionStatusDisabled, wantKeys: 1},
		{name: "unavailable without credentials", groupID: noCredentials.ID, wantStatus: GroupCollectionStatusUnavailable, wantReason: GroupUnavailableReasonNoAvailableCredentials, wantKeys: 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := fixture.service.GetGroupSummary(t.Context(), test.groupID)
			if err != nil {
				t.Fatalf("GetGroupSummary() error = %v", err)
			}
			if got.ID != test.groupID || got.ServiceStatus != test.wantStatus || got.CredentialCount != test.wantKeys {
				t.Fatalf("GetGroupSummary() = %#v", got)
			}
			if test.wantReason == "" {
				if got.ServiceStatusReason != nil {
					t.Fatalf("ServiceStatusReason = %q, want nil", *got.ServiceStatusReason)
				}
			} else if got.ServiceStatusReason == nil || *got.ServiceStatusReason != test.wantReason {
				t.Fatalf("ServiceStatusReason = %v, want %q", got.ServiceStatusReason, test.wantReason)
			}

			encoded, err := json.Marshal(got)
			if err != nil {
				t.Fatal(err)
			}
			var fields map[string]json.RawMessage
			if err := json.Unmarshal(encoded, &fields); err != nil {
				t.Fatal(err)
			}
			wantFields := map[string]struct{}{
				"id": {}, "name": {}, "channel_id": {}, "params": {}, "service_status": {}, "service_status_reason": {},
				"connection_type": {}, "credential_count": {}, "model_count": {}, "price_multiplier": {},
			}
			for name := range fields {
				if _, exists := wantFields[name]; !exists {
					t.Fatalf("summary exposes unexpected field %q: %s", name, encoded)
				}
			}
			for name := range wantFields {
				if _, exists := fields[name]; !exists {
					t.Fatalf("summary is missing expected field %q: %s", name, encoded)
				}
			}
			if string(fields["price_multiplier"]) != `"1"` {
				t.Fatalf("default price multiplier = %s, want string 1", fields["price_multiplier"])
			}
			for _, forbidden := range []string{
				"models", "config", "effective_config", "enabled", "weight_manual", "validation_model",
			} {
				if containsJSONToken(encoded, forbidden) {
					t.Fatalf("summary exposes %q: %s", forbidden, encoded)
				}
			}
		})
	}
}

func TestGroupDetailStatusRequiresAHealthyKeyAndRouteCapability(t *testing.T) {
	t.Parallel()
	fixture := newServiceFixture(t)
	group := createGroupCollectionGroup(t, fixture, "detail-zero-model-completions", true, nil)
	setGroupCollectionChannel(t, fixture, group, channel.Anthropic, models.JSON(`{}`))
	setGroupCollectionRoute(t, fixture, group, `["openai-completions"]`, `[]`)
	publishGroupCollectionRuntime(t, fixture, []state.CredentialEntry{
		createGroupCollectionKey(t, fixture, group.ID, models.CredentialStatusActive, nil),
	})

	got, err := fixture.service.GetGroupSummary(t.Context(), group.ID)
	if err != nil {
		t.Fatalf("GetGroupSummary() error = %v", err)
	}
	if got.ServiceStatus != GroupCollectionStatusUnavailable || got.CredentialCount != 1 || got.ModelCount != 0 {
		t.Fatalf("GetGroupSummary() = %#v, want unavailable with one key and zero models", got)
	}
}

func TestGetGroupHTTPContractAndAuthentication(t *testing.T) {
	t.Parallel()
	initControlI18n(t)
	fixture := newServiceFixture(t)
	group := validControlGroup("detail-http")
	if err := fixture.db.Create(group).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.manager.Publish(mustBuildCompileInput(t, fixture.db)); err != nil {
		t.Fatal(err)
	}
	engine := gin.New()
	NewServer(&config.Config{AuthKey: "test-auth-key"}, fixture.service).RegisterRoutes(engine)

	for _, test := range []struct {
		name       string
		path       string
		auth       string
		wantStatus int
		wantCode   any
	}{
		{name: "success", path: fmt.Sprintf("/api/groups/%d", group.ID), auth: "Bearer test-auth-key", wantStatus: http.StatusOK, wantCode: float64(0)},
		{name: "missing", path: "/api/groups/9999", auth: "Bearer test-auth-key", wantStatus: http.StatusNotFound, wantCode: "NOT_FOUND"},
		{name: "zero id", path: "/api/groups/0", auth: "Bearer test-auth-key", wantStatus: http.StatusBadRequest, wantCode: "BAD_REQUEST"},
		{name: "overflow id", path: "/api/groups/184467440737095516160", auth: "Bearer test-auth-key", wantStatus: http.StatusBadRequest, wantCode: "BAD_REQUEST"},
		{name: "unauthorized", path: fmt.Sprintf("/api/groups/%d", group.ID), wantStatus: http.StatusUnauthorized, wantCode: "UNAUTHORIZED"},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, test.path, nil)
			if test.auth != "" {
				request.Header.Set("Authorization", test.auth)
			}
			recorder := httptest.NewRecorder()
			engine.ServeHTTP(recorder, request)
			if recorder.Code != test.wantStatus {
				t.Fatalf("response = %d %s, want %d", recorder.Code, recorder.Body.String(), test.wantStatus)
			}
			var envelope map[string]any
			if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
				t.Fatal(err)
			}
			if envelope["code"] != test.wantCode {
				t.Fatalf("code = %#v, want %#v", envelope["code"], test.wantCode)
			}
			if test.name == "success" {
				data, ok := envelope["data"].(map[string]any)
				if !ok {
					t.Fatalf("summary data = %#v", envelope["data"])
				}
				for _, name := range []string{"models", "config", "effective_config", "enabled", "weight_manual", "validation_model"} {
					if _, exists := data[name]; exists {
						t.Fatalf("summary HTTP response exposes %q: %#v", name, data)
					}
				}
			}
		})
	}
}

func containsJSONToken(document []byte, token string) bool {
	return bytes.Contains(document, []byte(`"`+token+`"`))
}

func TestServiceCapabilitiesRetainEvidenceAcrossUnrelatedTraffic(t *testing.T) {
	t.Parallel()
	initControlI18n(t)
	fixture := newServiceFixture(t)
	now := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
	fixture.service.now = func() time.Time { return now }
	group := validControlGroup("service-evidence")
	group.ChannelID = "cohere"
	group.Params = models.JSON(`{}`)
	group.Models = models.JSON(`[{"id":"embed-v4.0"},{"id":"other-model"},{"id":"embed-v4.0","alias":"public-embed","alias_enabled":true}]`)
	if err := fixture.db.Create(group).Error; err != nil {
		t.Fatal(err)
	}
	entry := createGroupCollectionKey(t, fixture, group.ID, models.CredentialStatusActive, nil)
	publishGroupCollectionRuntime(t, fixture, []state.CredentialEntry{entry})
	makeRow := func(id, model, status string, at time.Time, evidence models.JSON) models.RequestLog {
		return models.RequestLog{ID: id, CompletedAtMS: at.UnixMilli(), GroupID: group.ID, CredentialID: entry.ID, Protocol: "cohere-embeddings", ClientModel: model, UpstreamModel: model, Status: status, ServiceEvidence: evidence}
	}
	for i := 0; i < 205; i++ {
		row := makeRow(fmt.Sprintf("ordinary-%03d", i), "other-model", "success", now, nil)
		if err := fixture.db.Create(&row).Error; err != nil {
			t.Fatal(err)
		}
	}
	e := telemetry.ServiceEvidence{Format: "cohere-embeddings", Input: []string{"image"}, IdentityGeneration: entry.IdentityGeneration, ResultCheck: "not_checked", Quality: "not_evaluated"}
	raw, _ := json.Marshal(e)
	spaced, _ := json.MarshalIndent(e, "", " ")
	for i, row := range []models.RequestLog{makeRow("verified-latest", "embed-v4.0", "success", now.Add(-time.Minute), raw), makeRow("same-shape", "embed-v4.0", "success", now.Add(-2*time.Minute), spaced), makeRow("failed", "embed-v4.0", "error", now.Add(-3*time.Minute), raw)} {
		if err := fixture.db.Create(&row).Error; err != nil {
			t.Fatalf("evidence %d: %v", i, err)
		}
	}
	engine := gin.New()
	NewServer(&config.Config{AuthKey: authTestKey}, fixture.service).RegisterRoutes(engine)
	retentionDays := fixture.manager.Current().Settings.RequestLogRetentionDays
	expired := makeRow("expired-evidence", "embed-v4.0", "success", now.AddDate(0, 0, -retentionDays).Add(-time.Minute), raw)
	if err := fixture.db.Create(&expired).Error; err != nil {
		t.Fatal(err)
	}
	response := performGroupCollectionRequest(engine, fmt.Sprintf("/api/groups/%d/services", group.ID), "Bearer "+authTestKey)
	if response.Code != 200 {
		t.Fatalf("response: %d %s", response.Code, response.Body.String())
	}
	var result struct {
		Data struct {
			Models     []modelServiceEvidence `json:"models"`
			WindowDays int                    `json:"verification_window_days"`
			Scanned    int                    `json:"verification_scanned_records"`
			Truncated  bool                   `json:"verification_truncated"`
		}
	}
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	got := result.Data.Models[0].AccountVerification
	if result.Data.WindowDays != min(30, retentionDays) || len(result.Data.Models) != 2 || len(got) != 2 || got[0].RequestID != "verified-latest" || !got[0].CurrentIdentity || got[1].Status != "error" || result.Data.Scanned != 3 || result.Data.Truncated {
		t.Fatalf("evidence=%+v, result=%+v", got, result.Data)
	}
	var frequent []models.RequestLog
	for i := 0; i <= serviceEvidenceScanLimit; i++ {
		frequent = append(frequent, makeRow(fmt.Sprintf("bounded-%03d", i), "embed-v4.0", "success", now.Add(-time.Second), raw))
	}
	if err := fixture.db.Create(&frequent).Error; err != nil {
		t.Fatal(err)
	}
	response = performGroupCollectionRequest(engine, fmt.Sprintf("/api/groups/%d/services", group.ID), "Bearer "+authTestKey)
	if response.Code != 200 {
		t.Fatalf("bounded response: %d %s", response.Code, response.Body.String())
	}
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if !result.Data.Truncated || result.Data.Scanned != serviceEvidenceScanLimit {
		t.Fatalf("bounded history was presented as complete: %+v", result.Data)
	}
}

func TestServiceRouteEvidenceUsesOperationSpecificMethodsAndPaths(t *testing.T) {
	for _, test := range []struct {
		o            execution.Operation
		method, path string
	}{
		{execution.OperationResponsesCreate, "POST", "/v1/responses"},
		{execution.OperationResponsesRetrieve, "GET", "/v1/responses/{response_id}"},
		{execution.OperationResponsesDelete, "DELETE", "/v1/responses/{response_id}"},
		{execution.OperationResponsesCancel, "POST", "/v1/responses/{response_id}/cancel"},
		{execution.OperationResponsesInputItems, "GET", "/v1/responses/{response_id}/input_items"},
	} {
		path, _ := servicePath(protocol.OpenAIResponses, test.o, "openai")
		if path != test.path || serviceMethod(test.o) != test.method {
			t.Fatalf("operation %s: %s %s", test.o, serviceMethod(test.o), path)
		}
	}
	provider := modelEvidence("siliconflow", "Qwen/Qwen3-VL-Reranker-8B")
	route := serviceRoute(protocol.Rerank, execution.OperationRerank, execution.RouteNative, "POST", "/v1/rerank", "relevance")
	if len(provider.QueryInput) != 3 || len(route.QueryInput) != 2 || route.Limitations[0] != "video_input_not_validated" {
		t.Fatal("provider declaration and implementation were conflated")
	}
}

func TestGroupSettingsShowEffectiveSharedAccountLimits(t *testing.T) {
	t.Parallel()
	fixture := newServiceFixture(t)
	first, second := validControlGroup("account-wide"), validControlGroup("account-narrow")
	first.Overrides = models.JSON(`{"account_pool":"same-account","account_concurrency_limit":10,"account_rpm_limit":30}`)
	second.Overrides = models.JSON(`{"account_pool":"same-account","account_concurrency_limit":2,"account_rpm_limit":8}`)
	if err := fixture.db.Create([]*models.Group{first, second}).Error; err != nil {
		t.Fatal(err)
	}
	publishGroupCollectionRuntime(t, fixture, nil)
	got, err := fixture.service.GetGroupSettings(t.Context(), first.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Effective.AccountConcurrencyLimit != 2 || got.Effective.AccountRPMLimit != 8 || fmt.Sprint(got.Overrides["account_concurrency_limit"]) != "10" {
		t.Fatalf("effective/override=%+v / %+v", got.Effective, got.Overrides)
	}
}

func TestTransparentSourceCanBeConfiguredWithoutModels(t *testing.T) {
	t.Parallel()
	initControlI18n(t)
	fixture := newServiceFixture(t)
	var request GroupCreateRequest
	if err := json.Unmarshal([]byte(`{"name":"HTTP source","channel_id":"transparent_http","connection_type":"api_key","params":{},"models":[],"credentials":"synthetic-static-key"}`), &request); err != nil {
		t.Fatal(err)
	}
	created, err := fixture.service.CreateGroup(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}
	initial, err := fixture.service.GetGroupSummary(t.Context(), created.GroupID)
	if err != nil {
		t.Fatal(err)
	}
	if initial.ServiceStatusReason == nil || *initial.ServiceStatusReason != GroupUnavailableReasonTransparentDisabled {
		t.Fatalf("unconfigured transparent source status: %+v", initial)
	}
	proxy := state.TransparentProxyConfig{Enabled: true, BaseURL: "https://service.example/prefix", AuthHeader: "X-API-Key", AuthPrefix: "", Strategy: "rotate"}
	settings, err := fixture.service.UpdateGroupSettings(t.Context(), created.GroupID, GroupSettingsUpdateRequest{Overrides: optionalField[config.Settings]{Set: true, Value: config.Settings{state.SettingTransparentProxy: proxy}}})
	if err != nil {
		t.Fatal(err)
	}
	if settings.Effective.TransparentProxy != proxy {
		t.Fatalf("transparent effective settings: %+v", settings.Effective.TransparentProxy)
	}
	groups, err := fixture.service.ListModernGroups(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(groups.Items) != 1 || groups.Items[0].Availability != "ready" || groups.Items[0].Endpoint != proxy.BaseURL || groups.Items[0].ModelCount != 0 {
		t.Fatalf("transparent group presentation: %+v", groups.Items)
	}
	proxy.Strategy = "fixed"
	proxy.FixedCredentialID = fixture.registry.CaptureActiveCredentialRefs([]uint{created.GroupID})[0].ID
	body, _ := json.Marshal(map[string]any{"overrides": map[string]any{"transparent_proxy": proxy}})
	engine := gin.New()
	NewServer(&config.Config{AuthKey: authTestKey}, fixture.service).RegisterRoutes(engine)
	put := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/groups/%d/settings", created.GroupID), bytes.NewReader(body))
	put.Header.Set("Authorization", "Bearer "+authTestKey)
	put.Header.Set("Content-Type", "application/json")
	setRequiredTestIdempotencyHeader(put)
	saved := httptest.NewRecorder()
	engine.ServeHTTP(saved, put)
	if saved.Code != http.StatusOK {
		t.Fatalf("fixed credential HTTP save failed: %d %s", saved.Code, saved.Body.String())
	}
	settings, err = fixture.service.GetGroupSettings(t.Context(), created.GroupID)
	if err != nil {
		t.Fatal(err)
	}
	if settings.Effective.TransparentProxy != proxy {
		t.Fatalf("fixed credential was not saved: %+v", settings.Effective.TransparentProxy)
	}
	name := "Another HTTP source"
	request.Name = &name
	if _, err = fixture.service.CreateGroup(t.Context(), request); err != nil {
		t.Fatalf("unconfigured targets were incorrectly considered identical: %v", err)
	}
}
