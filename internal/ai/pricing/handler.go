package pricing

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

// GET /ai/price-recommendations?shop_id=xxx&status=pending&page=1&page_size=20
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

	status := strings.TrimSpace(c.Query("status"))

	page, pageSize := parsePage(c)

	recs, total, err := h.svc.List(c.Request.Context(), ownerID, shopID, status, page, pageSize)
	if err != nil {
		if err.Error() == "unauthorized" {
			c.JSON(http.StatusForbidden, gin.H{"error": msgs.Unauthorized})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": msgs.InternalError})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":      recs,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// POST /ai/price-recommendations/:id/accept
func (h *Handler) Accept(c *gin.Context) {
	msgs := i18n.FromCtx(c)

	ownerID, ok := middleware.GetUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": msgs.Unauthorized})
		return
	}

	recID := strings.TrimSpace(c.Param("id"))
	if recID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": msgs.BadRequest})
		return
	}

	if err := h.svc.Accept(c.Request.Context(), recID, ownerID); err != nil {
		switch err.Error() {
		case "recommendation not found":
			c.JSON(http.StatusNotFound, gin.H{"error": msgs.NotFound})
		case "unauthorized":
			c.JSON(http.StatusForbidden, gin.H{"error": msgs.Unauthorized})
		case "recommendation already acted upon":
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": msgs.InternalError})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": msgs.Updated})
}

// POST /ai/price-recommendations/:id/dismiss
func (h *Handler) Dismiss(c *gin.Context) {
	msgs := i18n.FromCtx(c)

	ownerID, ok := middleware.GetUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": msgs.Unauthorized})
		return
	}

	recID := strings.TrimSpace(c.Param("id"))
	if recID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": msgs.BadRequest})
		return
	}

	if err := h.svc.Dismiss(c.Request.Context(), recID, ownerID); err != nil {
		switch err.Error() {
		case "recommendation not found":
			c.JSON(http.StatusNotFound, gin.H{"error": msgs.NotFound})
		case "unauthorized":
			c.JSON(http.StatusForbidden, gin.H{"error": msgs.Unauthorized})
		case "recommendation already acted upon":
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": msgs.InternalError})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": msgs.Updated})
}

func parsePage(c *gin.Context) (page, pageSize int) {
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
