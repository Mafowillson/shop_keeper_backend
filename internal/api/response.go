package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type ErrorResponse struct {
	Error   string      `json:"error"`
	Code    string      `json:"code,omitempty"`
	Details interface{} `json:"details,omitempty"`
}

type Pagination struct {
	Page       int   `json:"page"`
	PageSize   int   `json:"page_size"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"total_pages"`
}

func JSONError(c *gin.Context, status int, message string, code string, details interface{}) {
	c.JSON(status, ErrorResponse{Error: message, Code: code, Details: details})
}

func BadRequest(c *gin.Context, message string) {
	JSONError(c, http.StatusBadRequest, message, "bad_request", nil)
}

func Unauthorized(c *gin.Context, message string) {
	JSONError(c, http.StatusUnauthorized, message, "unauthorized", nil)
}

func NotFound(c *gin.Context, message string) {
	JSONError(c, http.StatusNotFound, message, "not_found", nil)
}

func InternalError(c *gin.Context, message string) {
	JSONError(c, http.StatusInternalServerError, message, "internal_error", nil)
}

func InvalidJSON(c *gin.Context) {
	JSONError(c, http.StatusBadRequest, "Invalid JSON body", "invalid_json", nil)
}
