package main

import (
	"context"
	"log"
	"net/http"
	"shop_keeper_backend/internal/app"
	"shop_keeper_backend/internal/httpserver"
	"time"

	"github.com/gin-contrib/cors"
)

func main() {

	// root context
	ctx := context.Background()

	ap, err := app.New(ctx)
	if err != nil {
		log.Fatalf("Startup failed: %v", err)
	}

	defer func() {
		if err := ap.Close(ctx); err != nil {
			log.Printf("Shutdown warning: %v", err)
		}
	}()

	// CORS configuration
	corsConfig := cors.Config{}

	corsConfig.AllowAllOrigins = true
	corsConfig.AllowMethods = []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}
	corsConfig.AllowHeaders = []string{"Origin", "Content-Type", "Authorization"}
	corsConfig.ExposeHeaders = []string{"Content-Length"}
	corsConfig.AllowCredentials = true
	corsConfig.MaxAge = 12 * time.Hour

	router := httpserver.NewRouter(ap)
	router.Use(cors.New(corsConfig))

	// standard go type that runs a http server
	srv := &http.Server{
		Addr:              ":8080",
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("Api running on %s", srv.Addr)

	if err := srv.ListenAndServe(); err != nil {
		if err == http.ErrServerClosed {
			log.Printf("Server closed")
			return
		}
		log.Fatalf("Server error: %v", err)
	}
}
