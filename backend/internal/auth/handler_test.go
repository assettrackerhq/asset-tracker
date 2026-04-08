package auth_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/assettrackerhq/asset-tracker/backend/internal/auth"
	"github.com/assettrackerhq/asset-tracker/backend/internal/license"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func mockSDKServer(t *testing.T, userLimit int) *license.Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"name":"user_limit","title":"User Limit","type":"Integer","value":%d}`, userLimit)
	}))
	t.Cleanup(srv.Close)
	return license.NewClient(srv.URL)
}

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
	lc := mockSDKServer(t, 100)
	handler := auth.NewHandler(pool, jwtSecret, lc)

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

	lc := mockSDKServer(t, 100)
	handler := auth.NewHandler(pool, "test-secret", lc)

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
