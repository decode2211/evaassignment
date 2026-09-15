// Register and login endpoints.
package handlers

import (
	"errors"
	"net/http"
	"strings"

	"github.com/decode2211/evaassignment/internal/auth"
	"github.com/decode2211/evaassignment/internal/httpx"
	"github.com/decode2211/evaassignment/internal/store"
)

// registerRequest is the JSON body for POST /auth/register.
// Username is accepted as an alternate spelling of Name.
type registerRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
	Username string `json:"username"`
}

// registerResponse never includes a password or hash field.
type registerResponse struct {
	ID        int64  `json:"id"`
	Email     string `json:"email"`
	Name      string `json:"name"`
	CreatedAt string `json:"created_at"`
}

// Register handles POST /auth/register.
func (h *Handlers) Register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	email := normalizeEmail(req.Email)
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = strings.TrimSpace(req.Username)
	}

	if email == "" || !isValidEmail(email) {
		httpx.WriteError(w, http.StatusBadRequest, "a valid email address is required")
		return
	}
	if len(req.Password) < minPasswordLength {
		httpx.WriteError(w, http.StatusBadRequest, "password must be at least 6 characters")
		return
	}

	// bcrypt is slow on purpose and salts automatically - see internal/auth/password.go.
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "could not process password")
		return
	}

	user, err := h.Store.CreateUser(r.Context(), email, hash, name)
	if err != nil {
		if errors.Is(err, store.ErrDuplicateEmail) {
			httpx.WriteError(w, http.StatusConflict, "an account with this email already exists")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "could not create account")
		return
	}

	httpx.WriteJSON(w, http.StatusCreated, registerResponse{
		ID:        user.ID,
		Email:     user.Email,
		Name:      user.Name,
		CreatedAt: user.CreatedAt.Format(rfc3339),
	})
}

// loginRequest is the JSON body for POST /auth/login.
type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginResponse struct {
	Token     string `json:"token"`
	TokenType string `json:"token_type"`
	ExpiresIn int    `json:"expires_in"`
}

// tokenExpirySeconds must match tokenTTL in internal/auth/jwt.go.
const tokenExpirySeconds = 24 * 60 * 60

// Login handles POST /auth/login.
func (h *Handlers) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	email := normalizeEmail(req.Email)
	if email == "" || req.Password == "" {
		httpx.WriteError(w, http.StatusBadRequest, "email and password are required")
		return
	}

	// Same message for unknown email and wrong password, so a caller can't
	// use the response to find out which emails have an account.
	const genericAuthError = "invalid email or password"

	user, err := h.Store.GetUserByEmail(r.Context(), email)
	if err != nil {
		httpx.WriteError(w, http.StatusUnauthorized, genericAuthError)
		return
	}
	if !auth.CheckPassword(user.PasswordHash, req.Password) {
		httpx.WriteError(w, http.StatusUnauthorized, genericAuthError)
		return
	}

	token, err := auth.IssueToken(h.JWTSecret, user.ID, user.Email)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "could not issue token")
		return
	}

	httpx.WriteJSON(w, http.StatusOK, loginResponse{
		Token:     token,
		TokenType: "Bearer",
		ExpiresIn: tokenExpirySeconds,
	})
}
