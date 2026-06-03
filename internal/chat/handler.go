package chat

import (
	"net/http"

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
	ownerID, ok := middleware.GetUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req SendRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	saved, err := h.svc.Send(c.Request.Context(), ownerID, req.Message)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
	ownerID, ok := middleware.GetUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	msgs, err := h.svc.GetHistory(c.Request.Context(), ownerID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	resp := HistoryResponse{Messages: make([]MessageResponse, len(msgs))}
	for i, m := range msgs {
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
	ownerID, ok := middleware.GetUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	if err := h.svc.Clear(c.Request.Context(), ownerID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "chat history cleared"})
}
