# Asset Tracker Web Application — Design Spec

## Overview

A personal asset tracking web application. Users register with username/password, then manage their assets and track asset values over time.

## Architecture

```
┌─────────────┐     HTTP/JSON     ┌──────────────┐     SQL      ┌──────────┐
│  React+Vite │  ◄──────────────► │  Go API      │ ◄──────────► │ Postgres │
│  (frontend) │                   │  (chi router) │              │          │
└─────────────┘                   └──────────────┘              └──────────┘
```

### Project Structure

```
/frontend        — React + Vite app
/backend         — Go REST API
/schemas         — SchemaHero table YAML definitions
docker-compose.yml — Postgres, SchemaHero, frontend, backend
```

### Services

| Service | Port | Description |
|---|---|---|
| Frontend | 5173 | React + Vite dev server |
| Backend | 8080 | Go REST API |
| Postgres | 5432 | Database |
| SchemaHero | — | Declarative schema migrations |

## Database Schema

Three tables. No foreign key constraints in the database — relationships are enforced at the application layer.

### users

| Column | Type | Notes |
|---|---|---|
| id | UUID | PK, auto-generated |
| username | VARCHAR(255) | Unique, not null |
| password_hash | VARCHAR(255) | bcrypt hash |
| created_at | TIMESTAMP | Default now() |

### assets

| Column | Type | Notes |
|---|---|---|
| id | VARCHAR(50) | PK (composite with user_id) |
| user_id | UUID | Logical FK to users.id, not null |
| name | VARCHAR(255) | Not null |
| description | TEXT | Nullable |
| created_at | TIMESTAMP | Default now() |
| updated_at | TIMESTAMP | Default now() |

Unique constraint on `(id, user_id)` — different users can reuse the same asset ID.

### asset_value_points

| Column | Type | Notes |
|---|---|---|
| id | UUID | PK, auto-generated |
| asset_id | VARCHAR(50) | Logical FK to assets (composite with user_id) |
| user_id | UUID | Logical FK to users.id |
| timestamp | TIMESTAMP | Not null, default now() |
| value | DECIMAL(15,2) | Not null |
| currency | VARCHAR(3) | ISO 4217 code (e.g., "USD", "EUR") |

## REST API

### Auth Endpoints

| Method | Path | Description |
|---|---|---|
| POST | /api/auth/register | Create new user |
| POST | /api/auth/login | Login, returns JWT |

### Asset Endpoints (JWT required)

| Method | Path | Description |
|---|---|---|
| GET | /api/assets | List current user's assets |
| POST | /api/assets | Create asset (user provides ID) |
| PUT | /api/assets/:id | Update asset name/description |
| DELETE | /api/assets/:id | Delete asset |

### Value Point Endpoints (JWT required)

| Method | Path | Description |
|---|---|---|
| GET | /api/assets/:id/values | List value points for an asset |
| POST | /api/assets/:id/values | Add a value point |
| PUT | /api/assets/:id/values/:valueId | Update a value point |
| DELETE | /api/assets/:id/values/:valueId | Delete a value point |

### Response Format

- All responses are JSON
- Auth middleware extracts user_id from JWT, scopes all queries to that user
- Error responses: 400 (bad request), 401 (unauthorized), 404 (not found), 409 (conflict for duplicate asset ID)

## Authentication

### Registration Flow

1. User submits username + password
2. Backend validates: username must be unique, password minimum 8 characters
3. Password hashed with bcrypt (cost 12)
4. User row inserted, JWT returned

### Login Flow

1. User submits username + password
2. Backend looks up user by username
3. Compares bcrypt hash
4. Returns JWT (expires in 24h)

### JWT

- `sub`: user ID (UUID)
- `exp`: expiration timestamp
- Signed with a server-side secret (configured via `JWT_SECRET` environment variable)

### Auth Middleware

- All `/api/assets*` routes go through auth middleware
- Middleware validates JWT, extracts user_id, attaches to request context
- Invalid/expired token returns 401

## Frontend

### Routes

| Route | Page | Description |
|---|---|---|
| /login | Login | Username/password form |
| /register | Register | Registration form |
| /assets | Asset List | Table of user's assets with add/edit/delete |
| /assets/:id | Asset Detail | Asset info + value points table with add/edit/delete |

### Behavior

- JWT stored in localStorage, sent as `Authorization: Bearer <token>` header
- Unauthenticated users redirected to /login
- Simple, functional UI — no CSS framework, clean minimal styles
- Asset list shows columns: ID, name, description
- Asset detail page shows asset info at top, value points table below (timestamp, value, currency)

## Tech Stack

| Layer | Technology |
|---|---|
| Frontend | React + Vite |
| Routing (FE) | react-router |
| Backend | Go + chi router |
| DB driver | pgx |
| DB | PostgreSQL |
| Migrations | SchemaHero (declarative YAML) |
| Auth | bcrypt + JWT |
| Dev environment | docker-compose |
