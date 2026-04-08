package metrics_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/assettrackerhq/asset-tracker/backend/internal/metrics"
)

// mockDB implements metrics.UserCounter for testing.
type mockDB struct {
	count int64
	err   error
}

func (m *mockDB) QueryRow(ctx context.Context, sql string, args ...any) metrics.Row {
	return &mockRow{count: m.count, err: m.err}
}

type mockRow struct {
	count int64
	err   error
}

func (r *mockRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	*dest[0].(*int64) = r.count
	return nil
}

func TestReportSendsCorrectPayload(t *testing.T) {
	var mu sync.Mutex
	var received map[string]map[string]int64

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		json.Unmarshal(body, &received)
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	db := &mockDB{count: 42}
	reporter := metrics.New(db, srv.URL+"/api/v1/app/custom-metrics", 50*time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	reporter.Run(ctx)

	mu.Lock()
	defer mu.Unlock()

	if received == nil {
		t.Fatal("expected to receive a metrics payload")
	}
	if received["data"]["num_registered_users"] != 42 {
		t.Fatalf("expected num_registered_users=42, got %d", received["data"]["num_registered_users"])
	}
}

func TestReportHandlesDBError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not call SDK when DB query fails")
	}))
	defer srv.Close()

	db := &mockDB{err: io.ErrUnexpectedEOF}
	reporter := metrics.New(db, srv.URL+"/api/v1/app/custom-metrics", 50*time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Should not panic or crash
	reporter.Run(ctx)
}

func TestReportHandlesHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	db := &mockDB{count: 5}
	reporter := metrics.New(db, srv.URL+"/api/v1/app/custom-metrics", 50*time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Should not panic or crash
	reporter.Run(ctx)
}
