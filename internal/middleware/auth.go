package middleware

import (
	"net/http"
	"shop_keeper_backend/internal/auth"
	"strings"

	"github.com/gin-gonic/gin"
)

const (
	ctxUserIDkey = "auth.userId"
	ctxRoleKey   = "auth.role"
)

func AuthRequired(jwtSecret string) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		authHeader := strings.TrimSpace(ctx.GetHeader("Authorization"))
		if authHeader == "" {
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "Missing authorization token",
			})
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 {
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid authorization token format",
			})
			return
		}

		scheme := strings.TrimSpace(parts[0])
		tokenString := strings.TrimSpace(parts[1])

		if !strings.EqualFold(scheme, "Bearer") {
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "Authorization scheme must be Bearer",
			})
			return
		}

		if tokenString == "" {
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "Authorization scheme must be bearer",
			})
			return
		}

		claims, err := auth.ParseToken(jwtSecret, tokenString)
		if err != nil {
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid token or expired",
			})
			return
		}

		ctx.Set(ctxUserIDkey, claims.Subject)
		ctx.Set(ctxRoleKey, claims.Role)

		ctx.Next()
	}
}

func GetUserID(ctx *gin.Context) (string, bool) {
	res, ok := ctx.Get(ctxUserIDkey)
	if !ok {
		return "", false
	}

	userID, ok := res.(string)
	return userID, ok
}

func GetRole(ctx *gin.Context) (string, bool) {
	res, ok := ctx.Get(ctxRoleKey)
	if !ok {
		return "", false
	}

	role, ok := res.(string)
	return role, ok
}
