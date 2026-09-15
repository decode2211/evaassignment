// Integration tests exercise the real HTTP router (the same one main.go
// builds) against a real, temporary SQLite database file. Unlike the unit
// tests elsewhere in the project, these tests send actual HTTP requests
// through httptest and check the actual JSON responses and status codes -
// they are the closest thing to "does the finished API really behave as
// documented" that can run without a network.
package handlers_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/decode2211/evaassignment/internal/auth"
	"github.com/decode2211/evaassignment/internal/handlers"
	"github.com/decode2211/evaassignment/internal/store"
)

// newTestRouter builds a full router backed by a fresh temporary SQLite
// database, so each test starts from a clean, empty database and cannot
// interfere with any other test.
func newTestRouter(t *testing.T) http.Handler {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := store.New(dbPath)
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	const secret = "integration-test-secret"
	h := handlers.New(db, secret)
	mw := auth.NewMiddleware(secret, db)
	return handlers.NewRouter(h, mw, nil)
}

// doRequest sends a JSON request (or no body, if body is nil) through the
// router and decodes the JSON response into out (if out is not nil). It
// returns the HTTP status code.
func doRequest(t *testing.T, router http.Handler, method, path string, body any, token string, out any) int {
	t.Helper()

	var reader *bytes.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("failed to marshal request body: %v", err)
		}
		reader = bytes.NewReader(b)
	} else {
		reader = bytes.NewReader(nil)
	}

	req := httptest.NewRequest(method, path, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if out != nil && rec.Body.Len() > 0 {
		if err := json.Unmarshal(rec.Body.Bytes(), out); err != nil {
			t.Fatalf("failed to decode response body %q: %v", rec.Body.String(), err)
		}
	}
	return rec.Code
}

// registerAndLogin is a small helper that creates one user and returns a
// valid login token for them, so tests that only care about ticket
// behaviour don't need to repeat the register+login boilerplate.
func registerAndLogin(t *testing.T, router http.Handler, email string) string {
	t.Helper()
	registerBody := map[string]string{"email": email, "password": "secret123", "name": "Test User"}
	if code := doRequest(t, router, http.MethodPost, "/auth/register", registerBody, "", nil); code != http.StatusCreated {
		t.Fatalf("register failed with status %d", code)
	}

	var loginResp struct {
		Token string `json:"token"`
	}
	loginBody := map[string]string{"email": email, "password": "secret123"}
	if code := doRequest(t, router, http.MethodPost, "/auth/login", loginBody, "", &loginResp); code != http.StatusOK {
		t.Fatalf("login failed with status %d", code)
	}
	if loginResp.Token == "" {
		t.Fatal("login response did not contain a token")
	}
	return loginResp.Token
}

func TestHealth(t *testing.T) {
	router := newTestRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := rec.Body.String(); got != `{"status":"ok"}` {
		t.Fatalf("body = %q, want exactly {\"status\":\"ok\"}", got)
	}
}

func TestRegister(t *testing.T) {
	router := newTestRouter(t)

	t.Run("success", func(t *testing.T) {
		var resp map[string]any
		code := doRequest(t, router, http.MethodPost, "/auth/register",
			map[string]string{"email": "alice@example.com", "password": "secret123", "name": "Alice"}, "", &resp)
		if code != http.StatusCreated {
			t.Fatalf("status = %d, want 201", code)
		}
		if _, hasHash := resp["password"]; hasHash {
			t.Error("response must not contain a password field")
		}
		if _, hasHash := resp["password_hash"]; hasHash {
			t.Error("response must not contain a password_hash field")
		}
	})

	t.Run("duplicate email", func(t *testing.T) {
		code := doRequest(t, router, http.MethodPost, "/auth/register",
			map[string]string{"email": "alice@example.com", "password": "anotherpass", "name": "Alice2"}, "", nil)
		if code != http.StatusConflict {
			t.Fatalf("status = %d, want 409", code)
		}
	})

	t.Run("invalid email", func(t *testing.T) {
		code := doRequest(t, router, http.MethodPost, "/auth/register",
			map[string]string{"email": "not-an-email", "password": "secret123"}, "", nil)
		if code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", code)
		}
	})

	t.Run("password too short", func(t *testing.T) {
		code := doRequest(t, router, http.MethodPost, "/auth/register",
			map[string]string{"email": "bob@example.com", "password": "abc"}, "", nil)
		if code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", code)
		}
	})
}

func TestLogin(t *testing.T) {
	router := newTestRouter(t)
	doRequest(t, router, http.MethodPost, "/auth/register",
		map[string]string{"email": "carol@example.com", "password": "secret123"}, "", nil)

	t.Run("success", func(t *testing.T) {
		var resp map[string]any
		code := doRequest(t, router, http.MethodPost, "/auth/login",
			map[string]string{"email": "carol@example.com", "password": "secret123"}, "", &resp)
		if code != http.StatusOK {
			t.Fatalf("status = %d, want 200", code)
		}
		if resp["token"] == "" || resp["token"] == nil {
			t.Error("expected a non-empty token")
		}
	})

	t.Run("wrong password", func(t *testing.T) {
		code := doRequest(t, router, http.MethodPost, "/auth/login",
			map[string]string{"email": "carol@example.com", "password": "wrongpass"}, "", nil)
		if code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", code)
		}
	})
}

func TestTickets_RequireAuth(t *testing.T) {
	router := newTestRouter(t)

	if code := doRequest(t, router, http.MethodGet, "/tickets", nil, "", nil); code != http.StatusUnauthorized {
		t.Fatalf("no token: status = %d, want 401", code)
	}
	if code := doRequest(t, router, http.MethodGet, "/tickets", nil, "garbage.token.value", nil); code != http.StatusUnauthorized {
		t.Fatalf("garbage token: status = %d, want 401", code)
	}
}

func TestTickets_CreateListGetLifecycle(t *testing.T) {
	router := newTestRouter(t)
	token := registerAndLogin(t, router, "dave@example.com")

	// A brand-new user has no tickets yet - the list must be "[]", not null.
	req := httptest.NewRequest(http.MethodGet, "/tickets", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if got := strings.TrimSpace(rec.Body.String()); got != "[]" {
		t.Fatalf("empty ticket list body = %q, want []", got)
	}

	var created map[string]any
	code := doRequest(t, router, http.MethodPost, "/tickets",
		map[string]string{"title": "Printer is on fire", "description": "send help"}, token, &created)
	if code != http.StatusCreated {
		t.Fatalf("create: status = %d, want 201", code)
	}
	if created["status"] != "open" {
		t.Fatalf("new ticket status = %v, want open", created["status"])
	}
	id := int(created["id"].(float64))

	var list []map[string]any
	code = doRequest(t, router, http.MethodGet, "/tickets", nil, token, &list)
	if code != http.StatusOK || len(list) != 1 {
		t.Fatalf("list: status=%d len=%d, want 200 and 1 ticket", code, len(list))
	}

	var fetched map[string]any
	code = doRequest(t, router, http.MethodGet, fmt.Sprintf("/tickets/%d", id), nil, token, &fetched)
	if code != http.StatusOK {
		t.Fatalf("get: status = %d, want 200", code)
	}

	code = doRequest(t, router, http.MethodGet, "/tickets/not-a-number", nil, token, nil)
	if code != http.StatusBadRequest {
		t.Fatalf("non-numeric id: status = %d, want 400", code)
	}

	code = doRequest(t, router, http.MethodGet, "/tickets/999999", nil, token, nil)
	if code != http.StatusNotFound {
		t.Fatalf("missing ticket: status = %d, want 404", code)
	}
}

func TestTickets_OwnershipIsolation(t *testing.T) {
	router := newTestRouter(t)
	tokenA := registerAndLogin(t, router, "userA@example.com")
	tokenB := registerAndLogin(t, router, "userB@example.com")

	var created map[string]any
	doRequest(t, router, http.MethodPost, "/tickets", map[string]string{"title": "A's secret ticket"}, tokenA, &created)
	id := int(created["id"].(float64))

	// User B must not be able to see user A's ticket at all.
	code := doRequest(t, router, http.MethodGet, fmt.Sprintf("/tickets/%d", id), nil, tokenB, nil)
	if code != http.StatusNotFound {
		t.Fatalf("user B GET user A's ticket: status = %d, want 404", code)
	}

	// Nor change its status.
	code = doRequest(t, router, http.MethodPatch, fmt.Sprintf("/tickets/%d/status", id),
		map[string]string{"status": "in_progress"}, tokenB, nil)
	if code != http.StatusNotFound {
		t.Fatalf("user B PATCH user A's ticket: status = %d, want 404", code)
	}

	// User B's own ticket list must stay empty - A's ticket must never
	// leak into it.
	var listB []map[string]any
	doRequest(t, router, http.MethodGet, "/tickets", nil, tokenB, &listB)
	if len(listB) != 0 {
		t.Fatalf("user B's ticket list should be empty, got %d tickets", len(listB))
	}
}

func TestTickets_StatusTransitions(t *testing.T) {
	router := newTestRouter(t)
	token := registerAndLogin(t, router, "erin@example.com")

	var created map[string]any
	doRequest(t, router, http.MethodPost, "/tickets", map[string]string{"title": "Something"}, token, &created)
	id := int(created["id"].(float64))
	statusPath := fmt.Sprintf("/tickets/%d/status", id)

	code := doRequest(t, router, http.MethodPatch, statusPath, map[string]string{"status": "bogus"}, token, nil)
	if code != http.StatusBadRequest {
		t.Fatalf("invalid status value: status = %d, want 400", code)
	}

	code = doRequest(t, router, http.MethodPatch, statusPath, map[string]string{"status": "in_progress"}, token, nil)
	if code != http.StatusOK {
		t.Fatalf("open->in_progress: status = %d, want 200", code)
	}

	code = doRequest(t, router, http.MethodPatch, statusPath, map[string]string{"status": "closed"}, token, nil)
	if code != http.StatusOK {
		t.Fatalf("in_progress->closed: status = %d, want 200", code)
	}

	code = doRequest(t, router, http.MethodPatch, statusPath, map[string]string{"status": "open"}, token, nil)
	if code != http.StatusBadRequest {
		t.Fatalf("closed->open: status = %d, want 400", code)
	}

	code = doRequest(t, router, http.MethodPatch, statusPath, map[string]string{"status": "in_progress"}, token, nil)
	if code != http.StatusBadRequest {
		t.Fatalf("closed->in_progress: status = %d, want 400", code)
	}
}

func TestUnknownRouteAndWrongMethod(t *testing.T) {
	router := newTestRouter(t)

	if code := doRequest(t, router, http.MethodGet, "/this/route/does/not/exist", nil, "", nil); code != http.StatusNotFound {
		t.Fatalf("unknown route: status = %d, want 404", code)
	}
	if code := doRequest(t, router, http.MethodDelete, "/tickets", nil, "", nil); code != http.StatusMethodNotAllowed {
		t.Fatalf("wrong method on known route: status = %d, want 405", code)
	}
}
