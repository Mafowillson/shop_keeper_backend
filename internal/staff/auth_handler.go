package staff

import (
	"net/http"

	"shop_keeper_backend/internal/i18n"

	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	authService *AuthService
}

func NewAuthHandler(authService *AuthService) *AuthHandler {
	return &AuthHandler{authService: authService}
}

func (h *AuthHandler) Refresh(c *gin.Context) {
	msgs := i18n.FromCtx(c)

	var body struct {
		RefreshToken string `json:"refresh_token" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": msgs.MissingRequiredField})
		return
	}

	result, err := h.authService.RefreshToken(c.Request.Context(), body.RefreshToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": msgs.InvalidToken})
		return
	}

	c.JSON(http.StatusOK, result)
}

func (h *AuthHandler) Login(c *gin.Context) {
	msgs := i18n.FromCtx(c)

	var input StaffLoginInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": msgs.InvalidJSON})
		return
	}

	result, err := h.authService.Login(c.Request.Context(), input)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": msgs.InvalidStaffCredentials})
		return
	}

	c.JSON(http.StatusOK, result)
}
