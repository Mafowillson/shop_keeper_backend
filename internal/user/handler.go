package user

import (
	"net/http"

	"shop_keeper_backend/internal/i18n"
	"shop_keeper_backend/internal/middleware"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{service: svc}
}

func (h *Handler) Register(c *gin.Context) {
	msgs := i18n.FromCtx(c)

	var input RegisterInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": msgs.InvalidJSON})
		return
	}
	out, err := h.service.Register(c.Request.Context(), input)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, out)
}

func (h *Handler) Login(c *gin.Context) {
	msgs := i18n.FromCtx(c)

	var input LoginInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": msgs.InvalidJSON})
		return
	}
	out, err := h.service.Login(c.Request.Context(), input)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": msgs.InvalidCredentials})
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) Refresh(c *gin.Context) {
	msgs := i18n.FromCtx(c)

	var input RefreshInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": msgs.InvalidJSON})
		return
	}
	out, err := h.service.Refresh(c.Request.Context(), input)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": msgs.InvalidToken})
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) Logout(c *gin.Context) {
	msgs := i18n.FromCtx(c)

	var input LogoutInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": msgs.InvalidJSON})
		return
	}
	if err := h.service.Logout(c.Request.Context(), input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *Handler) VerifyEmail(c *gin.Context) {
	msgs := i18n.FromCtx(c)

	userID, ok := middleware.GetUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": msgs.Unauthorized})
		return
	}

	var input VerifyEmailInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": msgs.InvalidJSON})
		return
	}

	out, err := h.service.VerifyEmail(c.Request.Context(), userID, input)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, out)
}

func (h *Handler) ResendVerificationCode(c *gin.Context) {
	msgs := i18n.FromCtx(c)

	userID, ok := middleware.GetUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": msgs.Unauthorized})
		return
	}

	if err := h.service.ResendVerificationCode(c.Request.Context(), userID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": msgs.VerificationCodeSent})
}

// ForgotPassword always returns 200 to prevent email-enumeration attacks.
func (h *Handler) ForgotPassword(c *gin.Context) {
	msgs := i18n.FromCtx(c)

	var input ForgotPasswordInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": msgs.InvalidJSON})
		return
	}

	_ = h.service.ForgotPassword(c.Request.Context(), input)

	c.JSON(http.StatusOK, gin.H{"message": msgs.ForgotPasswordSent})
}

func (h *Handler) ResetPassword(c *gin.Context) {
	msgs := i18n.FromCtx(c)

	var input ResetPasswordInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": msgs.InvalidJSON})
		return
	}

	if err := h.service.ResetPassword(c.Request.Context(), input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": msgs.PasswordResetSuccess})
}
