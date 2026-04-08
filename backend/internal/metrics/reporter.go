package metrics

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"
)

// Row is the interface for scanning a single row result.
type Row interface {
	Scan(dest ...any) error
}

// UserCounter is the interface the reporter needs from the database.
type UserCounter interface {
	QueryRow(ctx context.Context, sql string, args ...any) Row
}

// Reporter periodically reports custom metrics to the Replicated SDK.
type Reporter struct {
	db       UserCounter
	endpoint string
	interval time.Duration
}

// New creates a Reporter.
func New(db UserCounter, endpoint string, interval time.Duration) *Reporter {
	return &Reporter{
		db:       db,
		endpoint: endpoint,
		interval: interval,
	}
}

// Run starts the reporting loop. It reports immediately, then on each tick.
// It returns when ctx is cancelled.
func (r *Reporter) Run(ctx context.Context) {
	r.report(ctx)

	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.report(ctx)
		}
	}
}

func (r *Reporter) report(ctx context.Context) {
	var count int64
	err := r.db.QueryRow(ctx, "SELECT COUNT(*) FROM users").Scan(&count)
	if err != nil {
		log.Printf("metrics: failed to query user count: %v", err)
		return
	}

	payload := map[string]any{
		"data": map[string]int64{
			"num_registered_users": count,
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		log.Printf("metrics: failed to marshal payload: %v", err)
		return
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.endpoint, bytes.NewReader(body))
	if err != nil {
		log.Printf("metrics: failed to create request: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("metrics: failed to send metrics: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("metrics: SDK returned status %d", resp.StatusCode)
		return
	}

	log.Printf("metrics: reported num_registered_users=%d", count)
}
