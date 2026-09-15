// Package httpx has shared HTTP helpers: JSON encode/decode, errors, middleware.
package httpx

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

// maxBodyBytes caps request body size to guard against huge payloads.
const maxBodyBytes = 1 << 20 // 1 MB

// WriteJSON encodes v as JSON and writes it with the given status code.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// ErrorResponse is the shared shape for every error body: {"error": "..."}.
type ErrorResponse struct {
	Error string `json:"error"`
}

// WriteError sends a JSON error body with the given status code.
func WriteError(w http.ResponseWriter, status int, message string) {
	WriteJSON(w, status, ErrorResponse{Error: message})
}

// DecodeJSON reads the request body into dst, rejecting empty, malformed,
// or multi-value bodies.
func DecodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)

	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(dst); err != nil {
		if err == io.EOF {
			return errors.New("request body must not be empty")
		}
		return errors.New("request body must be valid JSON")
	}

	if dec.More() {
		return errors.New("request body must contain a single JSON object")
	}
	return nil
}
