// This file wires every URL path to the handler function that serves it,
// and applies the shared middleware (CORS, logging, panic recovery) around
// the whole thing. It is the "table of contents" for the API: reading this
// file top to bottom tells you every route the server understands.
package handlers

import (
	"io/fs"
	"net/http"

	"github.com/decode2211/evaassignment/internal/auth"
	"github.com/decode2211/evaassignment/internal/httpx"
)

// methodNotAllowed responds 405 in our standard JSON error shape. It is
// registered for each known path without a method restriction, so it only
// ever gets used when a request's method doesn't match one of the specific
// handlers registered for that same path (Go's router prefers the more
// specific, method-matching pattern whenever one matches).
func methodNotAllowed(w http.ResponseWriter, r *http.Request) {
	httpx.WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
}

// notFound responds 404 in our standard JSON error shape. It is registered
// as the catch-all for any path that doesn't match one of the routes below.
func notFound(w http.ResponseWriter, r *http.Request) {
	httpx.WriteError(w, http.StatusNotFound, "route not found")
}

// NewRouter builds the complete HTTP handler for the server: every API
// route, the optional embedded frontend, and the middleware chain that
// wraps all of it. frontend may be nil, in which case GET / is not served
// (the API still works fine without a frontend).
func NewRouter(h *Handlers, mw *auth.Middleware, frontend fs.FS) http.Handler {
	mux := http.NewServeMux()

	// --- Health check (public) ---
	mux.HandleFunc("GET /health", Health)
	mux.HandleFunc("/health", methodNotAllowed)

	// --- Account endpoints (public) ---
	mux.HandleFunc("POST /auth/register", h.Register)
	mux.HandleFunc("/auth/register", methodNotAllowed)

	mux.HandleFunc("POST /auth/login", h.Login)
	mux.HandleFunc("/auth/login", methodNotAllowed)

	// --- Ticket endpoints (all require a valid login token) ---
	mux.HandleFunc("POST /tickets", mw.RequireAuth(h.CreateTicket))
	mux.HandleFunc("GET /tickets", mw.RequireAuth(h.ListTickets))
	mux.HandleFunc("/tickets", methodNotAllowed)

	mux.HandleFunc("GET /tickets/{id}", mw.RequireAuth(h.GetTicket))
	mux.HandleFunc("/tickets/{id}", methodNotAllowed)

	mux.HandleFunc("PATCH /tickets/{id}/status", mw.RequireAuth(h.UpdateTicketStatus))
	mux.HandleFunc("/tickets/{id}/status", methodNotAllowed)

	// --- Optional frontend (public), served only at the exact root path ---
	// "{$}" means "nothing left in the path", so this matches "/" only -
	// it does not swallow unmatched API-like paths such as "/foo".
	if frontend != nil {
		mux.Handle("GET /{$}", http.FileServer(http.FS(frontend)))
	}

	// --- Catch-all for anything else: unknown routes get a JSON 404 ---
	mux.HandleFunc("/", notFound)

	// Middleware runs outside-in: Recover first (so it can catch a panic
	// from anything below it), then request logging, then CORS (so
	// preflight OPTIONS requests are answered before reaching any route).
	var wrapped http.Handler = mux
	wrapped = httpx.CORS(wrapped)
	wrapped = httpx.RequestLogger(wrapped)
	wrapped = httpx.Recover(wrapped)
	return wrapped
}
