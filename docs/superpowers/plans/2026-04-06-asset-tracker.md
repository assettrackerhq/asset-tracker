# Asset Tracker Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a multi-user personal asset tracking webapp with React frontend, Go REST API backend, and Postgres storage.

**Architecture:** React+Vite SPA communicates with a Go chi-based REST API over JSON. Auth uses bcrypt+JWT. Data stored in Postgres with SchemaHero managing schema. Docker-compose orchestrates all services.

**Tech Stack:** React, Vite, react-router, Go, chi, pgx, bcrypt, JWT (golang-jwt), PostgreSQL, SchemaHero, Docker

---

## File Structure

```
/docker-compose.yml                  — Orchestrates postgres, backend, frontend, schemahero
/.env.example                        — Template for environment variables

/schemas/
  tables/users.yaml                  — SchemaHero users table
  tables/assets.yaml                 — SchemaHero assets table
  tables/asset_value_points.yaml     — SchemaHero asset_value_points table

/backend/
  go.mod                             — Go module definition
  go.sum                             — Go dependency checksums
  main.go                            — Entrypoint: config, DB connect, router setup, server start
  Dockerfile                         — Multi-stage build for Go backend

  internal/
    config/
      config.go                      — Env var loading (DB_URL, JWT_SECRET, PORT)

    database/
      db.go                          — pgx pool creation and health check

    auth/
      jwt.go                         — JWT generation and validation
      middleware.go                   — HTTP middleware: validate token, set user_id in context
      handler.go                     — POST /api/auth/register, POST /api/auth/login
      handler_test.go                — Tests for auth handlers

    assets/
      handler.go                     — CRUD handlers for assets
      handler_test.go                — Tests for asset handlers

    values/
      handler.go                     — CRUD handlers for value points
      handler_test.go                — Tests for value point handlers

/frontend/
  package.json                       — Dependencies and scripts
  vite.config.js                     — Vite config with API proxy
  index.html                         — HTML shell
  Dockerfile                         — Serves built frontend

  src/
    main.jsx                         — React entrypoint, router setup
    api.js                           — Fetch wrapper with JWT header injection
    App.jsx                          — Root component with routes
    App.css                          — Global styles

    pages/
      Login.jsx                      — Login form page
      Register.jsx                   — Registration form page
      AssetList.jsx                  — List assets with add/edit/delete
      AssetDetail.jsx                — Asset info + value points table
```

---

### Task 1: Docker Compose and SchemaHero Schemas

**Files:**
- Create: `docker-compose.yml`
- Create: `.env.example`
- Create: `schemas/tables/users.yaml`
- Create: `schemas/tables/assets.yaml`
- Create: `schemas/tables/asset_value_points.yaml`

- [ ] **Step 1: Create `.env.example`**

```env
POSTGRES_USER=asset_tracker
POSTGRES_PASSWORD=asset_tracker
POSTGRES_DB=asset_tracker
JWT_SECRET=change-me-in-production
DATABASE_URL=postgres://asset_tracker:asset_tracker@postgres:5432/asset_tracker?sslmode=disable
```

- [ ] **Step 2: Create `docker-compose.yml`**

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

  schemahero-apply:
    image: schemahero/schemahero:latest
    command:
      - apply
      - --driver=postgres
      - --uri=postgres://${POSTGRES_USER:-asset_tracker}:${POSTGRES_PASSWORD:-asset_tracker}@postgres:5432/${POSTGRES_DB:-asset_tracker}?sslmode=disable
      - --ddi=/schemas/tables
    volumes:
      - ./schemas:/schemas
    depends_on:
      postgres:
        condition: service_healthy

  backend:
    build: ./backend
    ports:
      - "8080:8080"
    environment:
      DATABASE_URL: ${DATABASE_URL:-postgres://asset_tracker:asset_tracker@postgres:5432/asset_tracker?sslmode=disable}
      JWT_SECRET: ${JWT_SECRET:-change-me-in-production}
    depends_on:
      schemahero-apply:
        condition: service_completed_successfully

  frontend:
    build: ./frontend
    ports:
      - "5173:5173"
    depends_on:
      - backend

volumes:
  pgdata:
```

- [ ] **Step 3: Create `schemas/tables/users.yaml`**

```yaml
apiVersion: schemas.schemahero.io/v1alpha4
kind: Table
metadata:
  name: users
spec:
  database:
    postgres:
      uri:
        value: will-be-overridden
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
```

- [ ] **Step 4: Create `schemas/tables/assets.yaml`**

```yaml
apiVersion: schemas.schemahero.io/v1alpha4
kind: Table
metadata:
  name: assets
spec:
  database:
    postgres:
      uri:
        value: will-be-overridden
  name: assets
  schema:
    postgres:
      primaryKey:
        - id
        - user_id
      columns:
        - name: id
          type: varchar(50)
          constraints:
            notNull: true
        - name: user_id
          type: uuid
          constraints:
            notNull: true
        - name: name
          type: varchar(255)
          constraints:
            notNull: true
        - name: description
          type: text
        - name: created_at
          type: timestamp
          default: "now()"
          constraints:
            notNull: true
        - name: updated_at
          type: timestamp
          default: "now()"
          constraints:
            notNull: true
```

- [ ] **Step 5: Create `schemas/tables/asset_value_points.yaml`**

```yaml
apiVersion: schemas.schemahero.io/v1alpha4
kind: Table
metadata:
  name: asset_value_points
spec:
  database:
    postgres:
      uri:
        value: will-be-overridden
  name: asset_value_points
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
        - name: asset_id
          type: varchar(50)
          constraints:
            notNull: true
        - name: user_id
          type: uuid
          constraints:
            notNull: true
        - name: timestamp
          type: timestamp
          default: "now()"
          constraints:
            notNull: true
        - name: value
          type: numeric(15,2)
          constraints:
            notNull: true
        - name: currency
          type: varchar(3)
          constraints:
            notNull: true
```

- [ ] **Step 6: Verify docker-compose config is valid**

Run: `docker compose config --quiet`
Expected: No output (valid config)

- [ ] **Step 7: Commit**

```bash
git add docker-compose.yml .env.example schemas/
git commit -m "feat: add docker-compose and SchemaHero table schemas"
```

---

### Task 2: Go Backend Scaffolding (Config, DB, Main)

**Files:**
- Create: `backend/go.mod`
- Create: `backend/main.go`
- Create: `backend/Dockerfile`
- Create: `backend/internal/config/config.go`
- Create: `backend/internal/database/db.go`

- [ ] **Step 1: Initialize Go module and install dependencies**

Run:
```bash
cd backend
go mod init github.com/assettrackerhq/asset-tracker/backend
go get github.com/go-chi/chi/v5
go get github.com/go-chi/cors
go get github.com/jackc/pgx/v5
go get golang.org/x/crypto/bcrypt
go get github.com/golang-jwt/jwt/v5
go get github.com/google/uuid
```

- [ ] **Step 2: Create `backend/internal/config/config.go`**

```go
package config

import (
	"fmt"
	"os"
)

type Config struct {
	DatabaseURL string
	JWTSecret   string
	Port        string
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

	return &Config{
		DatabaseURL: dbURL,
		JWTSecret:   jwtSecret,
		Port:        port,
	}, nil
}
```

- [ ] **Step 3: Create `backend/internal/database/db.go`**

```go
package database

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

func Connect(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("unable to create connection pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("unable to ping database: %w", err)
	}

	return pool, nil
}
```

- [ ] **Step 4: Create `backend/main.go`**

```go
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"

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

	addr := fmt.Sprintf(":%s", cfg.Port)
	log.Printf("starting server on %s", addr)
	if err := http.ListenAndServe(addr, r); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
```

- [ ] **Step 5: Create `backend/Dockerfile`**

```dockerfile
FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o server .

FROM alpine:3.20
WORKDIR /app
COPY --from=builder /app/server .
EXPOSE 8080
CMD ["./server"]
```

- [ ] **Step 6: Verify it compiles**

Run: `cd backend && go build ./...`
Expected: No errors

- [ ] **Step 7: Commit**

```bash
git add backend/
git commit -m "feat: add Go backend scaffolding with config, DB, and health endpoint"
```

---

### Task 3: Auth — JWT and Middleware

**Files:**
- Create: `backend/internal/auth/jwt.go`
- Create: `backend/internal/auth/middleware.go`

- [ ] **Step 1: Create `backend/internal/auth/jwt.go`**

```go
package auth

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func GenerateToken(userID string, secret string) (string, error) {
	claims := jwt.MapClaims{
		"sub": userID,
		"exp": time.Now().Add(24 * time.Hour).Unix(),
		"iat": time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

func ValidateToken(tokenString string, secret string) (string, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(secret), nil
	})
	if err != nil {
		return "", fmt.Errorf("invalid token: %w", err)
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return "", fmt.Errorf("invalid token claims")
	}

	userID, ok := claims["sub"].(string)
	if !ok {
		return "", fmt.Errorf("invalid sub claim")
	}

	return userID, nil
}
```

- [ ] **Step 2: Create `backend/internal/auth/middleware.go`**

```go
package auth

import (
	"context"
	"net/http"
	"strings"
)

type contextKey string

const UserIDKey contextKey = "userID"

func Middleware(jwtSecret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				http.Error(w, `{"error":"missing authorization header"}`, http.StatusUnauthorized)
				return
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || parts[0] != "Bearer" {
				http.Error(w, `{"error":"invalid authorization header"}`, http.StatusUnauthorized)
				return
			}

			userID, err := ValidateToken(parts[1], jwtSecret)
			if err != nil {
				http.Error(w, `{"error":"invalid or expired token"}`, http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), UserIDKey, userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func GetUserID(ctx context.Context) string {
	userID, _ := ctx.Value(UserIDKey).(string)
	return userID
}
```

- [ ] **Step 3: Verify it compiles**

Run: `cd backend && go build ./...`
Expected: No errors

- [ ] **Step 4: Commit**

```bash
git add backend/internal/auth/
git commit -m "feat: add JWT generation, validation, and auth middleware"
```

---

### Task 4: Auth Handlers (Register + Login)

**Files:**
- Create: `backend/internal/auth/handler.go`
- Create: `backend/internal/auth/handler_test.go`
- Modify: `backend/main.go` (wire auth routes)

- [ ] **Step 1: Write the failing test for register and login**

Create `backend/internal/auth/handler_test.go`:

```go
package auth_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/assettrackerhq/asset-tracker/backend/internal/auth"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func setupTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dbURL := "postgres://asset_tracker:asset_tracker@localhost:5432/asset_tracker?sslmode=disable"
	pool, err := pgxpool.New(context.Background(), dbURL)
	if err != nil {
		t.Skipf("skipping: cannot connect to test DB: %v", err)
	}
	// Clean up users table before each test
	pool.Exec(context.Background(), "DELETE FROM users")
	return pool
}

func TestRegisterAndLogin(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	jwtSecret := "test-secret"
	handler := auth.NewHandler(pool, jwtSecret)

	r := chi.NewRouter()
	r.Post("/api/auth/register", handler.Register)
	r.Post("/api/auth/login", handler.Login)

	// Test registration
	body := map[string]string{"username": "testuser", "password": "password123"}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("register: expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var regResp map[string]string
	json.NewDecoder(rec.Body).Decode(&regResp)
	if regResp["token"] == "" {
		t.Fatal("register: expected token in response")
	}

	// Test duplicate registration
	req = httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("duplicate register: expected 409, got %d", rec.Code)
	}

	// Test login
	req = httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("login: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var loginResp map[string]string
	json.NewDecoder(rec.Body).Decode(&loginResp)
	if loginResp["token"] == "" {
		t.Fatal("login: expected token in response")
	}

	// Test login with wrong password
	wrongBody := map[string]string{"username": "testuser", "password": "wrongpassword"}
	jsonWrong, _ := json.Marshal(wrongBody)
	req = httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(jsonWrong))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("wrong password: expected 401, got %d", rec.Code)
	}
}

func TestRegisterValidation(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	handler := auth.NewHandler(pool, "test-secret")

	r := chi.NewRouter()
	r.Post("/api/auth/register", handler.Register)

	// Test short password
	body := map[string]string{"username": "testuser", "password": "short"}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("short password: expected 400, got %d", rec.Code)
	}

	// Test empty username
	body = map[string]string{"username": "", "password": "password123"}
	jsonBody, _ = json.Marshal(body)
	req = httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("empty username: expected 400, got %d", rec.Code)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd backend && go test ./internal/auth/ -v -run TestRegister`
Expected: Compilation error — `auth.NewHandler` not defined

- [ ] **Step 3: Create `backend/internal/auth/handler.go`**

```go
package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

type Handler struct {
	db        *pgxpool.Pool
	jwtSecret string
}

func NewHandler(db *pgxpool.Pool, jwtSecret string) *Handler {
	return &Handler{db: db, jwtSecret: jwtSecret}
}

type authRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type authResponse struct {
	Token string `json:"token"`
}

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var req authRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	req.Username = strings.TrimSpace(req.Username)
	if req.Username == "" {
		http.Error(w, `{"error":"username is required"}`, http.StatusBadRequest)
		return
	}
	if len(req.Password) < 8 {
		http.Error(w, `{"error":"password must be at least 8 characters"}`, http.StatusBadRequest)
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), 12)
	if err != nil {
		http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
		return
	}

	userID := uuid.New().String()
	_, err = h.db.Exec(context.Background(),
		"INSERT INTO users (id, username, password_hash) VALUES ($1, $2, $3)",
		userID, req.Username, string(hash),
	)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "unique") {
			http.Error(w, `{"error":"username already exists"}`, http.StatusConflict)
			return
		}
		http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
		return
	}

	token, err := GenerateToken(userID, h.jwtSecret)
	if err != nil {
		http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(authResponse{Token: token})
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req authRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	var userID, passwordHash string
	err := h.db.QueryRow(context.Background(),
		"SELECT id, password_hash FROM users WHERE username = $1",
		strings.TrimSpace(req.Username),
	).Scan(&userID, &passwordHash)
	if err != nil {
		http.Error(w, `{"error":"invalid username or password"}`, http.StatusUnauthorized)
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.Password)); err != nil {
		http.Error(w, `{"error":"invalid username or password"}`, http.StatusUnauthorized)
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
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd backend && go test ./internal/auth/ -v`
Expected: All tests PASS (requires Postgres running with schema applied)

- [ ] **Step 5: Wire auth routes into `backend/main.go`**

Add these lines after the health endpoint in `main.go`:

```go
	// Auth routes
	authHandler := auth.NewHandler(pool, cfg.JWTSecret)
	r.Post("/api/auth/register", authHandler.Register)
	r.Post("/api/auth/login", authHandler.Login)
```

Add the import:
```go
	"github.com/assettrackerhq/asset-tracker/backend/internal/auth"
```

- [ ] **Step 6: Verify it compiles**

Run: `cd backend && go build ./...`
Expected: No errors

- [ ] **Step 7: Commit**

```bash
git add backend/internal/auth/handler.go backend/internal/auth/handler_test.go backend/main.go
git commit -m "feat: add auth register and login handlers with tests"
```

---

### Task 5: Asset CRUD Handlers

**Files:**
- Create: `backend/internal/assets/handler.go`
- Create: `backend/internal/assets/handler_test.go`
- Modify: `backend/main.go` (wire asset routes)

- [ ] **Step 1: Write the failing test**

Create `backend/internal/assets/handler_test.go`:

```go
package assets_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/assettrackerhq/asset-tracker/backend/internal/assets"
	"github.com/assettrackerhq/asset-tracker/backend/internal/auth"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func setupTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dbURL := "postgres://asset_tracker:asset_tracker@localhost:5432/asset_tracker?sslmode=disable"
	pool, err := pgxpool.New(context.Background(), dbURL)
	if err != nil {
		t.Skipf("skipping: cannot connect to test DB: %v", err)
	}
	pool.Exec(context.Background(), "DELETE FROM asset_value_points")
	pool.Exec(context.Background(), "DELETE FROM assets")
	pool.Exec(context.Background(), "DELETE FROM users")
	return pool
}

func createTestUser(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	userID := "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
	pool.Exec(context.Background(),
		"INSERT INTO users (id, username, password_hash) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING",
		userID, "testuser", "$2a$12$dummyhashvalue",
	)
	return userID
}

func setupRouter(pool *pgxpool.Pool) *chi.Mux {
	handler := assets.NewHandler(pool)
	r := chi.NewRouter()
	r.Route("/api/assets", func(r chi.Router) {
		r.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				ctx := context.WithValue(r.Context(), auth.UserIDKey, "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee")
				next.ServeHTTP(w, r.WithContext(ctx))
			})
		})
		r.Get("/", handler.List)
		r.Post("/", handler.Create)
		r.Put("/{id}", handler.Update)
		r.Delete("/{id}", handler.Delete)
	})
	return r
}

func TestAssetCRUD(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()
	createTestUser(t, pool)

	r := setupRouter(pool)

	// Create
	body := map[string]string{"id": "A001", "name": "Laptop", "description": "Work laptop"}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/assets", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("create: expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	// List
	req = httptest.NewRequest(http.MethodGet, "/api/assets", nil)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("list: expected 200, got %d", rec.Code)
	}

	var listResp []map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&listResp)
	if len(listResp) != 1 {
		t.Fatalf("list: expected 1 asset, got %d", len(listResp))
	}
	if listResp[0]["id"] != "A001" {
		t.Fatalf("list: expected id A001, got %v", listResp[0]["id"])
	}

	// Update
	updateBody := map[string]string{"name": "Updated Laptop", "description": "Personal laptop"}
	jsonUpdate, _ := json.Marshal(updateBody)
	req = httptest.NewRequest(http.MethodPut, "/api/assets/A001", bytes.NewReader(jsonUpdate))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("update: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// Delete
	req = httptest.NewRequest(http.MethodDelete, "/api/assets/A001", nil)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete: expected 204, got %d", rec.Code)
	}

	// List after delete — should be empty
	req = httptest.NewRequest(http.MethodGet, "/api/assets", nil)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	json.NewDecoder(rec.Body).Decode(&listResp)
	if len(listResp) != 0 {
		t.Fatalf("list after delete: expected 0 assets, got %d", len(listResp))
	}
}

func TestAssetDuplicateID(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()
	createTestUser(t, pool)

	r := setupRouter(pool)

	body := map[string]string{"id": "A001", "name": "Laptop", "description": "Work laptop"}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/assets", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	// Create duplicate
	req = httptest.NewRequest(http.MethodPost, "/api/assets", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("duplicate: expected 409, got %d", rec.Code)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd backend && go test ./internal/assets/ -v`
Expected: Compilation error — `assets.NewHandler` not defined

- [ ] **Step 3: Create `backend/internal/assets/handler.go`**

```go
package assets

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/assettrackerhq/asset-tracker/backend/internal/auth"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Handler struct {
	db *pgxpool.Pool
}

func NewHandler(db *pgxpool.Pool) *Handler {
	return &Handler{db: db}
}

type Asset struct {
	ID          string    `json:"id"`
	UserID      string    `json:"user_id"`
	Name        string    `json:"name"`
	Description *string   `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type createRequest struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Description *string `json:"description"`
}

type updateRequest struct {
	Name        string  `json:"name"`
	Description *string `json:"description"`
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserID(r.Context())

	rows, err := h.db.Query(context.Background(),
		"SELECT id, user_id, name, description, created_at, updated_at FROM assets WHERE user_id = $1 ORDER BY created_at DESC",
		userID,
	)
	if err != nil {
		http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	assets := []Asset{}
	for rows.Next() {
		var a Asset
		if err := rows.Scan(&a.ID, &a.UserID, &a.Name, &a.Description, &a.CreatedAt, &a.UpdatedAt); err != nil {
			http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
			return
		}
		assets = append(assets, a)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(assets)
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserID(r.Context())

	var req createRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	req.ID = strings.TrimSpace(req.ID)
	req.Name = strings.TrimSpace(req.Name)
	if req.ID == "" {
		http.Error(w, `{"error":"id is required"}`, http.StatusBadRequest)
		return
	}
	if req.Name == "" {
		http.Error(w, `{"error":"name is required"}`, http.StatusBadRequest)
		return
	}

	var asset Asset
	err := h.db.QueryRow(context.Background(),
		"INSERT INTO assets (id, user_id, name, description) VALUES ($1, $2, $3, $4) RETURNING id, user_id, name, description, created_at, updated_at",
		req.ID, userID, req.Name, req.Description,
	).Scan(&asset.ID, &asset.UserID, &asset.Name, &asset.Description, &asset.CreatedAt, &asset.UpdatedAt)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "unique") {
			http.Error(w, `{"error":"asset with this ID already exists"}`, http.StatusConflict)
			return
		}
		http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(asset)
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserID(r.Context())
	assetID := chi.URLParam(r, "id")

	var req updateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		http.Error(w, `{"error":"name is required"}`, http.StatusBadRequest)
		return
	}

	var asset Asset
	err := h.db.QueryRow(context.Background(),
		"UPDATE assets SET name = $1, description = $2, updated_at = now() WHERE id = $3 AND user_id = $4 RETURNING id, user_id, name, description, created_at, updated_at",
		req.Name, req.Description, assetID, userID,
	).Scan(&asset.ID, &asset.UserID, &asset.Name, &asset.Description, &asset.CreatedAt, &asset.UpdatedAt)
	if err != nil {
		http.Error(w, `{"error":"asset not found"}`, http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(asset)
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserID(r.Context())
	assetID := chi.URLParam(r, "id")

	tag, err := h.db.Exec(context.Background(),
		"DELETE FROM assets WHERE id = $1 AND user_id = $2",
		assetID, userID,
	)
	if err != nil {
		http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
		return
	}
	if tag.RowsAffected() == 0 {
		http.Error(w, `{"error":"asset not found"}`, http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd backend && go test ./internal/assets/ -v`
Expected: All tests PASS

- [ ] **Step 5: Wire asset routes into `backend/main.go`**

Add after the auth routes in `main.go`:

```go
	// Asset routes (protected)
	assetHandler := assets.NewHandler(pool)
	r.Route("/api/assets", func(r chi.Router) {
		r.Use(auth.Middleware(cfg.JWTSecret))
		r.Get("/", assetHandler.List)
		r.Post("/", assetHandler.Create)
		r.Put("/{id}", assetHandler.Update)
		r.Delete("/{id}", assetHandler.Delete)
	})
```

Add the import:
```go
	"github.com/assettrackerhq/asset-tracker/backend/internal/assets"
```

- [ ] **Step 6: Verify it compiles**

Run: `cd backend && go build ./...`
Expected: No errors

- [ ] **Step 7: Commit**

```bash
git add backend/internal/assets/ backend/main.go
git commit -m "feat: add asset CRUD handlers with tests"
```

---

### Task 6: Value Point CRUD Handlers

**Files:**
- Create: `backend/internal/values/handler.go`
- Create: `backend/internal/values/handler_test.go`
- Modify: `backend/main.go` (wire value point routes)

- [ ] **Step 1: Write the failing test**

Create `backend/internal/values/handler_test.go`:

```go
package values_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/assettrackerhq/asset-tracker/backend/internal/auth"
	"github.com/assettrackerhq/asset-tracker/backend/internal/values"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const testUserID = "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"

func setupTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dbURL := "postgres://asset_tracker:asset_tracker@localhost:5432/asset_tracker?sslmode=disable"
	pool, err := pgxpool.New(context.Background(), dbURL)
	if err != nil {
		t.Skipf("skipping: cannot connect to test DB: %v", err)
	}
	pool.Exec(context.Background(), "DELETE FROM asset_value_points")
	pool.Exec(context.Background(), "DELETE FROM assets")
	pool.Exec(context.Background(), "DELETE FROM users")
	pool.Exec(context.Background(),
		"INSERT INTO users (id, username, password_hash) VALUES ($1, $2, $3)",
		testUserID, "testuser", "$2a$12$dummyhashvalue",
	)
	pool.Exec(context.Background(),
		"INSERT INTO assets (id, user_id, name, description) VALUES ($1, $2, $3, $4)",
		"A001", testUserID, "Laptop", "Work laptop",
	)
	return pool
}

func setupRouter(pool *pgxpool.Pool) *chi.Mux {
	handler := values.NewHandler(pool)
	r := chi.NewRouter()
	r.Route("/api/assets/{assetID}/values", func(r chi.Router) {
		r.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				ctx := context.WithValue(r.Context(), auth.UserIDKey, testUserID)
				next.ServeHTTP(w, r.WithContext(ctx))
			})
		})
		r.Get("/", handler.List)
		r.Post("/", handler.Create)
		r.Put("/{valueID}", handler.Update)
		r.Delete("/{valueID}", handler.Delete)
	})
	return r
}

func TestValuePointCRUD(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	r := setupRouter(pool)

	// Create
	body := map[string]interface{}{"value": 1250.50, "currency": "USD"}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/assets/A001/values", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("create: expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var createResp map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&createResp)
	valueID := createResp["id"].(string)
	if valueID == "" {
		t.Fatal("create: expected id in response")
	}

	// List
	req = httptest.NewRequest(http.MethodGet, "/api/assets/A001/values", nil)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("list: expected 200, got %d", rec.Code)
	}

	var listResp []map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&listResp)
	if len(listResp) != 1 {
		t.Fatalf("list: expected 1 value point, got %d", len(listResp))
	}

	// Update
	updateBody := map[string]interface{}{"value": 1500.00, "currency": "EUR"}
	jsonUpdate, _ := json.Marshal(updateBody)
	req = httptest.NewRequest(http.MethodPut, "/api/assets/A001/values/"+valueID, bytes.NewReader(jsonUpdate))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("update: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// Delete
	req = httptest.NewRequest(http.MethodDelete, "/api/assets/A001/values/"+valueID, nil)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete: expected 204, got %d", rec.Code)
	}

	// List after delete
	req = httptest.NewRequest(http.MethodGet, "/api/assets/A001/values", nil)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	json.NewDecoder(rec.Body).Decode(&listResp)
	if len(listResp) != 0 {
		t.Fatalf("list after delete: expected 0, got %d", len(listResp))
	}
}

func TestValuePointForNonexistentAsset(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	r := setupRouter(pool)

	body := map[string]interface{}{"value": 100.00, "currency": "USD"}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/assets/NONEXISTENT/values", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("nonexistent asset: expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd backend && go test ./internal/values/ -v`
Expected: Compilation error — `values.NewHandler` not defined

- [ ] **Step 3: Create `backend/internal/values/handler.go`**

```go
package values

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/assettrackerhq/asset-tracker/backend/internal/auth"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Handler struct {
	db *pgxpool.Pool
}

func NewHandler(db *pgxpool.Pool) *Handler {
	return &Handler{db: db}
}

type ValuePoint struct {
	ID        string    `json:"id"`
	AssetID   string    `json:"asset_id"`
	UserID    string    `json:"user_id"`
	Timestamp time.Time `json:"timestamp"`
	Value     float64   `json:"value"`
	Currency  string    `json:"currency"`
}

type createRequest struct {
	Value    float64 `json:"value"`
	Currency string  `json:"currency"`
}

type updateRequest struct {
	Value    float64 `json:"value"`
	Currency string  `json:"currency"`
}

func (h *Handler) assetExists(ctx context.Context, assetID, userID string) bool {
	var exists bool
	h.db.QueryRow(ctx,
		"SELECT EXISTS(SELECT 1 FROM assets WHERE id = $1 AND user_id = $2)",
		assetID, userID,
	).Scan(&exists)
	return exists
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserID(r.Context())
	assetID := chi.URLParam(r, "assetID")

	rows, err := h.db.Query(context.Background(),
		"SELECT id, asset_id, user_id, timestamp, value, currency FROM asset_value_points WHERE asset_id = $1 AND user_id = $2 ORDER BY timestamp DESC",
		assetID, userID,
	)
	if err != nil {
		http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	points := []ValuePoint{}
	for rows.Next() {
		var vp ValuePoint
		if err := rows.Scan(&vp.ID, &vp.AssetID, &vp.UserID, &vp.Timestamp, &vp.Value, &vp.Currency); err != nil {
			http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
			return
		}
		points = append(points, vp)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(points)
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserID(r.Context())
	assetID := chi.URLParam(r, "assetID")

	if !h.assetExists(r.Context(), assetID, userID) {
		http.Error(w, `{"error":"asset not found"}`, http.StatusNotFound)
		return
	}

	var req createRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	if req.Currency == "" {
		http.Error(w, `{"error":"currency is required"}`, http.StatusBadRequest)
		return
	}

	var vp ValuePoint
	err := h.db.QueryRow(context.Background(),
		"INSERT INTO asset_value_points (asset_id, user_id, value, currency) VALUES ($1, $2, $3, $4) RETURNING id, asset_id, user_id, timestamp, value, currency",
		assetID, userID, req.Value, req.Currency,
	).Scan(&vp.ID, &vp.AssetID, &vp.UserID, &vp.Timestamp, &vp.Value, &vp.Currency)
	if err != nil {
		http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(vp)
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserID(r.Context())
	assetID := chi.URLParam(r, "assetID")
	valueID := chi.URLParam(r, "valueID")

	var req updateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	if req.Currency == "" {
		http.Error(w, `{"error":"currency is required"}`, http.StatusBadRequest)
		return
	}

	var vp ValuePoint
	err := h.db.QueryRow(context.Background(),
		"UPDATE asset_value_points SET value = $1, currency = $2 WHERE id = $3 AND asset_id = $4 AND user_id = $5 RETURNING id, asset_id, user_id, timestamp, value, currency",
		req.Value, req.Currency, valueID, assetID, userID,
	).Scan(&vp.ID, &vp.AssetID, &vp.UserID, &vp.Timestamp, &vp.Value, &vp.Currency)
	if err != nil {
		http.Error(w, `{"error":"value point not found"}`, http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(vp)
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserID(r.Context())
	assetID := chi.URLParam(r, "assetID")
	valueID := chi.URLParam(r, "valueID")

	tag, err := h.db.Exec(context.Background(),
		"DELETE FROM asset_value_points WHERE id = $1 AND asset_id = $2 AND user_id = $3",
		valueID, assetID, userID,
	)
	if err != nil {
		http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
		return
	}
	if tag.RowsAffected() == 0 {
		http.Error(w, `{"error":"value point not found"}`, http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd backend && go test ./internal/values/ -v`
Expected: All tests PASS

- [ ] **Step 5: Wire value point routes into `backend/main.go`**

Inside the existing `/api/assets` route group, add the value point sub-routes. Update the route block to:

```go
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
```

Add the import:
```go
	"github.com/assettrackerhq/asset-tracker/backend/internal/values"
```

- [ ] **Step 6: Verify it compiles**

Run: `cd backend && go build ./...`
Expected: No errors

- [ ] **Step 7: Commit**

```bash
git add backend/internal/values/ backend/main.go
git commit -m "feat: add value point CRUD handlers with tests"
```

---

### Task 7: Frontend Scaffolding

**Files:**
- Create: `frontend/package.json`
- Create: `frontend/vite.config.js`
- Create: `frontend/index.html`
- Create: `frontend/src/main.jsx`
- Create: `frontend/src/App.jsx`
- Create: `frontend/src/App.css`
- Create: `frontend/src/api.js`
- Create: `frontend/Dockerfile`

- [ ] **Step 1: Initialize frontend project**

Run:
```bash
cd frontend
npm create vite@latest . -- --template react
```

If prompted to overwrite, select yes.

- [ ] **Step 2: Install dependencies**

Run:
```bash
cd frontend
npm install react-router-dom
```

- [ ] **Step 3: Create `frontend/src/api.js`**

```javascript
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

export function register(username, password) {
  return request('/auth/register', {
    method: 'POST',
    body: JSON.stringify({ username, password }),
  });
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
```

- [ ] **Step 4: Replace `frontend/src/App.jsx`**

```jsx
import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import Login from './pages/Login';
import Register from './pages/Register';
import AssetList from './pages/AssetList';
import AssetDetail from './pages/AssetDetail';
import './App.css';

function ProtectedRoute({ children }) {
  const token = localStorage.getItem('token');
  if (!token) {
    return <Navigate to="/login" replace />;
  }
  return children;
}

export default function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route path="/login" element={<Login />} />
        <Route path="/register" element={<Register />} />
        <Route path="/assets" element={<ProtectedRoute><AssetList /></ProtectedRoute>} />
        <Route path="/assets/:id" element={<ProtectedRoute><AssetDetail /></ProtectedRoute>} />
        <Route path="*" element={<Navigate to="/assets" replace />} />
      </Routes>
    </BrowserRouter>
  );
}
```

- [ ] **Step 5: Replace `frontend/src/App.css`**

```css
* {
  margin: 0;
  padding: 0;
  box-sizing: border-box;
}

body {
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
  background: #f5f5f5;
  color: #333;
}

.container {
  max-width: 800px;
  margin: 40px auto;
  padding: 0 20px;
}

h1 {
  margin-bottom: 24px;
}

.form-group {
  margin-bottom: 16px;
}

.form-group label {
  display: block;
  margin-bottom: 4px;
  font-weight: 600;
}

.form-group input,
.form-group textarea {
  width: 100%;
  padding: 8px 12px;
  border: 1px solid #ccc;
  border-radius: 4px;
  font-size: 14px;
}

button {
  padding: 8px 16px;
  border: none;
  border-radius: 4px;
  cursor: pointer;
  font-size: 14px;
  margin-right: 8px;
}

button.primary {
  background: #2563eb;
  color: white;
}

button.danger {
  background: #dc2626;
  color: white;
}

button.secondary {
  background: #6b7280;
  color: white;
}

table {
  width: 100%;
  border-collapse: collapse;
  margin-top: 16px;
}

th, td {
  text-align: left;
  padding: 10px 12px;
  border-bottom: 1px solid #ddd;
}

th {
  background: #f9fafb;
  font-weight: 600;
}

.error {
  color: #dc2626;
  margin-bottom: 16px;
}

a {
  color: #2563eb;
}

.actions {
  display: flex;
  gap: 8px;
}

.header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 24px;
}

.auth-form {
  max-width: 400px;
  margin: 80px auto;
  padding: 32px;
  background: white;
  border-radius: 8px;
  box-shadow: 0 1px 3px rgba(0,0,0,0.1);
}
```

- [ ] **Step 6: Replace `frontend/src/main.jsx`**

```jsx
import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import App from './App';

createRoot(document.getElementById('root')).render(
  <StrictMode>
    <App />
  </StrictMode>
);
```

- [ ] **Step 7: Update `frontend/vite.config.js`**

```javascript
import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

export default defineConfig({
  plugins: [react()],
  server: {
    host: '0.0.0.0',
    port: 5173,
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
    },
  },
});
```

- [ ] **Step 8: Create `frontend/Dockerfile`**

```dockerfile
FROM node:20-alpine
WORKDIR /app
COPY package*.json ./
RUN npm install
COPY . .
EXPOSE 5173
CMD ["npm", "run", "dev", "--", "--host"]
```

- [ ] **Step 9: Verify frontend builds**

Run: `cd frontend && npm run build`
Expected: Build succeeds with no errors

- [ ] **Step 10: Commit**

```bash
git add frontend/
git commit -m "feat: add React frontend scaffolding with API client and routing"
```

---

### Task 8: Login and Register Pages

**Files:**
- Create: `frontend/src/pages/Login.jsx`
- Create: `frontend/src/pages/Register.jsx`

- [ ] **Step 1: Create `frontend/src/pages/Login.jsx`**

```jsx
import { useState } from 'react';
import { useNavigate, Link } from 'react-router-dom';
import { login } from '../api';

export default function Login() {
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const navigate = useNavigate();

  async function handleSubmit(e) {
    e.preventDefault();
    setError('');
    try {
      const data = await login(username, password);
      localStorage.setItem('token', data.token);
      navigate('/assets');
    } catch (err) {
      setError(err.message);
    }
  }

  return (
    <div className="auth-form">
      <h1>Login</h1>
      {error && <p className="error">{error}</p>}
      <form onSubmit={handleSubmit}>
        <div className="form-group">
          <label>Username</label>
          <input value={username} onChange={(e) => setUsername(e.target.value)} required />
        </div>
        <div className="form-group">
          <label>Password</label>
          <input type="password" value={password} onChange={(e) => setPassword(e.target.value)} required />
        </div>
        <button type="submit" className="primary">Login</button>
      </form>
      <p style={{ marginTop: '16px' }}>
        Don't have an account? <Link to="/register">Register</Link>
      </p>
    </div>
  );
}
```

- [ ] **Step 2: Create `frontend/src/pages/Register.jsx`**

```jsx
import { useState } from 'react';
import { useNavigate, Link } from 'react-router-dom';
import { register } from '../api';

export default function Register() {
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const navigate = useNavigate();

  async function handleSubmit(e) {
    e.preventDefault();
    setError('');
    try {
      const data = await register(username, password);
      localStorage.setItem('token', data.token);
      navigate('/assets');
    } catch (err) {
      setError(err.message);
    }
  }

  return (
    <div className="auth-form">
      <h1>Register</h1>
      {error && <p className="error">{error}</p>}
      <form onSubmit={handleSubmit}>
        <div className="form-group">
          <label>Username</label>
          <input value={username} onChange={(e) => setUsername(e.target.value)} required />
        </div>
        <div className="form-group">
          <label>Password</label>
          <input type="password" value={password} onChange={(e) => setPassword(e.target.value)} required minLength={8} />
        </div>
        <button type="submit" className="primary">Register</button>
      </form>
      <p style={{ marginTop: '16px' }}>
        Already have an account? <Link to="/login">Login</Link>
      </p>
    </div>
  );
}
```

- [ ] **Step 3: Verify frontend builds**

Run: `cd frontend && npm run build`
Expected: Build succeeds

- [ ] **Step 4: Commit**

```bash
git add frontend/src/pages/Login.jsx frontend/src/pages/Register.jsx
git commit -m "feat: add login and register pages"
```

---

### Task 9: Asset List Page

**Files:**
- Create: `frontend/src/pages/AssetList.jsx`

- [ ] **Step 1: Create `frontend/src/pages/AssetList.jsx`**

```jsx
import { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { listAssets, createAsset, updateAsset, deleteAsset } from '../api';

export default function AssetList() {
  const [assets, setAssets] = useState([]);
  const [error, setError] = useState('');
  const [showForm, setShowForm] = useState(false);
  const [editingId, setEditingId] = useState(null);
  const [formData, setFormData] = useState({ id: '', name: '', description: '' });
  const navigate = useNavigate();

  useEffect(() => {
    loadAssets();
  }, []);

  async function loadAssets() {
    try {
      const data = await listAssets();
      setAssets(data);
    } catch (err) {
      setError(err.message);
    }
  }

  async function handleSubmit(e) {
    e.preventDefault();
    setError('');
    try {
      if (editingId) {
        await updateAsset(editingId, formData.name, formData.description || null);
      } else {
        await createAsset(formData.id, formData.name, formData.description || null);
      }
      setShowForm(false);
      setEditingId(null);
      setFormData({ id: '', name: '', description: '' });
      await loadAssets();
    } catch (err) {
      setError(err.message);
    }
  }

  async function handleDelete(id) {
    if (!confirm('Delete this asset?')) return;
    try {
      await deleteAsset(id);
      await loadAssets();
    } catch (err) {
      setError(err.message);
    }
  }

  function startEdit(asset) {
    setEditingId(asset.id);
    setFormData({ id: asset.id, name: asset.name, description: asset.description || '' });
    setShowForm(true);
  }

  function handleLogout() {
    localStorage.removeItem('token');
    navigate('/login');
  }

  return (
    <div className="container">
      <div className="header">
        <h1>My Assets</h1>
        <div>
          <button className="primary" onClick={() => { setShowForm(true); setEditingId(null); setFormData({ id: '', name: '', description: '' }); }}>
            Add Asset
          </button>
          <button className="secondary" onClick={handleLogout}>Logout</button>
        </div>
      </div>

      {error && <p className="error">{error}</p>}

      {showForm && (
        <form onSubmit={handleSubmit} style={{ marginBottom: '24px', padding: '16px', background: 'white', borderRadius: '8px' }}>
          {!editingId && (
            <div className="form-group">
              <label>ID</label>
              <input value={formData.id} onChange={(e) => setFormData({ ...formData, id: e.target.value })} required />
            </div>
          )}
          <div className="form-group">
            <label>Name</label>
            <input value={formData.name} onChange={(e) => setFormData({ ...formData, name: e.target.value })} required />
          </div>
          <div className="form-group">
            <label>Description</label>
            <textarea value={formData.description} onChange={(e) => setFormData({ ...formData, description: e.target.value })} />
          </div>
          <button type="submit" className="primary">{editingId ? 'Update' : 'Create'}</button>
          <button type="button" className="secondary" onClick={() => { setShowForm(false); setEditingId(null); }}>Cancel</button>
        </form>
      )}

      <table>
        <thead>
          <tr>
            <th>ID</th>
            <th>Name</th>
            <th>Description</th>
            <th>Actions</th>
          </tr>
        </thead>
        <tbody>
          {assets.map((asset) => (
            <tr key={asset.id}>
              <td><a href="#" onClick={(e) => { e.preventDefault(); navigate(`/assets/${asset.id}`); }}>{asset.id}</a></td>
              <td>{asset.name}</td>
              <td>{asset.description}</td>
              <td className="actions">
                <button className="secondary" onClick={() => startEdit(asset)}>Edit</button>
                <button className="danger" onClick={() => handleDelete(asset.id)}>Delete</button>
              </td>
            </tr>
          ))}
          {assets.length === 0 && (
            <tr><td colSpan="4" style={{ textAlign: 'center', color: '#999' }}>No assets yet</td></tr>
          )}
        </tbody>
      </table>
    </div>
  );
}
```

- [ ] **Step 2: Verify frontend builds**

Run: `cd frontend && npm run build`
Expected: Build succeeds

- [ ] **Step 3: Commit**

```bash
git add frontend/src/pages/AssetList.jsx
git commit -m "feat: add asset list page with CRUD"
```

---

### Task 10: Asset Detail Page (Value Points)

**Files:**
- Create: `frontend/src/pages/AssetDetail.jsx`

- [ ] **Step 1: Create `frontend/src/pages/AssetDetail.jsx`**

```jsx
import { useState, useEffect } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { listValuePoints, createValuePoint, updateValuePoint, deleteValuePoint } from '../api';

export default function AssetDetail() {
  const { id } = useParams();
  const navigate = useNavigate();
  const [values, setValues] = useState([]);
  const [error, setError] = useState('');
  const [showForm, setShowForm] = useState(false);
  const [editingId, setEditingId] = useState(null);
  const [formData, setFormData] = useState({ value: '', currency: 'USD' });

  useEffect(() => {
    loadValues();
  }, [id]);

  async function loadValues() {
    try {
      const data = await listValuePoints(id);
      setValues(data);
    } catch (err) {
      setError(err.message);
    }
  }

  async function handleSubmit(e) {
    e.preventDefault();
    setError('');
    try {
      if (editingId) {
        await updateValuePoint(id, editingId, formData.value, formData.currency);
      } else {
        await createValuePoint(id, formData.value, formData.currency);
      }
      setShowForm(false);
      setEditingId(null);
      setFormData({ value: '', currency: 'USD' });
      await loadValues();
    } catch (err) {
      setError(err.message);
    }
  }

  async function handleDelete(valueId) {
    if (!confirm('Delete this value point?')) return;
    try {
      await deleteValuePoint(id, valueId);
      await loadValues();
    } catch (err) {
      setError(err.message);
    }
  }

  function startEdit(vp) {
    setEditingId(vp.id);
    setFormData({ value: vp.value, currency: vp.currency });
    setShowForm(true);
  }

  function formatTimestamp(ts) {
    return new Date(ts).toLocaleString();
  }

  function formatValue(value, currency) {
    return new Intl.NumberFormat(undefined, { style: 'currency', currency }).format(value);
  }

  return (
    <div className="container">
      <div className="header">
        <h1>Asset: {id}</h1>
        <div>
          <button className="primary" onClick={() => { setShowForm(true); setEditingId(null); setFormData({ value: '', currency: 'USD' }); }}>
            Add Value Point
          </button>
          <button className="secondary" onClick={() => navigate('/assets')}>Back</button>
        </div>
      </div>

      {error && <p className="error">{error}</p>}

      {showForm && (
        <form onSubmit={handleSubmit} style={{ marginBottom: '24px', padding: '16px', background: 'white', borderRadius: '8px' }}>
          <div className="form-group">
            <label>Value</label>
            <input type="number" step="0.01" value={formData.value} onChange={(e) => setFormData({ ...formData, value: e.target.value })} required />
          </div>
          <div className="form-group">
            <label>Currency</label>
            <input value={formData.currency} onChange={(e) => setFormData({ ...formData, currency: e.target.value.toUpperCase() })} maxLength={3} required />
          </div>
          <button type="submit" className="primary">{editingId ? 'Update' : 'Add'}</button>
          <button type="button" className="secondary" onClick={() => { setShowForm(false); setEditingId(null); }}>Cancel</button>
        </form>
      )}

      <table>
        <thead>
          <tr>
            <th>Timestamp</th>
            <th>Value</th>
            <th>Currency</th>
            <th>Actions</th>
          </tr>
        </thead>
        <tbody>
          {values.map((vp) => (
            <tr key={vp.id}>
              <td>{formatTimestamp(vp.timestamp)}</td>
              <td>{formatValue(vp.value, vp.currency)}</td>
              <td>{vp.currency}</td>
              <td className="actions">
                <button className="secondary" onClick={() => startEdit(vp)}>Edit</button>
                <button className="danger" onClick={() => handleDelete(vp.id)}>Delete</button>
              </td>
            </tr>
          ))}
          {values.length === 0 && (
            <tr><td colSpan="4" style={{ textAlign: 'center', color: '#999' }}>No value points yet</td></tr>
          )}
        </tbody>
      </table>
    </div>
  );
}
```

- [ ] **Step 2: Verify frontend builds**

Run: `cd frontend && npm run build`
Expected: Build succeeds

- [ ] **Step 3: Commit**

```bash
git add frontend/src/pages/AssetDetail.jsx
git commit -m "feat: add asset detail page with value point CRUD"
```

---

### Task 11: End-to-End Smoke Test

**Files:** None (verification only)

- [ ] **Step 1: Start all services**

Run: `docker compose up --build -d`
Expected: All services start successfully

- [ ] **Step 2: Wait for services to be ready**

Run: `docker compose logs schemahero-apply` and verify schema was applied.
Run: `curl -s http://localhost:8080/api/health`
Expected: `{"status":"ok"}`

- [ ] **Step 3: Test register**

Run:
```bash
curl -s -X POST http://localhost:8080/api/auth/register \
  -H "Content-Type: application/json" \
  -d '{"username":"testuser","password":"password123"}'
```
Expected: `{"token":"eyJ..."}`

- [ ] **Step 4: Test login and asset CRUD**

Run (using the token from register):
```bash
TOKEN=$(curl -s -X POST http://localhost:8080/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"testuser","password":"password123"}' | grep -o '"token":"[^"]*"' | cut -d'"' -f4)

# Create asset
curl -s -X POST http://localhost:8080/api/assets \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"id":"A001","name":"Laptop","description":"Work laptop"}'

# List assets
curl -s http://localhost:8080/api/assets \
  -H "Authorization: Bearer $TOKEN"

# Add value point
curl -s -X POST http://localhost:8080/api/assets/A001/values \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"value":1250.50,"currency":"USD"}'

# List value points
curl -s http://localhost:8080/api/assets/A001/values \
  -H "Authorization: Bearer $TOKEN"
```
Expected: All requests return expected JSON responses

- [ ] **Step 5: Verify frontend loads**

Open `http://localhost:5173` in a browser. Verify the login page renders.

- [ ] **Step 6: Stop services**

Run: `docker compose down`

- [ ] **Step 7: Commit any fixes if needed**

If any issues were found and fixed during smoke testing, commit the fixes.
