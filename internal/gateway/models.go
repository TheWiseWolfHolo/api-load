package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"gpt-load/internal/connection"
	"gpt-load/internal/dialect"
	"gpt-load/internal/execution"
	"gpt-load/internal/protocol"
	"gpt-load/internal/scheduler"
	"gpt-load/internal/state"
)

const (
	modelPlaceholderRFC3339 = "2025-01-01T00:00:00Z"
	modelPlaceholderUnix    = int64(1735689600)
)

var (
	errModelListTooLarge     = errors.New("model list exceeds response limit")
	errLiveModelsUnavailable = errors.New("upstream model list unavailable")
)

// liveModelListTimeout bounds the whole upstream lookup behind one /models call.
const liveModelListTimeout = 20 * time.Second

// LiveModelLister reads the model IDs a group's upstream reports right now.
type LiveModelLister interface {
	LiveGroupModels(ctx context.Context, groupID uint) ([]string, error)
}

// SetLiveModelLister turns on transparent model lists. It is called during
// dependency assembly, before HTTP traffic.
func (handler *Handler) SetLiveModelLister(lister LiveModelLister) {
	handler.liveModels = lister
}

type openAIModelList struct {
	Object string        `json:"object"`
	Data   []openAIModel `json:"data"`
}

type openAIModel struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

type anthropicModelList struct {
	Data    []anthropicModel `json:"data"`
	HasMore bool             `json:"has_more"`
	FirstID string           `json:"first_id"`
	LastID  string           `json:"last_id"`
}

type anthropicModel struct {
	Type        string `json:"type"`
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	CreatedAt   string `json:"created_at"`
}

type geminiModelList struct {
	Models []geminiModel `json:"models"`
}

type geminiModel struct {
	Name string `json:"name"`
}

func visibleModelIDs(
	snapshot *state.ConfigSnapshot,
	accessKey state.AccessKeyView,
	value protocol.Protocol,
) []string {
	result, err := collectVisibleModelIDs(snapshot, accessKey, value, math.MaxInt64)
	if err != nil {
		return []string{}
	}
	return result
}

func collectVisibleModelIDs(
	snapshot *state.ConfigSnapshot,
	accessKey state.AccessKeyView,
	value protocol.Protocol,
	limit int64,
) ([]string, error) {
	return collectVisibleModelIDsExcluding(snapshot, accessKey, value, limit, nil)
}

// collectVisibleModelIDsExcluding lists catalog models, leaving out routes of
// the excluded groups. Transparent groups publish their live list instead.
func collectVisibleModelIDsExcluding(
	snapshot *state.ConfigSnapshot,
	accessKey state.AccessKeyView,
	value protocol.Protocol,
	limit int64,
	exclude map[uint]struct{},
) ([]string, error) {
	emptyBody, err := marshalModelList(value, nil)
	if err != nil {
		return nil, err
	}
	encodedLowerBound := int64(len(emptyBody))
	if limit < encodedLowerBound {
		return nil, errModelListTooLarge
	}
	if snapshot == nil {
		return []string{}, nil
	}
	visible := make(map[string]struct{})
	for _, selectedProtocol := range modelListProtocols(value) {
		if len(accessKey.Filters.Protocols) > 0 {
			if _, ok := accessKey.Filters.Protocols[selectedProtocol]; !ok {
				continue
			}
		}
		for _, byModel := range snapshot.ExecutionCandidates[selectedProtocol] {
			for modelID, targets := range byModel {
				if modelID == state.NoModelRouteKey {
					continue
				}
				if len(accessKey.Filters.Models) > 0 {
					if _, ok := accessKey.Filters.Models[modelID]; !ok {
						continue
					}
				}
				if len(exclude) > 0 {
					targets = targetsOutside(targets, exclude)
				}
				if !anyVisibleTarget(targets, accessKey.Filters.Groups, accessKey.Filters.GroupsRestricted) {
					continue
				}
				visible[modelID] = struct{}{}
			}
		}
		if snapshot.AutoModels.Enabled() {
			operation := execution.OperationChatCompletion
			if selectedProtocol == protocol.OpenAIResponses {
				operation = execution.OperationResponsesCreate
			}
			if selectedProtocol == protocol.OpenAICompletions || selectedProtocol == protocol.OpenAIResponses || selectedProtocol == protocol.Anthropic || selectedProtocol == protocol.Gemini {
				for _, entry := range snapshot.AutoModels.Config().Models {
					compiled, _ := snapshot.AutoModels.Lookup(entry.Name)
					if !compiled.Enabled {
						continue
					}
					_, allowed := allowedAutoPresets(snapshot, accessKey, compiled, dialect.RequestMetadata{Operation: operation}, scheduler.Query{ClientProtocol: selectedProtocol})
					if allowed {
						visible[entry.Name] = struct{}{}
					}
				}
			}
		}
	}
	result := make([]string, 0, len(visible))
	for modelID := range visible {
		result = append(result, modelID)
	}
	sort.Strings(result)
	for index, modelID := range result {
		item, err := marshalModelListItem(value, modelID)
		if err != nil {
			return nil, err
		}
		required := int64(len(item))
		if index > 0 {
			required++
		}
		if required > limit-encodedLowerBound {
			return nil, errModelListTooLarge
		}
		encodedLowerBound += required
	}
	return result, nil
}

func modelListProtocols(value protocol.Protocol) []protocol.Protocol {
	if value == protocol.OpenAICompletions {
		return []protocol.Protocol{
			protocol.OpenAICompletions,
			protocol.OpenAIResponses,
			protocol.OpenAIImages,
			protocol.OpenAIEmbeddings,
			protocol.CodexLive,
			protocol.Rerank,
			protocol.Decisions,
		}
	}
	if value == protocol.Gemini {
		return []protocol.Protocol{protocol.Gemini, protocol.GeminiEmbeddings}
	}
	return []protocol.Protocol{value}
}

func targetsOutside(targets []state.RouteTarget, exclude map[uint]struct{}) []state.RouteTarget {
	kept := make([]state.RouteTarget, 0, len(targets))
	for _, target := range targets {
		if _, skipped := exclude[target.GroupID]; !skipped {
			kept = append(kept, target)
		}
	}
	return kept
}

// transparentModelGroups returns the enabled API-key groups this key may use
// that publish whatever their upstream reports.
func transparentModelGroups(snapshot *state.ConfigSnapshot, accessKey state.AccessKeyView) []uint {
	groups := make([]uint, 0)
	if snapshot == nil {
		return groups
	}
	for id, group := range snapshot.Groups {
		if !group.TransparentModels || connection.Normalize(group.ConnectionType) != connection.APIKey {
			continue
		}
		if len(accessKey.Filters.Groups) > 0 || accessKey.Filters.GroupsRestricted {
			if _, allowed := accessKey.Filters.Groups[id]; !allowed {
				continue
			}
		}
		groups = append(groups, id)
	}
	sort.Slice(groups, func(i, j int) bool { return groups[i] < groups[j] })
	return groups
}

// buildLiveModelList publishes the upstream's current models for transparent
// groups and the catalog for the others. A group whose upstream cannot be
// reached contributes nothing; the call fails only when nothing is left.
func (handler *Handler) buildLiveModelList(
	ctx context.Context,
	snapshot *state.ConfigSnapshot,
	accessKey state.AccessKeyView,
	value protocol.Protocol,
) ([]byte, error) {
	transparent := transparentModelGroups(snapshot, accessKey)
	exclude := make(map[uint]struct{}, len(transparent))
	for _, id := range transparent {
		exclude[id] = struct{}{}
	}
	ids, err := collectVisibleModelIDsExcluding(snapshot, accessKey, value, math.MaxInt64, exclude)
	if err != nil {
		return nil, err
	}
	seen := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		seen[id] = struct{}{}
	}
	ctx, cancel := context.WithTimeout(ctx, liveModelListTimeout)
	defer cancel()
	failed := 0
	for _, groupID := range transparent {
		live, liveErr := handler.liveModels.LiveGroupModels(ctx, groupID)
		if liveErr != nil {
			failed++
			continue
		}
		for _, id := range live {
			if _, duplicate := seen[id]; !duplicate {
				seen[id] = struct{}{}
				ids = append(ids, id)
			}
		}
	}
	if failed > 0 && len(ids) == 0 {
		return nil, errLiveModelsUnavailable
	}
	sort.Strings(ids)
	body, err := marshalModelList(value, ids)
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > handler.modelListLimit {
		return nil, errModelListTooLarge
	}
	return body, nil
}

func anyVisibleTarget(targets []state.RouteTarget, groups map[uint]struct{}, restricted ...bool) bool {
	if len(targets) == 0 {
		return false
	}
	if len(groups) == 0 && (len(restricted) == 0 || !restricted[0]) {
		return true
	}
	for _, target := range targets {
		if _, ok := groups[target.GroupID]; ok {
			return true
		}
	}
	return false
}

func (handler *Handler) writeVisibleModelList(
	ginContext *gin.Context,
	snapshot *state.ConfigSnapshot,
	accessKey state.AccessKeyView,
	value protocol.Protocol,
) {
	var body []byte
	var err error
	codexRequest := value == protocol.OpenAICompletions && ginContext.Query("client_version") != ""
	switch {
	case codexRequest:
		body, err = buildCodexModelList(snapshot, accessKey, handler.modelListLimit, ginContext.Query("client_version"))
	case handler.liveModels != nil:
		body, err = handler.buildLiveModelList(ginContext.Request.Context(), snapshot, accessKey, value)
	default:
		body, err = buildVisibleModelList(snapshot, accessKey, value, handler.modelListLimit)
	}
	if err != nil {
		if errors.Is(err, errLiveModelsUnavailable) {
			_ = handler.writeReason(ginContext, reasonModelListUpstream)
			return
		}
		_ = handler.writeReason(ginContext, reasonModelListTooLarge)
		return
	}
	headers := http.Header{
		"Content-Length": {strconv.Itoa(len(body))},
		"Content-Type":   {"application/json; charset=utf-8"},
	}
	if codexRequest {
		headers.Set("Cache-Control", "private, no-store")
	}
	if err := handler.writeBufferedResponse(ginContext, http.StatusOK, headers, body); err != nil {
		return
	}
}

func buildVisibleModelList(
	snapshot *state.ConfigSnapshot,
	accessKey state.AccessKeyView,
	value protocol.Protocol,
	limit int64,
) ([]byte, error) {
	if limit < 0 {
		return nil, errModelListTooLarge
	}
	ids, err := collectVisibleModelIDs(snapshot, accessKey, value, limit)
	if err != nil {
		return nil, err
	}
	body, err := marshalModelList(value, ids)
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > limit {
		return nil, errModelListTooLarge
	}
	return body, nil
}

func marshalModelListItem(value protocol.Protocol, id string) ([]byte, error) {
	switch value {
	case protocol.Anthropic:
		return json.Marshal(anthropicModel{
			Type: "model", ID: id, DisplayName: id, CreatedAt: modelPlaceholderRFC3339,
		})
	case protocol.Gemini:
		return json.Marshal(geminiModel{Name: "models/" + id})
	default:
		return json.Marshal(openAIModel{
			ID: id, Object: "model", Created: modelPlaceholderUnix, OwnedBy: "gpt-load",
		})
	}
}

func marshalModelList(value protocol.Protocol, ids []string) ([]byte, error) {
	switch value {
	case protocol.Anthropic:
		data := make([]anthropicModel, 0, len(ids))
		for _, id := range ids {
			data = append(data, anthropicModel{
				Type: "model", ID: id, DisplayName: id, CreatedAt: modelPlaceholderRFC3339,
			})
		}
		firstID, lastID := "", ""
		if len(ids) > 0 {
			firstID, lastID = ids[0], ids[len(ids)-1]
		}
		return json.Marshal(anthropicModelList{
			Data: data, HasMore: false, FirstID: firstID, LastID: lastID,
		})
	case protocol.Gemini:
		models := make([]geminiModel, 0, len(ids))
		for _, id := range ids {
			models = append(models, geminiModel{Name: "models/" + id})
		}
		return json.Marshal(geminiModelList{Models: models})
	default:
		data := make([]openAIModel, 0, len(ids))
		for _, id := range ids {
			data = append(data, openAIModel{
				ID: id, Object: "model", Created: modelPlaceholderUnix, OwnedBy: "gpt-load",
			})
		}
		return json.Marshal(openAIModelList{Object: "list", Data: data})
	}
}
