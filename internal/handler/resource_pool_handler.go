package handler

import (
	"fmt"
	"strconv"

	app_errors "api-load/internal/errors"
	"api-load/internal/response"
	"api-load/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

type ResourcePoolCreateRequest struct {
	Name                 string `json:"name" binding:"required"`
	Description          string `json:"description"`
	Strategy             string `json:"strategy"`
	AffinityTTLSeconds   int    `json:"affinity_ttl_seconds"`
	BusyWaitMilliseconds int    `json:"busy_wait_milliseconds"`
	AutoRestoreSchedule  string `json:"auto_restore_schedule"`
}

type ResourcePoolUpdateRequest struct {
	Name                 *string `json:"name,omitempty"`
	Description          *string `json:"description,omitempty"`
	AffinityTTLSeconds   *int    `json:"affinity_ttl_seconds,omitempty"`
	BusyWaitMilliseconds *int    `json:"busy_wait_milliseconds,omitempty"`
	AutoRestoreSchedule  *string `json:"auto_restore_schedule,omitempty"`
}

type ResourceStatusUpdateRequest struct {
	Status string `json:"status" binding:"required"`
}

type ResourceUpdateRequest struct {
	Name        string  `json:"name"`
	UpstreamURL string  `json:"upstream_url"`
	Key         *string `json:"key,omitempty"`
	Enabled     *bool   `json:"enabled,omitempty"`
	Status      *string `json:"status,omitempty"`
	Priority    *int    `json:"priority,omitempty"`
	Weight      *int    `json:"weight,omitempty"`
}

type BulkResourceStatusUpdateRequest struct {
	ResourceIDs []uint `json:"resource_ids" binding:"required"`
	Status      string `json:"status" binding:"required"`
}

type BulkResourceDeleteRequest struct {
	ResourceIDs []uint   `json:"resource_ids"`
	Keys        []string `json:"keys"`
}

type BulkResourceUpdateRequest struct {
	ResourceIDs []uint  `json:"resource_ids" binding:"required"`
	Enabled     *bool   `json:"enabled,omitempty"`
	Status      *string `json:"status,omitempty"`
	Priority    *int    `json:"priority,omitempty"`
	Weight      *int    `json:"weight,omitempty"`
}

type ResourceImportRequest struct {
	Content string `json:"content" binding:"required"`
}

type ResourceTestRequest struct {
	GroupID uint `json:"group_id" binding:"required"`
}

type ResourceEndpointCreateRequest struct {
	Name        string `json:"name" binding:"required"`
	ChannelType string `json:"channel_type" binding:"required"`
	BaseURL     string `json:"base_url" binding:"required"`
	Enabled     *bool  `json:"enabled,omitempty"`
}

type ResourceEndpointUpdateRequest struct {
	Name        *string `json:"name,omitempty"`
	ChannelType *string `json:"channel_type,omitempty"`
	BaseURL     *string `json:"base_url,omitempty"`
	Enabled     *bool   `json:"enabled,omitempty"`
}

func (s *Server) handleResourcePoolError(c *gin.Context, err error) bool {
	if err == nil {
		return false
	}
	if apiErr, ok := err.(*app_errors.APIError); ok {
		response.Error(c, apiErr)
		return true
	}
	logrus.WithContext(c.Request.Context()).WithError(err).Error("unexpected resource pool service error")
	response.Error(c, app_errors.ErrInternalServer)
	return true
}

func (s *Server) CreateResourcePool(c *gin.Context) {
	var req ResourcePoolCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrInvalidJSON, err.Error()))
		return
	}
	pool, err := s.ResourcePoolService.CreatePool(c.Request.Context(), services.ResourcePoolCreateParams{
		Name:                 req.Name,
		Description:          req.Description,
		Strategy:             req.Strategy,
		AffinityTTLSeconds:   req.AffinityTTLSeconds,
		BusyWaitMilliseconds: req.BusyWaitMilliseconds,
		AutoRestoreSchedule:  req.AutoRestoreSchedule,
	})
	if s.handleResourcePoolError(c, err) {
		return
	}
	response.Success(c, pool)
}

func (s *Server) ListResourcePools(c *gin.Context) {
	pools, err := s.ResourcePoolService.ListPools(c.Request.Context())
	if s.handleResourcePoolError(c, err) {
		return
	}
	response.Success(c, pools)
}

func (s *Server) GetResourcePool(c *gin.Context) {
	id, ok := parseResourceID(c, "id")
	if !ok {
		return
	}
	pool, err := s.ResourcePoolService.GetPool(c.Request.Context(), id)
	if s.handleResourcePoolError(c, err) {
		return
	}
	response.Success(c, pool)
}

func (s *Server) UpdateResourcePool(c *gin.Context) {
	id, ok := parseResourceID(c, "id")
	if !ok {
		return
	}
	var req ResourcePoolUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrInvalidJSON, err.Error()))
		return
	}
	pool, err := s.ResourcePoolService.UpdatePool(c.Request.Context(), id, services.ResourcePoolUpdateParams{
		Name:                 req.Name,
		Description:          req.Description,
		AffinityTTLSeconds:   req.AffinityTTLSeconds,
		BusyWaitMilliseconds: req.BusyWaitMilliseconds,
		AutoRestoreSchedule:  req.AutoRestoreSchedule,
	})
	if s.handleResourcePoolError(c, err) {
		return
	}
	response.Success(c, pool)
}

func (s *Server) DeleteResourcePool(c *gin.Context) {
	id, ok := parseResourceID(c, "id")
	if !ok {
		return
	}
	if s.handleResourcePoolError(c, s.ResourcePoolService.DeletePool(c.Request.Context(), id)) {
		return
	}
	response.Success(c, nil)
}

func (s *Server) AddResourcePoolResources(c *gin.Context) {
	id, ok := parseResourceID(c, "id")
	if !ok {
		return
	}
	var req []services.ResourceCreateParams
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrInvalidJSON, err.Error()))
		return
	}
	resources, err := s.ResourcePoolService.AddResources(c.Request.Context(), id, req)
	if s.handleResourcePoolError(c, err) {
		return
	}
	response.Success(c, resources)
}

func (s *Server) CreateResourcePoolEndpoint(c *gin.Context) {
	poolID, ok := parseResourceID(c, "id")
	if !ok {
		return
	}
	var req ResourceEndpointCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrInvalidJSON, err.Error()))
		return
	}
	endpoint, err := s.ResourcePoolService.CreateEndpoint(c.Request.Context(), poolID, services.ResourceEndpointCreateParams{
		Name: req.Name, ChannelType: req.ChannelType, BaseURL: req.BaseURL, Enabled: req.Enabled,
	})
	if s.handleResourcePoolError(c, err) {
		return
	}
	response.Success(c, endpoint)
}

func (s *Server) ListResourcePoolEndpoints(c *gin.Context) {
	poolID, ok := parseResourceID(c, "id")
	if !ok {
		return
	}
	endpoints, err := s.ResourcePoolService.ListEndpoints(c.Request.Context(), poolID)
	if s.handleResourcePoolError(c, err) {
		return
	}
	response.Success(c, endpoints)
}

func (s *Server) UpdateResourcePoolEndpoint(c *gin.Context) {
	poolID, ok := parseResourceID(c, "id")
	if !ok {
		return
	}
	endpointID, ok := parseResourceID(c, "endpointId")
	if !ok {
		return
	}
	var req ResourceEndpointUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrInvalidJSON, err.Error()))
		return
	}
	endpoint, err := s.ResourcePoolService.UpdateEndpoint(c.Request.Context(), poolID, endpointID, services.ResourceEndpointUpdateParams{
		Name: req.Name, ChannelType: req.ChannelType, BaseURL: req.BaseURL, Enabled: req.Enabled,
	})
	if s.handleResourcePoolError(c, err) {
		return
	}
	response.Success(c, endpoint)
}

func (s *Server) DeleteResourcePoolEndpoint(c *gin.Context) {
	poolID, ok := parseResourceID(c, "id")
	if !ok {
		return
	}
	endpointID, ok := parseResourceID(c, "endpointId")
	if !ok {
		return
	}
	if s.handleResourcePoolError(c, s.ResourcePoolService.DeleteEndpoint(c.Request.Context(), poolID, endpointID)) {
		return
	}
	response.Success(c, nil)
}

func (s *Server) ImportResourcePoolResources(c *gin.Context) {
	poolID, ok := parseResourceID(c, "id")
	if !ok {
		return
	}
	var req ResourceImportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrInvalidJSON, err.Error()))
		return
	}
	items, err := services.ParseResourceImportInput(req.Content)
	if err != nil {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrValidation, err.Error()))
		return
	}
	resources, err := s.ResourcePoolService.AddResources(c.Request.Context(), poolID, items)
	if s.handleResourcePoolError(c, err) {
		return
	}
	response.Success(c, resources)
}

func (s *Server) ListResourcePoolResources(c *gin.Context) {
	id, ok := parseResourceID(c, "id")
	if !ok {
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	var enabled *bool
	if raw := c.Query("enabled"); raw != "" {
		parsed, err := strconv.ParseBool(raw)
		if err != nil {
			response.Error(c, app_errors.NewAPIError(app_errors.ErrValidation, "enabled must be true or false"))
			return
		}
		enabled = &parsed
	}
	resources, err := s.ResourcePoolService.ListResources(c.Request.Context(), id, services.ResourceListParams{
		Page:     page,
		PageSize: pageSize,
		Search:   c.Query("search"),
		Status:   c.Query("status"),
		Enabled:  enabled,
	})
	if s.handleResourcePoolError(c, err) {
		return
	}
	response.Success(c, resources)
}

func (s *Server) ExportResourcePoolResources(c *gin.Context) {
	poolID, ok := parseResourceID(c, "id")
	if !ok {
		return
	}
	content := c.DefaultQuery("content", "full")
	format := c.Query("format")
	if format == "" {
		if content == "keys" {
			format = "txt"
		} else {
			format = "jsonl"
		}
	}
	var enabled *bool
	if raw := c.Query("enabled"); raw != "" {
		parsed, err := strconv.ParseBool(raw)
		if err != nil {
			response.Error(c, app_errors.NewAPIError(app_errors.ErrValidation, "enabled must be true or false"))
			return
		}
		enabled = &parsed
	}
	status := c.DefaultQuery("status", "all")
	suffix := content
	if status != "" && status != "all" {
		suffix += "-" + status
	}
	filename := fmt.Sprintf("resource-pool-%d-%s.%s", poolID, suffix, format)
	c.Header("Content-Disposition", "attachment; filename="+filename)
	switch format {
	case "txt":
		c.Header("Content-Type", "text/plain; charset=utf-8")
	case "csv":
		c.Header("Content-Type", "text/csv; charset=utf-8")
	default:
		c.Header("Content-Type", "application/x-ndjson; charset=utf-8")
	}
	if _, err := s.ResourcePoolService.ExportResourcesToWriter(c.Request.Context(), poolID, status, enabled, content, format, c.Writer); err != nil {
		logrus.WithContext(c.Request.Context()).WithError(err).Error("failed to export resource pool")
	}
}

func (s *Server) ListResourcePoolValidationGroups(c *gin.Context) {
	poolID, ok := parseResourceID(c, "id")
	if !ok {
		return
	}
	groups, err := s.ResourceValidationService.ListValidationGroups(c.Request.Context(), poolID)
	if s.handleResourcePoolError(c, err) {
		return
	}
	response.Success(c, groups)
}

func (s *Server) TestResourcePoolResource(c *gin.Context) {
	poolID, ok := parseResourceID(c, "id")
	if !ok {
		return
	}
	resourceID, ok := parseResourceID(c, "resourceId")
	if !ok {
		return
	}
	var req ResourceTestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrInvalidJSON, err.Error()))
		return
	}
	result, err := s.ResourceValidationService.TestResource(c.Request.Context(), poolID, resourceID, req.GroupID)
	if s.handleResourcePoolError(c, err) {
		return
	}
	response.Success(c, result)
}

func (s *Server) UpdateResourcePoolResource(c *gin.Context) {
	poolID, ok := parseResourceID(c, "id")
	if !ok {
		return
	}
	resourceID, ok := parseResourceID(c, "resourceId")
	if !ok {
		return
	}
	var req ResourceUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrInvalidJSON, err.Error()))
		return
	}
	resource, err := s.ResourcePoolService.UpdateResource(c.Request.Context(), poolID, resourceID, services.ResourceUpdateParams{
		Name:        req.Name,
		UpstreamURL: req.UpstreamURL,
		Key:         req.Key,
		Enabled:     req.Enabled,
		Status:      req.Status,
		Priority:    req.Priority,
		Weight:      req.Weight,
	})
	if s.handleResourcePoolError(c, err) {
		return
	}
	response.Success(c, resource)
}

func (s *Server) BulkUpdateResourcePoolResources(c *gin.Context) {
	poolID, ok := parseResourceID(c, "id")
	if !ok {
		return
	}
	var req BulkResourceUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrInvalidJSON, err.Error()))
		return
	}
	result, err := s.ResourcePoolService.BulkUpdateResources(c.Request.Context(), poolID, req.ResourceIDs, services.ResourceBatchUpdateParams{
		Enabled: req.Enabled, Status: req.Status, Priority: req.Priority, Weight: req.Weight,
	})
	if s.handleResourcePoolError(c, err) {
		return
	}
	response.Success(c, result)
}

func (s *Server) BulkUpdateResourcePoolResourceStatus(c *gin.Context) {
	poolID, ok := parseResourceID(c, "id")
	if !ok {
		return
	}
	var req BulkResourceStatusUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrInvalidJSON, err.Error()))
		return
	}
	result, err := s.ResourcePoolService.BulkUpdateResourceStatus(c.Request.Context(), poolID, req.ResourceIDs, req.Status)
	if s.handleResourcePoolError(c, err) {
		return
	}
	response.Success(c, result)
}

func (s *Server) BulkDeleteResourcePoolResources(c *gin.Context) {
	poolID, ok := parseResourceID(c, "id")
	if !ok {
		return
	}
	var req BulkResourceDeleteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrInvalidJSON, err.Error()))
		return
	}
	result, err := s.ResourcePoolService.BulkDeleteResources(c.Request.Context(), poolID, req.ResourceIDs, req.Keys)
	if s.handleResourcePoolError(c, err) {
		return
	}
	response.Success(c, result)
}

func (s *Server) UpdateResourcePoolResourceStatus(c *gin.Context) {
	poolID, ok := parseResourceID(c, "id")
	if !ok {
		return
	}
	resourceID, ok := parseResourceID(c, "resourceId")
	if !ok {
		return
	}
	var req ResourceStatusUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrInvalidJSON, err.Error()))
		return
	}
	resource, err := s.ResourcePoolService.UpdateResourceStatus(c.Request.Context(), poolID, resourceID, req.Status)
	if s.handleResourcePoolError(c, err) {
		return
	}
	response.Success(c, resource)
}

func (s *Server) DeleteResourcePoolResource(c *gin.Context) {
	poolID, ok := parseResourceID(c, "id")
	if !ok {
		return
	}
	resourceID, ok := parseResourceID(c, "resourceId")
	if !ok {
		return
	}
	if s.handleResourcePoolError(c, s.ResourcePoolService.DeleteResource(c.Request.Context(), poolID, resourceID)) {
		return
	}
	response.Success(c, nil)
}

func parseResourceID(c *gin.Context, name string) (uint, bool) {
	value, err := strconv.ParseUint(c.Param(name), 10, 32)
	if err != nil || value == 0 {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrBadRequest, "invalid resource identifier"))
		return 0, false
	}
	return uint(value), true
}
