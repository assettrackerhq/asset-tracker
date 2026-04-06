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
