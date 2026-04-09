# Preflight Checks & Email Verification Design

## Overview

Add Replicated preflight checks covering 5 deployment concerns, plus email verification on signup with Mailpit as a development SMTP server.

## Preflight Checks

A single `templates/preflight.yaml` in the Helm chart, defining a `troubleshoot.sh/v1beta2` Preflight resource packaged as a Secret (standard Replicated Helm pattern).

### Check 1: Database Connectivity

- **Collector**: `postgresql` with connection URI from `_helpers.tpl`
- **Condition**: Only when `postgresql.enabled` is false (external DB configured)
- **Analyzer**: `postgres` analyzer checking `isConnected`
- **Fail message**: "Cannot connect to PostgreSQL at <host>:<port>. Verify the database is running, the credentials are correct, and the host is reachable from the cluster."
- **Pass message**: "Successfully connected to external PostgreSQL database."

### Check 2: SMTP Endpoint Connectivity

- **Collector**: `tcpConnect` to `smtp.host:smtp.port`
- **Condition**: Only when `smtp.host` is non-empty
- **Analyzer**: `tcpConnect` analyzer checking connection-refused vs connected
- **Fail message**: "Cannot reach SMTP server at <host>:<port>. Verify the SMTP server is running and accessible from the cluster."
- **Pass message**: "SMTP server is reachable."

### Check 3: Cluster Resources

- **Analyzer**: `nodeResources`
- **CPU check**: `sum(cpuAllocatable) >= 1` — fail message: "Cluster has insufficient CPU. At least 1 CPU allocatable is required."
- **Memory check**: `sum(memoryAllocatable) >= 2Gi` — fail message: "Cluster has insufficient memory. At least 2Gi allocatable memory is required."
- **Pass message**: "Cluster has sufficient CPU and memory resources."

### Check 4: Kubernetes Version

- **Analyzer**: `clusterVersion`
- **Fail when**: `< 1.30`
- **Fail message**: "Kubernetes version is not supported. Minimum required version is 1.30. Upgrade your cluster before installing."
- **Pass message**: "Kubernetes version meets the minimum requirement of 1.30."

### Check 5: Distribution Check

- **Analyzer**: `distribution`
- **Fail on**: `docker-desktop`, `microk8s`
- **Fail messages**:
  - "Docker Desktop is not a supported Kubernetes distribution. See https://docs.assettracker.com/install/supported-clusters for supported options."
  - "MicroK8s is not a supported Kubernetes distribution. See https://docs.assettracker.com/install/supported-clusters for supported options."
- **Pass message**: "Supported Kubernetes distribution detected."

## Email Verification

### Database Changes

**users table** — add two columns in `schemas/tables/users.yaml`:
- `email` (VARCHAR 255, not null, unique)
- `email_verified` (BOOLEAN, default false)

**New table** `verification_codes` in `schemas/tables/verification_codes.yaml`:
- `id` (UUID, PK, auto-generated)
- `user_id` (UUID, not null)
- `code` (VARCHAR 6, not null)
- `expires_at` (TIMESTAMP, not null)
- `created_at` (TIMESTAMP, default now())

### Backend Changes

**New package** `internal/email/`:
- `sender.go` — SMTP client using Go's `net/smtp`. Supports authenticated (when username/password set) and unauthenticated modes. Configured via `SMTP_HOST`, `SMTP_PORT`, `SMTP_FROM`, `SMTP_USERNAME`, `SMTP_PASSWORD` env vars.
- `verification.go` — Generates 6-digit codes, stores in `verification_codes` table, sends email, validates submitted codes. Codes expire after 15 minutes.

**Modified** `internal/auth/`:
- Registration handler requires `email` field. After creating user, generates and sends verification code. Returns `user_id` in response for the verification step.
- Login checks `email_verified` — returns error with hint to verify if not verified.

**New endpoints**:
- `POST /api/auth/verify-email` — accepts `{ "user_id": "...", "code": "123456" }`, marks email as verified
- `POST /api/auth/resend-verification` — accepts `{ "user_id": "..." }`, generates new code and resends

### Frontend Changes

- Signup form adds `email` field
- After signup, redirect to verification screen with 6-digit code input and "Resend code" button
- Login shows actionable error if email not verified, with link to resend

## Mailpit Integration

### Helm Chart

- `templates/mailpit-deployment.yaml` — Deployment running `axllent/mailpit:latest`, ports 1025 (SMTP) and 8025 (web UI). Conditional on `mailpit.enabled`.
- `templates/mailpit-service.yaml` — ClusterIP service for both ports. Same conditional.

### Values.yaml Additions

```yaml
smtp:
  host: ""
  port: 587
  username: ""
  password: ""
  from: "noreply@assettracker.local"

mailpit:
  enabled: false
  image:
    repository: axllent/mailpit
    tag: latest
```

### Auto-wiring Logic

When `mailpit.enabled` is true and `smtp.host` is empty, the backend SMTP env vars auto-resolve to the Mailpit service (`<release>-asset-tracker-mailpit:1025`, no auth). For production: `mailpit.enabled: false`, configure `smtp.*` with real values.

### Backend Deployment Template

Add env vars to `backend-deployment.yaml`:
- `SMTP_HOST` — Mailpit service name when mailpit enabled, otherwise `smtp.host`
- `SMTP_PORT` — 1025 when mailpit enabled, otherwise `smtp.port`
- `SMTP_FROM` — from `smtp.from`
- `SMTP_USERNAME` — from `smtp.username` (empty when mailpit)
- `SMTP_PASSWORD` — from `smtp.password` (empty when mailpit)

## Playwright Test Updates

- Use Mailpit's REST API (`GET /api/v1/messages`, `GET /api/v1/message/{id}`) to retrieve verification codes
- Mailpit URL configured via environment variable in Playwright config
- After signup in tests: call Mailpit API, extract 6-digit code from email body, submit on verification screen
- Update existing auth tests to include verification step
- Add test for resend-verification flow

## Demonstration Plan

Show preflights running twice per acceptance criteria:

1. **All checks failing** — run against an environment with unreachable external DB, unreachable SMTP, insufficient resources, old k8s version, and unsupported distribution (docker-desktop). Show clear fail messages for each.
2. **All checks passing** — run against a properly configured environment. Show pass messages.

This will be demonstrated via the preflight YAML itself and documented expected output, since actually running preflights requires a live cluster with the `kubectl preflight` plugin.
