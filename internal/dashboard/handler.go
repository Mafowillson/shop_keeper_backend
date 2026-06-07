package dashboard

import (
	"net/http"

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

// GET /api/v1/dashboard  (owner-only)
func (h *Handler) GetOwnerDashboard(c *gin.Context) {
	msgs := i18n.FromCtx(c)
	locale := i18n.LocaleFromCtx(c)

	ownerID, ok := middleware.GetUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": msgs.Unauthorized})
		return
	}

	stats, err := h.service.GetOwnerStats(c.Request.Context(), ownerID, locale)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": msgs.InternalError})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// GET /api/v1/staff/dashboard  (any authenticated user — staff use their own ID)
func (h *Handler) GetStaffDashboard(c *gin.Context) {
	msgs := i18n.FromCtx(c)

	userID, ok := middleware.GetUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": msgs.Unauthorized})
		return
	}

	stats, err := h.service.GetStaffStats(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": msgs.InternalError})
		return
	}

	c.JSON(http.StatusOK, stats)
}
