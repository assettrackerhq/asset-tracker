package analytics

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"time"

	"github.com/assettrackerhq/asset-tracker/backend/internal/auth"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Handler struct {
	db *pgxpool.Pool
}

func NewHandler(db *pgxpool.Pool) *Handler {
	return &Handler{db: db}
}

type portfolioResponse struct {
	TotalValue float64       `json:"total_value"`
	Currency   string        `json:"currency"`
	Series     []seriesPoint `json:"series"`
}

type seriesPoint struct {
	Date  string  `json:"date"`
	Value float64 `json:"value"`
}

type valuePoint struct {
	AssetID   string
	Timestamp time.Time
	Value     float64
	Currency  string
}

type exchangeRate struct {
	BaseCurrency   string
	TargetCurrency string
	Rate           float64
}

func (h *Handler) Portfolio(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserID(r.Context())
	displayCurrency := r.URL.Query().Get("currency")
	if displayCurrency == "" {
		displayCurrency = "USD"
	}

	// Fetch all value points for this user
	rows, err := h.db.Query(context.Background(),
		"SELECT asset_id, timestamp, value, currency FROM asset_value_points WHERE user_id = $1 ORDER BY timestamp ASC",
		userID,
	)
	if err != nil {
		http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var points []valuePoint
	for rows.Next() {
		var vp valuePoint
		if err := rows.Scan(&vp.AssetID, &vp.Timestamp, &vp.Value, &vp.Currency); err != nil {
			http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
			return
		}
		points = append(points, vp)
	}

	// Fetch all exchange rates
	rateRows, err := h.db.Query(context.Background(),
		"SELECT base_currency, target_currency, rate FROM exchange_rates",
	)
	if err != nil {
		http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
		return
	}
	defer rateRows.Close()

	rates := map[string]float64{}
	for rateRows.Next() {
		var er exchangeRate
		if err := rateRows.Scan(&er.BaseCurrency, &er.TargetCurrency, &er.Rate); err != nil {
			http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
			return
		}
		rates[er.BaseCurrency+"->"+er.TargetCurrency] = er.Rate
	}

	// Convert values to display currency
	convert := func(value float64, fromCurrency string) (float64, bool) {
		if fromCurrency == displayCurrency {
			return value, true
		}
		// Direct rate: fromCurrency -> displayCurrency
		if rate, ok := rates[fromCurrency+"->"+displayCurrency]; ok {
			return value * rate, true
		}
		// Inverse rate: displayCurrency -> fromCurrency
		if rate, ok := rates[displayCurrency+"->"+fromCurrency]; ok && rate != 0 {
			return value / rate, true
		}
		return 0, false
	}

	// Build series: for each date, compute portfolio total
	// by carrying forward each asset's latest known value
	type dateAssetValue struct {
		date    string
		assetID string
		value   float64
	}

	var converted []dateAssetValue
	for _, vp := range points {
		val, ok := convert(vp.Value, vp.Currency)
		if !ok {
			continue
		}
		converted = append(converted, dateAssetValue{
			date:    vp.Timestamp.Format("2006-01-02"),
			assetID: vp.AssetID,
			value:   val,
		})
	}

	// Collect all unique dates, sorted
	dateSet := map[string]bool{}
	for _, c := range converted {
		dateSet[c.date] = true
	}
	var dates []string
	for d := range dateSet {
		dates = append(dates, d)
	}
	sort.Strings(dates)

	// For each date, track latest value per asset, then sum
	latestByAsset := map[string]float64{}
	series := []seriesPoint{}
	ci := 0

	for _, date := range dates {
		for ci < len(converted) && converted[ci].date == date {
			latestByAsset[converted[ci].assetID] = converted[ci].value
			ci++
		}
		total := 0.0
		for _, v := range latestByAsset {
			total += v
		}
		series = append(series, seriesPoint{
			Date:  date,
			Value: roundTo2(total),
		})
	}

	// Total value: sum of latest value per asset
	totalValue := 0.0
	for _, v := range latestByAsset {
		totalValue += v
	}

	resp := portfolioResponse{
		TotalValue: roundTo2(totalValue),
		Currency:   displayCurrency,
		Series:     series,
	}
	if resp.Series == nil {
		resp.Series = []seriesPoint{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func roundTo2(v float64) float64 {
	return float64(int(v*100+0.5)) / 100
}
