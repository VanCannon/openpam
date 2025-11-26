package middleware

import (
	"net/http"

	"github.com/bvanc/openpam/gateway/internal/logger"
	"github.com/bvanc/openpam/gateway/internal/models"
)

// RequireRole returns a middleware that requires a specific role
func RequireRole(role string, log *logger.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userRole := GetUserRole(r.Context())

			if userRole == "" {
				log.Warn("User role not found in context", map[string]interface{}{
					"path": r.URL.Path,
				})
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			// Admin can access everything
			if userRole == models.RoleAdmin {
				next.ServeHTTP(w, r)
				return
			}

			if userRole != role {
				log.Warn("Access denied: insufficient privileges", map[string]interface{}{
					"path":      r.URL.Path,
					"user_role": userRole,
					"required":  role,
				})
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RequireAnyRole returns a middleware that requires any of the specified roles
func RequireAnyRole(roles []string, log *logger.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userRole := GetUserRole(r.Context())

			if userRole == "" {
				log.Warn("User role not found in context", map[string]interface{}{
					"path": r.URL.Path,
				})
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			// Admin can access everything
			if userRole == models.RoleAdmin {
				next.ServeHTTP(w, r)
				return
			}

			allowed := false
			for _, role := range roles {
				if userRole == role {
					allowed = true
					break
				}
			}

			if !allowed {
				log.Warn("Access denied: insufficient privileges", map[string]interface{}{
					"path":      r.URL.Path,
					"user_role": userRole,
					"required":  roles,
				})
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
