package i18n

import (
	"context"
	"strings"

	"github.com/gin-gonic/gin"
)

const localeKey = "locale"

// LocaleSaver is implemented by any repo that can persist a user's locale.
// Using an interface avoids a circular import with the user package.
type LocaleSaver interface {
	SaveLocale(ctx context.Context, userID, locale string) error
}

// Middleware reads the Accept-Language header, resolves it to "fr" or "en"
// (defaulting to "fr"), stores the result in the Gin context under "locale",
// and — after the downstream handler runs — asynchronously persists the locale
// to the user document for any authenticated request.
func Middleware(saver LocaleSaver) gin.HandlerFunc {
	return func(c *gin.Context) {
		locale := normalizeLocale(c.GetHeader("Accept-Language"))
		c.Set(localeKey, locale)

		c.Next()

		// Persist locale for authenticated users (fire-and-forget).
		if saver != nil {
			if val, exists := c.Get("auth.userId"); exists {
				if userID, ok := val.(string); ok && userID != "" {
					go func() {
						_ = saver.SaveLocale(context.Background(), userID, locale)
					}()
				}
			}
		}
	}
}

// normalizeLocale takes the raw Accept-Language header value and returns
// exactly "en" or "fr". Any unsupported locale falls back to "fr".
func normalizeLocale(lang string) string {
	lang = strings.TrimSpace(lang)
	if len(lang) >= 2 {
		prefix := strings.ToLower(lang[:2])
		if prefix == "en" {
			return "en"
		}
	}
	return "fr"
}

// FromCtx reads the locale from the Gin context and returns the matching
// Messages struct. Falls back to FR if the locale key is absent.
func FromCtx(c *gin.Context) Messages {
	return Get(LocaleFromCtx(c))
}

// LocaleFromCtx returns the locale string ("fr" or "en") stored by Middleware.
// Falls back to "fr" if absent.
func LocaleFromCtx(c *gin.Context) string {
	if v, exists := c.Get(localeKey); exists {
		if locale, ok := v.(string); ok {
			return locale
		}
	}
	return "fr"
}
