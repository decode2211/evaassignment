// This file creates and checks the login tokens (JWTs - "JSON Web Tokens")
// that prove a request comes from a logged-in user, without the server
// needing to remember every logged-in session in memory or in the
// database. Think of a JWT like a signed, tamper-proof wristband: the
// server hands it out at login, and on every later request checks the
// signature to make sure nobody forged or altered it.
package auth

import (
	"errors"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// tokenTTL is how long a login token stays valid after being issued. After
// this, the user must log in again. 24 hours is a reasonable balance for a
// small assignment: long enough to not be annoying, short enough that a
// leaked token doesn't stay dangerous forever.
const tokenTTL = 24 * time.Hour

// ErrInvalidToken is returned for any token problem: bad signature, wrong
// algorithm, expired, or malformed. We deliberately do not distinguish
// between these cases to the caller - from the client's point of view they
// all mean the same thing ("you are not authenticated"), and a generic
// error avoids giving an attacker clues about why their forged token failed.
var ErrInvalidToken = errors.New("invalid or expired token")

// Claims is the data we embed inside every token. We reuse the standard
// "sub" (subject) claim to carry the user's ID, as a string, which is the
// conventional JWT way of identifying "who this token is about". Email is
// added as an extra convenience claim. Expiry ("exp") and issued-at ("iat")
// come for free from jwt.RegisteredClaims.
type Claims struct {
	Email string `json:"email"`
	jwt.RegisteredClaims
}

// IssueToken creates a new signed JWT for the given user, valid for 24
// hours from now. secret is the server's private signing key - anyone who
// has it could forge tokens, so it must come from a secure configuration
// value (see internal/config).
func IssueToken(secret string, userID int64, email string) (string, error) {
	now := time.Now().UTC()
	claims := Claims{
		Email: email,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   strconv.FormatInt(userID, 10),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(tokenTTL)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// VerifyToken checks a token's signature and expiry, and returns the
// user ID and email it contains if valid.
//
// We explicitly check that the token's algorithm is HS256 before trusting
// it. This defends against a well-known JWT attack where a malicious
// caller sends a token that claims to use a different (weaker or "none")
// algorithm, hoping a careless verifier will accept it without really
// checking the signature.
func VerifyToken(secret, tokenString string) (userID int64, email string, err error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return []byte(secret), nil
	})
	if err != nil || !token.Valid {
		return 0, "", ErrInvalidToken
	}

	id, convErr := strconv.ParseInt(claims.Subject, 10, 64)
	if convErr != nil {
		return 0, "", ErrInvalidToken
	}
	return id, claims.Email, nil
}
