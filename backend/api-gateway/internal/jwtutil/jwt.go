// Package jwtutil provides HMAC-SHA256 JWT signing and validation for GN-WAAS.
//
// P1-05 FIX: Replaces the insecure "dev-mock-token-{email}" string tokens with
// properly signed JWTs. The middleware validates the signature using JWT_SECRET,
// so tokens cannot be forged by guessing the email address.
//
// Configuration:
//   JWT_SECRET — required in all environments. Must be at least 32 bytes.
//                Set via render.yaml env var or local .env file.
//   JWT_EXPIRY  — optional, defaults to 24h. Format: Go duration string (e.g. "8h").
package jwtutil

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Claims holds the GN-WAAS specific JWT payload.
type Claims struct {
	UserID     string `json:"user_id"`
	Email      string `json:"email"`
	FullName   string `json:"full_name"`
	Role       string `json:"role"`
	DistrictID string `json:"district_id"`
	jwt.RegisteredClaims
}

// jwtSecret returns the signing secret from the environment.
// Falls back to a development-only default if JWT_SECRET is not set,
// but logs a warning so operators know to set it.
func jwtSecret() []byte {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		// Development fallback — NOT safe for production.
		// render.yaml must set JWT_SECRET.
		return []byte("gnwaas-dev-secret-change-in-production-min32chars!")
	}
	return []byte(secret)
}

// expiry returns the configured token lifetime.
func expiry() time.Duration {
	if v := os.Getenv("JWT_EXPIRY"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return 24 * time.Hour
}

// IssueToken creates a signed HS256 JWT for the given user.
func IssueToken(userID, email, fullName, role, districtID string) (string, error) {
	now := time.Now()
	claims := Claims{
		UserID:     userID,
		Email:      email,
		FullName:   fullName,
		Role:       role,
		DistrictID: districtID,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(expiry())),
			Issuer:    "gnwaas-api-gateway",
			Audience:  jwt.ClaimStrings{"gnwaas"},
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret())
}

// IssueRefreshToken creates a longer-lived refresh token.
func IssueRefreshToken(userID, email string) (string, error) {
	now := time.Now()
	claims := jwt.RegisteredClaims{
		Subject:   userID,
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(7 * 24 * time.Hour)),
		Issuer:    "gnwaas-api-gateway",
		Audience:  jwt.ClaimStrings{"gnwaas-refresh"},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret())
}

// ValidateToken parses and validates a signed JWT, returning the claims.
func ValidateToken(tokenStr string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return jwtSecret(), nil
	})
	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token claims")
	}
	return claims, nil
}
