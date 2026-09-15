// This file implements the two "public" account endpoints: registering a
// brand-new account and logging in to get a login token (JWT). Neither of
// these endpoints requires the caller to already be logged in.
package handlers

import (
	"errors"
	"net/http"
	"strings"

	"github.com/decode2211/evaassignment/internal/auth"
	"github.com/decode2211/evaassignment/internal/httpx"
	"github.com/decode2211/evaassignment/internal/store"
)

// registerRequest is the expected JSON body for POST /auth/register.
// Username is accepted as an alternative spelling of Name, since different
// API clients sometimes use different field names for the same idea.
type registerRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
	Username string `json:"username"`
}

// registerResponse is what we send back after a successful registration.
// Notice there is no password or password hash field here at all - it is
// physically impossible for this struct to leak that information, because
// the field simply does not exist on it.
type registerResponse struct {
	ID        int64  `json:"id"`
	Email     string `json:"email"`
	Name      string `json:"name"`
	CreatedAt string `json:"created_at"`
}

// Register handles POST /auth/register: it validates the input, hashes the
// password, and creates a new user account.
func (h *Handlers) Register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	email := normalizeEmail(req.Email)
	name := strings.TrimSpace(req.Name)
	if name == "" {
		// Accept "username" as a fallback spelling of "name" if the
		// client sent that instead.
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

	// Why bcrypt: see internal/auth/password.go for the full explanation.
	// In short, it is slow-by-design and salts automatically, which makes
	// stolen password data far less useful to an attacker than a plain or
	// fast hash would be.
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

// loginRequest is the expected JSON body for POST /auth/login.
type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// loginResponse is what we send back after a successful login: a signed
// token the client must include on every future request to a protected
// endpoint, plus metadata about how to use it.
type loginResponse struct {
	Token     string `json:"token"`
	TokenType string `json:"token_type"`
	ExpiresIn int    `json:"expires_in"`
}

// tokenExpirySeconds must match the actual token lifetime used when
// signing (see internal/auth/jwt.go's tokenTTL, 24 hours).
const tokenExpirySeconds = 24 * 60 * 60

// Login handles POST /auth/login: it checks the given email and password
// and, if they match a real account, issues a JWT.
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

	// We deliberately give the exact same error, with the exact same
	// wording, whether the email doesn't exist at all or the password is
	// wrong. If we said "no such user" for one case and "wrong password"
	// for the other, an attacker could use that difference to discover
	// which email addresses have accounts on this system.
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
