# License Validity Enforcement Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Actively check license validity via the Replicated SDK and block login / invalidate sessions when the license is expired or invalid.

**Architecture:** A background goroutine polls the SDK's `/api/v1/license/info` every 60s, caching the result. A license middleware gates all routes except health/updates/license-status. The frontend intercepts 403 `license_expired` responses and redirects to a dedicated page.

**Tech Stack:** Go (chi router, net/http), React 19, React Router v7, Replicated SDK REST API

---

### Task 1: Add LicenseInfo method to existing license client

**Files:**
- Modify: `backend/internal/license/client.go`

- [ ] **Step 1: Add the LicenseInfo response struct and method**

Add the following to `backend/internal/license/client.go` after the existing `licenseFieldResponse` struct:

```go
// LicenseInfoResponse represents the response from the SDK license info endpoint.
type LicenseInfoResponse struct {
	LicenseID      string  `json:"license_id"`
	LicenseType    string  `json:"license_type"`
	ExpirationTime *string `json:"expiration_time"`
}

// LicenseInfo queries the SDK for the full license info.
func (c *Client) LicenseInfo(ctx context.Context) (*LicenseInfoResponse, error) {
	url := c.sdkEndpoint + "/api/v1/license/info"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("license: failed to create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("license: failed to query SDK: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("license: SDK returned status %d", resp.StatusCode)
	}

	var info LicenseInfoResponse
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("license: failed to decode response: %w", err)
	}

	return &info, nil
}
```

- [ ] **Step 2: Verify it compiles**

Run: `cd /Users/jdewinne/conductor/workspaces/asset-tracker/pattaya/backend && go build ./...`
Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add backend/internal/license/client.go
git commit -m "feat: add LicenseInfo method to license client"
```

---

### Task 2: Create license checker with background polling and cache

**Files:**
- Create: `backend/internal/license/checker.go`

- [ ] **Step 1: Create the checker file**

Create `backend/internal/license/checker.go`:

```go
package license

import (
	"context"
	"log"
	"sync"
	"time"
)

const (
	checkInterval = 60 * time.Second
	gracePeriod   = 15 * time.Minute
)

// Status represents the cached license state.
type Status struct {
	Valid       bool
	Message     string
	LastChecked time.Time
}

// Checker periodically polls the SDK and caches license validity.
type Checker struct {
	client *Client
	mu     sync.RWMutex
	status Status
	now    func() time.Time // for testing
}

// NewChecker creates a Checker that polls the given license client.
func NewChecker(client *Client) *Checker {
	return &Checker{
		client: client,
		now:    time.Now,
		status: Status{Valid: false, Message: "License status unknown — waiting for first check"},
	}
}

// Run polls the SDK on the given interval until the context is cancelled.
// It performs an immediate check before entering the loop.
func (c *Checker) Run(ctx context.Context) {
	c.check(ctx)
	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.check(ctx)
		}
	}
}

func (c *Checker) check(ctx context.Context) {
	info, err := c.client.LicenseInfo(ctx)
	if err != nil {
		log.Printf("license checker: SDK unreachable: %v", err)
		c.mu.Lock()
		defer c.mu.Unlock()
		// If we've never had a successful check, stay invalid
		if c.status.LastChecked.IsZero() {
			return
		}
		// Grace period: if last successful check was recent enough, keep current state
		if c.now().Sub(c.status.LastChecked) > gracePeriod {
			c.status.Valid = false
			c.status.Message = "License validation unavailable — please check your connection"
		}
		return
	}

	valid, message := evaluateLicense(info, c.now())

	c.mu.Lock()
	defer c.mu.Unlock()
	c.status = Status{
		Valid:       valid,
		Message:     message,
		LastChecked: c.now(),
	}
}

func evaluateLicense(info *LicenseInfoResponse, now time.Time) (bool, string) {
	if info.ExpirationTime == nil || *info.ExpirationTime == "" {
		return true, ""
	}
	expiry, err := time.Parse(time.RFC3339, *info.ExpirationTime)
	if err != nil {
		return false, "License has an invalid expiration date"
	}
	if now.After(expiry) {
		return false, "Your license expired on " + expiry.Format("January 2, 2006") + ". Contact your administrator."
	}
	return true, ""
}

// CurrentStatus returns the cached license status.
func (c *Checker) CurrentStatus() Status {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.status
}
```

- [ ] **Step 2: Verify it compiles**

Run: `cd /Users/jdewinne/conductor/workspaces/asset-tracker/pattaya/backend && go build ./...`
Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add backend/internal/license/checker.go
git commit -m "feat: add license checker with background polling and grace period"
```

---

### Task 3: Create license middleware

**Files:**
- Create: `backend/internal/license/middleware.go`

- [ ] **Step 1: Create the middleware file**

Create `backend/internal/license/middleware.go`:

```go
package license

import (
	"encoding/json"
	"net/http"
)

type licenseExpiredResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

// Middleware returns an HTTP middleware that blocks requests when the license is invalid.
func LicenseMiddleware(checker *Checker) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			status := checker.CurrentStatus()
			if !status.Valid {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				msg := status.Message
				if msg == "" {
					msg = "Your license is invalid. Contact your administrator."
				}
				json.NewEncoder(w).Encode(licenseExpiredResponse{
					Error:   "license_expired",
					Message: msg,
				})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
```

- [ ] **Step 2: Verify it compiles**

Run: `cd /Users/jdewinne/conductor/workspaces/asset-tracker/pattaya/backend && go build ./...`
Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add backend/internal/license/middleware.go
git commit -m "feat: add license enforcement middleware"
```

---

### Task 4: Wire checker, middleware, and status endpoint into main.go

**Files:**
- Modify: `backend/main.go`

- [ ] **Step 1: Start the license checker goroutine**

In `backend/main.go`, after the line `licenseClient := license.NewClient(cfg.ReplicatedSDKEndpoint)` (line 79), add:

```go
	// Start license validity checker
	licenseChecker := license.NewChecker(licenseClient)
	go licenseChecker.Run(ctx)
```

- [ ] **Step 2: Add the license status endpoint**

After the updates route block (after line 89 `r.Get("/api/app/updates", updateHandler.Check)`), add:

```go
	// License status route (public, exempt from license middleware)
	r.Get("/api/license/status", func(w http.ResponseWriter, r *http.Request) {
		status := licenseChecker.CurrentStatus()
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]any{"valid": status.Valid}
		if !status.Valid {
			resp["message"] = status.Message
		}
		json.NewEncoder(w).Encode(resp)
	})
```

- [ ] **Step 3: Apply license middleware to auth routes**

Replace the current auth route block (lines 82-85):

```go
	// Auth routes
	authHandler := auth.NewHandler(pool, cfg.JWTSecret, licenseClient)
	r.Post("/api/auth/register", authHandler.Register)
	r.Post("/api/auth/login", authHandler.Login)
	r.Get("/api/auth/user-limit", authHandler.UserLimitInfo)
```

With:

```go
	// Auth routes (protected by license check)
	authHandler := auth.NewHandler(pool, cfg.JWTSecret, licenseClient)
	r.Group(func(r chi.Router) {
		r.Use(license.LicenseMiddleware(licenseChecker))
		r.Post("/api/auth/register", authHandler.Register)
		r.Post("/api/auth/login", authHandler.Login)
		r.Get("/api/auth/user-limit", authHandler.UserLimitInfo)
	})
```

- [ ] **Step 4: Apply license middleware to asset routes**

Replace the asset route block (lines 91-106):

```go
	// Asset routes (protected)
	assetHandler := assets.NewHandler(pool)
	valueHandler := values.NewHandler(pool)
	r.Route("/api/assets", func(r chi.Router) {
		r.Use(auth.Middleware(cfg.JWTSecret))
		r.Get("/", assetHandler.List)
		r.Post("/", assetHandler.Create)
		r.Put("/{id}", assetHandler.Update)
		r.Delete("/{id}", assetHandler.Delete)

		// Value point routes
		r.Get("/{assetID}/values", valueHandler.List)
		r.Post("/{assetID}/values", valueHandler.Create)
		r.Put("/{assetID}/values/{valueID}", valueHandler.Update)
		r.Delete("/{assetID}/values/{valueID}", valueHandler.Delete)
	})
```

With:

```go
	// Asset routes (protected by license check + auth)
	assetHandler := assets.NewHandler(pool)
	valueHandler := values.NewHandler(pool)
	r.Route("/api/assets", func(r chi.Router) {
		r.Use(license.LicenseMiddleware(licenseChecker))
		r.Use(auth.Middleware(cfg.JWTSecret))
		r.Get("/", assetHandler.List)
		r.Post("/", assetHandler.Create)
		r.Put("/{id}", assetHandler.Update)
		r.Delete("/{id}", assetHandler.Delete)

		// Value point routes
		r.Get("/{assetID}/values", valueHandler.List)
		r.Post("/{assetID}/values", valueHandler.Create)
		r.Put("/{assetID}/values/{valueID}", valueHandler.Update)
		r.Delete("/{assetID}/values/{valueID}", valueHandler.Delete)
	})
```

- [ ] **Step 5: Verify it compiles**

Run: `cd /Users/jdewinne/conductor/workspaces/asset-tracker/pattaya/backend && go build ./...`
Expected: No errors

- [ ] **Step 6: Commit**

```bash
git add backend/main.go
git commit -m "feat: wire license checker, middleware, and status endpoint"
```

---

### Task 5: Update frontend API layer to handle license_expired 403

**Files:**
- Modify: `frontend/src/api.js`

- [ ] **Step 1: Add license_expired handling to the request function**

In `frontend/src/api.js`, replace the `request` function (lines 3-33) with:

```javascript
async function request(path, options = {}) {
  const token = localStorage.getItem('token');
  const headers = {
    'Content-Type': 'application/json',
    ...options.headers,
  };
  if (token) {
    headers['Authorization'] = `Bearer ${token}`;
  }

  const response = await fetch(`${API_BASE}${path}`, {
    ...options,
    headers,
  });

  if (response.status === 403) {
    const data = await response.json().catch(() => ({}));
    if (data.error === 'license_expired') {
      localStorage.removeItem('token');
      window.location.href = '/license-expired';
      return;
    }
    throw new Error(data.error || 'Forbidden');
  }

  if (response.status === 401) {
    localStorage.removeItem('token');
    window.location.href = '/login';
    return;
  }

  if (response.status === 204) {
    return null;
  }

  const data = await response.json();
  if (!response.ok) {
    throw new Error(data.error || 'Request failed');
  }
  return data;
}
```

- [ ] **Step 2: Add getLicenseStatus function**

Add the following export at the end of `frontend/src/api.js` (after the `deleteValuePoint` export):

```javascript
export async function getLicenseStatus() {
  try {
    const res = await fetch(`${API_BASE}/license/status`);
    if (!res.ok) return { valid: true };
    return await res.json();
  } catch {
    return { valid: true };
  }
}
```

- [ ] **Step 3: Commit**

```bash
git add frontend/src/api.js
git commit -m "feat: handle license_expired 403 in API layer and add getLicenseStatus"
```

---

### Task 6: Create LicenseExpired page

**Files:**
- Create: `frontend/src/pages/LicenseExpired.jsx`
- Modify: `frontend/src/App.css`

- [ ] **Step 1: Create the LicenseExpired page component**

Create `frontend/src/pages/LicenseExpired.jsx`:

```jsx
import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { getLicenseStatus } from '../api';

export default function LicenseExpired() {
  const [checking, setChecking] = useState(false);
  const [message, setMessage] = useState('');
  const navigate = useNavigate();

  async function handleCheckAgain() {
    setChecking(true);
    setMessage('');
    try {
      const status = await getLicenseStatus();
      if (status.valid) {
        navigate('/login');
      } else {
        setMessage(status.message || 'License is still invalid.');
      }
    } catch {
      setMessage('Unable to check license status.');
    } finally {
      setChecking(false);
    }
  }

  return (
    <div className="auth-form">
      <h1>License Expired</h1>
      <p className="license-expired-text">
        Your license has expired or is invalid. Access to the application is
        currently unavailable.
      </p>
      <p className="license-expired-text">
        Please contact your administrator to renew your license.
      </p>
      {message && <p className="error">{message}</p>}
      <button className="primary" onClick={handleCheckAgain} disabled={checking}>
        {checking ? 'Checking...' : 'Check Again'}
      </button>
    </div>
  );
}
```

- [ ] **Step 2: Add styles for the license expired page**

Add the following to the end of `frontend/src/App.css`:

```css
.license-expired-text {
  color: #6b7280;
  margin-bottom: 16px;
  line-height: 1.5;
}
```

- [ ] **Step 3: Commit**

```bash
git add frontend/src/pages/LicenseExpired.jsx frontend/src/App.css
git commit -m "feat: add LicenseExpired page"
```

---

### Task 7: Add license-expired route and license check on Login page

**Files:**
- Modify: `frontend/src/App.jsx`
- Modify: `frontend/src/pages/Login.jsx`

- [ ] **Step 1: Add the license-expired route to App.jsx**

Replace the entire content of `frontend/src/App.jsx` with:

```jsx
import { useState, useEffect } from 'react';
import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import Login from './pages/Login';
import Register from './pages/Register';
import AssetList from './pages/AssetList';
import AssetDetail from './pages/AssetDetail';
import LicenseExpired from './pages/LicenseExpired';
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
        <Route path="/license-expired" element={<LicenseExpired />} />
        <Route path="/assets" element={<ProtectedRoute><AssetList /></ProtectedRoute>} />
        <Route path="/assets/:id" element={<ProtectedRoute><AssetDetail /></ProtectedRoute>} />
        <Route path="*" element={<Navigate to="/assets" replace />} />
      </Routes>
    </BrowserRouter>
  );
}
```

- [ ] **Step 2: Add license check to Login page**

Replace the entire content of `frontend/src/pages/Login.jsx` with:

```jsx
import { useState, useEffect } from 'react';
import { useNavigate, Link } from 'react-router-dom';
import { login, getLicenseStatus } from '../api';

export default function Login() {
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [licenseValid, setLicenseValid] = useState(true);
  const [licenseMessage, setLicenseMessage] = useState('');
  const navigate = useNavigate();

  useEffect(() => {
    getLicenseStatus().then((status) => {
      setLicenseValid(status.valid);
      if (!status.valid) {
        setLicenseMessage(status.message || 'License is invalid.');
      }
    });
  }, []);

  async function handleSubmit(e) {
    e.preventDefault();
    setError('');
    try {
      const data = await login(username, password);
      localStorage.setItem('token', data.token);
      navigate('/assets');
    } catch (err) {
      setError(err.message);
    }
  }

  return (
    <div className="auth-form">
      <h1>Login</h1>
      {!licenseValid && (
        <p className="error">{licenseMessage}</p>
      )}
      {error && <p className="error">{error}</p>}
      <form onSubmit={handleSubmit}>
        <div className="form-group">
          <label>Username</label>
          <input value={username} onChange={(e) => setUsername(e.target.value)} required />
        </div>
        <div className="form-group">
          <label>Password</label>
          <input type="password" value={password} onChange={(e) => setPassword(e.target.value)} required />
        </div>
        <button type="submit" className="primary" disabled={!licenseValid}>Login</button>
      </form>
      <p style={{ marginTop: '16px' }}>
        Don't have an account? <Link to="/register">Register</Link>
      </p>
    </div>
  );
}
```

- [ ] **Step 3: Commit**

```bash
git add frontend/src/App.jsx frontend/src/pages/Login.jsx
git commit -m "feat: add license-expired route and license check on login page"
```

---

### Task 8: Verify full backend compiles and frontend builds

**Files:**
- No new files

- [ ] **Step 1: Verify backend compiles**

Run: `cd /Users/jdewinne/conductor/workspaces/asset-tracker/pattaya/backend && go build ./...`
Expected: No errors

- [ ] **Step 2: Verify frontend builds**

Run: `cd /Users/jdewinne/conductor/workspaces/asset-tracker/pattaya/frontend && npm run build`
Expected: Build succeeds with no errors

- [ ] **Step 3: Verify no linting issues**

Run: `cd /Users/jdewinne/conductor/workspaces/asset-tracker/pattaya/backend && go vet ./...`
Expected: No issues
