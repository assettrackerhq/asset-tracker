# Update Available Banner

## Overview

Show a persistent "Update available" indicator in the top-right corner of the app when the Replicated SDK reports available updates. Purely informational — no links, no version details.

## Backend

### New endpoint: `GET /api/app/updates`

New package `backend/internal/updates/` with a handler that proxies the Replicated SDK.

**Handler logic:**
1. GET `{REPLICATED_SDK_ENDPOINT}/api/v1/app/updates`
2. Parse the JSON array response
3. Return `{ "updatesAvailable": true }` if the array is non-empty, `{ "updatesAvailable": false }` otherwise

**SDK response format** (from Replicated docs):
```json
[
  {
    "versionLabel": "0.1.15",
    "createdAt": "2023-05-12T15:48:45.000Z",
    "releaseNotes": "Awesome new features!"
  }
]
```

**Error handling:** If the SDK is unreachable, return `{ "updatesAvailable": false }`. Same resilient pattern as the existing `license.Client` (defaults gracefully when SDK is down).

**No authentication required** on this endpoint — update availability is not sensitive information, consistent with the existing `/api/auth/user-limit` public endpoint.

### Route registration

Add `GET /api/app/updates` in `main.go`, following the existing routing pattern.

### Reuse existing config

Use `config.SDKEndpoint` (already reads `REPLICATED_SDK_ENDPOINT` env var, defaults to `http://asset-tracker-sdk:3000`). No new configuration needed.

## Frontend

### API layer

Add `checkForUpdates()` to `api.js`:
```js
export async function checkForUpdates() {
  const res = await fetch(`${API_BASE}/app/updates`);
  if (!res.ok) return { updatesAvailable: false };
  return res.json();
}
```

No auth token needed for this call.

### App layout

In `App.jsx`, call `checkForUpdates()` once on mount. If `updatesAvailable` is true, render an indicator in the top-right corner visible on all pages.

```jsx
<div className="update-banner">Update available</div>
```

### Styling

Small, non-intrusive indicator in the top-right corner. Uses existing color conventions — blue info style or a subtle accent. Fixed position so it persists across page navigation.

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
  z-index: 1000;
}
```

## Files to create/modify

| File | Action |
|------|--------|
| `backend/internal/updates/handler.go` | Create — update check handler |
| `backend/main.go` | Modify — register new route |
| `frontend/src/api.js` | Modify — add `checkForUpdates()` |
| `frontend/src/App.jsx` | Modify — call API on mount, render banner |
| `frontend/src/App.css` | Modify — add `.update-banner` style |

## Out of scope

- Version numbers or release notes in the banner
- Dismissibility / "don't show again"
- Periodic re-polling (single check on app load is sufficient)
- Admin-only visibility (shown to all users)
