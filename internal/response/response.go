// Package response provides standardized JSON response helpers.
package response

import (
	app_errors "gpt-load/internal/errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

// SuccessResponse defines the standard JSON success response structure.
type SuccessResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// ErrorResponse defines the standard JSON error response structure.
type ErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Success sends a standardized success response.
func Success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, SuccessResponse{
		Code:    0,
		Message: "Success",
		Data:    data,
	})
}

// Error sends a standardized error response using an APIError.
func Error(c *gin.Context, apiErr *app_errors.APIError) {
	c.JSON(apiErr.HTTPStatus, ErrorResponse{
		Code:    apiErr.Code,
		Message: apiErr.Message,
	})
}
