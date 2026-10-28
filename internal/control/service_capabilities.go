package control

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"

	"gpt-load/internal/channel"
	"gpt-load/internal/execution"
	"gpt-load/internal/platform/config"
	"gpt-load/internal/platform/response"
	"gpt-load/internal/platform/version"
	"gpt-load/internal/protocol"
	"gpt-load/internal/state"
	storagemodels "gpt-load/internal/storage/models"
	"gpt-load/internal/telemetry"
)

const serviceEvidenceScanLimit = 200
const serviceEvidencePerModelLimit = 5

type serviceRouteEvidence struct {
	Method        string              `json:"method"`
	Input         []string            `json:"input"`
	QueryInput    []string            `json:"query_input"`
	DocumentInput []string            `json:"document_input"`
	Limitations   []string            `json:"limitations"`
	Protocol      protocol.Protocol   `json:"protocol"`
	Operation     execution.Operation `json:"operation"`
	Mode          execution.RouteMode `json:"mode"`
	Path          string              `json:"path"`
	Output        string              `json:"output"`
}
type modelServiceEvidence struct {
	Model               string                   `json:"model"`
	OfficialInput       []string                 `json:"official_input"`
	OfficialOutput      []string                 `json:"official_output"`
	ProviderInput       []string                 `json:"provider_input"`
	QueryInput          []string                 `json:"query_input"`
	DocumentInput       []string                 `json:"document_input"`
	Source              string                   `json:"source"`
	CheckedAt           string                   `json:"checked_at"`
	AccountVerification []accountServiceEvidence `json:"account_verification"`
}

type accountServiceEvidence struct {
	telemetry.ServiceEvidence
	CredentialID    uint   `json:"credential_id"`
	CurrentIdentity bool   `json:"current_identity"`
	RequestID       string `json:"request_id"`
	TestedAtMS      int64  `json:"tested_at_ms"`
	Status          string `json:"status"`
}

func (s *Server) handleServiceCapabilities(c *gin.Context) {
	id, ok := groupID(c, "service_capabilities")
	if !ok {
		return
	}
	group, err := loadGroupRow(s.service.db.WithContext(c.Request.Context()), id)
	if err != nil {
		writeServiceError(c, "service_capabilities", err)
		return
	}
	descriptor, _ := s.service.channelRegistry.Get(channel.ID(group.ChannelID))
	routes := []serviceRouteEvidence{}
	for _, route := range descriptor.Routes {
		if route.ClientProtocol == protocol.TransparentHTTP {
			continue
		}
		if route.Operation == execution.OperationListModels || route.Operation == execution.OperationProbe {
			continue
		}
		path, output := servicePath(route.ClientProtocol, route.Operation, group.ChannelID)
		if route.ClientProtocol == protocol.NativeServices {
			for _, resourcePath := range nativeServicePaths(group.ChannelID, route.Operation) {
				method, path, _ := strings.Cut(resourcePath, " ")
				routes = append(routes, serviceRoute(route.ClientProtocol, route.Operation, route.RouteMode, method, path, output))
			}
			continue
		}
		routes = append(routes, serviceRoute(route.ClientProtocol, route.Operation, route.RouteMode, serviceMethod(route.Operation), path, output))
	}
	var overrides config.Settings
	if err := decodeGroupDiscoveryJSON(group.Overrides, &overrides); err != nil {
		writeServiceError(c, "service_capabilities", err)
		return
	}
	resolved, err := state.ResolveGroupRuntimeSettings(s.service.manager.Current().Settings, overrides)
	if err != nil {
		writeServiceError(c, "service_capabilities", err)
		return
	}
	if resolved.TransparentProxy.Enabled {
		routes = append(routes, serviceRoute(protocol.TransparentHTTP, execution.OperationTransparentRequest, execution.RouteNative, "GET/HEAD/POST/PUT/PATCH/DELETE/OPTIONS", fmt.Sprintf("/proxy/%d/{path}", id), "opaque"))
	}
	configured := []GroupModel{}
	if err = decodeGroupDiscoveryJSON(group.Models, &configured); err != nil {
		writeServiceError(c, "service_capabilities", err)
		return
	}
	models := make([]modelServiceEvidence, 0, len(configured))
	modelIndexes := make(map[string]int, len(configured))
	modelIDs := make([]string, 0, len(configured))
	for _, model := range configured {
		if _, exists := modelIndexes[model.ID]; exists {
			continue
		}
		e := modelEvidence(group.ChannelID, model.ID)
		e.AccountVerification = []accountServiceEvidence{}
		modelIndexes[model.ID] = len(models)
		modelIDs = append(modelIDs, model.ID)
		models = append(models, e)
	}
	var records []storagemodels.RequestLog
	windowDays := min(30, s.service.manager.Current().Settings.RequestLogRetentionDays)
	if len(modelIDs) > 0 {
		if err = s.service.db.WithContext(c.Request.Context()).Select("id", "credential_id", "upstream_model", "completed_at_ms", "status", "service_evidence").
			Where("group_id = ? AND completed_at_ms >= ? AND service_evidence IS NOT NULL AND upstream_model IN ?", id, s.service.now().AddDate(0, 0, -windowDays).UnixMilli(), modelIDs).
			Order("completed_at_ms DESC, id DESC").Limit(serviceEvidenceScanLimit + 1).Find(&records).Error; err != nil {
			writeServiceError(c, "service_capabilities", err)
			return
		}
	}
	truncated := len(records) > serviceEvidenceScanLimit
	if truncated {
		records = records[:serviceEvidenceScanLimit]
	}
	seen := map[string]bool{}
	for _, row := range records {
		var evidence telemetry.ServiceEvidence
		if err = json.Unmarshal(row.ServiceEvidence, &evidence); err != nil {
			writeServiceError(c, "service_capabilities", err)
			return
		}
		keyBytes, _ := json.Marshal([]any{row.UpstreamModel, row.CredentialID, row.Status, evidence})
		key := string(keyBytes)
		if seen[key] {
			continue
		}
		seen[key] = true
		e := &models[modelIndexes[row.UpstreamModel]]
		if len(e.AccountVerification) == serviceEvidencePerModelLimit {
			truncated = true
			continue
		}
		ref, exists := s.service.registry.CredentialRef(row.CredentialID)
		e.AccountVerification = append(e.AccountVerification, accountServiceEvidence{evidence, row.CredentialID, exists && ref.GroupID == id && ref.IdentityGeneration == evidence.IdentityGeneration, row.ID, row.CompletedAtMS, row.Status})
	}
	response.SuccessI18n(c, "common.success", gin.H{"routes": routes, "models": models, "implementation_version": version.Version, "verification_window_days": windowDays, "verification_scan_limit": serviceEvidenceScanLimit, "verification_per_model_limit": serviceEvidencePerModelLimit, "verification_scanned_records": len(records), "verification_truncated": truncated})
}

func nativeServicePaths(service string, operation execution.Operation) []string {
	root := "/native/" + service
	switch operation {
	case execution.OperationResourceCreate:
		switch service {
		case "mistral":
			return []string{"POST " + root + "/v1/files", "POST " + root + "/v1/batch/jobs"}
		case "soniox":
			return []string{"POST " + root + "/v1/files", "POST " + root + "/v1/transcriptions", "POST " + root + "/tts"}
		case "siliconflow":
			return []string{"POST " + root + "/v1/video/submit"}
		}
	case execution.OperationResourceRead:
		switch service {
		case "mistral":
			return []string{"GET " + root + "/v1/files/{id}", "GET " + root + "/v1/files/{id}/content", "GET " + root + "/v1/files/{id}/url", "GET " + root + "/v1/batch/jobs/{id}"}
		case "soniox":
			return []string{"GET " + root + "/v1/files/{id}", "GET " + root + "/v1/transcriptions/{id}", "GET " + root + "/v1/transcriptions/{id}/transcript"}
		case "siliconflow":
			return []string{"POST " + root + "/v1/video/status"}
		}
	case execution.OperationResourceDelete:
		switch service {
		case "mistral":
			return []string{"DELETE " + root + "/v1/files/{id}", "DELETE " + root + "/v1/batch/jobs/{id}"}
		case "soniox":
			return []string{"DELETE " + root + "/v1/files/{id}", "DELETE " + root + "/v1/transcriptions/{id}"}
		}
	case execution.OperationResourceCancel:
		if service == "mistral" {
			return []string{"POST " + root + "/v1/batch/jobs/{id}/cancel"}
		}
	}
	return nil
}

func modelEvidence(channelID, id string) modelServiceEvidence {
	e := modelServiceEvidence{Model: id}
	switch channelID {
	case "cohere":
		switch id {
		case "embed-v4.0", "embed-english-v3.0", "embed-english-light-v3.0", "embed-multilingual-v3.0", "embed-multilingual-light-v3.0":
			e.OfficialInput = []string{"text", "image"}
			e.ProviderInput = e.OfficialInput
			e.OfficialOutput = []string{"vector"}
			e.Source = "https://docs.cohere.com/docs/cohere-embed"
		}
	case "gemini":
		if id == "gemini-embedding-2" {
			e.OfficialInput = []string{"text", "image", "audio", "video", "pdf"}
		} else if id == "gemini-embedding-001" {
			e.OfficialInput = []string{"text"}
		}
		if len(e.OfficialInput) > 0 {
			e.ProviderInput = e.OfficialInput
			e.OfficialOutput = []string{"vector"}
			e.Source = "https://ai.google.dev/gemini-api/docs/embeddings"
		}
	case "siliconflow":
		if id == "Qwen/Qwen3-VL-Embedding-8B" {
			e.OfficialInput = []string{"text", "image", "video"}
			e.ProviderInput = []string{"text", "image"}
			e.OfficialOutput = []string{"vector"}
			e.Source = "https://docs.siliconflow.cn/cn/api-reference/embeddings/create-embeddings"
		}
		if id == "Qwen/Qwen3-VL-Reranker-8B" {
			e.OfficialInput = []string{"text", "image", "video"}
			e.ProviderInput = []string{"text", "image", "video"}
			e.QueryInput = e.ProviderInput
			e.DocumentInput = e.ProviderInput
			e.OfficialOutput = []string{"relevance"}
			e.Source = "https://docs.siliconflow.cn/cn/api-reference/rerank/create-rerank"
		}
	}
	if e.Source != "" {
		e.CheckedAt = "2026-10-27"
	}
	return e
}

func servicePath(p protocol.Protocol, o execution.Operation, channelID string) (string, string) {
	switch p {
	case protocol.CohereEmbeddings:
		return "/v2/embed", "vector"
	case protocol.OpenAIEmbeddings:
		return "/v1/embeddings", "vector"
	case protocol.GeminiEmbeddings:
		return "/v1beta/models/{model}:embedContent", "vector"
	case protocol.Rerank:
		return "/v1/rerank", "relevance"
	case protocol.Decisions:
		return "/v1/systemone", "decision"
	case protocol.AudioSpeech:
		return "/v1/audio/speech", "audio"
	case protocol.OpenAIImages:
		if o == execution.OperationImagesEdit {
			return "/v1/images/edits", "image"
		}
		return "/v1/images/generations", "image"
	case protocol.OpenAICompletions:
		return "/v1/chat/completions", "text/tools"
	case protocol.OpenAIResponses:
		switch o {
		case execution.OperationWebSearch:
			return "/v1/alpha/search", "search_results"
		case execution.OperationResponsesPassthrough:
			return "/v1/responses/{vendor_path}", "response"
		case execution.OperationResponsesRetrieve, execution.OperationResponsesDelete:
			return "/v1/responses/{response_id}", "response"
		case execution.OperationResponsesCancel:
			return "/v1/responses/{response_id}/cancel", "response"
		case execution.OperationResponsesInputItems:
			return "/v1/responses/{response_id}/input_items", "content"
		case execution.OperationResponsesCompact:
			return "/v1/responses/compact", "response"
		case execution.OperationResponsesInputTokens:
			return "/v1/responses/input_tokens", "token_count"
		}
		return "/v1/responses", "response"
	case protocol.Anthropic:
		if o == execution.OperationCountTokens {
			return "/v1/messages/count_tokens", "token_count"
		}
		return "/v1/messages", "text/tools"
	case protocol.Gemini:
		if o == execution.OperationCountTokens {
			return "/v1beta/models/{model}:countTokens", "token_count"
		}
		return "/v1beta/models/{model}:generateContent", "content"
	case protocol.CodexLive:
		return "/v1/realtime/calls", "audio"
	case protocol.NativeServices:
		return "/native/" + channelID + "/v1/{resource}", "resource"
	case protocol.Mistral:
		switch o {
		case execution.OperationMistralFIM:
			return "/v1/fim/completions", "text"
		case execution.OperationMistralModeration:
			return "/v1/moderations", "classification"
		case execution.OperationMistralChatModeration:
			return "/v1/chat/moderations", "classification"
		case execution.OperationMistralClassification:
			return "/v1/classifications", "classification"
		case execution.OperationMistralVoices:
			return "/mistral/v1/audio/voices", "voices"
		case execution.OperationMistralRealtimeTranscription:
			return "/v1/audio/transcriptions/realtime", "transcript"
		case execution.OperationMistralOCR:
			return "/v1/ocr", "document/pages"
		case execution.OperationMistralAudioSpeech:
			return "/v1/audio/speech", "audio"
		case execution.OperationMistralAudioTranscription:
			return "/v1/audio/transcriptions", "transcript"
		}
	}
	return "", "unknown"
}

// Route input describes gateway parsing and forwarding, never model or account support.
func serviceRoute(p protocol.Protocol, o execution.Operation, mode execution.RouteMode, method, path, output string) serviceRouteEvidence {
	r := serviceRouteEvidence{Protocol: p, Operation: o, Mode: mode, Method: method, Path: path, Output: output, Limitations: []string{}}
	switch p {
	case protocol.AudioSpeech:
		r.Input = []string{"text"}
		r.Limitations = []string{"buffered_audio"}
	case protocol.CohereEmbeddings:
		r.Input = []string{"text", "image"}
	case protocol.OpenAIEmbeddings:
		r.Input = []string{"text"}
		if mode == execution.RouteNative {
			r.Input = append(r.Input, "image")
		}
	case protocol.Rerank:
		r.QueryInput = []string{"text", "image"}
		r.DocumentInput = []string{"text", "image"}
		r.Limitations = []string{"video_input_not_validated"}
	case protocol.GeminiEmbeddings:
		if mode == execution.RouteNative {
			r.Input = []string{"text", "image", "audio", "video", "pdf"}
		}
	}
	if p == protocol.NativeServices && strings.HasSuffix(path, "/tts") {
		r.Input = []string{"text"}
		r.Output = "audio"
		r.Limitations = []string{"buffered_audio"}
	}
	return r
}
func serviceMethod(o execution.Operation) string {
	switch o {
	case execution.OperationResponsesPassthrough:
		return "ANY"
	case execution.OperationResponsesRetrieve, execution.OperationResponsesInputItems, execution.OperationMistralVoices, execution.OperationMistralRealtimeTranscription:
		return "GET"
	case execution.OperationResponsesDelete:
		return "DELETE"
	default:
		return "POST"
	}
}
