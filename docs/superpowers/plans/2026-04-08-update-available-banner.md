# Update Available Banner Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Show a persistent "Update available" indicator in the top-right corner when the Replicated SDK reports new releases.

**Architecture:** Backend proxies the Replicated SDK `/api/v1/app/updates` endpoint via a new `GET /api/app/updates` route. Frontend calls this once on mount and conditionally renders a fixed-position banner.

**Tech Stack:** Go (chi router), React 19, plain CSS

---

## File Structure

| File | Action | Responsibility |
|------|--------|----------------|
| `backend/internal/updates/handler.go` | Create | HTTP handler that queries Replicated SDK for updates |
| `backend/internal/updates/handler_test.go` | Create | Tests for the updates handler |
| `backend/main.go` | Modify | Register `GET /api/app/updates` route |
| `frontend/src/api.js` | Modify | Add `checkForUpdates()` function |
| `frontend/src/App.jsx` | Modify | Call update check on mount, render banner |
| `frontend/src/App.css` | Modify | Add `.update-banner` style |

---

### Task 1: Backend updates handler

**Files:**
- Create: `backend/internal/updates/handler_test.go`
- Create: `backend/internal/updates/handler.go`

- [ ] **Step 1: Write the failing test**

Create `backend/internal/updates/handler_test.go`:

```go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd backend && go test ./internal/updates/ -v`
Expected: FAIL — package does not exist yet.

- [ ] **Step 3: Write the implementation**

Create `backend/internal/updates/handler.go`:

```go
package updates

import (
	"encoding/json"
	"net/http"
)

// Handler checks the Replicated SDK for available application updates.
type Handler struct {
	sdkEndpoint string
}

// NewHandler creates a Handler that queries the Replicated SDK at the given base URL.
func NewHandler(sdkEndpoint string) *Handler {
	return &Handler{sdkEndpoint: sdkEndpoint}
}

// Check queries the SDK and returns {"updatesAvailable": true/false}.
func (h *Handler) Check(w http.ResponseWriter, r *http.Request) {
	available := h.hasUpdates(r)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{
		"updatesAvailable": available,
	})
}

func (h *Handler) hasUpdates(r *http.Request) bool {
	url := h.sdkEndpoint + "/api/v1/app/updates"
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, url, nil)
	if err != nil {
		return false
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false
	}

	var releases []json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return false
	}

	return len(releases) > 0
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd backend && go test ./internal/updates/ -v`
Expected: All 3 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/updates/
git commit -m "Add updates handler to check Replicated SDK for available updates"
```

---

### Task 2: Register route in main.go

**Files:**
- Modify: `backend/main.go:13-18` (imports) and `backend/main.go:84-85` (routes)

- [ ] **Step 1: Add import**

In `backend/main.go`, add to the import block (after the `metrics` import on line 18):

```go
"github.com/assettrackerhq/asset-tracker/backend/internal/updates"
```

- [ ] **Step 2: Add route**

In `backend/main.go`, after line 84 (`r.Get("/api/auth/user-limit", authHandler.UserLimitInfo)`), add:

```go
	// Update check route
	updateHandler := updates.NewHandler(cfg.ReplicatedSDKEndpoint)
	r.Get("/api/app/updates", updateHandler.Check)
```

- [ ] **Step 3: Verify build**

Run: `cd backend && go build ./...`
Expected: Builds successfully with no errors.

- [ ] **Step 4: Commit**

```bash
git add backend/main.go
git commit -m "Register GET /api/app/updates route"
```

---

### Task 3: Frontend API function and banner

**Files:**
- Modify: `frontend/src/api.js` (add function after line 51)
- Modify: `frontend/src/App.jsx` (add state + banner)
- Modify: `frontend/src/App.css` (add style at end)

- [ ] **Step 1: Add `checkForUpdates` to api.js**

In `frontend/src/api.js`, after the `getUserLimit` function (after line 51), add:

```js
export async function checkForUpdates() {
  try {
    const res = await fetch(`${API_BASE}/app/updates`);
    if (!res.ok) return { updatesAvailable: false };
    return res.json();
  } catch {
    return { updatesAvailable: false };
  }
}
```

Note: This uses `fetch` directly (not the `request` helper) because it doesn't need auth token handling or the 401 redirect logic.

- [ ] **Step 2: Add banner to App.jsx**

Replace the full contents of `frontend/src/App.jsx` with:

```jsx
import { useState, useEffect } from 'react';
import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import Login from './pages/Login';
import Register from './pages/Register';
import AssetList from './pages/AssetList';
import AssetDetail from './pages/AssetDetail';
import { checkForUpdates } from './api';
import './App.css';

function ProtectedRoute({ children }) {
  const token = localStorage.getItem('token');
  if (!token) {
    return <Navigate to="/login" replace />;
  }
  return children;
}

export default function App() {
  const [updateAvailable, setUpdateAvailable] = useState(false);

  useEffect(() => {
    checkForUpdates().then((data) => {
      setUpdateAvailable(data.updatesAvailable);
    });
  }, []);

  return (
    <BrowserRouter>
      {updateAvailable && (
        <div className="update-banner">Update available</div>
      )}
      <Routes>
        <Route path="/login" element={<Login />} />
        <Route path="/register" element={<Register />} />
        <Route path="/assets" element={<ProtectedRoute><AssetList /></ProtectedRoute>} />
        <Route path="/assets/:id" element={<ProtectedRoute><AssetDetail /></ProtectedRoute>} />
        <Route path="*" element={<Navigate to="/assets" replace />} />
      </Routes>
    </BrowserRouter>
  );
}
```

- [ ] **Step 3: Add banner CSS**

In `frontend/src/App.css`, add at the end of the file:

```css
.update-banner {
  position: fixed;
  top: 16px;
  right: 16px;
  background: #2563eb;
  color: white;
  padding: 8px 16px;
  border-radius: 6px;
  font-size: 14px;
  font-weight: 500;
  z-index: 1000;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.15);
}
```

- [ ] **Step 4: Verify frontend builds**

Run: `cd frontend && npm run build`
Expected: Build succeeds with no errors.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/api.js frontend/src/App.jsx frontend/src/App.css
git commit -m "Show update-available banner when Replicated SDK reports new releases"
```
