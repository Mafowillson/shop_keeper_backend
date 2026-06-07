package chat

import (
	"net/http"

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

// POST /api/v1/chat/message
func (h *Handler) Send(c *gin.Context) {
	l10n := i18n.FromCtx(c)

	ownerID, ok := middleware.GetUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": l10n.Unauthorized})
		return
	}

	var req SendRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": l10n.InvalidJSON})
		return
	}

	saved, err := h.svc.Send(c.Request.Context(), ownerID, req.Message)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": l10n.InternalError})
		return
	}

	c.JSON(http.StatusOK, SendResponse{
		Reply: saved.Text,
		Message: MessageResponse{
			ID:        saved.ID,
			Role:      saved.Role,
			Text:      saved.Text,
			CreatedAt: saved.CreatedAt,
		},
	})
}

// GET /api/v1/chat/history
func (h *Handler) GetHistory(c *gin.Context) {
	l10n := i18n.FromCtx(c)

	ownerID, ok := middleware.GetUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": l10n.Unauthorized})
		return
	}

	history, err := h.svc.GetHistory(c.Request.Context(), ownerID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": l10n.InternalError})
		return
	}

	resp := HistoryResponse{Messages: make([]MessageResponse, len(history))}
	for i, m := range history {
		resp.Messages[i] = MessageResponse{
			ID:        m.ID,
			Role:      m.Role,
			Text:      m.Text,
			CreatedAt: m.CreatedAt,
		}
	}

	c.JSON(http.StatusOK, resp)
}

// DELETE /api/v1/chat/history
func (h *Handler) Clear(c *gin.Context) {
	l10n := i18n.FromCtx(c)

	ownerID, ok := middleware.GetUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": l10n.Unauthorized})
		return
	}

	if err := h.svc.Clear(c.Request.Context(), ownerID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": l10n.InternalError})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": l10n.ChatHistoryCleared})
}
