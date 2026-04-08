package updates_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/assettrackerhq/asset-tracker/backend/internal/updates"
)

func TestCheckUpdates_Available(t *testing.T) {
	// Mock SDK returns a non-empty array
	sdk := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/app/updates" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[{"versionLabel":"0.2.0","createdAt":"2026-01-01T00:00:00Z","releaseNotes":"new stuff"}]`))
	}))
	defer sdk.Close()

	handler := updates.NewHandler(sdk.URL)
	req := httptest.NewRequest(http.MethodGet, "/api/app/updates", nil)
	rec := httptest.NewRecorder()

	handler.Check(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp map[string]bool
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !resp["updatesAvailable"] {
		t.Error("expected updatesAvailable=true")
	}
}

func TestCheckUpdates_None(t *testing.T) {
	sdk := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[]`))
	}))
	defer sdk.Close()

	handler := updates.NewHandler(sdk.URL)
	req := httptest.NewRequest(http.MethodGet, "/api/app/updates", nil)
	rec := httptest.NewRecorder()

	handler.Check(rec, req)

	var resp map[string]bool
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["updatesAvailable"] {
		t.Error("expected updatesAvailable=false")
	}
}

func TestCheckUpdates_SDKUnreachable(t *testing.T) {
	handler := updates.NewHandler("http://localhost:1") // unreachable
	req := httptest.NewRequest(http.MethodGet, "/api/app/updates", nil)
	rec := httptest.NewRecorder()

	handler.Check(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 even on SDK error, got %d", rec.Code)
	}

	var resp map[string]bool
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["updatesAvailable"] {
		t.Error("expected updatesAvailable=false when SDK unreachable")
	}
}
