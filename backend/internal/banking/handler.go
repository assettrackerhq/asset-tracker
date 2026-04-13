package banking

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/assettrackerhq/asset-tracker/backend/internal/auth"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Handler struct {
	db        *pgxpool.Pool
	providers map[string]BankProvider
}

func NewHandler(db *pgxpool.Pool, providers map[string]BankProvider) *Handler {
	return &Handler{db: db, providers: providers}
}

func BankingFeatureGate(providers map[string]BankProvider) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if len(providers) == 0 {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				json.NewEncoder(w).Encode(map[string]string{"error": "feature_disabled"})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

type linkTokenRequest struct {
	Provider string `json:"provider"`
}

func (h *Handler) CreateLinkToken(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserID(r.Context())

	var req linkTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	provider, ok := h.providers[req.Provider]
	if !ok {
		http.Error(w, `{"error":"unsupported provider"}`, http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	if req.Provider == "teller" {
		tp, ok := provider.(*TellerProvider)
		if !ok {
			http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(map[string]string{
			"application_id": tp.applicationID,
			"provider":       "teller",
		})
		return
	}

	linkToken, err := provider.CreateLinkToken(r.Context(), userID)
	if err != nil {
		http.Error(w, `{"error":"failed to create link token"}`, http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]string{
		"link_token": linkToken,
		"provider":   req.Provider,
	})
}

type connectRequest struct {
	Provider string `json:"provider"`
	Token    string `json:"token"`
}

func (h *Handler) Connect(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserID(r.Context())

	var req connectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	provider, ok := h.providers[req.Provider]
	if !ok {
		http.Error(w, `{"error":"unsupported provider"}`, http.StatusBadRequest)
		return
	}

	accessToken, err := provider.ExchangeToken(r.Context(), req.Token)
	if err != nil {
		http.Error(w, `{"error":"failed to exchange token"}`, http.StatusInternalServerError)
		return
	}

	accounts, err := provider.FetchAccounts(r.Context(), accessToken)
	if err != nil {
		http.Error(w, `{"error":"failed to fetch accounts"}`, http.StatusInternalServerError)
		return
	}

	source := provider.Name()
	linked := 0
	for _, acct := range accounts {
		assetID := fmt.Sprintf("%s-%s", source, acct.ExternalID)
		_, err := h.db.Exec(context.Background(),
			`INSERT INTO assets (id, user_id, name, description, source, external_id, access_token, institution)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (id, user_id) DO UPDATE SET
    name = EXCLUDED.name,
    description = EXCLUDED.description,
    access_token = EXCLUDED.access_token,
    institution = EXCLUDED.institution,
    updated_at = now()`,
			assetID, userID, acct.Name, nil, source, acct.ExternalID, accessToken, acct.Institution,
		)
		if err != nil {
			http.Error(w, `{"error":"failed to save account"}`, http.StatusInternalServerError)
			return
		}

		_, err = h.db.Exec(context.Background(),
			`INSERT INTO asset_value_points (asset_id, user_id, value, currency, timestamp) VALUES ($1, $2, $3, $4, $5)`,
			assetID, userID, acct.Balance, acct.Currency, time.Now(),
		)
		if err != nil {
			http.Error(w, `{"error":"failed to save account balance"}`, http.StatusInternalServerError)
			return
		}

		linked++
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]int{"accounts_linked": linked})
}

func (h *Handler) ListAccounts(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserID(r.Context())

	rows, err := h.db.Query(context.Background(),
		`SELECT a.id, a.name, a.source, a.external_id, a.institution, a.updated_at,
       COALESCE(v.value, 0), COALESCE(v.currency, 'USD')
FROM assets a
LEFT JOIN LATERAL (
    SELECT value, currency FROM asset_value_points
    WHERE asset_id = a.id AND user_id = a.user_id
    ORDER BY timestamp DESC LIMIT 1
) v ON true
WHERE a.user_id = $1 AND a.source != 'manual'
ORDER BY a.updated_at DESC`,
		userID,
	)
	if err != nil {
		http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	accounts := []LinkedAccount{}
	for rows.Next() {
		var a LinkedAccount
		a.UserID = userID
		if err := rows.Scan(&a.ID, &a.Name, &a.Source, &a.ExternalID, &a.Institution, &a.UpdatedAt, &a.Balance, &a.Currency); err != nil {
			http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
			return
		}
		accounts = append(accounts, a)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(accounts)
}

func (h *Handler) SyncAccounts(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserID(r.Context())

	rows, err := h.db.Query(context.Background(),
		`SELECT DISTINCT source, access_token FROM assets WHERE user_id = $1 AND source != 'manual' AND access_token IS NOT NULL`,
		userID,
	)
	if err != nil {
		http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type sourceToken struct {
		source      string
		accessToken string
	}
	var pairs []sourceToken
	for rows.Next() {
		var st sourceToken
		if err := rows.Scan(&st.source, &st.accessToken); err != nil {
			http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
			return
		}
		pairs = append(pairs, st)
	}
	rows.Close()

	synced := 0
	for _, pair := range pairs {
		provider, ok := h.providers[pair.source]
		if !ok {
			continue
		}

		accounts, err := provider.FetchAccounts(r.Context(), pair.accessToken)
		if err != nil {
			continue
		}

		for _, acct := range accounts {
			assetID := fmt.Sprintf("%s-%s", pair.source, acct.ExternalID)
			_, err = h.db.Exec(context.Background(),
				`INSERT INTO asset_value_points (asset_id, user_id, value, currency, timestamp) VALUES ($1, $2, $3, $4, $5)`,
				assetID, userID, acct.Balance, acct.Currency, time.Now(),
			)
			if err == nil {
				synced++
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]int{"accounts_synced": synced})
}

func (h *Handler) UnlinkAccount(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserID(r.Context())
	assetID := chi.URLParam(r, "id")

	// Verify asset exists and is not manual
	var source string
	err := h.db.QueryRow(context.Background(),
		`SELECT source FROM assets WHERE id = $1 AND user_id = $2`,
		assetID, userID,
	).Scan(&source)
	if err != nil {
		http.Error(w, `{"error":"account not found"}`, http.StatusNotFound)
		return
	}

	if source == "manual" {
		http.Error(w, `{"error":"account not found"}`, http.StatusNotFound)
		return
	}

	// Delete value points first
	_, err = h.db.Exec(context.Background(),
		`DELETE FROM asset_value_points WHERE asset_id = $1 AND user_id = $2`,
		assetID, userID,
	)
	if err != nil {
		http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
		return
	}

	// Delete the asset
	tag, err := h.db.Exec(context.Background(),
		`DELETE FROM assets WHERE id = $1 AND user_id = $2 AND source != 'manual'`,
		assetID, userID,
	)
	if err != nil {
		http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
		return
	}
	if tag.RowsAffected() == 0 {
		http.Error(w, `{"error":"account not found"}`, http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
