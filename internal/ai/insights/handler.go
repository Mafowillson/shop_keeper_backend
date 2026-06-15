package insights

import (
	"net/http"
	"strconv"
	"strings"

	"shop_keeper_backend/internal/i18n"
	"shop_keeper_backend/internal/middleware"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// GET /ai/weekly-insights?shop_id=xxx&limit=4
func (h *Handler) List(c *gin.Context) {
	msgs := i18n.FromCtx(c)

	ownerID, ok := middleware.GetUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": msgs.Unauthorized})
		return
	}

	shopID := strings.TrimSpace(c.Query("shop_id"))
	if shopID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": msgs.BadRequest})
		return
	}

	limit := 4
	if l, err := strconv.Atoi(c.Query("limit")); err == nil && l > 0 && l <= 12 {
		limit = l
	}

	insights, err := h.svc.ListByShop(c.Request.Context(), shopID, ownerID, limit)
	if err != nil {
		switch err.Error() {
		case "unauthorized":
			c.JSON(http.StatusForbidden, gin.H{"error": msgs.Unauthorized})
		case "shop not found":
			c.JSON(http.StatusNotFound, gin.H{"error": msgs.NotFound})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": msgs.InternalError})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": insights})
}
