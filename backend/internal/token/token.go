package token

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Claims holds the JWT payload. Subject (sub) contains the user ID.
type Claims struct {
	jwt.RegisteredClaims
}

// Generate creates a signed HS256 JWT for the given userID.
func Generate(userID, secret string, expiry time.Duration) (string, error) {
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(expiry)),
		},
	}

	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := t.SignedString([]byte(secret))
	if err != nil {
		return "", fmt.Errorf("sign token: %w", err)
	}

	return signed, nil
}

// Validate parses and verifies a JWT, returning its claims on success.
func Validate(tokenStr, secret string) (*Claims, error) {
	t, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, fmt.Errorf("parse token: %w", err)
	}

	// ParseWithClaims populates the *Claims we passed in; the assertion is safe.
	return t.Claims.(*Claims), nil
}
