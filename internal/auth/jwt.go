// JWT issuing and verification.
package auth

import (
	"errors"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// tokenTTL is how long a login token stays valid.
const tokenTTL = 24 * time.Hour

// ErrInvalidToken covers any token problem (bad signature, expired,
// malformed) - kept generic so a caller can't tell which check failed.
var ErrInvalidToken = errors.New("invalid or expired token")

// Claims holds the user ID (as the standard "sub" claim) and email.
type Claims struct {
	Email string `json:"email"`
	jwt.RegisteredClaims
}

// IssueToken creates a signed JWT for the given user, valid for 24 hours.
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

// VerifyToken checks a token's signature and expiry and returns its claims.
// The algorithm is checked explicitly to reject a token that claims a
// weaker (or "none") algorithm than the server actually signs with.
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
