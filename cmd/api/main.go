package main

import (
	"context"
	"log"
	"net/http"
	"shop_keeper_backend/internal/app"
	"shop_keeper_backend/internal/httpserver"
	"time"
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

	router := httpserver.NewRouter()

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
