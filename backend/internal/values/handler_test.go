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
