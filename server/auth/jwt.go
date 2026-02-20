package auth

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/MicahParks/keyfunc/v3"
)

// ValidateNeonToken validates a JWT from Neon Auth using JWKS and returns the claims.
// baseURL is the Neon Auth base URL (e.g. from NEON_AUTH_BASE_URL).
func ValidateNeonToken(baseURL, tokenString string) (jwt.MapClaims, error) {
	if baseURL == "" {
		return nil, fmt.Errorf("NEON_AUTH_BASE_URL is not set")
	}
	jwksURL := baseURL + "/.well-known/jwks.json"

	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid base URL: %w", err)
	}
	expectedIssuer := u.Scheme + "://" + u.Host

	jwks, err := keyfunc.NewDefault([]string{jwksURL})
	if err != nil {
		return nil, err
	}

	token, err := jwt.Parse(tokenString, jwks.Keyfunc,
		jwt.WithIssuer(expectedIssuer),
		jwt.WithValidMethods([]string{"EdDSA"}))
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}
	return claims, nil
}

// FirstNameFromClaims returns the first word of the "name" claim, or a fallback.
func FirstNameFromClaims(claims jwt.MapClaims) string {
	name, _ := claims["name"].(string)
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return "Jogador"
	}
	parts := strings.Fields(trimmed)
	if len(parts) > 0 {
		return parts[0]
	}
	return "Jogador"
}

// UserIDFromClaims returns the user id from claims ("sub" or "id").
func UserIDFromClaims(claims jwt.MapClaims) string {
	if sub, ok := claims["sub"].(string); ok && sub != "" {
		return sub
	}
	if id, ok := claims["id"].(string); ok && id != "" {
		return id
	}
	return ""
}
