package httpserver

import (
	"context"
	"fmt"
	"shop_keeper_backend/internal/app"
	"shop_keeper_backend/internal/chat"
	"shop_keeper_backend/internal/customer"
	"shop_keeper_backend/internal/dashboard"
	"shop_keeper_backend/internal/i18n"
	"shop_keeper_backend/internal/middleware"
	notification "shop_keeper_backend/internal/notifications"
	"shop_keeper_backend/internal/product"
	"shop_keeper_backend/internal/sale"
	"shop_keeper_backend/internal/shop"
	"shop_keeper_backend/internal/staff"
	"shop_keeper_backend/internal/user"

	"github.com/gin-gonic/gin"
)

// shopInfoAdapter adapts *shop.Repo to satisfy staff.ShopLookup.
type shopInfoAdapter struct{ repo *shop.Repo }

func (a shopInfoAdapter) GetShopSummary(ctx context.Context, shopID string) (string, string, error) {
	s, err := a.repo.FindByID(ctx, shopID)
	if err != nil {
		return "", "", err
	}
	return s.Name, s.Description, nil
}

func (a shopInfoAdapter) GetOwnerShopID(ctx context.Context, ownerID string) (string, error) {
	shops, _, err := a.repo.ListByOwner(ctx, ownerID, 1, 1)
	if err != nil {
		return "", err
	}
	if len(shops) == 0 {
		return "", fmt.Errorf("no shop found for owner")
	}
	return shops[0].ID, nil
}

func NewRouter(ap *app.App) *gin.Engine {
	router := gin.New()
	router.Use(gin.Logger())
	router.Use(gin.Recovery())
	router.GET("/health", health)

	userRepo := user.NewRepo(ap.DB)

	// i18n middleware runs on every request — reads Accept-Language header,
	// sets locale in context, and asynchronously saves it for authenticated users.
	router.Use(i18n.Middleware(userRepo))

	userSvc := user.NewService(userRepo, ap.EmailService, ap.Config.JWTSecret, ap.Config.JWTRefreshSecret)
	userHandler := user.NewHandler(userSvc)

	api := router.Group("/api/v1")

	// ── Public auth ─────────────────────────────────────────────────────────
	auth := api.Group("/auth")
	auth.POST("/register", userHandler.Register)
	auth.POST("/login", userHandler.Login)
	auth.POST("/refresh", userHandler.Refresh)
	auth.POST("/logout", userHandler.Logout)

	staffRepo := staff.NewRepo(ap.DB)
	shopRepo := shop.NewRepo(ap.DB)

	// notifSvc is created here (before staffAuthSvc) so it can be passed to
	// staffAuthSvc for the staff-login notification.
	notifRepo := notification.NewRepo(ap.DB)
	notifSvc := notification.NewService(notifRepo, ap.FCMClient, staffRepo, userRepo)

	staffAuthSvc := staff.NewAuthService(
		staffRepo,
		ap.Config.JWTSecret, ap.Config.JWTRefreshSecret,
		notifSvc,
		shopInfoAdapter{shopRepo},
	)
	staffAuthHandler := staff.NewAuthHandler(staffAuthSvc)
	auth.POST("/staff/login", staffAuthHandler.Login)
	auth.POST("/staff/refresh", staffAuthHandler.Refresh)
	auth.POST("/forgot-password", userHandler.ForgotPassword)
	auth.POST("/reset-password", userHandler.ResetPassword)

	// ── Protected routes (valid JWT required) ───────────────────────────────
	protected := api.Group("")
	protected.Use(middleware.AuthRequired(ap.Config.JWTSecret))

	// Email verification (owner must be logged in but not yet verified)
	protected.POST("/auth/verify-email", userHandler.VerifyEmail)
	protected.POST("/auth/resend-verification", userHandler.ResendVerificationCode)

	shopSvc := shop.NewService(shopRepo, userRepo)
	shopHandler := shop.NewHandler(shopSvc)
	notifHandler := notification.NewHandler(notifSvc)

	productRepo := product.NewRepo(ap.DB)
	productSvc := product.NewService(productRepo, shopRepo, notifSvc)
	productHandler := product.NewHandler(productSvc)

	customerRepo := customer.NewRepo(ap.DB)
	customerSvc := customer.NewService(customerRepo)
	customerHandler := customer.NewHandler(customerSvc)

	saleRepo := sale.NewRepo(ap.DB)
	saleSvc := sale.NewService(saleRepo, productRepo, shopRepo, staffRepo, customerSvc)
	saleHandler := sale.NewHandler(saleSvc)

	staffSvc := staff.NewService(staffRepo)
	staffHandler := staff.NewHandler(staffSvc, shopInfoAdapter{shopRepo})

	// FCM token upload — available to all authenticated users (owner + staff).
	protected.POST("/owner/fcm-token", notifHandler.SaveFCMToken)
	protected.POST("/staff/fcm-token", staffHandler.SaveFCMToken)
	protected.GET("/staff/my-shop", staffHandler.GetMyShop)

	// Staff notification inbox — accessible to all authenticated users (staff use their own ID).
	staffNotifs := protected.Group("/staff/notifications")
	staffNotifs.GET("", notifHandler.GetStaffInbox)
	staffNotifs.PATCH("/:id/read", notifHandler.MarkStaffRead)
	staffNotifs.PATCH("/read-all", notifHandler.MarkStaffAllRead)

	// Accessible to both staff and owner
	products := protected.Group("/products")
	products.GET("", productHandler.List)
	products.GET("/:id", productHandler.Get)

	salesPublic := protected.Group("/sales")
	salesPublic.POST("", saleHandler.Create)

	customers := protected.Group("/customers")
	customers.GET("", customerHandler.List)
	customers.GET("/:id", customerHandler.Get)
	customers.POST("", customerHandler.Create)
	customers.POST("/:id/payment", customerHandler.RecordPayment)

	// ── Owner-only routes ───────────────────────────────────────────────────
	ownerRoutes := protected.Group("")
	ownerRoutes.Use(middleware.RequireOwner())

	shops := ownerRoutes.Group("/shops")
	shops.GET("", shopHandler.List)
	shops.GET("/:id", shopHandler.Get)
	shops.POST("", shopHandler.Create)
	shops.PUT("/:id", shopHandler.Update)
	shops.DELETE("/:id", shopHandler.Delete)

	staffGroup := ownerRoutes.Group("/staff")
	staffGroup.GET("", staffHandler.List)
	staffGroup.GET("/:id", staffHandler.Get)
	staffGroup.GET("/:id/credentials", staffHandler.GetCredentials)
	staffGroup.POST("", staffHandler.Create)
	staffGroup.PUT("/:id", staffHandler.Update)
	staffGroup.DELETE("/:id", staffHandler.Delete)

	ownerProducts := ownerRoutes.Group("/products")
	ownerProducts.POST("", productHandler.Create)
	ownerProducts.PUT("/:id", productHandler.Update)
	ownerProducts.DELETE("/:id", productHandler.Delete)
	ownerProducts.POST("/sync", productHandler.Sync)

	sales := ownerRoutes.Group("/sales")
	sales.GET("", saleHandler.List)
	sales.GET("/:id", saleHandler.Get)

	// GET /customers and GET /customers/:id are already registered in the
	// protected group (accessible to all auth users). Only the debt history
	// endpoint stays owner-only.
	ownerRoutes.GET("/customers/:id/debts", customerHandler.GetDebtHistory)

	notifs := ownerRoutes.Group("/notifications")
	notifs.GET("", notifHandler.GetInbox)
	notifs.PATCH("/:id/read", notifHandler.MarkRead)
	notifs.PATCH("/read-all", notifHandler.MarkAllRead)
	notifs.GET("/preferences", notifHandler.GetPreferences)
	notifs.PUT("/preferences", notifHandler.UpdatePreferences)

	dashSvc := dashboard.NewService(userRepo, shopRepo, saleRepo, productRepo, customerRepo, staffRepo)
	dashHandler := dashboard.NewHandler(dashSvc)
	ownerRoutes.GET("/dashboard", dashHandler.GetOwnerDashboard)
	protected.GET("/staff/dashboard", dashHandler.GetStaffDashboard)

	chatRepo := chat.NewRepo(ap.DB)
	chatSvc := chat.NewService(chatRepo, userRepo, productRepo, saleRepo, ap.Config.GroqAPIKey)
	chatHandler := chat.NewHandler(chatSvc)
	chatRoutes := ownerRoutes.Group("/chat")
	chatRoutes.POST("/message", chatHandler.Send)
	chatRoutes.GET("/history", chatHandler.GetHistory)
	chatRoutes.DELETE("/history", chatHandler.Clear)

	return router
}
