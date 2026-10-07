package control

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"image/png"
	"net/url"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	app_errors "gpt-load/internal/platform/errors"
	"gpt-load/internal/platform/response"
	"gpt-load/internal/storage/models"
)

const groupPresentationSetting = models.InternalSystemSettingPrefix + "ui.group_presentation"

type GroupPresentation struct {
	Order []uint          `json:"order"`
	Icons map[uint]string `json:"icons"`
}

type GroupPresentationUpdate struct {
	Order   *[]uint `json:"order"`
	GroupID *uint   `json:"group_id"`
	Icon    *string `json:"icon"`
}

func loadGroupPresentation(tx *gorm.DB) (GroupPresentation, error) {
	result := GroupPresentation{Order: []uint{}, Icons: map[uint]string{}}
	var row models.SystemSetting
	err := tx.Where("key = ?", groupPresentationSetting).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return result, nil
	}
	if err != nil {
		return result, app_errors.ParseDBError(err)
	}
	if err := json.Unmarshal([]byte(row.Value), &result); err != nil {
		return result, app_errors.ErrInternalServer
	}
	if result.Order == nil {
		result.Order = []uint{}
	}
	if result.Icons == nil {
		result.Icons = map[uint]string{}
	}
	return result, nil
}

func (s *Service) GetGroupPresentation(ctx context.Context) (GroupPresentation, error) {
	return loadGroupPresentation(s.db.WithContext(ctx))
}

var groupIconName = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)

func validGroupIcon(value string) bool {
	if value == "" {
		return true
	}
	for _, prefix := range []string{"builtin:", "lobehub:"} {
		if strings.HasPrefix(value, prefix) {
			return groupIconName.MatchString(strings.TrimPrefix(value, prefix))
		}
	}
	if strings.HasPrefix(value, "data:image/png;base64,") {
		if len(value) > 350000 {
			return false
		}
		data, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(value, "data:image/png;base64,"))
		if err != nil {
			return false
		}
		config, err := png.DecodeConfig(bytes.NewReader(data))
		return err == nil && config.Width > 0 && config.Height > 0 && config.Width <= 512 && config.Height <= 512
	}
	if len(value) > 2048 {
		return false
	}
	parsed, err := url.Parse(value)
	return err == nil && parsed.Scheme == "https" && parsed.Hostname() != "" && parsed.User == nil
}

func (s *Service) UpdateGroupPresentation(ctx context.Context, request GroupPresentationUpdate) (GroupPresentation, error) {
	if request.Order == nil && request.GroupID == nil {
		return GroupPresentation{}, app_errors.ErrBadRequest
	}
	if (request.GroupID == nil) != (request.Icon == nil) {
		return GroupPresentation{}, app_errors.ErrBadRequest
	}
	if request.Icon != nil && !validGroupIcon(*request.Icon) {
		return GroupPresentation{}, app_errors.ErrValidation
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	var result GroupPresentation
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		current, err := loadGroupPresentation(tx)
		if err != nil {
			return err
		}
		var ids []uint
		if err := tx.Model(&models.Group{}).Pluck("id", &ids).Error; err != nil {
			return app_errors.ParseDBError(err)
		}
		known := make(map[uint]bool, len(ids))
		for _, id := range ids {
			known[id] = true
		}
		if request.Order != nil {
			seen := map[uint]bool{}
			for _, id := range *request.Order {
				if !known[id] || seen[id] {
					return app_errors.ErrValidation
				}
				seen[id] = true
			}
			current.Order = append([]uint{}, (*request.Order)...)
		}
		if request.GroupID != nil {
			if !known[*request.GroupID] {
				return app_errors.ErrResourceNotFound
			}
			if *request.Icon == "" {
				delete(current.Icons, *request.GroupID)
			} else {
				current.Icons[*request.GroupID] = *request.Icon
			}
		}
		cleaned := make([]uint, 0, len(current.Order))
		for _, id := range current.Order {
			if known[id] {
				cleaned = append(cleaned, id)
			}
		}
		current.Order = cleaned
		for id := range current.Icons {
			if !known[id] {
				delete(current.Icons, id)
			}
		}
		encoded, err := json.Marshal(current)
		if err != nil {
			return app_errors.ErrInternalServer
		}
		if len(encoded) > 4000000 {
			return app_errors.ErrValidation
		}
		row := models.SystemSetting{Key: groupPresentationSetting, Value: string(encoded)}
		if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "key"}}, DoUpdates: clause.AssignmentColumns([]string{"value", "updated_at_ms"})}).Create(&row).Error; err != nil {
			return app_errors.ParseDBError(err)
		}
		result = current
		return nil
	})
	return result, err
}

func (s *Server) handleGetGroupPresentation(c *gin.Context) {
	result, err := s.service.GetGroupPresentation(c.Request.Context())
	if err != nil {
		writeServiceError(c, "get_group_presentation", err)
		return
	}
	response.SuccessI18n(c, "common.success", result)
}

func (s *Server) handleUpdateGroupPresentation(c *gin.Context) {
	var request GroupPresentationUpdate
	if err := c.ShouldBindJSON(&request); err != nil {
		writeServiceError(c, "update_group_presentation", app_errors.ErrBadRequest)
		return
	}
	result, err := s.service.UpdateGroupPresentation(c.Request.Context(), request)
	if err != nil {
		writeServiceError(c, "update_group_presentation", err)
		return
	}
	response.SuccessI18n(c, "common.success", result)
}
