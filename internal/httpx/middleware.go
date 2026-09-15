// This file holds HTTP middleware: small wrappers that run for every
// request before it reaches the real handler. They add cross-cutting
// behaviour (logging, crash protection, cross-origin support) without every
// individual handler needing to remember to do it themselves.
package httpx

import (
	"log/slog"
	"net/http"
	"time"
)

// statusRecorder wraps a http.ResponseWriter so we can remember which HTTP
// status code was actually sent, purely for logging purposes (the standard
// ResponseWriter does not let you read this back otherwise).
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

// RequestLogger logs one line per HTTP request: method, path, resulting
// status code, and how long it took. This is invaluable when demoing or
// debugging the API, since you can see exactly what the server did.
func RequestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(rec, r)

		slog.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"duration_ms", time.Since(start).Milliseconds(),
		)
	})
}

// Recover catches any panic that happens while handling a request and turns
// it into a clean 500 JSON error instead of crashing the whole server
// process. A single bug in one handler should never bring down every other
// in-flight request.
func Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				slog.Error("panic recovered", "error", rec, "path", r.URL.Path)
				WriteError(w, http.StatusInternalServerError, "internal server error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// CORS allows this API to be called from a web page served on a different
// origin (for example, a frontend hosted elsewhere, or this project's own
// simple frontend during local development on a different port). It is
// deliberately permissive ("allow any origin") because this is a small demo
// project, not a system protecting sensitive multi-tenant data behind
// browser cookies.
func CORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")

		// Browsers send an OPTIONS "preflight" request before certain
		// cross-origin requests to ask permission. We answer it directly
		// here with no body, so it never needs to reach real handlers.
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}
