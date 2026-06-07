package sync

import (
	"net/http"

	"shop_keeper_backend/internal/api"
	"shop_keeper_backend/internal/i18n"
	"shop_keeper_backend/internal/middleware"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// Push handles POST /sync/push.
//
// Accessible to all authenticated users (owner + staff) since both roles can
// record sales and create customers while offline.
func (h *Handler) Push(c *gin.Context) {
	msgs := i18n.FromCtx(c)

	var input PushInput
	if err := c.ShouldBindJSON(&input); err != nil {
		api.InvalidJSON(c, msgs.InvalidJSON)
		return
	}

	// Empty batch is a valid no-op — return empty result immediately.
	if len(input.Records) == 0 {
		c.JSON(http.StatusOK, PushResult{
			Synced:    []string{},
			Conflicts: []string{},
			IDMap:     map[string]string{},
		})
		return
	}

	userID, ok := middleware.GetUserID(c)
	if !ok {
		api.Unauthorized(c, msgs.Unauthorized)
		return
	}

	result, err := h.service.Push(c.Request.Context(), userID, input)
	if err != nil {
		api.InternalError(c, msgs.InternalError)
		return
	}

	c.JSON(http.StatusOK, result)
}
