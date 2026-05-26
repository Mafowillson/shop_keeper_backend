package httpserver

import (
	"shop_keeper_backend/internal/app"
	"shop_keeper_backend/internal/middleware"
	"shop_keeper_backend/internal/product"
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

	userSvc := user.NewService(userRepo, ap.Config.JWTSecret, ap.Config.JWTRefreshSecret)

	userHandler := user.NewHandler(userSvc)

	// API versioning
	api := router.Group("/api/v1")

	// Public auth routes
	auth := api.Group("/auth")
	auth.POST("/register", userHandler.Register)
	auth.POST("/login", userHandler.Login)
	auth.POST("/refresh", userHandler.Refresh)
	auth.POST("/logout", userHandler.Logout)

	staffRepo := staff.NewRepo(ap.DB)
	staffAuthSvc := staff.NewAuthService(staffRepo, ap.Config.JWTSecret, ap.Config.JWTRefreshSecret)
	staffAuthHandler := staff.NewAuthHandler(staffAuthSvc)

	auth.POST("/staff/login", staffAuthHandler.Login)

	// Protected API routes
	protected := api.Group("")
	protected.Use(middleware.AuthRequired(ap.Config.JWTSecret))

	productRepo := product.NewRepo(ap.DB)
	shopRepo := shop.NewRepo(ap.DB)

	shopSvc := shop.NewService(shopRepo)
	shopHandler := shop.NewHandler(shopSvc)

	productSvc := product.NewService(productRepo, shopRepo)
	productHandler := product.NewHandler(productSvc)

	staffSvc := staff.NewService(staffRepo)
	staffHandler := staff.NewHandler(staffSvc)

	products := protected.Group("/products")
	products.GET("", productHandler.List)
	products.GET("/:id", productHandler.Get)

	ownerRoutes := protected.Group("")
	ownerRoutes.Use(middleware.RequireOwner())

	shops := ownerRoutes.Group("/shops")
	shops.GET("", shopHandler.List)
	shops.GET("/:id", shopHandler.Get)
	shops.POST("", shopHandler.Create)
	shops.PUT("/:id", shopHandler.Update)
	shops.DELETE("/:id", shopHandler.Delete)

	staff := ownerRoutes.Group("/staff")
	staff.GET("", staffHandler.List)
	staff.GET("/:id", staffHandler.Get)
	staff.GET("/:id/credentials", staffHandler.GetCredentials)
	staff.POST("", staffHandler.Create)
	staff.PUT("/:id", staffHandler.Update)
	staff.DELETE("/:id", staffHandler.Delete)

	ownerProducts := ownerRoutes.Group("/products")
	ownerProducts.POST("", productHandler.Create)
	ownerProducts.PUT("/:id", productHandler.Update)
	ownerProducts.DELETE("/:id", productHandler.Delete)
	ownerProducts.POST("/sync", productHandler.Sync)

	return router
}
