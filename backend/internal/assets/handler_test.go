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
