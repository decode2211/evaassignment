// This file implements the health check endpoint, used by monitoring tools
// (and by the Docker HEALTHCHECK instruction) to confirm the server is up
// and responding.
package handlers

import "net/http"

// Health responds to GET /health with a fixed, minimal body. It does not
// check the database or any other dependency on purpose - it answers one
// question only: "is the HTTP server itself alive and accepting requests?"
func Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}
