package exchangerates

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Handler struct {
	db *pgxpool.Pool
}

func NewHandler(db *pgxpool.Pool) *Handler {
	return &Handler{db: db}
}

type ExchangeRate struct {
	ID             string    `json:"id"`
	BaseCurrency   string    `json:"base_currency"`
	TargetCurrency string    `json:"target_currency"`
	Rate           float64   `json:"rate"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type upsertRequest struct {
	BaseCurrency   string  `json:"base_currency"`
	TargetCurrency string  `json:"target_currency"`
	Rate           float64 `json:"rate"`
}

type fetchRequest struct {
	BaseCurrency string `json:"base_currency"`
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.Query(context.Background(),
		"SELECT id, base_currency, target_currency, rate, updated_at FROM exchange_rates ORDER BY base_currency, target_currency",
	)
	if err != nil {
		http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	rates := []ExchangeRate{}
	for rows.Next() {
		var er ExchangeRate
		if err := rows.Scan(&er.ID, &er.BaseCurrency, &er.TargetCurrency, &er.Rate, &er.UpdatedAt); err != nil {
			http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
			return
		}
		rates = append(rates, er)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(rates)
}

func (h *Handler) Upsert(w http.ResponseWriter, r *http.Request) {
	var req upsertRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	req.BaseCurrency = strings.TrimSpace(strings.ToUpper(req.BaseCurrency))
	req.TargetCurrency = strings.TrimSpace(strings.ToUpper(req.TargetCurrency))

	if len(req.BaseCurrency) != 3 || len(req.TargetCurrency) != 3 {
		http.Error(w, `{"error":"currency codes must be 3 characters"}`, http.StatusBadRequest)
		return
	}
	if req.Rate <= 0 {
		http.Error(w, `{"error":"rate must be positive"}`, http.StatusBadRequest)
		return
	}

	var er ExchangeRate
	err := h.db.QueryRow(context.Background(),
		`INSERT INTO exchange_rates (base_currency, target_currency, rate)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (base_currency, target_currency)
		 DO UPDATE SET rate = EXCLUDED.rate, updated_at = now()
		 RETURNING id, base_currency, target_currency, rate, updated_at`,
		req.BaseCurrency, req.TargetCurrency, req.Rate,
	).Scan(&er.ID, &er.BaseCurrency, &er.TargetCurrency, &er.Rate, &er.UpdatedAt)
	if err != nil {
		http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(er)
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	tag, err := h.db.Exec(context.Background(),
		"DELETE FROM exchange_rates WHERE id = $1", id,
	)
	if err != nil {
		http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
		return
	}
	if tag.RowsAffected() == 0 {
		http.Error(w, `{"error":"exchange rate not found"}`, http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) Fetch(w http.ResponseWriter, r *http.Request) {
	var req fetchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	req.BaseCurrency = strings.TrimSpace(strings.ToUpper(req.BaseCurrency))
	if len(req.BaseCurrency) != 3 {
		http.Error(w, `{"error":"base_currency must be 3 characters"}`, http.StatusBadRequest)
		return
	}

	url := fmt.Sprintf("https://api.frankfurter.app/latest?base=%s", req.BaseCurrency)
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		http.Error(w, `{"error":"failed to fetch exchange rates: service unreachable"}`, http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		http.Error(w, fmt.Sprintf(`{"error":"exchange rate API returned status %d: %s"}`, resp.StatusCode, string(body)), http.StatusBadGateway)
		return
	}

	var apiResp struct {
		Base  string             `json:"base"`
		Rates map[string]float64 `json:"rates"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		http.Error(w, `{"error":"failed to parse exchange rate response"}`, http.StatusBadGateway)
		return
	}

	count := 0
	for currency, rate := range apiResp.Rates {
		_, err := h.db.Exec(context.Background(),
			`INSERT INTO exchange_rates (base_currency, target_currency, rate)
			 VALUES ($1, $2, $3)
			 ON CONFLICT (base_currency, target_currency)
			 DO UPDATE SET rate = EXCLUDED.rate, updated_at = now()`,
			apiResp.Base, currency, rate,
		)
		if err != nil {
			continue
		}
		count++
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"updated": count,
		"base":    apiResp.Base,
	})
}
