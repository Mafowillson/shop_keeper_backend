package middleware

import (
	"net/http"
	"shop_keeper_backend/internal/i18n"
	"strings"

	"github.com/gin-gonic/gin"
)

func RequireOwner() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		msgs := i18n.FromCtx(ctx)

		role, ok := GetRole(ctx)
		if !ok {
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": msgs.Unauthorized,
			})
			return
		}

		if !strings.EqualFold(role, "owner") {
			ctx.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": msgs.ForbiddenRole,
			})
			return
		}
		ctx.Next()
	}
}

func RequireStaff() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		msgs := i18n.FromCtx(ctx)

		role, ok := GetRole(ctx)
		if !ok {
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": msgs.Unauthorized,
			})
			return
		}

		if !strings.EqualFold(role, "staff") {
			ctx.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": msgs.ForbiddenRole,
			})
			return
		}
		ctx.Next()
	}
}
