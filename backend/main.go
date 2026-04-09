package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/assettrackerhq/asset-tracker/backend/internal/assets"
	"github.com/assettrackerhq/asset-tracker/backend/internal/auth"
	"github.com/assettrackerhq/asset-tracker/backend/internal/config"
	"github.com/assettrackerhq/asset-tracker/backend/internal/database"
	"github.com/assettrackerhq/asset-tracker/backend/internal/email"
	"github.com/assettrackerhq/asset-tracker/backend/internal/license"
	"github.com/assettrackerhq/asset-tracker/backend/internal/metrics"
	"github.com/assettrackerhq/asset-tracker/backend/internal/supportbundle"
	"github.com/assettrackerhq/asset-tracker/backend/internal/updates"
	"github.com/assettrackerhq/asset-tracker/backend/internal/values"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	pool, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer pool.Close()

	// Start metrics reporter
	sdkEndpoint := cfg.ReplicatedSDKEndpoint + "/api/v1/app/custom-metrics"
	reporter := metrics.New(&poolAdapter{pool}, sdkEndpoint, cfg.MetricsInterval)
	go reporter.Run(ctx)

	// Email sender + verifier
	sender := email.NewSender(email.SMTPConfig{
		Host:     cfg.SMTPHost,
		Port:     cfg.SMTPPort,
		Username: cfg.SMTPUsername,
		Password: cfg.SMTPPassword,
		From:     cfg.SMTPFrom,
	})
	verifier := email.NewVerifier(pool, sender)

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
		dbStatus := "connected"
		status := "ok"
		httpStatus := http.StatusOK

		if err := pool.Ping(r.Context()); err != nil {
			dbStatus = "disconnected"
			status = "degraded"
			httpStatus = http.StatusServiceUnavailable
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(httpStatus)
		json.NewEncoder(w).Encode(map[string]string{
			"status":    status,
			"database":  dbStatus,
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		})
	})

	// License client for entitlement checks
	licenseClient := license.NewClient(cfg.ReplicatedSDKEndpoint)

	// Start license validity checker
	licenseChecker := license.NewChecker(licenseClient)
	licenseChecker.CheckNow(ctx)
	go licenseChecker.Run(ctx)

	// Auth routes (protected by license check)
	authHandler := auth.NewHandler(pool, cfg.JWTSecret, licenseClient, verifier)
	r.Group(func(r chi.Router) {
		r.Use(license.LicenseMiddleware(licenseChecker))
		r.Post("/api/auth/register", authHandler.Register)
		r.Post("/api/auth/login", authHandler.Login)
		r.Post("/api/auth/verify-email", authHandler.VerifyEmail)
		r.Post("/api/auth/resend-verification", authHandler.ResendVerification)
		r.Get("/api/auth/user-limit", authHandler.UserLimitInfo)
	})

	// Update check route
	updateHandler := updates.NewHandler(cfg.ReplicatedSDKEndpoint)
	r.Get("/api/app/updates", updateHandler.Check)

	// Support bundle route (auth required, no license check — allow bundles even when license expired)
	bundleHandler := supportbundle.NewHandler(cfg.SupportBundleImage, cfg.SupportBundleServiceAccount, cfg.SupportBundleImagePullSecrets, cfg.ReplicatedSDKEndpoint)
	r.Group(func(r chi.Router) {
		r.Use(auth.Middleware(cfg.JWTSecret))
		r.Post("/api/support-bundle", bundleHandler.Generate)
		r.Get("/api/support-bundle/{name}", bundleHandler.Status)
	})

	// License status route (public, exempt from license middleware)
	r.Get("/api/license/status", func(w http.ResponseWriter, r *http.Request) {
		status := licenseChecker.CurrentStatus()
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]any{"valid": status.Valid}
		if !status.Valid {
			resp["message"] = status.Message
		}
		json.NewEncoder(w).Encode(resp)
	})

	// Asset routes (protected by license check + auth)
	assetHandler := assets.NewHandler(pool)
	valueHandler := values.NewHandler(pool)
	r.Route("/api/assets", func(r chi.Router) {
		r.Use(license.LicenseMiddleware(licenseChecker))
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
	srv := &http.Server{Addr: addr, Handler: r}

	go func() {
		log.Printf("starting server on %s", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server failed: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("shutting down...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	srv.Shutdown(shutdownCtx)
}

// poolAdapter adapts pgxpool.Pool to the metrics.UserCounter interface.
type poolAdapter struct {
	pool *pgxpool.Pool
}

func (a *poolAdapter) QueryRow(ctx context.Context, sql string, args ...any) metrics.Row {
	return a.pool.QueryRow(ctx, sql, args...)
}
