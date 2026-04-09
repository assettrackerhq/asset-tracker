# Preflight Checks & Email Verification Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add 5 Replicated preflight checks (DB connectivity, SMTP endpoint, cluster resources, k8s version, distribution) and email verification on signup with Mailpit for development SMTP.

**Architecture:** Preflight checks are defined as a `troubleshoot.sh/v1beta2` Preflight Secret in the Helm chart. Email verification uses 6-digit codes stored in a new `verification_codes` table, sent via Go's `net/smtp`. Mailpit runs as an optional in-chart deployment for dev/test environments.

**Tech Stack:** Troubleshoot.sh, Go `net/smtp`, PostgreSQL, React, Helm, Mailpit, Playwright

---

## File Structure

**Create:**
- `backend/internal/email/sender.go` — SMTP client (authenticated + unauthenticated)
- `backend/internal/email/verification.go` — Code generation, storage, sending, validation
- `frontend/src/pages/VerifyEmail.jsx` — 6-digit code entry screen
- `schemas/tables/verification_codes.yaml` — SchemaHero table definition
- `helm/asset-tracker/templates/preflight.yaml` — All 5 preflight checks
- `helm/asset-tracker/templates/mailpit-deployment.yaml` — Mailpit Deployment
- `helm/asset-tracker/templates/mailpit-service.yaml` — Mailpit Service

**Modify:**
- `schemas/tables/users.yaml` — Add email + email_verified columns
- `schemas/ddl/schema.sql` — Add columns + new table
- `helm/asset-tracker/schema.sql` — Mirror of ddl/schema.sql
- `backend/internal/config/config.go` — Add SMTP config fields
- `backend/internal/auth/handler.go` — Email in registration, verification check on login, new endpoints
- `backend/main.go` — Wire email sender, add verify/resend routes
- `frontend/src/api.js` — Add register with email, verifyEmail, resendVerification functions
- `frontend/src/pages/Register.jsx` — Add email field, redirect to verify page
- `frontend/src/pages/Login.jsx` — Handle email-not-verified error
- `frontend/src/App.jsx` — Add /verify-email route
- `frontend/src/App.css` — Add verification input styles
- `helm/asset-tracker/values.yaml` — Add smtp + mailpit sections
- `helm/asset-tracker/templates/backend-deployment.yaml` — Add SMTP env vars
- `docker-compose.yml` — Add mailpit service + SMTP env vars
- `.env.example` — Add SMTP vars
- `e2e/asset-tracker.spec.mjs` — Update auth tests for email verification flow
- `playwright.config.mjs` — Add MAILPIT_URL env var

---

### Task 1: Database Schema Changes

**Files:**
- Modify: `schemas/tables/users.yaml`
- Create: `schemas/tables/verification_codes.yaml`
- Modify: `schemas/ddl/schema.sql`
- Modify: `helm/asset-tracker/schema.sql`

- [ ] **Step 1: Update users table schema to add email columns**

Replace `schemas/tables/users.yaml` with:

```yaml
apiVersion: schemas.schemahero.io/v1alpha4
kind: Table
metadata:
  name: users
spec:
  name: users
  schema:
    postgres:
      primaryKey:
        - id
      columns:
        - name: id
          type: uuid
          default: "gen_random_uuid()"
          constraints:
            notNull: true
        - name: username
          type: varchar(255)
          constraints:
            notNull: true
        - name: email
          type: varchar(255)
          constraints:
            notNull: true
        - name: email_verified
          type: boolean
          default: "false"
          constraints:
            notNull: true
        - name: password_hash
          type: varchar(255)
          constraints:
            notNull: true
        - name: created_at
          type: timestamp
          default: "now()"
          constraints:
            notNull: true
      indexes:
        - columns:
            - username
          isUnique: true
          name: idx_users_username_unique
        - columns:
            - email
          isUnique: true
          name: idx_users_email_unique
```

- [ ] **Step 2: Create verification_codes table schema**

Create `schemas/tables/verification_codes.yaml`:

```yaml
apiVersion: schemas.schemahero.io/v1alpha4
kind: Table
metadata:
  name: verification-codes
spec:
  name: verification_codes
  schema:
    postgres:
      primaryKey:
        - id
      columns:
        - name: id
          type: uuid
          default: "gen_random_uuid()"
          constraints:
            notNull: true
        - name: user_id
          type: uuid
          constraints:
            notNull: true
        - name: code
          type: varchar(6)
          constraints:
            notNull: true
        - name: expires_at
          type: timestamp
          constraints:
            notNull: true
        - name: created_at
          type: timestamp
          default: "now()"
          constraints:
            notNull: true
```

- [ ] **Step 3: Update DDL SQL files**

Replace both `schemas/ddl/schema.sql` and `helm/asset-tracker/schema.sql` with:

```sql
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username VARCHAR(255) NOT NULL,
    email VARCHAR(255) NOT NULL,
    email_verified BOOLEAN NOT NULL DEFAULT false,
    password_hash VARCHAR(255) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_users_username_unique ON users (username);
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email_unique ON users (email);

CREATE TABLE IF NOT EXISTS assets (
    id VARCHAR(50) NOT NULL,
    user_id UUID NOT NULL,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT now(),
    updated_at TIMESTAMP NOT NULL DEFAULT now(),
    PRIMARY KEY (id, user_id)
);

CREATE TABLE IF NOT EXISTS asset_value_points (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    asset_id VARCHAR(50) NOT NULL,
    user_id UUID NOT NULL,
    timestamp TIMESTAMP NOT NULL DEFAULT now(),
    value NUMERIC(15,2) NOT NULL,
    currency VARCHAR(3) NOT NULL
);

CREATE TABLE IF NOT EXISTS verification_codes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL,
    code VARCHAR(6) NOT NULL,
    expires_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT now()
);
```

- [ ] **Step 4: Commit**

```bash
git add schemas/tables/users.yaml schemas/tables/verification_codes.yaml schemas/ddl/schema.sql helm/asset-tracker/schema.sql
git commit -m "feat: add email and verification_codes to database schema"
```

---

### Task 2: Backend Email Sender

**Files:**
- Create: `backend/internal/email/sender.go`

- [ ] **Step 1: Create the email sender package**

Create `backend/internal/email/sender.go`:

```go
package email

import (
	"fmt"
	"net"
	"net/smtp"
)

type SMTPConfig struct {
	Host     string
	Port     string
	Username string
	Password string
	From     string
}

type Sender struct {
	cfg SMTPConfig
}

func NewSender(cfg SMTPConfig) *Sender {
	return &Sender{cfg: cfg}
}

func (s *Sender) Send(to, subject, body string) error {
	addr := net.JoinHostPort(s.cfg.Host, s.cfg.Port)

	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=\"utf-8\"\r\n\r\n%s",
		s.cfg.From, to, subject, body)

	var auth smtp.Auth
	if s.cfg.Username != "" {
		auth = smtp.PlainAuth("", s.cfg.Username, s.cfg.Password, s.cfg.Host)
	}

	return smtp.SendMail(addr, auth, s.cfg.From, []string{to}, []byte(msg))
}
```

- [ ] **Step 2: Commit**

```bash
git add backend/internal/email/sender.go
git commit -m "feat: add SMTP email sender package"
```

---

### Task 3: Backend Email Verification Logic

**Files:**
- Create: `backend/internal/email/verification.go`

- [ ] **Step 1: Create the verification code logic**

Create `backend/internal/email/verification.go`:

```go
package email

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const codeExpiry = 15 * time.Minute

type Verifier struct {
	db     *pgxpool.Pool
	sender *Sender
}

func NewVerifier(db *pgxpool.Pool, sender *Sender) *Verifier {
	return &Verifier{db: db, sender: sender}
}

func (v *Verifier) SendCode(ctx context.Context, userID, emailAddr string) error {
	code, err := generateCode()
	if err != nil {
		return fmt.Errorf("generate code: %w", err)
	}

	expiresAt := time.Now().Add(codeExpiry)

	// Delete any existing codes for this user
	_, err = v.db.Exec(ctx, "DELETE FROM verification_codes WHERE user_id = $1", userID)
	if err != nil {
		return fmt.Errorf("clear old codes: %w", err)
	}

	_, err = v.db.Exec(ctx,
		"INSERT INTO verification_codes (user_id, code, expires_at) VALUES ($1, $2, $3)",
		userID, code, expiresAt,
	)
	if err != nil {
		return fmt.Errorf("store code: %w", err)
	}

	body := fmt.Sprintf("Your verification code is: %s\n\nThis code expires in 15 minutes.", code)
	if err := v.sender.Send(emailAddr, "Verify your email — Asset Tracker", body); err != nil {
		return fmt.Errorf("send email: %w", err)
	}

	return nil
}

func (v *Verifier) Verify(ctx context.Context, userID, code string) error {
	var storedCode string
	var expiresAt time.Time

	err := v.db.QueryRow(ctx,
		"SELECT code, expires_at FROM verification_codes WHERE user_id = $1 ORDER BY created_at DESC LIMIT 1",
		userID,
	).Scan(&storedCode, &expiresAt)
	if err != nil {
		return fmt.Errorf("no verification code found — request a new one")
	}

	if time.Now().After(expiresAt) {
		return fmt.Errorf("verification code has expired — request a new one")
	}

	if storedCode != code {
		return fmt.Errorf("incorrect verification code")
	}

	_, err = v.db.Exec(ctx, "UPDATE users SET email_verified = true WHERE id = $1", userID)
	if err != nil {
		return fmt.Errorf("update user: %w", err)
	}

	// Clean up used codes
	_, _ = v.db.Exec(ctx, "DELETE FROM verification_codes WHERE user_id = $1", userID)

	return nil
}

func (v *Verifier) GetEmail(ctx context.Context, userID string) (string, error) {
	var emailAddr string
	err := v.db.QueryRow(ctx, "SELECT email FROM users WHERE id = $1", userID).Scan(&emailAddr)
	if err != nil {
		return "", fmt.Errorf("user not found")
	}
	return emailAddr, nil
}

func generateCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}
```

- [ ] **Step 2: Commit**

```bash
git add backend/internal/email/verification.go
git commit -m "feat: add email verification code logic"
```

---

### Task 4: Backend Config — Add SMTP Fields

**Files:**
- Modify: `backend/internal/config/config.go`

- [ ] **Step 1: Add SMTP config fields**

Replace the full contents of `backend/internal/config/config.go` with:

```go
package config

import (
	"fmt"
	"os"
	"time"
)

type Config struct {
	DatabaseURL           string
	JWTSecret             string
	Port                  string
	ReplicatedSDKEndpoint string
	MetricsInterval       time.Duration
	SMTPHost              string
	SMTPPort              string
	SMTPUsername           string
	SMTPPassword           string
	SMTPFrom              string
}

func Load() (*Config, error) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET is required")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	sdkEndpoint := os.Getenv("REPLICATED_SDK_ENDPOINT")
	if sdkEndpoint == "" {
		sdkEndpoint = "http://asset-tracker-sdk:3000"
	}

	metricsInterval := 4 * time.Hour
	if v := os.Getenv("METRICS_INTERVAL"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return nil, fmt.Errorf("invalid METRICS_INTERVAL: %w", err)
		}
		metricsInterval = d
	}

	smtpPort := os.Getenv("SMTP_PORT")
	if smtpPort == "" {
		smtpPort = "587"
	}

	return &Config{
		DatabaseURL:           dbURL,
		JWTSecret:             jwtSecret,
		Port:                  port,
		ReplicatedSDKEndpoint: sdkEndpoint,
		MetricsInterval:       metricsInterval,
		SMTPHost:              os.Getenv("SMTP_HOST"),
		SMTPPort:              smtpPort,
		SMTPUsername:           os.Getenv("SMTP_USERNAME"),
		SMTPPassword:           os.Getenv("SMTP_PASSWORD"),
		SMTPFrom:              os.Getenv("SMTP_FROM"),
	}, nil
}
```

- [ ] **Step 2: Commit**

```bash
git add backend/internal/config/config.go
git commit -m "feat: add SMTP configuration fields"
```

---

### Task 5: Backend Auth Handler — Email Registration + Verification Endpoints

**Files:**
- Modify: `backend/internal/auth/handler.go`

- [ ] **Step 1: Update auth handler with email verification**

Replace the full contents of `backend/internal/auth/handler.go` with:

```go
package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/assettrackerhq/asset-tracker/backend/internal/email"
	"github.com/assettrackerhq/asset-tracker/backend/internal/license"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

type Handler struct {
	db            *pgxpool.Pool
	jwtSecret     string
	licenseClient *license.Client
	verifier      *email.Verifier
}

func NewHandler(db *pgxpool.Pool, jwtSecret string, licenseClient *license.Client, verifier *email.Verifier) *Handler {
	return &Handler{db: db, jwtSecret: jwtSecret, licenseClient: licenseClient, verifier: verifier}
}

type registerRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type registerResponse struct {
	UserID  string `json:"user_id"`
	Message string `json:"message"`
}

type authResponse struct {
	Token string `json:"token"`
}

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	req.Username = strings.TrimSpace(req.Username)
	req.Email = strings.TrimSpace(req.Email)
	if req.Username == "" {
		http.Error(w, `{"error":"username is required"}`, http.StatusBadRequest)
		return
	}
	if req.Email == "" || !strings.Contains(req.Email, "@") {
		http.Error(w, `{"error":"a valid email is required"}`, http.StatusBadRequest)
		return
	}
	if len(req.Password) < 8 {
		http.Error(w, `{"error":"password must be at least 8 characters"}`, http.StatusBadRequest)
		return
	}

	// Check license entitlement for user limit
	if err := h.checkUserLimit(r.Context()); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":%q}`, err.Error()), http.StatusForbidden)
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), 12)
	if err != nil {
		http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
		return
	}

	userID := uuid.New().String()
	_, err = h.db.Exec(context.Background(),
		"INSERT INTO users (id, username, email, password_hash) VALUES ($1, $2, $3, $4)",
		userID, req.Username, req.Email, string(hash),
	)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "unique") {
			if strings.Contains(err.Error(), "email") {
				http.Error(w, `{"error":"email already exists"}`, http.StatusConflict)
			} else {
				http.Error(w, `{"error":"username already exists"}`, http.StatusConflict)
			}
			return
		}
		http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
		return
	}

	// Send verification code
	if err := h.verifier.SendCode(r.Context(), userID, req.Email); err != nil {
		log.Printf("failed to send verification email: %v", err)
		// Don't fail registration — user can resend later
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(registerResponse{
		UserID:  userID,
		Message: "Registration successful. Check your email for a verification code.",
	})
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	var userID, passwordHash string
	var emailVerified bool
	err := h.db.QueryRow(context.Background(),
		"SELECT id, password_hash, email_verified FROM users WHERE username = $1",
		strings.TrimSpace(req.Username),
	).Scan(&userID, &passwordHash, &emailVerified)
	if err != nil {
		http.Error(w, `{"error":"invalid username or password"}`, http.StatusUnauthorized)
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.Password)); err != nil {
		http.Error(w, `{"error":"invalid username or password"}`, http.StatusUnauthorized)
		return
	}

	if !emailVerified {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "email_not_verified",
			"user_id": userID,
			"message": "Please verify your email before logging in. Check your inbox for a verification code.",
		})
		return
	}

	token, err := GenerateToken(userID, h.jwtSecret)
	if err != nil {
		http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(authResponse{Token: token})
}

func (h *Handler) VerifyEmail(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UserID string `json:"user_id"`
		Code   string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	if req.UserID == "" || req.Code == "" {
		http.Error(w, `{"error":"user_id and code are required"}`, http.StatusBadRequest)
		return
	}

	if err := h.verifier.Verify(r.Context(), req.UserID, req.Code); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":%q}`, err.Error()), http.StatusBadRequest)
		return
	}

	token, err := GenerateToken(req.UserID, h.jwtSecret)
	if err != nil {
		http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(authResponse{Token: token})
}

func (h *Handler) ResendVerification(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UserID string `json:"user_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	if req.UserID == "" {
		http.Error(w, `{"error":"user_id is required"}`, http.StatusBadRequest)
		return
	}

	emailAddr, err := h.verifier.GetEmail(r.Context(), req.UserID)
	if err != nil {
		http.Error(w, `{"error":"user not found"}`, http.StatusNotFound)
		return
	}

	if err := h.verifier.SendCode(r.Context(), req.UserID, emailAddr); err != nil {
		log.Printf("failed to resend verification email: %v", err)
		http.Error(w, `{"error":"failed to send verification email"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Verification code sent. Check your email.",
	})
}

func (h *Handler) checkUserLimit(ctx context.Context) error {
	limit, err := h.licenseClient.UserLimit(ctx)
	if err != nil {
		log.Printf("license: failed to check user_limit, allowing registration: %v", err)
		return nil
	}

	var count int
	err = h.db.QueryRow(ctx, "SELECT COUNT(*) FROM users").Scan(&count)
	if err != nil {
		log.Printf("license: failed to count users: %v", err)
		return nil
	}

	if count >= limit {
		return fmt.Errorf("user limit reached (%d/%d). Please contact your administrator to increase the user limit in your license.", count, limit)
	}
	return nil
}

// UserLimitInfo returns the current user count and license limit.
func (h *Handler) UserLimitInfo(w http.ResponseWriter, r *http.Request) {
	limit, err := h.licenseClient.UserLimit(r.Context())
	if err != nil {
		log.Printf("license: failed to check user_limit: %v", err)
		limit = 1
	}

	var count int
	if err := h.db.QueryRow(r.Context(), "SELECT COUNT(*) FROM users").Scan(&count); err != nil {
		http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]int{
		"user_count": count,
		"user_limit": limit,
	})
}
```

- [ ] **Step 2: Commit**

```bash
git add backend/internal/auth/handler.go
git commit -m "feat: add email to registration and verification endpoints"
```

---

### Task 6: Backend main.go — Wire Email and New Routes

**Files:**
- Modify: `backend/main.go`

- [ ] **Step 1: Update main.go to initialize email and register new routes**

Replace the full contents of `backend/main.go` with:

```go
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
```

- [ ] **Step 2: Verify backend compiles**

Run: `cd backend && go build ./...`
Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add backend/main.go
git commit -m "feat: wire email sender and verification routes in main"
```

---

### Task 7: Frontend API — Add Email Verification Functions

**Files:**
- Modify: `frontend/src/api.js`

- [ ] **Step 1: Update api.js to include email in register and add verification functions**

Replace the full contents of `frontend/src/api.js` with:

```js
const API_BASE = '/api';

async function request(path, options = {}) {
  const token = localStorage.getItem('token');
  const headers = {
    'Content-Type': 'application/json',
    ...options.headers,
  };
  if (token) {
    headers['Authorization'] = `Bearer ${token}`;
  }

  const response = await fetch(`${API_BASE}${path}`, {
    ...options,
    headers,
  });

  if (response.status === 403) {
    const data = await response.json().catch(() => ({}));
    if (data.error === 'license_expired') {
      localStorage.removeItem('token');
      window.location.href = '/license-expired';
      return;
    }
    throw new Error(data.error || 'Forbidden');
  }

  if (response.status === 401) {
    localStorage.removeItem('token');
    window.location.href = '/login';
    return;
  }

  if (response.status === 204) {
    return null;
  }

  const data = await response.json();
  if (!response.ok) {
    throw new Error(data.error || 'Request failed');
  }
  return data;
}

export function login(username, password) {
  return request('/auth/login', {
    method: 'POST',
    body: JSON.stringify({ username, password }),
  });
}

export function register(username, email, password) {
  return request('/auth/register', {
    method: 'POST',
    body: JSON.stringify({ username, email, password }),
  });
}

export function verifyEmail(userId, code) {
  return request('/auth/verify-email', {
    method: 'POST',
    body: JSON.stringify({ user_id: userId, code }),
  });
}

export function resendVerification(userId) {
  return request('/auth/resend-verification', {
    method: 'POST',
    body: JSON.stringify({ user_id: userId }),
  });
}

export function getUserLimit() {
  return request('/auth/user-limit');
}

export async function checkForUpdates() {
  try {
    const res = await fetch(`${API_BASE}/app/updates`);
    if (!res.ok) return { updatesAvailable: false };
    return await res.json();
  } catch {
    return { updatesAvailable: false };
  }
}

export function listAssets() {
  return request('/assets');
}

export function createAsset(id, name, description) {
  return request('/assets', {
    method: 'POST',
    body: JSON.stringify({ id, name, description }),
  });
}

export function updateAsset(id, name, description) {
  return request(`/assets/${id}`, {
    method: 'PUT',
    body: JSON.stringify({ name, description }),
  });
}

export function deleteAsset(id) {
  return request(`/assets/${id}`, { method: 'DELETE' });
}

export function listValuePoints(assetId) {
  return request(`/assets/${assetId}/values`);
}

export function createValuePoint(assetId, value, currency) {
  return request(`/assets/${assetId}/values`, {
    method: 'POST',
    body: JSON.stringify({ value: parseFloat(value), currency }),
  });
}

export function updateValuePoint(assetId, valueId, value, currency) {
  return request(`/assets/${assetId}/values/${valueId}`, {
    method: 'PUT',
    body: JSON.stringify({ value: parseFloat(value), currency }),
  });
}

export function deleteValuePoint(assetId, valueId) {
  return request(`/assets/${assetId}/values/${valueId}`, { method: 'DELETE' });
}

export async function getLicenseStatus() {
  try {
    const res = await fetch(`${API_BASE}/license/status`);
    if (!res.ok) return { valid: true };
    return await res.json();
  } catch {
    return { valid: true };
  }
}
```

- [ ] **Step 2: Commit**

```bash
git add frontend/src/api.js
git commit -m "feat: add email verification API functions"
```

---

### Task 8: Frontend — Register Page with Email Field

**Files:**
- Modify: `frontend/src/pages/Register.jsx`

- [ ] **Step 1: Update Register.jsx to include email field and redirect to verification**

Replace the full contents of `frontend/src/pages/Register.jsx` with:

```jsx
import { useState, useEffect } from 'react';
import { useNavigate, Link } from 'react-router-dom';
import { register, getUserLimit } from '../api';

export default function Register() {
  const [username, setUsername] = useState('');
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [limitInfo, setLimitInfo] = useState(null);
  const navigate = useNavigate();

  useEffect(() => {
    getUserLimit()
      .then(setLimitInfo)
      .catch(() => {});
  }, []);

  const limitReached = limitInfo && limitInfo.user_count >= limitInfo.user_limit;

  async function handleSubmit(e) {
    e.preventDefault();
    setError('');
    try {
      const data = await register(username, email, password);
      navigate('/verify-email', { state: { userId: data.user_id } });
    } catch (err) {
      setError(err.message);
      getUserLimit().then(setLimitInfo).catch(() => {});
    }
  }

  return (
    <div className="auth-form">
      <h1>Register</h1>
      {limitInfo && (
        <p className={limitReached ? 'error' : 'info'}>
          Users: {limitInfo.user_count} / {limitInfo.user_limit}
          {limitReached && ' — Registration is currently unavailable. Contact your administrator to increase the user limit.'}
        </p>
      )}
      {error && <p className="error">{error}</p>}
      <form onSubmit={handleSubmit}>
        <div className="form-group">
          <label>Username</label>
          <input value={username} onChange={(e) => setUsername(e.target.value)} required />
        </div>
        <div className="form-group">
          <label>Email</label>
          <input type="email" value={email} onChange={(e) => setEmail(e.target.value)} required />
        </div>
        <div className="form-group">
          <label>Password</label>
          <input type="password" value={password} onChange={(e) => setPassword(e.target.value)} required minLength={8} />
        </div>
        <button type="submit" className="primary" disabled={limitReached}>Register</button>
      </form>
      <p style={{ marginTop: '16px' }}>
        Already have an account? <Link to="/login">Login</Link>
      </p>
    </div>
  );
}
```

- [ ] **Step 2: Commit**

```bash
git add frontend/src/pages/Register.jsx
git commit -m "feat: add email field to registration form"
```

---

### Task 9: Frontend — VerifyEmail Page

**Files:**
- Create: `frontend/src/pages/VerifyEmail.jsx`

- [ ] **Step 1: Create the verification code entry page**

Create `frontend/src/pages/VerifyEmail.jsx`:

```jsx
import { useState } from 'react';
import { useNavigate, useLocation, Link } from 'react-router-dom';
import { verifyEmail, resendVerification } from '../api';

export default function VerifyEmail() {
  const [code, setCode] = useState('');
  const [error, setError] = useState('');
  const [info, setInfo] = useState('');
  const navigate = useNavigate();
  const location = useLocation();
  const userId = location.state?.userId;

  if (!userId) {
    return (
      <div className="auth-form">
        <h1>Verify Email</h1>
        <p className="error">No user ID found. Please <Link to="/register">register</Link> first.</p>
      </div>
    );
  }

  async function handleSubmit(e) {
    e.preventDefault();
    setError('');
    setInfo('');
    try {
      const data = await verifyEmail(userId, code);
      localStorage.setItem('token', data.token);
      navigate('/assets');
    } catch (err) {
      setError(err.message);
    }
  }

  async function handleResend() {
    setError('');
    setInfo('');
    try {
      const data = await resendVerification(userId);
      setInfo(data.message);
    } catch (err) {
      setError(err.message);
    }
  }

  return (
    <div className="auth-form">
      <h1>Verify Email</h1>
      <p className="info">Enter the 6-digit code sent to your email.</p>
      {error && <p className="error">{error}</p>}
      {info && <p className="info">{info}</p>}
      <form onSubmit={handleSubmit}>
        <div className="form-group">
          <label>Verification Code</label>
          <input
            className="verification-code-input"
            value={code}
            onChange={(e) => setCode(e.target.value.replace(/\D/g, '').slice(0, 6))}
            placeholder="000000"
            maxLength={6}
            required
          />
        </div>
        <button type="submit" className="primary" disabled={code.length !== 6}>Verify</button>
      </form>
      <p style={{ marginTop: '16px' }}>
        Didn't receive a code? <button type="button" onClick={handleResend} style={{ background: 'none', border: 'none', color: '#2563eb', cursor: 'pointer', padding: 0, fontSize: '14px' }}>Resend code</button>
      </p>
    </div>
  );
}
```

- [ ] **Step 2: Commit**

```bash
git add frontend/src/pages/VerifyEmail.jsx
git commit -m "feat: add email verification page"
```

---

### Task 10: Frontend — Update App.jsx, Login.jsx, and CSS

**Files:**
- Modify: `frontend/src/App.jsx`
- Modify: `frontend/src/pages/Login.jsx`
- Modify: `frontend/src/App.css`

- [ ] **Step 1: Add verify-email route to App.jsx**

Replace the full contents of `frontend/src/App.jsx` with:

```jsx
import { useState, useEffect } from 'react';
import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import Login from './pages/Login';
import Register from './pages/Register';
import VerifyEmail from './pages/VerifyEmail';
import AssetList from './pages/AssetList';
import AssetDetail from './pages/AssetDetail';
import LicenseExpired from './pages/LicenseExpired';
import { checkForUpdates } from './api';
import './App.css';

function ProtectedRoute({ children }) {
  const token = localStorage.getItem('token');
  if (!token) {
    return <Navigate to="/login" replace />;
  }
  return children;
}

export default function App() {
  const [updateAvailable, setUpdateAvailable] = useState(false);

  useEffect(() => {
    checkForUpdates().then((data) => {
      setUpdateAvailable(data.updatesAvailable);
    });
  }, []);

  return (
    <BrowserRouter>
      {updateAvailable && (
        <div className="update-banner">Update available</div>
      )}
      <Routes>
        <Route path="/login" element={<Login />} />
        <Route path="/register" element={<Register />} />
        <Route path="/verify-email" element={<VerifyEmail />} />
        <Route path="/license-expired" element={<LicenseExpired />} />
        <Route path="/assets" element={<ProtectedRoute><AssetList /></ProtectedRoute>} />
        <Route path="/assets/:id" element={<ProtectedRoute><AssetDetail /></ProtectedRoute>} />
        <Route path="*" element={<Navigate to="/assets" replace />} />
      </Routes>
    </BrowserRouter>
  );
}
```

- [ ] **Step 2: Update Login.jsx to handle email_not_verified error**

Replace the full contents of `frontend/src/pages/Login.jsx` with:

```jsx
import { useState, useEffect } from 'react';
import { useNavigate, Link } from 'react-router-dom';
import { login, getLicenseStatus, resendVerification } from '../api';

export default function Login() {
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [licenseValid, setLicenseValid] = useState(true);
  const [licenseMessage, setLicenseMessage] = useState('');
  const [unverifiedUserId, setUnverifiedUserId] = useState(null);
  const navigate = useNavigate();

  useEffect(() => {
    getLicenseStatus().then((status) => {
      setLicenseValid(status.valid);
      if (!status.valid) {
        setLicenseMessage(status.message || 'License is invalid.');
      }
    });
  }, []);

  async function handleSubmit(e) {
    e.preventDefault();
    setError('');
    setUnverifiedUserId(null);
    try {
      const data = await login(username, password);
      localStorage.setItem('token', data.token);
      navigate('/assets');
    } catch (err) {
      if (err.message === 'email_not_verified') {
        // The 403 response is caught by the request() helper which throws the error string.
        // We need to get the user_id from the response. Since request() only throws the error
        // string, we handle this by attempting login again to get the full response.
        setError('Please verify your email before logging in.');
        // Re-fetch to get user_id
        try {
          const resp = await fetch('/api/auth/login', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ username, password }),
          });
          if (resp.status === 403) {
            const data = await resp.json();
            if (data.error === 'email_not_verified') {
              setUnverifiedUserId(data.user_id);
            }
          }
        } catch {
          // ignore
        }
      } else {
        setError(err.message);
      }
    }
  }

  function handleGoToVerify() {
    navigate('/verify-email', { state: { userId: unverifiedUserId } });
  }

  async function handleResendAndVerify() {
    if (unverifiedUserId) {
      try {
        await resendVerification(unverifiedUserId);
      } catch {
        // ignore
      }
      navigate('/verify-email', { state: { userId: unverifiedUserId } });
    }
  }

  return (
    <div className="auth-form">
      <h1>Login</h1>
      {!licenseValid && (
        <p className="error">{licenseMessage}</p>
      )}
      {error && <p className="error">{error}</p>}
      {unverifiedUserId && (
        <p className="info">
          <button type="button" onClick={handleGoToVerify} style={{ background: 'none', border: 'none', color: '#2563eb', cursor: 'pointer', padding: 0, fontSize: '14px' }}>Enter verification code</button>
          {' or '}
          <button type="button" onClick={handleResendAndVerify} style={{ background: 'none', border: 'none', color: '#2563eb', cursor: 'pointer', padding: 0, fontSize: '14px' }}>resend code</button>
        </p>
      )}
      <form onSubmit={handleSubmit}>
        <div className="form-group">
          <label>Username</label>
          <input value={username} onChange={(e) => setUsername(e.target.value)} required />
        </div>
        <div className="form-group">
          <label>Password</label>
          <input type="password" value={password} onChange={(e) => setPassword(e.target.value)} required />
        </div>
        <button type="submit" className="primary" disabled={!licenseValid}>Login</button>
      </form>
      <p style={{ marginTop: '16px' }}>
        Don't have an account? <Link to="/register">Register</Link>
      </p>
    </div>
  );
}
```

- [ ] **Step 3: Add verification input styles to App.css**

Append to `frontend/src/App.css`:

```css
.verification-code-input {
  text-align: center;
  font-size: 24px;
  letter-spacing: 8px;
  font-family: monospace;
}
```

- [ ] **Step 4: Commit**

```bash
git add frontend/src/App.jsx frontend/src/pages/Login.jsx frontend/src/App.css
git commit -m "feat: add verify-email route, handle unverified login, add styles"
```

---

### Task 11: Helm — Mailpit Deployment and Service

**Files:**
- Create: `helm/asset-tracker/templates/mailpit-deployment.yaml`
- Create: `helm/asset-tracker/templates/mailpit-service.yaml`

- [ ] **Step 1: Create Mailpit deployment template**

Create `helm/asset-tracker/templates/mailpit-deployment.yaml`:

```yaml
{{- if .Values.mailpit.enabled }}
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ include "asset-tracker.fullname" . }}-mailpit
  labels:
    {{- include "asset-tracker.labels" . | nindent 4 }}
    app.kubernetes.io/component: mailpit
spec:
  replicas: 1
  selector:
    matchLabels:
      app.kubernetes.io/component: mailpit
      app.kubernetes.io/instance: {{ .Release.Name }}
  template:
    metadata:
      labels:
        {{- include "asset-tracker.labels" . | nindent 8 }}
        app.kubernetes.io/component: mailpit
    spec:
      {{- include "asset-tracker.imagePullSecrets" . | nindent 6 }}
      containers:
        - name: mailpit
          image: {{ include "asset-tracker.proxyImage" (dict "root" . "image" (printf "docker.io/%s:%s" .Values.mailpit.image.repository .Values.mailpit.image.tag)) }}
          ports:
            - containerPort: 1025
              name: smtp
            - containerPort: 8025
              name: http
          resources:
            requests:
              cpu: 25m
              memory: 32Mi
            limits:
              cpu: 100m
              memory: 128Mi
{{- end }}
```

- [ ] **Step 2: Create Mailpit service template**

Create `helm/asset-tracker/templates/mailpit-service.yaml`:

```yaml
{{- if .Values.mailpit.enabled }}
apiVersion: v1
kind: Service
metadata:
  name: {{ include "asset-tracker.fullname" . }}-mailpit
  labels:
    {{- include "asset-tracker.labels" . | nindent 4 }}
    app.kubernetes.io/component: mailpit
spec:
  type: ClusterIP
  ports:
    - port: 1025
      targetPort: smtp
      name: smtp
    - port: 8025
      targetPort: http
      name: http
  selector:
    app.kubernetes.io/component: mailpit
    app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}
```

- [ ] **Step 3: Commit**

```bash
git add helm/asset-tracker/templates/mailpit-deployment.yaml helm/asset-tracker/templates/mailpit-service.yaml
git commit -m "feat: add Mailpit deployment and service templates"
```

---

### Task 12: Helm — Values, Backend Deployment SMTP Env Vars

**Files:**
- Modify: `helm/asset-tracker/values.yaml`
- Modify: `helm/asset-tracker/templates/backend-deployment.yaml`

- [ ] **Step 1: Add smtp and mailpit sections to values.yaml**

In `helm/asset-tracker/values.yaml`, add the following after the `certManager` block (at the end of the file):

```yaml

smtp:
  host: ""
  port: 587
  username: ""
  password: ""
  from: "noreply@assettracker.local"

mailpit:
  enabled: false
  image:
    repository: axllent/mailpit
    tag: latest
```

- [ ] **Step 2: Add SMTP env vars to backend deployment**

In `helm/asset-tracker/templates/backend-deployment.yaml`, add the following environment variables after the `METRICS_INTERVAL` env var (after line 52):

```yaml
            - name: SMTP_HOST
              {{- if and .Values.mailpit.enabled (eq .Values.smtp.host "") }}
              value: {{ include "asset-tracker.fullname" . }}-mailpit
              {{- else }}
              value: {{ .Values.smtp.host | quote }}
              {{- end }}
            - name: SMTP_PORT
              {{- if and .Values.mailpit.enabled (eq .Values.smtp.host "") }}
              value: "1025"
              {{- else }}
              value: {{ .Values.smtp.port | quote }}
              {{- end }}
            - name: SMTP_FROM
              value: {{ .Values.smtp.from | quote }}
            - name: SMTP_USERNAME
              value: {{ .Values.smtp.username | quote }}
            - name: SMTP_PASSWORD
              value: {{ .Values.smtp.password | quote }}
```

- [ ] **Step 3: Commit**

```bash
git add helm/asset-tracker/values.yaml helm/asset-tracker/templates/backend-deployment.yaml
git commit -m "feat: add SMTP and Mailpit configuration to Helm chart"
```

---

### Task 13: Helm — Preflight Checks

**Files:**
- Create: `helm/asset-tracker/templates/preflight.yaml`

- [ ] **Step 1: Create the preflight spec with all 5 checks**

Create `helm/asset-tracker/templates/preflight.yaml`:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: {{ include "asset-tracker.fullname" . }}-preflight
  labels:
    {{- include "asset-tracker.labels" . | nindent 4 }}
    troubleshoot.sh/kind: preflight
type: Opaque
stringData:
  preflight.yaml: |
    apiVersion: troubleshoot.sh/v1beta2
    kind: Preflight
    metadata:
      name: asset-tracker
    spec:
      collectors:
        {{- if not .Values.postgresql.enabled }}
        - postgresql:
            collectorName: external-postgres
            uri: {{ include "asset-tracker.databaseURL" . }}
        {{- end }}
      analyzers:
        {{- if not .Values.postgresql.enabled }}
        # Check 1: External database connectivity
        - postgres:
            checkName: Database Connectivity
            collectorName: external-postgres
            outcomes:
              - fail:
                  when: "connected == false"
                  message: |
                    Cannot connect to PostgreSQL at {{ include "asset-tracker.postgresHost" . }}:{{ include "asset-tracker.postgresPort" . }}.
                    Verify the database is running, the credentials are correct, and the host is reachable from the cluster.
              - pass:
                  message: Successfully connected to external PostgreSQL database.
        {{- end }}

        # Check 2: SMTP endpoint connectivity (only when smtp.host is configured)
        {{- if .Values.smtp.host }}
        - textAnalyze:
            checkName: SMTP Connectivity
            fileName: host-collectors/tcpConnect/smtp-connectivity.json
            regex: "connected"
            outcomes:
              - fail:
                  when: "false"
                  message: |
                    Cannot reach SMTP server at {{ .Values.smtp.host }}:{{ .Values.smtp.port }}.
                    Verify the SMTP server is running and accessible from the cluster network.
              - pass:
                  when: "true"
                  message: SMTP server at {{ .Values.smtp.host }}:{{ .Values.smtp.port }} is reachable.
        {{- end }}

        # Check 3: Cluster resources — CPU
        - nodeResources:
            checkName: Cluster CPU Resources
            outcomes:
              - fail:
                  when: "sum(cpuAllocatable) < 1000m"
                  message: |
                    Cluster has insufficient CPU. At least 1 CPU core allocatable is required.
                    Add more nodes or increase node CPU capacity.
              - pass:
                  message: Cluster has sufficient CPU resources (at least 1 CPU core allocatable).

        # Check 3: Cluster resources — Memory
        - nodeResources:
            checkName: Cluster Memory Resources
            outcomes:
              - fail:
                  when: "sum(memoryAllocatable) < 2Gi"
                  message: |
                    Cluster has insufficient memory. At least 2Gi allocatable memory is required.
                    Add more nodes or increase node memory capacity.
              - pass:
                  message: Cluster has sufficient memory resources (at least 2Gi allocatable).

        # Check 4: Kubernetes version
        - clusterVersion:
            checkName: Kubernetes Version
            outcomes:
              - fail:
                  when: "< 1.30.0"
                  message: |
                    Kubernetes version is not supported. Minimum required version is 1.30.
                    Upgrade your cluster before installing Asset Tracker.
              - warn:
                  when: "< 1.31.0"
                  message: |
                    Kubernetes version 1.31 or later is recommended for best compatibility.
                    Your cluster will work but consider upgrading.
              - pass:
                  message: Kubernetes version meets the minimum requirement of 1.30.

        # Check 5: Distribution check
        - distribution:
            checkName: Kubernetes Distribution
            outcomes:
              - fail:
                  when: "== docker-desktop"
                  message: |
                    Docker Desktop is not a supported Kubernetes distribution for Asset Tracker.
                    See https://docs.assettracker.com/install/supported-clusters for supported options.
              - fail:
                  when: "== microk8s"
                  message: |
                    MicroK8s is not a supported Kubernetes distribution for Asset Tracker.
                    See https://docs.assettracker.com/install/supported-clusters for supported options.
              - pass:
                  message: Supported Kubernetes distribution detected.

      {{- if .Values.smtp.host }}
      hostCollectors:
        - tcpConnect:
            collectorName: smtp-connectivity
            address: {{ .Values.smtp.host }}:{{ .Values.smtp.port }}
            timeout: 10s
      {{- end }}
```

- [ ] **Step 2: Commit**

```bash
git add helm/asset-tracker/templates/preflight.yaml
git commit -m "feat: add preflight checks for DB, SMTP, resources, k8s version, and distribution"
```

---

### Task 14: Docker Compose — Add Mailpit and SMTP Env Vars

**Files:**
- Modify: `docker-compose.yml`
- Modify: `.env.example`

- [ ] **Step 1: Update docker-compose.yml to add Mailpit and SMTP environment variables**

Replace the full contents of `docker-compose.yml` with:

```yaml
services:
  postgres:
    image: postgres:16
    environment:
      POSTGRES_USER: ${POSTGRES_USER:-asset_tracker}
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD:-asset_tracker}
      POSTGRES_DB: ${POSTGRES_DB:-asset_tracker}
    ports:
      - "5432:5432"
    volumes:
      - pgdata:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U ${POSTGRES_USER:-asset_tracker}"]
      interval: 5s
      timeout: 5s
      retries: 5

  mailpit:
    image: axllent/mailpit:latest
    ports:
      - "1025:1025"
      - "8025:8025"

  schemahero-apply:
    image: schemahero/schemahero:latest
    command:
      - apply
      - --driver=postgres
      - --uri=postgres://${POSTGRES_USER:-asset_tracker}:${POSTGRES_PASSWORD:-asset_tracker}@postgres:5432/${POSTGRES_DB:-asset_tracker}?sslmode=disable
      - --ddl=/schemas/ddl/schema.sql
    volumes:
      - ./schemas:/schemas
    depends_on:
      postgres:
        condition: service_healthy

  backend:
    image: unawake2068/asset-tracker-backend:latest
    build: ./backend
    ports:
      - "8080:8080"
    environment:
      DATABASE_URL: ${DATABASE_URL:-postgres://asset_tracker:asset_tracker@postgres:5432/asset_tracker?sslmode=disable}
      JWT_SECRET: ${JWT_SECRET:-change-me-in-production}
      SMTP_HOST: mailpit
      SMTP_PORT: "1025"
      SMTP_FROM: ${SMTP_FROM:-noreply@assettracker.local}
      SMTP_USERNAME: ""
      SMTP_PASSWORD: ""
    depends_on:
      schemahero-apply:
        condition: service_completed_successfully
      mailpit:
        condition: service_started

  frontend:
    image: unawake2068/asset-tracker-frontend:latest
    build: ./frontend
    ports:
      - "5173:5173"
    depends_on:
      - backend

volumes:
  pgdata:
```

- [ ] **Step 2: Update .env.example**

Replace the full contents of `.env.example` with:

```
POSTGRES_USER=asset_tracker
POSTGRES_PASSWORD=asset_tracker
POSTGRES_DB=asset_tracker
JWT_SECRET=change-me-in-production
DATABASE_URL=postgres://asset_tracker:asset_tracker@postgres:5432/asset_tracker?sslmode=disable
SMTP_HOST=mailpit
SMTP_PORT=1025
SMTP_FROM=noreply@assettracker.local
SMTP_USERNAME=
SMTP_PASSWORD=
```

- [ ] **Step 3: Commit**

```bash
git add docker-compose.yml .env.example
git commit -m "feat: add Mailpit to docker-compose and SMTP env vars"
```

---

### Task 15: E2E Tests — Update for Email Verification Flow

**Files:**
- Modify: `e2e/asset-tracker.spec.mjs`
- Modify: `playwright.config.mjs`

- [ ] **Step 1: Update playwright config to add MAILPIT_URL**

Replace the full contents of `playwright.config.mjs` with:

```js
import { defineConfig } from '@playwright/test';

export default defineConfig({
  testDir: './e2e',
  timeout: 30000,
  retries: 0,
  use: {
    baseURL: process.env.BASE_URL || 'https://assets.assettracker.tech',
    ignoreHTTPSErrors: true,
    screenshot: 'only-on-failure',
  },
  projects: [
    {
      name: 'chromium',
      use: { browserName: 'chromium' },
    },
  ],
});
```

Note: MAILPIT_URL will be read directly in the test file from `process.env`.

- [ ] **Step 2: Update e2e tests with email verification flow**

Replace the full contents of `e2e/asset-tracker.spec.mjs` with:

```js
import { test, expect } from '@playwright/test';

const BASE_URL = process.env.BASE_URL || 'https://assets.assettracker.tech';
const API_URL = BASE_URL + '/api';
const MAILPIT_URL = process.env.MAILPIT_URL || BASE_URL.replace(/:\d+$/, ':8025');

// Helper: fetch the latest verification code from Mailpit for a given email
async function getVerificationCode(request, emailAddr) {
  // Wait briefly for the email to arrive
  await new Promise((r) => setTimeout(r, 1000));

  const resp = await request.get(`${MAILPIT_URL}/api/v1/messages?limit=5`);
  expect(resp.status()).toBe(200);
  const data = await resp.json();

  // Find the message sent to our email address
  const message = data.messages.find((m) =>
    m.To.some((to) => to.Address === emailAddr)
  );
  expect(message).toBeTruthy();

  // Get the full message to read the body
  const msgResp = await request.get(`${MAILPIT_URL}/api/v1/message/${message.ID}`);
  expect(msgResp.status()).toBe(200);
  const msgData = await msgResp.json();

  // Extract 6-digit code from body
  const match = msgData.Text.match(/(\d{6})/);
  expect(match).toBeTruthy();
  return match[1];
}

// Shared state across tests in this file
let token;
let username;
let userId;
const testEmail = `e2e_${Date.now()}@test.assettracker.local`;

test.describe.serial('Asset Tracker', () => {

  test.describe('Health & Infrastructure', () => {
    test('health endpoint returns ok with database connected', async ({ request }) => {
      const resp = await request.get(`${API_URL}/health`);
      expect(resp.status()).toBe(200);

      const body = await resp.json();
      expect(body.status).toBe('ok');
      expect(body.database).toBe('connected');
      expect(body.timestamp).toBeTruthy();
    });
  });

  test.describe('Authentication', () => {
    test('register a new user', async ({ request }) => {
      username = `e2e_${Date.now()}`;
      const resp = await request.post(`${API_URL}/auth/register`, {
        data: { username, email: testEmail, password: 'TestPass123!' },
      });
      expect(resp.status()).toBe(201);

      const body = await resp.json();
      expect(body.user_id).toBeTruthy();
      expect(body.message).toContain('verification');
      userId = body.user_id;
    });

    test('verify email with code from Mailpit', async ({ request }) => {
      const code = await getVerificationCode(request, testEmail);

      const resp = await request.post(`${API_URL}/auth/verify-email`, {
        data: { user_id: userId, code },
      });
      expect(resp.status()).toBe(200);

      const body = await resp.json();
      expect(body.token).toBeTruthy();
      token = body.token;
    });

    test('login with verified user', async ({ request }) => {
      const resp = await request.post(`${API_URL}/auth/login`, {
        data: { username, password: 'TestPass123!' },
      });
      expect(resp.status()).toBe(200);

      const body = await resp.json();
      expect(body.token).toBeTruthy();
    });

    test('reject invalid credentials', async ({ request }) => {
      const resp = await request.post(`${API_URL}/auth/login`, {
        data: { username, password: 'wrong' },
      });
      expect(resp.status()).toBe(401);
    });
  });

  test.describe('UI - Login & Register', () => {
    test('login page renders', async ({ page }) => {
      await page.goto('/login');
      await expect(page.locator('h1')).toHaveText('Login');
      await expect(page.locator('.form-group')).toHaveCount(2);
      await expect(page.locator('button[type="submit"]')).toHaveText('Login');
      await expect(page.locator('a:has-text("Register")')).toBeVisible();
    });

    test('register page renders with email field', async ({ page }) => {
      await page.goto('/register');
      await expect(page.locator('h1')).toHaveText('Register');
      await expect(page.locator('.form-group')).toHaveCount(3);
      await expect(page.locator('button[type="submit"]')).toHaveText('Register');
      await expect(page.locator('a:has-text("Login")')).toBeVisible();
    });

    test('redirect to login when not authenticated', async ({ page }) => {
      await page.goto('/assets');
      await expect(page).toHaveURL(/\/login/);
    });
  });

  test.describe('UI - Asset Management', () => {
    test.beforeEach(async ({ page }) => {
      // Inject auth token
      await page.goto('/');
      await page.evaluate((t) => localStorage.setItem('token', t), token);
    });

    test('asset list page renders with Add Asset button', async ({ page }) => {
      await page.goto('/assets');
      await expect(page.locator('h1')).toHaveText('My Assets');
      await expect(page.locator('button:has-text("Add Asset")')).toBeVisible();
      await expect(page.locator('button:has-text("Logout")')).toBeVisible();
    });

    test('create an asset via UI', async ({ page }) => {
      await page.goto('/assets');

      await page.locator('button:has-text("Add Asset")').click();
      await expect(page.locator('form')).toBeVisible();

      await page.locator('form .form-group').nth(0).locator('input').type('E2E-ASSET-001', { delay: 10 });
      await page.locator('form .form-group').nth(1).locator('input').type('Test Asset', { delay: 10 });
      await page.locator('form .form-group').nth(2).locator('textarea').type('Created by e2e test', { delay: 10 });

      await page.locator('button:has-text("Create")').click();
      await expect(page.locator('td:has-text("E2E-ASSET-001")')).toBeVisible({ timeout: 5000 });
      await expect(page.locator('td:has-text("Test Asset")')).toBeVisible();
    });

    test('navigate to asset detail page', async ({ page }) => {
      await page.goto('/assets');
      await expect(page.locator('td a:has-text("E2E-ASSET-001")')).toBeVisible({ timeout: 5000 });

      await page.locator('td a:has-text("E2E-ASSET-001")').click();
      await expect(page.locator('h1')).toHaveText('Asset: E2E-ASSET-001');
      await expect(page.locator('button:has-text("Add Value Point")')).toBeVisible();
      await expect(page.locator('button:has-text("Back")')).toBeVisible();
    });

    test('add value points to an asset', async ({ page }) => {
      await page.goto('/assets');
      await page.locator('td a:has-text("E2E-ASSET-001")').click({ timeout: 5000 });
      await expect(page.locator('h1')).toHaveText('Asset: E2E-ASSET-001');

      // Add first value point
      await page.locator('button:has-text("Add Value Point")').click();
      await expect(page.locator('form')).toBeVisible();
      await page.locator('form .form-group').nth(0).locator('input').type('1000', { delay: 10 });
      // Currency defaults to USD
      await page.locator('form button[type="submit"]').click();

      await expect(page.locator('td:has-text("$1,000.00")')).toBeVisible({ timeout: 5000 });
      await expect(page.locator('td:has-text("USD")')).toBeVisible();

      // Add second value point
      await page.locator('button:has-text("Add Value Point")').click();
      await page.locator('form .form-group').nth(0).locator('input').type('1250', { delay: 10 });
      await page.locator('form button[type="submit"]').click();

      await expect(page.locator('td:has-text("$1,250.00")')).toBeVisible({ timeout: 5000 });
    });

    test('back button returns to asset list', async ({ page }) => {
      await page.goto('/assets');
      await page.locator('td a:has-text("E2E-ASSET-001")').click({ timeout: 5000 });
      await expect(page.locator('h1')).toHaveText('Asset: E2E-ASSET-001');

      await page.locator('button:has-text("Back")').click();
      await expect(page.locator('h1')).toHaveText('My Assets');
    });

    test('logout returns to login page', async ({ page }) => {
      await page.goto('/assets');
      await page.locator('button:has-text("Logout")').click();
      await expect(page).toHaveURL(/\/login/);
    });
  });

  test.describe('API - Asset CRUD', () => {
    const authHeaders = () => ({
      'Content-Type': 'application/json',
      'Authorization': `Bearer ${token}`,
    });

    test('create asset via API', async ({ request }) => {
      const resp = await request.post(`${API_URL}/assets`, {
        headers: authHeaders(),
        data: { id: 'API-TEST-001', name: 'API Test Asset', description: 'Created via API' },
      });
      expect(resp.status()).toBe(201);

      const body = await resp.json();
      expect(body.id).toBe('API-TEST-001');
      expect(body.name).toBe('API Test Asset');
      expect(body.created_at).toBeTruthy();
    });

    test('list assets returns created assets', async ({ request }) => {
      const resp = await request.get(`${API_URL}/assets`, {
        headers: authHeaders(),
      });
      expect(resp.status()).toBe(200);

      const assets = await resp.json();
      expect(assets.length).toBeGreaterThanOrEqual(2);
      const ids = assets.map(a => a.id);
      expect(ids).toContain('API-TEST-001');
      expect(ids).toContain('E2E-ASSET-001');
    });

    test('update asset via API', async ({ request }) => {
      const resp = await request.put(`${API_URL}/assets/API-TEST-001`, {
        headers: authHeaders(),
        data: { name: 'Updated Name', description: 'Updated description' },
      });
      expect(resp.status()).toBe(200);

      const body = await resp.json();
      expect(body.name).toBe('Updated Name');
    });

    test('create value point via API', async ({ request }) => {
      const resp = await request.post(`${API_URL}/assets/API-TEST-001/values`, {
        headers: authHeaders(),
        data: { value: 5000, currency: 'EUR' },
      });
      expect(resp.status()).toBe(201);

      const body = await resp.json();
      expect(Number(body.value)).toBe(5000);
      expect(body.currency).toBe('EUR');
      expect(body.timestamp).toBeTruthy();
    });

    test('list value points returns created values', async ({ request }) => {
      const resp = await request.get(`${API_URL}/assets/API-TEST-001/values`, {
        headers: authHeaders(),
      });
      expect(resp.status()).toBe(200);

      const values = await resp.json();
      expect(values.length).toBe(1);
      expect(values[0].currency).toBe('EUR');
    });

    test('delete asset via API', async ({ request }) => {
      const resp = await request.delete(`${API_URL}/assets/API-TEST-001`, {
        headers: authHeaders(),
      });
      expect(resp.status()).toBe(204);

      // Verify it's gone
      const listResp = await request.get(`${API_URL}/assets`, {
        headers: authHeaders(),
      });
      const assets = await listResp.json();
      const ids = assets.map(a => a.id);
      expect(ids).not.toContain('API-TEST-001');
    });

    test('reject unauthenticated requests', async ({ request }) => {
      const resp = await request.get(`${API_URL}/assets`);
      expect(resp.status()).toBe(401);
    });
  });
});
```

- [ ] **Step 3: Commit**

```bash
git add e2e/asset-tracker.spec.mjs playwright.config.mjs
git commit -m "feat: update e2e tests for email verification flow with Mailpit"
```

---

### Task 16: Verify Backend Compiles and Helm Templates Render

- [ ] **Step 1: Verify Go backend compiles**

Run: `cd /Users/jdewinne/conductor/workspaces/asset-tracker/hangzhou/backend && go build ./...`
Expected: No errors

- [ ] **Step 2: Verify Helm template renders correctly**

Run: `cd /Users/jdewinne/conductor/workspaces/asset-tracker/hangzhou && helm template test-release helm/asset-tracker --set mailpit.enabled=true 2>&1 | head -20`
Expected: YAML output without errors

- [ ] **Step 3: Verify preflight template renders with external DB**

Run: `helm template test-release helm/asset-tracker --set postgresql.enabled=false --set externalDatabase.host=db.example.com --set externalDatabase.username=user --set externalDatabase.password=pass --set externalDatabase.database=mydb --set smtp.host=smtp.example.com 2>&1 | grep -A 100 "preflight.yaml"`
Expected: Preflight YAML with all 5 checks including DB and SMTP

- [ ] **Step 4: Verify preflight template renders without external DB (embedded postgres)**

Run: `helm template test-release helm/asset-tracker 2>&1 | grep -A 100 "preflight.yaml"`
Expected: Preflight YAML with 3 checks (resources, k8s version, distribution) — no DB or SMTP checks

- [ ] **Step 5: Commit any fixes if needed**
