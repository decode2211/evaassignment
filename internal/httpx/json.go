// Package httpx contains small, reusable helpers for writing HTTP servers:
// sending JSON responses, reading JSON request bodies safely, and shared
// middleware (CORS, request logging, panic recovery). None of this file
// knows anything about tickets or users - it is generic "plumbing" that any
// handler in the project can reuse, so every endpoint behaves consistently.
package httpx

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

// maxBodyBytes caps how large an incoming request body may be (1 MB). This
// protects the server from a caller sending a huge or endless body just to
// waste memory/time - a basic and cheap safety net.
const maxBodyBytes = 1 << 20 // 1 MB

// WriteJSON encodes v as JSON and writes it to w with the given HTTP status
// code. Centralizing this means every response gets the same
// "Content-Type: application/json" header and consistent formatting.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	// Errors from Encode here would mean the connection is already broken;
	// there is nothing useful left to do but ignore it, since the response
	// status/headers were already sent.
	_ = json.NewEncoder(w).Encode(v)
}

// ErrorResponse is the one and only shape of body we ever send when
// something goes wrong. Using a single consistent shape everywhere makes
// life easy for whoever writes client code against this API - they only
// need to know one error format.
type ErrorResponse struct {
	Error string `json:"error"`
}

// WriteError sends a JSON error body ({"error": "..."}) with the given
// HTTP status code.
func WriteError(w http.ResponseWriter, status int, message string) {
	WriteJSON(w, status, ErrorResponse{Error: message})
}

// DecodeJSON reads the request body into dst (a pointer to a struct),
// enforcing a maximum size and rejecting bodies that contain anything other
// than exactly one valid JSON value. Returning a plain error keeps this
// function reusable; callers decide how to translate it into an HTTP status
// (in practice, always 400 Bad Request for a decode failure).
func DecodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)

	dec := json.NewDecoder(r.Body)
	// DisallowUnknownFields would be too strict for optional aliases like
	// "username", so we intentionally allow unknown fields but still guard
	// against garbage/multiple JSON values below.
	if err := dec.Decode(dst); err != nil {
		if err == io.EOF {
			return errors.New("request body must not be empty")
		}
		return errors.New("request body must be valid JSON")
	}

	// If there is anything left after the first JSON value, the body
	// contained more than one JSON object/value, which we treat as
	// malformed input rather than silently ignoring the extra data.
	if dec.More() {
		return errors.New("request body must contain a single JSON object")
	}
	return nil
}
