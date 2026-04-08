package license

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// helpers

func makeLicenseInfo(expiresAt *string) LicenseInfoResponse {
	info := LicenseInfoResponse{
		LicenseID:   "test-id",
		LicenseType: "prod",
	}
	if expiresAt != nil {
		info.Entitlements = map[string]entitlement{
			"expires_at": {
				Title:     "Expiration",
				Value:     *expiresAt,
				ValueType: "String",
			},
		}
	}
	return info
}

func makeFakeSDKServer(t *testing.T, resp LicenseInfoResponse) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/license/info" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Errorf("fake SDK: encode error: %v", err)
		}
	}))
}

func makeErrorSDKServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}))
}

// --- evaluateLicense tests ---

func TestEvaluateLicense_NilExpiration(t *testing.T) {
	info := makeLicenseInfo(nil)
	valid, msg := evaluateLicense(&info, time.Now())
	if !valid {
		t.Errorf("expected valid, got invalid: %s", msg)
	}
	if msg != "" {
		t.Errorf("expected empty message, got: %s", msg)
	}
}

func TestEvaluateLicense_EmptyExpiration(t *testing.T) {
	empty := ""
	info := makeLicenseInfo(&empty)
	valid, msg := evaluateLicense(&info, time.Now())
	if !valid {
		t.Errorf("expected valid, got invalid: %s", msg)
	}
	if msg != "" {
		t.Errorf("expected empty message, got: %s", msg)
	}
}

func TestEvaluateLicense_FutureExpiration(t *testing.T) {
	future := time.Now().Add(24 * time.Hour).UTC().Format(time.RFC3339)
	info := makeLicenseInfo(&future)
	valid, msg := evaluateLicense(&info, time.Now())
	if !valid {
		t.Errorf("expected valid, got invalid: %s", msg)
	}
	if msg != "" {
		t.Errorf("expected empty message, got: %s", msg)
	}
}

func TestEvaluateLicense_PastExpiration(t *testing.T) {
	past := "2020-06-15T00:00:00Z"
	info := makeLicenseInfo(&past)
	valid, msg := evaluateLicense(&info, time.Now())
	if valid {
		t.Error("expected invalid, got valid")
	}
	if !strings.Contains(msg, "June 15, 2020") {
		t.Errorf("expected message to contain expiration date 'June 15, 2020', got: %s", msg)
	}
}

func TestEvaluateLicense_InvalidDateFormat(t *testing.T) {
	bad := "not-a-date"
	info := makeLicenseInfo(&bad)
	valid, msg := evaluateLicense(&info, time.Now())
	if valid {
		t.Error("expected invalid for bad date format, got valid")
	}
	if msg == "" {
		t.Error("expected non-empty message for invalid date format")
	}
}

// --- Checker state transition tests ---

func newCheckerWithServer(t *testing.T, srv *httptest.Server) *Checker {
	t.Helper()
	client := NewClient(srv.URL)
	return NewChecker(client)
}

func TestChecker_ValidLicense(t *testing.T) {
	future := time.Now().Add(24 * time.Hour).UTC().Format(time.RFC3339)
	srv := makeFakeSDKServer(t, makeLicenseInfo(&future))
	defer srv.Close()

	checker := newCheckerWithServer(t, srv)
	checker.now = time.Now
	checker.check(context.Background())

	status := checker.CurrentStatus()
	if !status.Valid {
		t.Errorf("expected valid status, got invalid: %s", status.Message)
	}
	if status.LastChecked.IsZero() {
		t.Error("expected LastChecked to be set after successful check")
	}
}

func TestChecker_ExpiredLicense(t *testing.T) {
	past := "2020-01-01T00:00:00Z"
	srv := makeFakeSDKServer(t, makeLicenseInfo(&past))
	defer srv.Close()

	checker := newCheckerWithServer(t, srv)
	checker.now = time.Now
	checker.check(context.Background())

	status := checker.CurrentStatus()
	if status.Valid {
		t.Error("expected invalid status for expired license, got valid")
	}
	if !strings.Contains(status.Message, "January 1, 2020") {
		t.Errorf("expected expiration message to contain date, got: %s", status.Message)
	}
}

func TestChecker_SDKUnreachable_NeverSucceeded(t *testing.T) {
	srv := makeErrorSDKServer()
	defer srv.Close()

	checker := newCheckerWithServer(t, srv)
	checker.now = time.Now
	checker.check(context.Background())

	status := checker.CurrentStatus()
	if status.Valid {
		t.Error("expected invalid status when SDK unreachable and never succeeded")
	}
	if !status.LastChecked.IsZero() {
		t.Error("expected LastChecked to be zero when SDK never succeeded")
	}
}

func TestChecker_SDKUnreachable_WithinGracePeriod(t *testing.T) {
	future := time.Now().Add(48 * time.Hour).UTC().Format(time.RFC3339)
	srv := makeFakeSDKServer(t, makeLicenseInfo(&future))

	fixedNow := time.Now()
	checker := newCheckerWithServer(t, srv)
	checker.now = func() time.Time { return fixedNow }
	checker.check(context.Background())

	status := checker.CurrentStatus()
	if !status.Valid {
		t.Fatalf("setup: expected valid status after first check, got: %s", status.Message)
	}

	srv.Close()

	withinGrace := fixedNow.Add(5 * time.Minute)
	checker.now = func() time.Time { return withinGrace }
	checker.check(context.Background())

	status = checker.CurrentStatus()
	if !status.Valid {
		t.Errorf("expected status to remain valid within grace period, got invalid: %s", status.Message)
	}
}

func TestChecker_SDKUnreachable_BeyondGracePeriod(t *testing.T) {
	future := time.Now().Add(48 * time.Hour).UTC().Format(time.RFC3339)
	srv := makeFakeSDKServer(t, makeLicenseInfo(&future))

	fixedNow := time.Now()
	checker := newCheckerWithServer(t, srv)
	checker.now = func() time.Time { return fixedNow }
	checker.check(context.Background())

	status := checker.CurrentStatus()
	if !status.Valid {
		t.Fatalf("setup: expected valid status after first check, got: %s", status.Message)
	}

	srv.Close()

	beyondGrace := fixedNow.Add(20 * time.Minute)
	checker.now = func() time.Time { return beyondGrace }
	checker.check(context.Background())

	status = checker.CurrentStatus()
	if status.Valid {
		t.Error("expected status to become invalid beyond grace period")
	}
	if !strings.Contains(status.Message, "unavailable") {
		t.Errorf("expected unavailability message, got: %s", status.Message)
	}
}
