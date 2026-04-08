# License Validity Enforcement via Replicated SDK

## Overview

The asset tracker must actively check license expiry and validity via the Replicated SDK. When the license is expired or invalid, the app blocks login and terminates active sessions. Normal operation resumes when a valid license is detected.

## Architecture

### Backend: Periodic License Checker with In-Memory Cache

A background goroutine polls the Replicated SDK `/api/v1/license/info` endpoint every 60 seconds. It stores the result in a thread-safe struct containing:

- `valid` (bool): whether the license is currently valid and not expired
- `message` (string): human-readable status (e.g., "License expired on 2026-03-01")
- `lastChecked` (time.Time): when the cache was last successfully updated

**Grace period**: If the SDK is unreachable for more than 15 minutes (based on `lastChecked`), the cached state is treated as invalid. This handles SDK downtime without allowing indefinite use of an expired license.

**License validity logic**: A license is considered valid when:
- The SDK responds successfully
- The license has no `ExpirationTime`, OR `ExpirationTime` is in the future

A license is considered invalid when:
- The SDK reports `ExpirationTime` in the past
- The SDK has been unreachable for more than 15 minutes

The checker is created in `backend/internal/license/checker.go` and started from `main.go` alongside the existing metrics reporter.

### Backend: License Status Client Method

Add a `LicenseInfo()` method to the existing `license.Client` in `backend/internal/license/client.go`. This method calls `GET /api/v1/license/info` and returns the parsed response including expiration time.

### Backend: License Middleware

A new HTTP middleware in `backend/internal/license/middleware.go` consults the checker's cached state on every request. If the license is invalid or expired, it returns:

```json
HTTP 403
{"error": "license_expired", "message": "Your license has expired. Contact your administrator."}
```

**Route coverage**:
- Applied to: `/api/auth/login`, `/api/auth/register`, `/api/auth/user-limit`, `/api/assets/*`
- Exempt: `/api/health`, `/api/app/updates`, `/api/license/status`

The middleware is applied before auth middleware so that license state is checked before token validation.

### Backend: Public License Status Endpoint

A new `GET /api/license/status` endpoint (public, no auth required) returns:

```json
{"valid": true}
```
or
```json
{"valid": false, "message": "Your license has expired. Contact your administrator."}
```

This allows the frontend to check license state before attempting login and to display status on the license-expired page.

### Frontend: 403 Handling in API Layer

In `frontend/src/api.js`, the central `request()` function is updated to intercept 403 responses containing `license_expired`. On receiving this error:

1. Clear the token from localStorage
2. Redirect to `/license-expired`

This ensures any logged-in user is immediately redirected when their next API call encounters an expired license.

### Frontend: License Expired Page

A new page at `/license-expired` (`frontend/src/pages/LicenseExpired.jsx`) displays:

- A clear heading: "License Expired"
- The message from the API (or a default)
- Instruction to contact their administrator
- A "Check Again" button that calls `/api/license/status` and redirects to `/login` if valid

Styled consistently with the existing login/register pages.

### Frontend: Login Page License Check

The login page calls `/api/license/status` on mount. If the license is invalid:

- Display a warning message above the login form
- Disable the login button

This prevents users from attempting login when it will be rejected, providing a better UX than a confusing 403 after submitting credentials.

## File Changes

| File | Change |
|---|---|
| `backend/internal/license/client.go` | Add `LicenseInfo()` method and response struct |
| `backend/internal/license/checker.go` | New file: background poller with thread-safe cache |
| `backend/internal/license/middleware.go` | New file: HTTP middleware checking cached license state |
| `backend/main.go` | Start checker goroutine, wire middleware and `/api/license/status` endpoint |
| `frontend/src/api.js` | Handle 403 `license_expired`, add `getLicenseStatus()` |
| `frontend/src/pages/LicenseExpired.jsx` | New file: license expired page |
| `frontend/src/pages/Login.jsx` | Check license status on mount, disable form if invalid |
| `frontend/src/App.jsx` | Add `/license-expired` route |
| `frontend/src/App.css` | Styles for license expired page (reuse existing patterns) |

## Error Handling

- SDK unreachable on first startup: license is treated as invalid from the start (no grace period for initial check). The app will become usable once the SDK responds with a valid license.
- SDK returns unexpected response format: treated as a check failure, grace period clock continues from last successful check.
- Network errors during polling: logged, cached state preserved until grace period expires.

## Testing Approach

- **Valid license**: Start app with valid license, verify login works, assets accessible.
- **Expired license**: Set license expiry in the past, verify login blocked, active sessions terminated on next API call, license-expired page shown.
- **SDK unavailable**: Stop SDK, verify app continues working for up to 15 minutes, then blocks access.
- **License renewal**: Transition from expired to valid, verify "Check Again" button works, login re-enabled.
