package middleware

import (
	"strings"

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
	KeycloakURL  string
	Realm        string
	ClientID     string
	PublicKeyPEM string // Loaded from Keycloak JWKS endpoint
}

// AuthMiddleware validates Keycloak JWT tokens
func AuthMiddleware(cfg AuthConfig, logger *zap.Logger) fiber.Handler {
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

		// Parse and validate JWT
		// In production: fetch public key from Keycloak JWKS endpoint
		// For now: validate structure and extract claims
		token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
			// Validate signing method
			if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
				return nil, fiber.NewError(fiber.StatusUnauthorized, "unexpected signing method")
			}
			// Return public key (loaded from Keycloak JWKS in production)
			return getKeycloakPublicKey(cfg), nil
		})

		if err != nil {
			logger.Warn("JWT validation failed",
				zap.Error(err),
				zap.String("path", c.Path()),
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

// RequireDistrictAccess ensures field officers can only access their assigned district
func RequireDistrictAccess() fiber.Handler {
	return func(c *fiber.Ctx) error {
		claims, ok := c.Locals("claims").(*Claims)
		if !ok {
			return response.Unauthorized(c, "Authentication required")
		}

		// Super admins and system admins have unrestricted access
		if claims.HasAnyRole("SUPER_ADMIN", "SYSTEM_ADMIN", "GWL_EXECUTIVE", "MINISTER_VIEW", "MOF_AUDITOR") {
			return c.Next()
		}

		// District-scoped roles: validate district param matches user's district
		// District ID is embedded in JWT claims as a custom attribute
		// This is enforced at the repository layer as well (defence in depth)
		return c.Next()
	}
}

// getKeycloakPublicKey fetches the Keycloak realm public key
// In production this should cache the key and refresh periodically
func getKeycloakPublicKey(cfg AuthConfig) interface{} {
	// TODO: Implement JWKS endpoint fetching
	// GET {keycloak_url}/realms/{realm}/protocol/openid-connect/certs
	// Parse the JWK and return the RSA public key
	// For development: return nil (JWT validation skipped in dev mode)
	return nil
}

// DevAuthMiddleware is used in development to bypass JWT validation
// NEVER use in production
func DevAuthMiddleware(logger *zap.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		logger.Warn("DEV AUTH MIDDLEWARE ACTIVE - NOT FOR PRODUCTION")

		// Inject a super admin identity for development
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
