// Route table: wires URL paths to handlers and wraps them in middleware.
package handlers

import (
	"io/fs"
	"net/http"

	"github.com/decode2211/evaassignment/internal/auth"
	"github.com/decode2211/evaassignment/internal/httpx"
)

func methodNotAllowed(w http.ResponseWriter, r *http.Request) {
	httpx.WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
}

func notFound(w http.ResponseWriter, r *http.Request) {
	httpx.WriteError(w, http.StatusNotFound, "route not found")
}

// NewRouter builds the full HTTP handler. frontend may be nil, in which
// case GET / is not served.
func NewRouter(h *Handlers, mw *auth.Middleware, frontend fs.FS) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", Health)
	mux.HandleFunc("/health", methodNotAllowed)

	mux.HandleFunc("POST /auth/register", h.Register)
	mux.HandleFunc("/auth/register", methodNotAllowed)

	mux.HandleFunc("POST /auth/login", h.Login)
	mux.HandleFunc("/auth/login", methodNotAllowed)

	mux.HandleFunc("POST /tickets", mw.RequireAuth(h.CreateTicket))
	mux.HandleFunc("GET /tickets", mw.RequireAuth(h.ListTickets))
	mux.HandleFunc("/tickets", methodNotAllowed)

	mux.HandleFunc("GET /tickets/{id}", mw.RequireAuth(h.GetTicket))
	mux.HandleFunc("/tickets/{id}", methodNotAllowed)

	mux.HandleFunc("PATCH /tickets/{id}/status", mw.RequireAuth(h.UpdateTicketStatus))
	mux.HandleFunc("/tickets/{id}/status", methodNotAllowed)

	if frontend != nil {
		// "{$}" matches only "/", so it won't swallow unmatched API paths.
		mux.Handle("GET /{$}", http.FileServer(http.FS(frontend)))
	}

	mux.HandleFunc("/", notFound)

	var wrapped http.Handler = mux
	wrapped = httpx.CORS(wrapped)
	wrapped = httpx.RequestLogger(wrapped)
	wrapped = httpx.Recover(wrapped)
	return wrapped
}
