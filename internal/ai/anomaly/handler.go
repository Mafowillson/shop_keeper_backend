package anomaly

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"shop_keeper_backend/internal/i18n"
	"shop_keeper_backend/internal/middleware"

	"go.mongodb.org/mongo-driver/v2/mongo"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	repo *Repo
	svc  *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{repo: svc.repo, svc: svc}
}

// GET /ai/fraud-alerts?shop_id=xxx&status=open&page=1&page_size=20
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

	if err := h.svc.verifyOwner(c.Request.Context(), shopID, ownerID); err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": msgs.Unauthorized})
		return
	}

	status := strings.TrimSpace(c.Query("status"))
	page, pageSize := parseAlertPage(c)

	alerts, total, err := h.repo.ListByShop(c.Request.Context(), shopID, status, page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": msgs.InternalError})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":      alerts,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// PATCH /ai/fraud-alerts/:id/acknowledge
func (h *Handler) Acknowledge(c *gin.Context) {
	msgs := i18n.FromCtx(c)

	ownerID, ok := middleware.GetUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": msgs.Unauthorized})
		return
	}

	alertID := strings.TrimSpace(c.Param("id"))
	if alertID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": msgs.BadRequest})
		return
	}

	alert, err := h.repo.FindByID(c.Request.Context(), alertID)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusNotFound, gin.H{"error": msgs.NotFound})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": msgs.InternalError})
		return
	}

	if err := h.svc.verifyOwner(c.Request.Context(), alert.ShopID, ownerID); err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": msgs.Unauthorized})
		return
	}

	if err := h.repo.Acknowledge(c.Request.Context(), alertID, time.Now().UTC()); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": msgs.InternalError})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": msgs.Updated})
}

func parseAlertPage(c *gin.Context) (page, pageSize int) {
	page, _ = strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ = strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	return
}
