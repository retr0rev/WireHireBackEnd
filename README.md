# WireHire Backend

Go API server powering the WireHire job board platform. Serves public job listings, employer management, and admin moderation.

## Stack

- **Go 1.25** + Chi router
- **SQLite** (via `database/sql`)
- **JWT** (HS256, `golang-jwt/jwt/v5`)
- **bcrypt** password hashing
- **golang.org/x/time/rate** IP-based rate limiting

## Quick Start

```bash
# Set required env var
export JWT_SECRET=$(openssl rand -hex 32)

# Run
go run ./cmd/server

# Seed admin account
go run ./cmd/server seed-admin --email=admin@wirehire.com --password=YourSecurePass123!
```

Server starts on `:8080` by default (set `PORT` to override).

## Environment Variables

| Variable | Required | Default | Description |
|---|---|---|---|---|
| `JWT_SECRET` | ✅ | — | Min 32 chars. Generate: `openssl rand -hex 32` |
| `DB_PATH` | — | `./database/database.db` | SQLite file path |
| `CORS_ORIGIN` | — | `*` (dev) | Comma-separated origins for production |
| `TLS_ENABLED` | — | `false` | Set to `true` in production |
| `COOKIE_DOMAIN` | — | (from `TLS_ENABLED`) | `.wirehire.com` for cross-subdomain cookies |
| `PORT` | — | `8080` | Server port |
| `APP_URL` | — | `http://localhost:8080` | Base URL for email links |
| `RESEND_API_KEY` | — | — | Resend email (falls back to SMTP or console) |
| `SMTP_HOST` | — | — | SMTP host (with `SMTP_PORT`, `SMTP_USER`, `SMTP_PASS`) |
| `R2_ACCOUNT_ID` | — | — | Cloudflare R2 account ID (required for image upload) |
| `R2_ACCESS_KEY_ID` | — | — | R2 API access key ID |
| `R2_SECRET_ACCESS_KEY` | — | — | R2 API secret access key |
| `R2_BUCKET_NAME` | — | — | R2 bucket name (e.g. `wirehire-images`) |
| `R2_PUBLIC_URL` | — | — | R2 bucket public URL or custom domain (e.g. `https://images.wirehire.com`) |

## API Endpoints

### Public
| Method | Path | Auth | Description |
|---|---|---|---|
| GET | `/health` | No | Health check |
| GET | `/api/public/jobs` | No | Approved job listings |
| GET | `/api/public/employers` | No | Verified employers |

### Auth
| Method | Path | Auth | Rate Limit |
|---|---|---|---|
| POST | `/api/auth/signup` | No | 0.2 rps |
| POST | `/api/auth/login` | No | 0.2 rps |
| GET | `/api/auth/verify` | No | 0.2 rps |
| POST | `/api/auth/forgot-password` | No | 0.1 rps |
| POST | `/api/auth/reset-password` | No | 0.1 rps |
| GET | `/api/auth/me` | Client | — |
| PATCH | `/api/auth/me` | Client | — |
| POST | `/api/auth/logout` | Client | — |

### Jobs (Employer)
| Method | Path | Auth |
|---|---|---|
| GET | `/api/jobs` | Client |
| POST | `/api/jobs` | Client |
| GET | `/api/jobs/{id}` | Client |
| PUT | `/api/jobs/{id}` | Client |
| DELETE | `/api/jobs/{id}` | Client |

### Admin
| Method | Path | Auth |
|---|---|---|
| POST | `/api/admin/login` | No |
| GET | `/api/admin/me` | Admin |
| POST | `/api/admin/logout` | Admin |
| GET | `/api/admin/jobs` | Admin |
| PUT | `/api/admin/jobs/{id}/status` | Admin |
| DELETE | `/api/admin/jobs/{id}` | Admin |
| GET | `/api/admin/employers/pending` | Admin |
| PUT | `/api/admin/employers/{id}/verify` | Admin |
| GET | `/api/admin/employers` | Super Admin |
| GET | `/api/admin/employers/{id}` | Super Admin |
| POST | `/api/admin/employers` | Super Admin |
| PATCH | `/api/admin/employers/{id}` | Super Admin |
| DELETE | `/api/admin/employers/{id}` | Super Admin |
| GET | `/api/admin/admins` | Super Admin |
| POST | `/api/admin/admins` | Super Admin |

## Authentication

- JWT tokens with 72h expiry, signed with HS256
- Dual delivery: `Authorization: Bearer <token>` header + httpOnly cookie
- Cookie attributes: `Path=/`, `HttpOnly`, `Secure` (if TLS), `SameSite=None` (prod)
- Logout clears the cookie server-side

## Rate Limiting

All endpoints are rate-limited by IP. Limits scale to production via `X-Forwarded-For` header.

## Deployment (Railway)

The `railway.json` config handles Go build + deploy. Required env vars:
- `JWT_SECRET` — generate a strong random key
- `DB_PATH` — set to `/data/wirehire.db` (Railway persistent volume)
- `CORS_ORIGIN` — your Vercel frontend URLs
- `TLS_ENABLED` — `true`
- `COOKIE_DOMAIN` — your shared cookie domain

Seed admin after deploy:
```bash
go run ./cmd/server seed-admin --email=admin@wirehire.com --password=YourSecurePass123!
```

## Architecture

```
cmd/server/main.go          — entry point, routing
internal/
  handlers/                  — HTTP handlers
  middleware/                 — auth, CORS, rate limiting, security headers, validation
  models/                     — domain types
  repository/                 — database access layer
  email/                      — email sender (Resend, SMTP, console)
pkg/database/                 — SQLite connection
```
