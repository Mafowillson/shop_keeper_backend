package notification

import (
	"net/http"

	"shop_keeper_backend/internal/i18n"
	"shop_keeper_backend/internal/middleware"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/v2/bson"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func ownerIDFromCtx(c *gin.Context) (bson.ObjectID, bool) {
	msgs := i18n.FromCtx(c)
	userIDStr, ok := middleware.GetUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": msgs.Unauthorized})
		return bson.NilObjectID, false
	}
	id, err := bson.ObjectIDFromHex(userIDStr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": msgs.InvalidID})
		return bson.NilObjectID, false
	}
	return id, true
}

func staffIDFromCtx(c *gin.Context) (string, bool) {
	msgs := i18n.FromCtx(c)
	id, ok := middleware.GetUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": msgs.Unauthorized})
	}
	return id, ok
}

func (h *Handler) GetInbox(c *gin.Context) {
	msgs := i18n.FromCtx(c)

	ownerID, ok := ownerIDFromCtx(c)
	if !ok {
		return
	}

	unreadOnly := c.Query("unread") == "true"

	var limit int64 = 20
	var skip int64 = 0

	notifications, unreadCount, err := h.service.GetInbox(c.Request.Context(), ownerID, unreadOnly, limit, skip)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": msgs.InternalError})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"notifications": notifications,
		"unread_count":  unreadCount,
		"limit":         limit,
		"skip":          skip,
	})
}

func (h *Handler) MarkRead(c *gin.Context) {
	msgs := i18n.FromCtx(c)

	ownerID, ok := ownerIDFromCtx(c)
	if !ok {
		return
	}

	notifID, err := bson.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": msgs.InvalidID})
		return
	}

	if err := h.service.MarkRead(c.Request.Context(), notifID, ownerID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": msgs.NotificationNotFound})
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *Handler) MarkAllRead(c *gin.Context) {
	msgs := i18n.FromCtx(c)

	ownerID, ok := ownerIDFromCtx(c)
	if !ok {
		return
	}

	if err := h.service.MarkAllRead(c.Request.Context(), ownerID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": msgs.InternalError})
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *Handler) GetPreferences(c *gin.Context) {
	msgs := i18n.FromCtx(c)

	ownerID, ok := ownerIDFromCtx(c)
	if !ok {
		return
	}

	prefs, err := h.service.GetPreferences(c.Request.Context(), ownerID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": msgs.InternalError})
		return
	}

	c.JSON(http.StatusOK, prefs)
}

func (h *Handler) UpdatePreferences(c *gin.Context) {
	msgs := i18n.FromCtx(c)

	ownerID, ok := ownerIDFromCtx(c)
	if !ok {
		return
	}

	var req UpdatePreferencesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": msgs.InvalidJSON})
		return
	}

	prefs, err := h.service.UpdatePreferences(c.Request.Context(), ownerID, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": msgs.InternalError})
		return
	}

	c.JSON(http.StatusOK, prefs)
}

func (h *Handler) GetStaffInbox(c *gin.Context) {
	msgs := i18n.FromCtx(c)

	staffID, ok := staffIDFromCtx(c)
	if !ok {
		return
	}

	notifications, unreadCount, err := h.service.GetStaffInbox(c.Request.Context(), staffID, 20, 0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": msgs.InternalError})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"notifications": notifications,
		"unread_count":  unreadCount,
	})
}

func (h *Handler) MarkStaffRead(c *gin.Context) {
	msgs := i18n.FromCtx(c)

	staffID, ok := staffIDFromCtx(c)
	if !ok {
		return
	}

	notifID, err := bson.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": msgs.InvalidID})
		return
	}

	if err := h.service.MarkStaffNotifRead(c.Request.Context(), notifID, staffID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": msgs.NotificationNotFound})
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *Handler) MarkStaffAllRead(c *gin.Context) {
	msgs := i18n.FromCtx(c)

	staffID, ok := staffIDFromCtx(c)
	if !ok {
		return
	}

	if err := h.service.MarkAllStaffNotifsRead(c.Request.Context(), staffID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": msgs.InternalError})
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *Handler) SaveFCMToken(c *gin.Context) {
	msgs := i18n.FromCtx(c)

	ownerID, ok := ownerIDFromCtx(c)
	if !ok {
		return
	}

	var body struct {
		Token string `json:"token" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": msgs.MissingRequiredField})
		return
	}

	if err := h.service.SaveFCMToken(c.Request.Context(), ownerID, body.Token); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": msgs.InternalError})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": msgs.FCMTokenSaved})
}
