// Unit tests for password hashing and JWT issuing/verifying. These cover
// the security-critical primitives in isolation, before any HTTP code gets
// involved.
package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestHashAndCheckPassword(t *testing.T) {
	hash, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("HashPassword returned error: %v", err)
	}
	if hash == "correct horse battery staple" {
		t.Fatal("hash must not equal the plain-text password")
	}
	if !CheckPassword(hash, "correct horse battery staple") {
		t.Error("CheckPassword should accept the correct password")
	}
	if CheckPassword(hash, "wrong password") {
		t.Error("CheckPassword should reject an incorrect password")
	}
}

func TestIssueAndVerifyToken(t *testing.T) {
	secret := "test-secret"
	token, err := IssueToken(secret, 42, "user@example.com")
	if err != nil {
		t.Fatalf("IssueToken returned error: %v", err)
	}

	id, email, err := VerifyToken(secret, token)
	if err != nil {
		t.Fatalf("VerifyToken returned error for a freshly issued token: %v", err)
	}
	if id != 42 {
		t.Errorf("got user id %d, want 42", id)
	}
	if email != "user@example.com" {
		t.Errorf("got email %q, want user@example.com", email)
	}
}

func TestVerifyToken_WrongSecret(t *testing.T) {
	token, err := IssueToken("secret-a", 1, "a@example.com")
	if err != nil {
		t.Fatalf("IssueToken returned error: %v", err)
	}
	if _, _, err := VerifyToken("secret-b", token); err == nil {
		t.Error("VerifyToken should reject a token signed with a different secret")
	}
}

func TestVerifyToken_Expired(t *testing.T) {
	secret := "test-secret"
	now := time.Now().UTC()
	claims := Claims{
		Email: "a@example.com",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "1",
			IssuedAt:  jwt.NewNumericDate(now.Add(-2 * time.Hour)),
			ExpiresAt: jwt.NewNumericDate(now.Add(-1 * time.Hour)), // expired 1h ago
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("failed to sign test token: %v", err)
	}

	if _, _, err := VerifyToken(secret, signed); err == nil {
		t.Error("VerifyToken should reject an expired token")
	}
}

func TestVerifyToken_WrongAlgorithm(t *testing.T) {
	secret := "test-secret"
	// Build a token signed with the "none" algorithm, which some
	// libraries mistakenly accept if they don't check the algorithm
	// explicitly. VerifyToken must reject this.
	claims := Claims{
		Email: "a@example.com",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "1",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodNone, claims)
	signed, err := token.SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatalf("failed to build 'none'-algorithm token: %v", err)
	}

	if _, _, err := VerifyToken(secret, signed); err == nil {
		t.Error("VerifyToken should reject a token using the 'none' algorithm")
	}
}

func TestVerifyToken_Garbage(t *testing.T) {
	if _, _, err := VerifyToken("test-secret", "not-a-real-token"); err == nil {
		t.Error("VerifyToken should reject a malformed token string")
	}
}
