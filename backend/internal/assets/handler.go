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
