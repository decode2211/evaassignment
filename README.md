# Ticket System – EVA Bharat Backend Intern Assignment

Author: Dev Vashist
Repository: https://github.com/decode2211/evaassignment

## 0. Live Deployment

- App: https://evaassignment.onrender.com
- Health check: https://evaassignment.onrender.com/health

## 1. What This Project Is

A small backend for a ticket system, the kind a support team might use to track issues. A person creates an account, logs in, and raises tickets describing a problem. Every user can only see and manage the tickets they created.

A ticket's life is one-directional: it starts open, can move to in_progress while someone works on it, and finally to closed once it's resolved (it can also jump straight from open to closed). Once closed, a ticket stays closed - it can never be reopened. There is no admin role, no assigning tickets to other people, and no comments.

## 2. Live Deployment URL

See section 0 above.

## 3. Tech Stack and Why

| Choice | Reason |
|---|---|
| Go, standard library `net/http` | Go 1.22's router supports method-based routing and path parameters (`GET /tickets/{id}`) natively, so a framework isn't needed for an API this size. |
| SQLite (`modernc.org/sqlite`) | One file, no separate database server. The driver is pure Go (no CGO), so the final Docker image can be a small static binary. |
| JWT (`golang-jwt/jwt/v5`), HS256 | Verifies who a request is from without a server-side session store. |
| bcrypt (`golang.org/x/crypto/bcrypt`) | Slow by design and salts automatically, so a stolen hash database can't be brute-forced quickly. |
| `log/slog` | Standard library structured logger, one line per request. |

Dependencies are kept to these three (plus their transitive deps) - no web framework, no ORM.

## 4. Project Structure

```
.
├── cmd/server/main.go     # entry point: config, DB, router, graceful shutdown
├── internal/
│   ├── config/            # environment variables into a Config struct
│   ├── models/            # User, Ticket types and the status transition rules
│   ├── store/             # Store interface plus SQLite implementation and schema
│   ├── auth/              # password hashing, JWT issue/verify, auth middleware
│   ├── handlers/          # HTTP handlers (health, auth, tickets) and the router
│   └── httpx/             # JSON helpers, error responses, CORS, logging, panic recovery
├── web/                   # optional frontend (index.html), embedded with go:embed
├── scripts/smoke_test.sh  # curl-based end-to-end check against a running instance
├── Dockerfile             # multi-stage build, static binary, non-root user
├── .env.example           # environment variables with example values
└── README.md
```

## 5. Run Locally Without Docker

Requires Go 1.23 or newer.

```bash
git clone https://github.com/decode2211/evaassignment.git
cd evaassignment
go run ./cmd/server
```

The server starts on port 8080 with a SQLite database created at `./data/tickets.db`. No environment variables are required.

## 6. Run With Docker

```bash
docker build -t ticket-system .
docker run -p 8080:8080 ticket-system
curl http://localhost:8080/health
# -> {"status":"ok"}
```

The container needs no environment variables to start. To set a real JWT secret:

```bash
docker run -p 8080:8080 -e JWT_SECRET="some-long-random-string" ticket-system
```

## 7. Environment Variables

| Variable | Default | Meaning |
|---|---|---|
| `PORT` | `8080` | TCP port the server listens on. |
| `DB_PATH` | `./data/tickets.db` | Path to the SQLite database file. Created automatically if missing. |
| `JWT_SECRET` | insecure dev default, with a logged warning | Signs and verifies login tokens. Must be set to a long random value in any real deployment. |

See `.env.example` for a ready-to-copy file.

## 8. API Reference

All responses are `application/json`. Every error response has the shape:

```json
{ "error": "human readable message" }
```

Timestamps are RFC3339 in UTC, e.g. `"2025-01-15T10:30:00Z"`.

### GET /health
No auth required.

Response 200:
```json
{"status":"ok"}
```

### POST /auth/register
No auth required.

Request:
```json
{ "email": "jane@example.com", "password": "secret123", "name": "Jane Doe" }
```
`name` is optional; `username` is accepted as an alternate spelling of `name`.

Response 201:
```json
{
  "id": 1,
  "email": "jane@example.com",
  "name": "Jane Doe",
  "created_at": "2025-01-15T10:30:00Z"
}
```

Errors: 400 invalid email or password too short (min 6 characters); 409 email already registered.

### POST /auth/login
No auth required.

Request:
```json
{ "email": "jane@example.com", "password": "secret123" }
```

Response 200:
```json
{ "token": "<jwt>", "token_type": "Bearer", "expires_in": 86400 }
```

Errors: 400 missing email/password; 401 wrong email or password (same message for both, on purpose).

### POST /tickets
Requires header `Authorization: Bearer <token>`.

Request:
```json
{ "title": "Printer is on fire", "description": "Send help" }
```
`title` is required (non-empty after trimming, max 200 characters); `description` is optional. Any `status` sent by the client is ignored - new tickets always start as `open`.

Response 201:
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

### GET /tickets
Requires auth. Returns only the logged-in user's tickets, newest first.

Response 200:
```json
[ { "id": 1, "title": "...", "...": "..." } ]
```
An empty result is `[]`, never `null`.

### GET /tickets/{id}
Requires auth.

Response 200: the ticket, same shape as above.

Errors: 400 `{id}` is not a positive integer; 404 the ticket doesn't exist or belongs to another user (see design decisions below for why these are indistinguishable).

### PATCH /tickets/{id}/status
Requires auth.

Request:
```json
{ "status": "in_progress" }
```
Must be one of `open`, `in_progress`, `closed`.

Response 200: the updated ticket.

Allowed transitions: `open -> in_progress`, `in_progress -> closed`, `open -> closed`.
Rejected (400): any transition away from `closed`, `in_progress -> open`, and setting a ticket to the status it already has.
Errors: 400 invalid status value or disallowed transition; 404 ticket doesn't exist or isn't yours.

### Other responses
- Unknown route: 404 `{"error":"route not found"}`
- Known route, wrong HTTP method: 405 `{"error":"method not allowed"}`
- Missing/invalid/expired token on a `/tickets` route: 401

## 9. Curl Walkthrough

Run against a local server (`go run ./cmd/server`) or the deployed URL.

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

Or run all of this automatically with `./scripts/smoke_test.sh $BASE`.

## 10. Assumptions & Design Decisions

- 404, not 403, for other users' tickets. A 403 would confirm the ticket exists, just not yours - which leaks the existence of other people's data. 404 makes "doesn't exist" and "isn't yours" indistinguishable. Enforced at the SQL level (`WHERE id = ? AND user_id = ?`), not just in application code.
- Allowed status transitions: `open -> in_progress`, `in_progress -> closed`, and `open -> closed` (skipping straight to closed covers the "never mind" case). Every other move, including anything starting from `closed`, is rejected.
- Setting the same status is rejected, so a successful 200 always means something actually changed.
- Closed tickets are permanent - this was an explicit requirement, and it mirrors how real ticket systems treat "closed" as a completed record.
- Email is the login identifier, normalized to lowercase and trimmed on both register and login.
- Password minimum length is 6 characters - simple on purpose, not a full password-strength policy.
- SQLite keeps the whole system to one file. On Render's free tier the filesystem is not persistent across deploys/restarts unless you attach a paid disk, so the database may reset. For real use, attach a Render disk mounted at the `DB_PATH` directory, or use a hosted database.
- 24-hour token expiry: long enough to not be annoying, short enough that a leaked token doesn't stay valid indefinitely.
- Same error message for unknown email and wrong password on login, so a caller can't use the response to find out which emails have accounts.
- CORS is wide open (`Access-Control-Allow-Origin: *`) - there's no cookie-based session state tied to a browser origin here.

## 11. How to Run Tests

```bash
go vet ./...
gofmt -l .          # should print nothing
go test ./...       # add -race if your platform supports CGO with a 64-bit C toolchain
```

Note on `-race`: it requires cgo with a 64-bit-capable C compiler, which wasn't available in the Windows environment this was built in (32-bit MinGW `cc1.exe`). Tests were run and verified without `-race`. On a typical Linux or macOS toolchain, `go test ./... -race` should work as normal.

Test coverage:
- Unit tests (`internal/models`, `internal/auth`): every status-transition pair, password hashing/verification, JWT issuing/verifying (valid, expired, wrong secret, wrong algorithm, garbage input).
- Integration tests (`internal/handlers`): the real HTTP router against a temporary SQLite database via `httptest` - health check body, register/login success and failure paths, missing/garbage-token 401s, ticket creation always starting open, empty list is `[]`, cross-user ownership isolation, invalid status values, the full transition lifecycle, non-numeric IDs.

To smoke-test a running instance end-to-end (local binary, Docker container, or deployed URL):

```bash
./scripts/smoke_test.sh http://localhost:8080
```

## 12. Deployment Guide: Render (Free Tier, Docker)

1. Push this repository to GitHub (already done at https://github.com/decode2211/evaassignment).
2. Go to render.com and sign in.
3. Click New + -> Web Service.
4. Connect your GitHub account if prompted, then select the `evaassignment` repository.
5. Render should auto-detect the Dockerfile and set Runtime to Docker.
6. Fill in the service settings:
   - Name: anything you like, e.g. `evaassignment-ticket-system`
   - Region: whichever is closest to you
   - Branch: `main`
   - Instance Type: Free
7. Under Environment Variables, add `JWT_SECRET` (a long random string, e.g. `openssl rand -base64 32`). You don't need to set `PORT` - Render provides its own and the app already reads it.
8. Click Create Web Service. Render builds the Docker image and deploys it.
9. Once the deploy finishes, Render gives you a public URL. That's what's listed in section 0 above.
10. Verify with `curl https://evaassignment.onrender.com/health` - should return `{"status":"ok"}`.

Note on persistence: Render's free tier has no persistent disk, so the SQLite file lives on ephemeral storage - data may be lost on redeploy or when the free instance restarts after spinning down. Fine for a demo; for real use, attach a Render disk or use a hosted database.
