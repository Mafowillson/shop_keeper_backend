package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// abc -> only admin can access -> 2. level check -> auth ? -> admin requirement
func RequireOwner() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		role, ok := GetRole(ctx)
		if !ok {
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "Unauthorized",
			})
			return
		}

		if !strings.EqualFold(role, "owner") {
			ctx.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": "This route can only be accessed by owner",
			})
			return
		}
		ctx.Next()
	}
}
