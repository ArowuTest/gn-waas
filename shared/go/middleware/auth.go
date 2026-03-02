package middleware

import (
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/ArowuTest/gn-waas/shared/go/http/response"
	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"
)

// Claims represents the JWT claims from Keycloak
type Claims struct {
	Sub               string              `json:"sub"`
	Email             string              `json:"email"`
	Name              string              `json:"name"`
	PreferredUsername string              `json:"preferred_username"`
	RealmAccess       RealmAccess         `json:"realm_access"`
	ResourceAccess    map[string]Resource `json:"resource_access"`
	// DistrictID is a custom Keycloak claim populated via a User Attribute mapper.
	// It restricts district-scoped roles to their assigned district.
	DistrictID        string              `json:"district_id,omitempty"`
	jwt.RegisteredClaims
}

// RealmAccess holds realm-level roles from Keycloak
type RealmAccess struct {
	Roles []string `json:"roles"`
}

// Resource holds resource-level roles from Keycloak
type Resource struct {
	Roles []string `json:"roles"`
}

// HasRole checks if the claims contain a specific role
func (c *Claims) HasRole(role string) bool {
	for _, r := range c.RealmAccess.Roles {
		if r == role {
			return true
		}
	}
	return false
}

// HasAnyRole checks if the claims contain any of the specified roles
func (c *Claims) HasAnyRole(roles ...string) bool {
	for _, role := range roles {
		if c.HasRole(role) {
			return true
		}
	}
	return false
}

// AuthConfig holds Keycloak configuration for JWT validation
type AuthConfig struct {
	KeycloakURL string
	Realm       string
	ClientID    string
}

// jwksCache caches the JWKS keys with a TTL to avoid fetching on every request
type jwksCache struct {
	mu        sync.RWMutex
	keys      map[string]*rsa.PublicKey // kid → public key
	fetchedAt time.Time
	ttl       time.Duration
}

var globalJWKSCache = &jwksCache{
	keys: make(map[string]*rsa.PublicKey),
	ttl:  15 * time.Minute,
}

// jwk represents a single JSON Web Key
type jwk struct {
	Kid string `json:"kid"`
	Kty string `json:"kty"`
	Alg string `json:"alg"`
	Use string `json:"use"`
	N   string `json:"n"`
	E   string `json:"e"`
}

// jwks represents the JSON Web Key Set response from Keycloak
type jwks struct {
	Keys []jwk `json:"keys"`
}

// fetchJWKS fetches and caches the public keys from Keycloak's JWKS endpoint.
// Keycloak endpoint: GET {keycloak_url}/realms/{realm}/protocol/openid-connect/certs
func fetchJWKS(cfg AuthConfig) error {
	url := fmt.Sprintf("%s/realms/%s/protocol/openid-connect/certs", cfg.KeycloakURL, cfg.Realm)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("JWKS fetch failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("JWKS endpoint returned HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("JWKS read body failed: %w", err)
	}

	var keySet jwks
	if err := json.Unmarshal(body, &keySet); err != nil {
		return fmt.Errorf("JWKS parse failed: %w", err)
	}

	newKeys := make(map[string]*rsa.PublicKey)
	for _, k := range keySet.Keys {
		if k.Kty != "RSA" || k.Use != "sig" {
			continue
		}
		pubKey, err := jwkToRSAPublicKey(k)
		if err != nil {
			continue
		}
		newKeys[k.Kid] = pubKey
	}

	globalJWKSCache.mu.Lock()
	globalJWKSCache.keys = newKeys
	globalJWKSCache.fetchedAt = time.Now()
	globalJWKSCache.mu.Unlock()

	return nil
}

// jwkToRSAPublicKey converts a JWK to an *rsa.PublicKey
func jwkToRSAPublicKey(k jwk) (*rsa.PublicKey, error) {
	// Decode base64url-encoded modulus (N) and exponent (E)
	nBytes, err := base64.RawURLEncoding.DecodeString(k.N)
	if err != nil {
		return nil, fmt.Errorf("decode N: %w", err)
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(k.E)
	if err != nil {
		return nil, fmt.Errorf("decode E: %w", err)
	}

	n := new(big.Int).SetBytes(nBytes)
	e := new(big.Int).SetBytes(eBytes)

	return &rsa.PublicKey{
		N: n,
		E: int(e.Int64()),
	}, nil
}

// getPublicKeyForToken returns the RSA public key matching the token's kid header.
// Refreshes the JWKS cache if stale or if the kid is not found.
func getPublicKeyForToken(cfg AuthConfig, token *jwt.Token) (interface{}, error) {
	kid, ok := token.Header["kid"].(string)
	if !ok {
		return nil, fmt.Errorf("token missing kid header")
	}

	// Check cache (read lock)
	globalJWKSCache.mu.RLock()
	key, found := globalJWKSCache.keys[kid]
	stale := time.Since(globalJWKSCache.fetchedAt) > globalJWKSCache.ttl
	globalJWKSCache.mu.RUnlock()

	if found && !stale {
		return key, nil
	}

	// Cache miss or stale — refresh
	if err := fetchJWKS(cfg); err != nil {
		// If refresh fails but we have a cached key, use it (graceful degradation)
		if found {
			return key, nil
		}
		return nil, fmt.Errorf("JWKS refresh failed and no cached key: %w", err)
	}

	globalJWKSCache.mu.RLock()
	key, found = globalJWKSCache.keys[kid]
	globalJWKSCache.mu.RUnlock()

	if !found {
		return nil, fmt.Errorf("no public key found for kid=%s", kid)
	}
	return key, nil
}

// AuthMiddleware validates Keycloak JWT tokens using live JWKS endpoint.
// Fetches and caches RSA public keys from Keycloak; refreshes every 15 minutes.
func AuthMiddleware(cfg AuthConfig, logger *zap.Logger) fiber.Handler {
	// Pre-warm the JWKS cache at startup
	go func() {
		if err := fetchJWKS(cfg); err != nil {
			logger.Warn("JWKS pre-warm failed (will retry on first request)",
				zap.String("keycloak_url", cfg.KeycloakURL),
				zap.String("realm", cfg.Realm),
				zap.Error(err),
			)
		} else {
			logger.Info("JWKS cache pre-warmed",
				zap.String("keycloak_url", cfg.KeycloakURL),
				zap.String("realm", cfg.Realm),
			)
		}
	}()

	return func(c *fiber.Ctx) error {
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return response.Unauthorized(c, "Authorization header is required")
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
			return response.Unauthorized(c, "Invalid authorization header format")
		}

		tokenString := parts[1]

		token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
			// Enforce RS256 — Keycloak default
			if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return getPublicKeyForToken(cfg, token)
		})

		if err != nil {
			logger.Warn("JWT validation failed",
				zap.Error(err),
				zap.String("path", c.Path()),
				zap.String("ip", c.IP()),
			)
			return response.Unauthorized(c, "Invalid or expired token")
		}

		claims, ok := token.Claims.(*Claims)
		if !ok || !token.Valid {
			return response.Unauthorized(c, "Invalid token claims")
		}

		// Store claims in context for downstream handlers
		c.Locals("user_id", claims.Sub)
		c.Locals("user_email", claims.Email)
		c.Locals("user_name", claims.Name)
		c.Locals("user_roles", claims.RealmAccess.Roles)
		c.Locals("claims", claims)

		return c.Next()
	}
}

// RequireRoles creates a middleware that enforces role-based access
func RequireRoles(roles ...string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		claims, ok := c.Locals("claims").(*Claims)
		if !ok {
			return response.Unauthorized(c, "Authentication required")
		}

		if !claims.HasAnyRole(roles...) {
			return response.Forbidden(c, "Insufficient permissions for this operation")
		}

		return c.Next()
	}
}

// RequireDistrictAccess ensures field officers can only access their assigned district.
// Super admins and executive roles bypass this check.
// District-scoped roles must have a district_id claim in their JWT that matches the
// district_id URL parameter or query string.
//
// Keycloak configuration required:
//   - Add a "district_id" user attribute to each district-scoped user
//   - Create a Keycloak mapper: User Attribute → Token Claim (name: "district_id")
//   - This populates claims.DistrictID from the JWT
func RequireDistrictAccess() fiber.Handler {
	return func(c *fiber.Ctx) error {
		claims, ok := c.Locals("claims").(*Claims)
		if !ok {
			return response.Unauthorized(c, "Authentication required")
		}

		// Unrestricted roles bypass district check
		if claims.HasAnyRole("SUPER_ADMIN", "SYSTEM_ADMIN", "GWL_EXECUTIVE", "MINISTER_VIEW", "MOF_AUDITOR") {
			return c.Next()
		}

		// Resolve the requested district from URL param or query string
		requestedDistrict := c.Params("district_id")
		if requestedDistrict == "" {
			requestedDistrict = c.Query("district_id")
		}

		// If no district is specified in the request, allow (endpoint may not be district-scoped)
		if requestedDistrict == "" {
			return c.Next()
		}

		// Enforce: the JWT district_id must match the requested district
		if claims.DistrictID == "" {
			// User has no district assigned — deny access to district-scoped resources
			return response.Forbidden(c, "No district assigned to your account. Contact your administrator.")
		}

		if claims.DistrictID != requestedDistrict {
			return response.Forbidden(c, "Access denied: you do not have permission to access this district's data")
		}

		return c.Next()
	}
}

// DevAuthMiddleware is used in development/testing to bypass JWT validation.
// SAFETY: panics immediately if APP_ENV=production to prevent accidental bypass.
func DevAuthMiddleware(logger *zap.Logger) fiber.Handler {
	// Hard production guard — this middleware must NEVER run in production.
	if appEnv := os.Getenv("APP_ENV"); appEnv == "production" {
		panic("FATAL: DevAuthMiddleware activated in production (APP_ENV=production). " +
			"Set DEV_MODE=false and use Keycloak JWT validation.")
	}
	return func(c *fiber.Ctx) error {
		logger.Warn("DEV AUTH MIDDLEWARE ACTIVE — NOT FOR PRODUCTION USE",
			zap.String("path", c.Path()),
			zap.String("ip", c.IP()),
		)

		c.Locals("user_id", "a0000001-0000-0000-0000-000000000001")
		c.Locals("user_email", "superadmin@gnwaas.gov.gh")
		c.Locals("user_name", "GN-WAAS Super Admin")
		c.Locals("user_roles", []string{"SUPER_ADMIN"})
		c.Locals("claims", &Claims{
			Sub:   "a0000001-0000-0000-0000-000000000001",
			Email: "superadmin@gnwaas.gov.gh",
			Name:  "GN-WAAS Super Admin",
			RealmAccess: RealmAccess{
				Roles: []string{"SUPER_ADMIN"},
			},
		})

		return c.Next()
	}
}
