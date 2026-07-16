package handler

import (
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
}

type ResourcePoolUpdateRequest struct {
	Name                 *string `json:"name,omitempty"`
	Description          *string `json:"description,omitempty"`
	AffinityTTLSeconds   *int    `json:"affinity_ttl_seconds,omitempty"`
	BusyWaitMilliseconds *int    `json:"busy_wait_milliseconds,omitempty"`
}

type ResourceStatusUpdateRequest struct {
	Status string `json:"status" binding:"required"`
}

type ResourceUpdateRequest struct {
	Name        string  `json:"name"`
	UpstreamURL string  `json:"upstream_url" binding:"required"`
	Key         *string `json:"key,omitempty"`
}

type BulkResourceStatusUpdateRequest struct {
	ResourceIDs []uint `json:"resource_ids" binding:"required"`
	Status      string `json:"status" binding:"required"`
}

type BulkResourceDeleteRequest struct {
	ResourceIDs []uint   `json:"resource_ids"`
	Keys        []string `json:"keys"`
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

func (s *Server) ListResourcePoolResources(c *gin.Context) {
	id, ok := parseResourceID(c, "id")
	if !ok {
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	resources, err := s.ResourcePoolService.ListResources(c.Request.Context(), id, services.ResourceListParams{
		Page:     page,
		PageSize: pageSize,
		Search:   c.Query("search"),
		Status:   c.Query("status"),
	})
	if s.handleResourcePoolError(c, err) {
		return
	}
	response.Success(c, resources)
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
	})
	if s.handleResourcePoolError(c, err) {
		return
	}
	response.Success(c, resource)
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
