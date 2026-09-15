# Ticket System – EVA Bharat Backend Intern Assignment

**Author:** Dev Vashist
**Repository:** https://github.com/decode2211/evaassignment

---

## 0. Live Deployment

- **App:** `<DEPLOYED_URL>`
- **Health check:** `<DEPLOYED_URL>/health`

*(These are placeholders until the app is deployed - see the [Render deployment guide](#12-deployment-guide-render-free-tier-docker) below.)*

---

## 1. What This Project Is

This is a small backend service for a ticket system - the kind of thing a support team might use to track issues. A person creates an account, logs in, and can then raise tickets describing a problem. Every user can only ever see and manage the tickets *they* created - nobody can peek at or change someone else's tickets.

A ticket's life is simple and one-directional: it starts **open**, can move to **in_progress** while someone works on it, and finally to **closed** once it's resolved (a ticket can also jump straight from open to closed). Once a ticket is closed, it stays closed forever - it can never be reopened. There is no admin role, no assigning tickets to other people, and no comment threads; the goal was to build a small, correct system rather than a feature-heavy one.

## 2. Live Deployment URL

See [section 0](#0-live-deployment) above.

## 3. Tech Stack and Why

| Choice | Reason |
|---|---|
| **Go, standard library `net/http`** | Go's built-in HTTP router (as of Go 1.22) supports method-based routing and path parameters (`GET /tickets/{id}`) natively, so a full web framework would just add complexity without adding capability for an API this size. |
| **SQLite (`modernc.org/sqlite`)** | The whole database is a single file - no separate database server to install, configure, or connect to. `modernc.org/sqlite` is a *pure Go* driver (no CGO), which means the final Docker image can be a tiny, fully static binary with no C toolchain baked in. |
| **JWT (`golang-jwt/jwt/v5`), HS256** | Lets the server verify "who is this request from" without keeping a session store in memory or in the database - the token itself carries the proof, cryptographically signed. |
| **bcrypt (`golang.org/x/crypto/bcrypt`)** | Deliberately slow, and automatically salts every password. This means a stolen database of password hashes can't be cracked quickly, and two users with the same password get different-looking hashes. |
| **`log/slog`** | Go's standard structured logger - one line per request, easy to read, no extra dependency. |

Dependencies are kept to exactly these three (plus what they pull in transitively) - no web framework, no ORM, no test framework beyond Go's built-in `testing` package.

## 4. Project Structure

```
.
├── cmd/server/main.go     # Entry point: reads config, opens the DB, builds the router, runs the server, shuts down cleanly
├── internal/
│   ├── config/            # Reads environment variables into a typed Config struct, with sane defaults
│   ├── models/            # The core data types (User, Ticket) and the ticket status transition rules
│   ├── store/             # The database layer - a Store interface plus its SQLite implementation and schema
│   ├── auth/              # Password hashing, JWT issuing/verifying, and the auth middleware that protects /tickets routes
│   ├── handlers/          # One file per group of HTTP endpoints (health, auth, tickets) plus the router that wires them up
│   └── httpx/             # Small shared HTTP helpers: JSON encode/decode, error responses, CORS, logging, panic recovery
├── web/                   # The optional frontend (index.html), embedded into the binary with go:embed
├── scripts/smoke_test.sh  # A curl-based script that exercises the whole API against any running instance
├── Dockerfile             # Multi-stage build producing a small, static, non-root container image
├── .env.example           # Every environment variable the server understands, with example values
└── README.md              # This file
```

## 5. Run Locally Without Docker

Requires Go 1.23 or newer.

```bash
git clone https://github.com/decode2211/evaassignment.git
cd evaassignment
go run ./cmd/server
```

The server starts on port 8080 by default (`http://localhost:8080`) with a SQLite database created automatically at `./data/tickets.db`. No environment variables are required to get started - see the [environment variables table](#7-environment-variables) if you want to change anything.

## 6. Run With Docker

```bash
docker build -t ticket-system .
docker run -p 8080:8080 ticket-system
curl http://localhost:8080/health
# -> {"status":"ok"}
```

The container needs **zero** environment variables to start. To set a real JWT secret (recommended for anything beyond a quick local test):

```bash
docker run -p 8080:8080 -e JWT_SECRET="some-long-random-string" ticket-system
```

## 7. Environment Variables

| Variable | Default | Meaning |
|---|---|---|
| `PORT` | `8080` | TCP port the HTTP server listens on. |
| `DB_PATH` | `./data/tickets.db` | Path to the SQLite database file. The parent directory is created automatically if missing. |
| `JWT_SECRET` | *(insecure dev default, with a logged warning)* | Secret key used to sign and verify login tokens. **Must** be set to a long, random value in any real deployment - anyone who has it can forge valid login tokens. |

See [`.env.example`](.env.example) for a ready-to-copy file.

## 8. API Reference

All responses are `application/json`. Every error response has the shape:

```json
{ "error": "human readable message" }
```

Timestamps are RFC3339 in UTC, e.g. `"2025-01-15T10:30:00Z"`.

---

### `GET /health`
No auth required.

**Response `200`:**
```json
{"status":"ok"}
```

---

### `POST /auth/register`
No auth required.

**Request:**
```json
{ "email": "jane@example.com", "password": "secret123", "name": "Jane Doe" }
```
`name` is optional; `username` is accepted as an alternate spelling of `name`.

**Response `201`:**
```json
{
  "id": 1,
  "email": "jane@example.com",
  "name": "Jane Doe",
  "created_at": "2025-01-15T10:30:00Z"
}
```

**Errors:** `400` invalid email or password too short (min 6 characters) · `409` email already registered.

---

### `POST /auth/login`
No auth required.

**Request:**
```json
{ "email": "jane@example.com", "password": "secret123" }
```

**Response `200`:**
```json
{ "token": "<jwt>", "token_type": "Bearer", "expires_in": 86400 }
```

**Errors:** `400` missing email/password · `401` wrong email or password (same message for both, on purpose).

---

### `POST /tickets` 🔒
Requires header `Authorization: Bearer <token>`.

**Request:**
```json
{ "title": "Printer is on fire", "description": "Send help" }
```
`title` is required (non-empty after trimming whitespace, max 200 characters); `description` is optional. Any `status` sent by the client is ignored - new tickets always start as `open`.

**Response `201`:**
```json
{
  "id": 1,
  "title": "Printer is on fire",
  "description": "Send help",
  "status": "open",
  "user_id": 1,
  "created_at": "2025-01-15T10:30:00Z",
  "updated_at": "2025-01-15T10:30:00Z"
}
```

---

### `GET /tickets` 🔒
Returns only the logged-in user's tickets, newest first.

**Response `200`:**
```json
[ { "id": 1, "title": "...", "...": "..." } ]
```
An empty result is `[]`, never `null`.

---

### `GET /tickets/{id}` 🔒
**Response `200`:** the ticket, in the same shape as above.

**Errors:** `400` `{id}` is not a positive integer · `404` the ticket doesn't exist *or* belongs to another user (see [design decisions](#10-assumptions--design-decisions) for why these two cases are indistinguishable).

---

### `PATCH /tickets/{id}/status` 🔒

**Request:**
```json
{ "status": "in_progress" }
```
Must be one of `open`, `in_progress`, `closed`.

**Response `200`:** the updated ticket.

**Allowed transitions:** `open → in_progress`, `in_progress → closed`, `open → closed`.
**Rejected (`400`):** any transition away from `closed`, `in_progress → open`, and setting a ticket to the status it already has.
**Errors:** `400` invalid status value or disallowed transition · `404` ticket doesn't exist or isn't yours.

---

### Other responses
- Unknown route → `404` `{"error":"route not found"}`
- Known route, wrong HTTP method → `405` `{"error":"method not allowed"}`
- Missing/invalid/expired token on a `/tickets` route → `401`

## 9. Curl Walkthrough

Run this against a local server (`go run ./cmd/server`) or the deployed URL. Replace `BASE` as needed.

```bash
BASE=http://localhost:8080

# 1. Register
curl -s -X POST $BASE/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email":"demo@example.com","password":"secret123","name":"Demo User"}'

# 2. Login (save the token)
TOKEN=$(curl -s -X POST $BASE/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"demo@example.com","password":"secret123"}' | grep -o '"token":"[^"]*"' | cut -d'"' -f4)

# 3. Create a ticket
curl -s -X POST $BASE/tickets \
  -H "Content-Type: application/json" -H "Authorization: Bearer $TOKEN" \
  -d '{"title":"My printer is on fire","description":"Send help"}'

# 4. List my tickets
curl -s $BASE/tickets -H "Authorization: Bearer $TOKEN"

# 5. Get one ticket by ID (assume it's id 1)
curl -s $BASE/tickets/1 -H "Authorization: Bearer $TOKEN"

# 6. Update its status: open -> in_progress -> closed
curl -s -X PATCH $BASE/tickets/1/status \
  -H "Content-Type: application/json" -H "Authorization: Bearer $TOKEN" \
  -d '{"status":"in_progress"}'

curl -s -X PATCH $BASE/tickets/1/status \
  -H "Content-Type: application/json" -H "Authorization: Bearer $TOKEN" \
  -d '{"status":"closed"}'

# 7. Try to reopen it - this must fail with 400
curl -s -X PATCH $BASE/tickets/1/status \
  -H "Content-Type: application/json" -H "Authorization: Bearer $TOKEN" \
  -d '{"status":"open"}'
# -> {"error":"invalid status transition"}
```

You can also run all of this automatically with `./scripts/smoke_test.sh $BASE`.

## 10. Assumptions & Design Decisions

- **404, not 403, for other users' tickets.** If a logged-in user requests a ticket that exists but belongs to someone else, the API returns `404 Not Found` rather than `403 Forbidden`. A `403` would confirm "this ID exists, you just can't see it" - which leaks the existence of other people's data. `404` makes "doesn't exist" and "isn't yours" indistinguishable from the outside, which is the safer default. This is enforced at the SQL level (`WHERE id = ? AND user_id = ?`), not just checked in application code afterward, so there's no code path that can accidentally skip the check.
- **Allowed status transitions.** `open → in_progress`, `in_progress → closed`, and `open → closed` (skipping straight to closed) are all allowed - the second one covers the common "actually, never mind" case. Every other move, including anything starting from `closed`, is rejected.
- **Setting the same status is rejected.** A `PATCH` that "changes" a ticket to the status it already has returns `400`, so a successful `200` response always means something actually changed.
- **Closed tickets are permanent.** Once closed, a ticket cannot move to any other status. This mirrors how real ticketing systems treat "closed" as a completed record rather than an editable one, and it was an explicit hard requirement.
- **Email as the login identifier**, normalized to lowercase and trimmed of whitespace on both register and login, so `Jane@Example.com` and `jane@example.com ` are treated as the same account.
- **Password minimum length of 6 characters.** Deliberately simple - this is a demonstration project, not a system needing a full password-strength policy.
- **SQLite persistence, and free-hosting disks may be ephemeral.** SQLite keeps the whole system to one file, which is perfect for local development and grading. On Render's free tier (used in the deployment guide below), the filesystem is **not** persistent across deploys/restarts unless you attach a paid disk - so the database may reset when the service restarts. For a production deployment, either add a Render persistent disk mounted at the `DB_PATH` directory, or swap the store for a hosted database.
- **24-hour token expiry.** Long enough that a user isn't constantly forced to log back in during normal use, short enough that a leaked token doesn't stay valid indefinitely.
- **Same error message for "no such email" and "wrong password"** on login, so a caller can't use the API to discover which email addresses have accounts.
- **CORS is wide open** (`Access-Control-Allow-Origin: *`). This is a small assignment project with no cookies or session state tied to a browser origin, so there's no meaningful cross-origin risk to guard against here.

## 11. How to Run Tests

```bash
go vet ./...
gofmt -l .          # should print nothing
go test ./...       # add -race if your platform supports CGO with a 64-bit C toolchain
```

> **Note on `-race`:** the race detector requires cgo with a 64-bit-capable C compiler. It was not available in the Windows development environment this project was built in (32-bit MinGW `cc1.exe`), so tests here were run and verified without `-race`. On a typical Linux CI runner or macOS machine with a standard 64-bit toolchain, `go test ./... -race` should work as normal.

Test coverage includes:
- **Unit tests** (`internal/models`, `internal/auth`): every status-transition pair (all 9 combinations of the 3 statuses), password hashing/verification, and JWT issuing/verifying (valid, expired, wrong secret, wrong algorithm, garbage input).
- **Integration tests** (`internal/handlers`): the real HTTP router wired to a temporary SQLite database via `httptest` - exact health check body, register/login success and failure paths, missing/garbage-token 401s, ticket creation always starting `open`, empty-list-is-`[]`, cross-user ownership isolation (user B gets `404` on user A's tickets for both `GET` and `PATCH`), invalid status values, the full transition lifecycle including the closed-can't-reopen rule, and non-numeric IDs.

To smoke-test a *running* instance end-to-end (local binary, or a Docker container, or a deployed URL):

```bash
./scripts/smoke_test.sh http://localhost:8080
```

## 12. Deployment Guide: Render (Free Tier, Docker)

1. Push this repository to GitHub (already done at https://github.com/decode2211/evaassignment).
2. Go to [render.com](https://render.com) and sign in (GitHub login is easiest).
3. Click **New +** → **Web Service**.
4. Connect your GitHub account if prompted, then select the `evaassignment` repository.
5. Render should auto-detect the `Dockerfile` and set **Runtime** to **Docker**. If it doesn't, select Docker manually.
6. Fill in the service settings:
   - **Name:** anything you like, e.g. `evaassignment-ticket-system`
   - **Region:** whichever is closest to you
   - **Branch:** `main`
   - **Instance Type:** **Free**
7. Under **Environment Variables**, add:
   - `JWT_SECRET` = a long random string (e.g. generate one locally with `openssl rand -base64 32`)
   - You do **not** need to set `PORT` - Render provides its own `PORT` value and the app already reads it.
8. Click **Create Web Service**. Render will build the Docker image and deploy it - this takes a few minutes on the first deploy.
9. Once the deploy finishes, Render gives you a public URL like `https://evaassignment-ticket-system.onrender.com`. Update the placeholders at the [top of this README](#0-live-deployment) with that URL.
10. Verify it: `curl https://<your-app>.onrender.com/health` should return `{"status":"ok"}`.

**Note on persistence:** Render's free tier does not include a persistent disk, so the SQLite file is stored on ephemeral container storage - data may be lost on redeploy or when the free instance spins down from inactivity and restarts. This is fine for a demo/assignment; for real use, attach a Render Disk (paid) mounted at the directory containing `DB_PATH`, or migrate to a managed database.

---

## Manual Deployment Reference

For convenience, here is the exact deployment info repeated in one place:

- **Render dashboard:** https://dashboard.render.com
- **Runtime:** Docker (uses the repo's `Dockerfile` as-is)
- **Required env var:** `JWT_SECRET`
- **Health check path to verify after deploy:** `/health`
