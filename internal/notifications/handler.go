package notification

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// Handler holds the service and exposes HTTP endpoints.
// All handlers follow the same pattern your other packages use:
// extract claims → validate input → call service → return JSON.
type Handler struct {
	service *Service
}

// NewHandler constructs the handler.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// -----------------------------------------------------------------------
// Helper: extract the authenticated owner's ID from the Gin context.
//
// Your auth middleware (middleware/auth.go) sets "userID" and "role" into
// the Gin context after validating the JWT. We read "userID" here.
// -----------------------------------------------------------------------

func ownerIDFromCtx(c *gin.Context) (bson.ObjectID, bool) {
	raw, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return bson.NilObjectID, false
	}
	id, ok := raw.(bson.ObjectID)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid user id in context"})
		return bson.NilObjectID, false
	}
	return id, true
}

// -----------------------------------------------------------------------
// GET /api/v1/notifications
// FR-27: owner inbox with optional ?unread=true filter
// -----------------------------------------------------------------------

// GetInbox returns a paginated list of the owner's notifications.
//
// Query params:
//   - unread=true  : only return unread notifications
//   - limit        : how many to return (default 20, max 100)
//   - skip         : how many to skip for pagination (default 0)
//
// Response:
//
//	{
//	  "notifications": [...],
//	  "unread_count": 3,
//	  "limit": 20,
//	  "skip": 0
//	}
func (h *Handler) GetInbox(c *gin.Context) {
	ownerID, ok := ownerIDFromCtx(c)
	if !ok {
		return
	}

	// Parse optional ?unread=true
	unreadOnly := c.Query("unread") == "true"

	// Parse pagination. Use sensible defaults.
	var limit int64 = 20
	var skip int64 = 0
	// (You can use your existing pagination.go helper here if preferred.)

	notifications, unreadCount, err := h.service.GetInbox(c.Request.Context(), ownerID, unreadOnly, limit, skip)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"notifications": notifications,
		"unread_count":  unreadCount,
		"limit":         limit,
		"skip":          skip,
	})
}

// -----------------------------------------------------------------------
// PATCH /api/v1/notifications/:id/read
// FR-28: mark a single notification as read
// -----------------------------------------------------------------------

// MarkRead marks one notification as read.
// The :id path parameter is the MongoDB ObjectID hex string of the notification.
//
// Returns 204 No Content on success.
// Returns 404 if the notification doesn't exist or belongs to another owner.
func (h *Handler) MarkRead(c *gin.Context) {
	ownerID, ok := ownerIDFromCtx(c)
	if !ok {
		return
	}

	// Parse the notification ID from the URL path.
	notifID, err := bson.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid notification id"})
		return
	}

	if err := h.service.MarkRead(c.Request.Context(), notifID, ownerID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "notification not found or access denied"})
		return
	}

	// 204 No Content — success with no body, common for PATCH/DELETE.
	c.Status(http.StatusNoContent)
}

// -----------------------------------------------------------------------
// PATCH /api/v1/notifications/read-all
// Mark every notification as read
// -----------------------------------------------------------------------

// MarkAllRead marks all of the owner's notifications as read at once.
// Useful for a "clear all" button in Flutter.
func (h *Handler) MarkAllRead(c *gin.Context) {
	ownerID, ok := ownerIDFromCtx(c)
	if !ok {
		return
	}

	if err := h.service.MarkAllRead(c.Request.Context(), ownerID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}

// -----------------------------------------------------------------------
// GET /api/v1/notifications/preferences
// Return current preferences
// -----------------------------------------------------------------------

// GetPreferences returns the owner's current notification preferences.
func (h *Handler) GetPreferences(c *gin.Context) {
	ownerID, ok := ownerIDFromCtx(c)
	if !ok {
		return
	}

	prefs, err := h.service.GetPreferences(c.Request.Context(), ownerID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, prefs)
}

// -----------------------------------------------------------------------
// PUT /api/v1/notifications/preferences
// FR-26: update which notification types are active
// -----------------------------------------------------------------------

// UpdatePreferences lets the owner toggle notification types on/off
// and set their large-sale threshold.
//
// Request body (all fields optional — only sent fields are updated):
//
//	{
//	  "low_stock": true,
//	  "large_sale": false,
//	  "debt_payment": true,
//	  "staff_login": false,
//	  "large_sale_threshold": 75000
//	}
func (h *Handler) UpdatePreferences(c *gin.Context) {
	ownerID, ok := ownerIDFromCtx(c)
	if !ok {
		return
	}

	var req UpdatePreferencesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	prefs, err := h.service.UpdatePreferences(c.Request.Context(), ownerID, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, prefs)
}

// -----------------------------------------------------------------------
// POST /api/v1/owner/fcm-token
// Flutter calls this on every app start to keep the token fresh
// -----------------------------------------------------------------------

// SaveFCMToken stores the owner's FCM device token.
//
// Flutter's FirebaseMessaging.instance.getToken() returns a token that
// can change (e.g. after app reinstall). Flutter should call this endpoint
// on every cold start so the backend always has the latest token.
//
// Request body:
//
//	{ "token": "dGhpcyBpcyBhIHRva2Vu..." }
func (h *Handler) SaveFCMToken(c *gin.Context) {
	ownerID, ok := ownerIDFromCtx(c)
	if !ok {
		return
	}

	var body struct {
		Token string `json:"token" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "token is required"})
		return
	}

	if err := h.service.SaveFCMToken(c.Request.Context(), ownerID, body.Token); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "FCM token saved"})
}
