package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/bvanc/openpam/gateway/internal/auth"
	"github.com/bvanc/openpam/gateway/internal/logger"
)

// contextKey is a custom type for context keys
type contextKey string

const (
	userIDKey      contextKey = "user_id"
	userEmailKey   contextKey = "user_email"
	displayNameKey contextKey = "display_name"
	roleKey        contextKey = "role"
)

// RequireAuth returns a middleware that requires authentication
func RequireAuth(tokenManager *auth.TokenManager, log *logger.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Try to get token from cookie first
			var token string
			cookie, err := r.Cookie("openpam_token")
			if err == nil && cookie.Value != "" {
				token = cookie.Value
			} else {
				// Try Authorization header
				authHeader := r.Header.Get("Authorization")
				if authHeader == "" {
					log.Warn("Missing authorization", map[string]interface{}{
						"path": r.URL.Path,
					})
					http.Error(w, "Unauthorized", http.StatusUnauthorized)
					return
				}

				// Expect format: "Bearer <token>"
				parts := strings.SplitN(authHeader, " ", 2)
				if len(parts) != 2 || parts[0] != "Bearer" {
					log.Warn("Invalid authorization header format", map[string]interface{}{
						"path": r.URL.Path,
					})
					http.Error(w, "Unauthorized", http.StatusUnauthorized)
					return
				}

				token = parts[1]
			}

			// Validate JWT token
			claims, err := tokenManager.ValidateToken(token)
			if err != nil {
				log.Warn("Invalid token", map[string]interface{}{
					"path":  r.URL.Path,
					"error": err.Error(),
				})
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			// Add user info to context
			ctx := r.Context()
			ctx = context.WithValue(ctx, userIDKey, claims.UserID)
			ctx = context.WithValue(ctx, userEmailKey, claims.Email)
			ctx = context.WithValue(ctx, displayNameKey, claims.DisplayName)
			ctx = context.WithValue(ctx, roleKey, claims.Role)

			// Continue with the request
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetUserID retrieves the user ID from the request context
func GetUserID(ctx context.Context) string {
	if userID, ok := ctx.Value(userIDKey).(string); ok {
		return userID
	}
	return ""
}

// GetUserEmail retrieves the user email from the request context
func GetUserEmail(ctx context.Context) string {
	if email, ok := ctx.Value(userEmailKey).(string); ok {
		return email
	}
	return ""
}

// GetDisplayName retrieves the display name from the request context
func GetDisplayName(ctx context.Context) string {
	if name, ok := ctx.Value(displayNameKey).(string); ok {
		return name
	}
	return ""
}

// GetUserRole retrieves the user role from the request context
func GetUserRole(ctx context.Context) string {
	if role, ok := ctx.Value(roleKey).(string); ok {
		return role
	}
	return ""
}

// CORS returns a middleware that adds CORS headers
func CORS(allowedOrigins []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			// Check if origin is allowed
			allowed := false
			for _, allowedOrigin := range allowedOrigins {
				if allowedOrigin == "*" || allowedOrigin == origin {
					allowed = true
					w.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
					break
				}
			}

			if !allowed && len(allowedOrigins) > 0 {
				w.Header().Set("Access-Control-Allow-Origin", allowedOrigins[0])
			}

			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			w.Header().Set("Access-Control-Allow-Credentials", "true")

			// Handle preflight requests
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusOK)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
