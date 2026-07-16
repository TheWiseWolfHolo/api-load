package proxy

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	app_errors "api-load/internal/errors"
	"api-load/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type upstreamObjectRouting struct {
	ForcedPoolID             uint
	ForcedResourceID         uint
	ResponseObjectType       string
	NonReplayableOnUncertain bool
}

func (ps *ProxyServer) resolveUpstreamObjectRouting(
	c *gin.Context,
	originalGroup, selectedGroup *models.Group,
	body []byte,
) (upstreamObjectRouting, *app_errors.APIError) {
	var routing upstreamObjectRouting
	if selectedGroup.ResourcePoolID == nil || *selectedGroup.ResourcePoolID == 0 {
		return routing, nil
	}

	objectType, objectID, isCollection := parseUpstreamObjectPath(proxyRelativePath(c, originalGroup.Name))
	method := c.Request.Method
	switch objectType {
	case models.UpstreamObjectTypeBatch:
		if isCollection {
			if method != http.MethodPost {
				return routing, nil
			}
			var request struct {
				InputFileID string `json:"input_file_id"`
			}
			if err := json.Unmarshal(body, &request); err != nil || strings.TrimSpace(request.InputFileID) == "" {
				return routing, app_errors.NewAPIError(app_errors.ErrValidation, "batch creation requires input_file_id")
			}
			binding, apiErr := ps.requireObjectBinding(c.Request.Context(), originalGroup.ID, models.UpstreamObjectTypeFile, request.InputFileID)
			if apiErr != nil {
				return routing, apiErr
			}
			if binding.ResourcePoolID != *selectedGroup.ResourcePoolID {
				return routing, app_errors.NewAPIError(app_errors.ErrObjectOwnerUnknown, "batch input file belongs to a different resource pool")
			}
			routing.ForcedPoolID = binding.ResourcePoolID
			routing.ForcedResourceID = binding.ResourceID
			routing.ResponseObjectType = models.UpstreamObjectTypeBatch
			routing.NonReplayableOnUncertain = true
			return routing, nil
		}
		binding, apiErr := ps.requireObjectBinding(c.Request.Context(), originalGroup.ID, objectType, objectID)
		if apiErr != nil {
			return routing, apiErr
		}
		if binding.ResourcePoolID != *selectedGroup.ResourcePoolID {
			return routing, app_errors.NewAPIError(app_errors.ErrObjectOwnerUnknown, "batch belongs to a different resource pool")
		}
		routing.ForcedPoolID = binding.ResourcePoolID
		routing.ForcedResourceID = binding.ResourceID
		routing.ResponseObjectType = models.UpstreamObjectTypeBatch
	case models.UpstreamObjectTypeFile:
		if isCollection {
			if method == http.MethodPost {
				routing.ResponseObjectType = models.UpstreamObjectTypeFile
				routing.NonReplayableOnUncertain = true
			}
			return routing, nil
		}
		binding, apiErr := ps.requireObjectBinding(c.Request.Context(), originalGroup.ID, objectType, objectID)
		if apiErr != nil {
			return routing, apiErr
		}
		if binding.ResourcePoolID != *selectedGroup.ResourcePoolID {
			return routing, app_errors.NewAPIError(app_errors.ErrObjectOwnerUnknown, "file belongs to a different resource pool")
		}
		routing.ForcedPoolID = binding.ResourcePoolID
		routing.ForcedResourceID = binding.ResourceID
		routing.ResponseObjectType = models.UpstreamObjectTypeFile
	}
	return routing, nil
}

func (ps *ProxyServer) requireObjectBinding(ctx context.Context, groupID uint, objectType, objectID string) (*models.UpstreamObjectBinding, *app_errors.APIError) {
	binding, err := ps.resourceProvider.FindObjectBinding(ctx, groupID, objectType, objectID)
	if err == nil {
		return binding, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, app_errors.NewAPIError(app_errors.ErrObjectOwnerUnknown, "upstream object owner is unknown; refusing cross-key routing")
	}
	return nil, app_errors.NewAPIError(app_errors.ErrDatabase, err.Error())
}

func (ps *ProxyServer) persistUpstreamObjectBindings(
	ctx context.Context,
	groupID uint,
	routing upstreamObjectRouting,
	resource *models.UpstreamResource,
	responseBody []byte,
) error {
	if routing.ResponseObjectType == "" || resource == nil {
		return nil
	}
	var payload map[string]any
	if err := json.Unmarshal(responseBody, &payload); err != nil {
		return err
	}
	bind := func(objectType, objectID string) error {
		objectID = strings.TrimSpace(objectID)
		if objectID == "" {
			return nil
		}
		return ps.resourceProvider.BindObject(ctx, models.UpstreamObjectBinding{
			GroupID:        groupID,
			ResourcePoolID: resource.ResourcePoolID,
			ResourceID:     resource.ID,
			ObjectType:     objectType,
			ObjectID:       objectID,
		})
	}

	rootID, _ := payload["id"].(string)
	if err := bind(routing.ResponseObjectType, rootID); err != nil {
		return err
	}
	if routing.ResponseObjectType == models.UpstreamObjectTypeBatch {
		for _, field := range []string{"input_file_id", "output_file_id", "error_file_id"} {
			if objectID, _ := payload[field].(string); objectID != "" {
				if err := bind(models.UpstreamObjectTypeFile, objectID); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func parseUpstreamObjectPath(path string) (objectType, objectID string, isCollection bool) {
	segments := strings.Split(strings.Trim(path, "/"), "/")
	for i, segment := range segments {
		switch segment {
		case "batches":
			if i+1 >= len(segments) || segments[i+1] == "" {
				return models.UpstreamObjectTypeBatch, "", true
			}
			return models.UpstreamObjectTypeBatch, segments[i+1], false
		case "files":
			if i+1 >= len(segments) || segments[i+1] == "" {
				return models.UpstreamObjectTypeFile, "", true
			}
			return models.UpstreamObjectTypeFile, segments[i+1], false
		}
	}
	return "", "", false
}

func proxyRelativePath(c *gin.Context, groupName string) string {
	if wildcard := c.Param("path"); wildcard != "" {
		if strings.HasPrefix(wildcard, "/") {
			return wildcard
		}
		return "/" + wildcard
	}
	prefix := "/proxy/" + groupName
	return strings.TrimPrefix(c.Request.URL.Path, prefix)
}

func objectRoutingFromContext(c *gin.Context) upstreamObjectRouting {
	value, _ := c.Get(upstreamObjectRoutingContextKey)
	routing, _ := value.(upstreamObjectRouting)
	return routing
}
