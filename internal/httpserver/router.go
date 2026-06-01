package httpserver

import (
	"shop_keeper_backend/internal/app"
	"shop_keeper_backend/internal/customer"
	"shop_keeper_backend/internal/dashboard"
	"shop_keeper_backend/internal/middleware"
	notification "shop_keeper_backend/internal/notifications"
	"shop_keeper_backend/internal/product"
	"shop_keeper_backend/internal/sale"
	"shop_keeper_backend/internal/shop"
	"shop_keeper_backend/internal/staff"
	"shop_keeper_backend/internal/user"

	"github.com/gin-gonic/gin"
)

func NewRouter(ap *app.App) *gin.Engine {
	router := gin.New()
	router.Use(gin.Logger())
	router.Use(gin.Recovery())
	router.GET("/health", health)

	userRepo := user.NewRepo(ap.DB)
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
	staffAuthSvc := staff.NewAuthService(staffRepo, ap.Config.JWTSecret, ap.Config.JWTRefreshSecret)
	staffAuthHandler := staff.NewAuthHandler(staffAuthSvc)
	auth.POST("/staff/login", staffAuthHandler.Login)
	auth.POST("/forgot-password", userHandler.ForgotPassword)
	auth.POST("/reset-password", userHandler.ResetPassword)

	// ── Protected routes (valid JWT required) ───────────────────────────────
	protected := api.Group("")
	protected.Use(middleware.AuthRequired(ap.Config.JWTSecret))

	// Email verification (owner must be logged in but not yet verified)
	protected.POST("/auth/verify-email", userHandler.VerifyEmail)
	protected.POST("/auth/resend-verification", userHandler.ResendVerificationCode)

	shopRepo := shop.NewRepo(ap.DB)
	shopSvc := shop.NewService(shopRepo, userRepo)
	shopHandler := shop.NewHandler(shopSvc)

	// notifSvc must be created before productSvc — product depends on it.
	notifRepo := notification.NewRepo(ap.DB)
	notifSvc := notification.NewService(notifRepo, ap.FCMClient, staffRepo)
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
	staffHandler := staff.NewHandler(staffSvc)

	// FCM token upload — available to all authenticated users (owner + staff).
	protected.POST("/owner/fcm-token", notifHandler.SaveFCMToken)
	protected.POST("/staff/fcm-token", staffHandler.SaveFCMToken)

	// Accessible to both staff and owner
	products := protected.Group("/products")
	products.GET("", productHandler.List)
	products.GET("/:id", productHandler.Get)

	salesPublic := protected.Group("/sales")
	salesPublic.POST("", saleHandler.Create)

	customers := protected.Group("/customers")
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

	ownerCustomers := ownerRoutes.Group("/customers")
	ownerCustomers.GET("", customerHandler.List)
	ownerCustomers.GET("/:id/debts", customerHandler.GetDebtHistory)

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

	return router
}
