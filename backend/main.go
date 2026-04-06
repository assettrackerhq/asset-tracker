package main

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/assettrackerhq/asset-tracker/backend/internal/assets"
	"github.com/assettrackerhq/asset-tracker/backend/internal/auth"
	"github.com/assettrackerhq/asset-tracker/backend/internal/values"
	"github.com/assettrackerhq/asset-tracker/backend/internal/config"
	"github.com/assettrackerhq/asset-tracker/backend/internal/database"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	ctx := context.Background()
	pool, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer pool.Close()

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"http://localhost:5173"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	r.Get("/api/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	// Auth routes
	authHandler := auth.NewHandler(pool, cfg.JWTSecret)
	r.Post("/api/auth/register", authHandler.Register)
	r.Post("/api/auth/login", authHandler.Login)

	// Asset routes (protected)
	assetHandler := assets.NewHandler(pool)
	valueHandler := values.NewHandler(pool)
	r.Route("/api/assets", func(r chi.Router) {
		r.Use(auth.Middleware(cfg.JWTSecret))
		r.Get("/", assetHandler.List)
		r.Post("/", assetHandler.Create)
		r.Put("/{id}", assetHandler.Update)
		r.Delete("/{id}", assetHandler.Delete)

		// Value point routes
		r.Get("/{assetID}/values", valueHandler.List)
		r.Post("/{assetID}/values", valueHandler.Create)
		r.Put("/{assetID}/values/{valueID}", valueHandler.Update)
		r.Delete("/{assetID}/values/{valueID}", valueHandler.Delete)
	})

	addr := fmt.Sprintf(":%s", cfg.Port)
	log.Printf("starting server on %s", addr)
	if err := http.ListenAndServe(addr, r); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
