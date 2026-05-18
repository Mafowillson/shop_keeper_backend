package main

import (
	"log"
	"net/http"
	"shop_keeper_backend/internal/httpserver"
	"time"
)

func main() {
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
