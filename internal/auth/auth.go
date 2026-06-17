// Package auth implements password hashing and JWT session tokens for the
// ForgeGuardian dashboard's cookie-based login. This is independent of the
// static API-key auth in internal/api/middleware — both mechanisms coexist.
package auth

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

// HashPassword bcrypt-hashes a plaintext password.
func HashPassword(plaintext string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(plaintext), 12)
	if err != nil {
		return "", fmt.Errorf("auth: hash password: %w", err)
	}
	return string(hash), nil
}

// CheckPassword verifies plaintext against a bcrypt hash. Returns nil on match.
func CheckPassword(hash, plaintext string) error {
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(plaintext)); err != nil {
		return fmt.Errorf("auth: password mismatch: %w", err)
	}
	return nil
}

// Claims is the JWT payload for a dashboard session token.
type Claims struct {
	Email string `json:"email"`
	jwt.RegisteredClaims
}

// IssueToken creates a signed JWT (HS256) for the given email, expiring
// after ttl, signed with secret.
func IssueToken(email string, secret []byte, ttl time.Duration) (string, error) {
	now := time.Now()
	claims := Claims{
		Email: email,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(secret)
	if err != nil {
		return "", fmt.Errorf("auth: sign token: %w", err)
	}
	return signed, nil
}

// ParseToken verifies and parses a JWT string, returning its Claims.
// Any failure (expired, bad signature, malformed) is returned as an error;
// callers should treat any error as "unauthenticated".
func ParseToken(tokenString string, secret []byte) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("auth: unexpected signing method %v", t.Header["alg"])
		}
		return secret, nil
	})
	if err != nil {
		return nil, fmt.Errorf("auth: parse token: %w", err)
	}
	if !token.Valid {
		return nil, fmt.Errorf("auth: invalid token")
	}
	return claims, nil
}
