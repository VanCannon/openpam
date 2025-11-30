package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/VanCannon/openpam/gateway/internal/logger"
	"github.com/VanCannon/openpam/gateway/internal/models"
)

func TestRequireRole(t *testing.T) {
	log := logger.Default()

	tests := []struct {
		name           string
		userRole       string
		requiredRole   string
		expectedStatus int
	}{
		{
			name:           "Admin Access",
			userRole:       models.RoleAdmin,
			requiredRole:   models.RoleUser,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Correct Role Access",
			userRole:       models.RoleUser,
			requiredRole:   models.RoleUser,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Incorrect Role Access",
			userRole:       models.RoleAuditor,
			requiredRole:   models.RoleUser,
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "No Role",
			userRole:       "",
			requiredRole:   models.RoleUser,
			expectedStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := RequireRole(tt.requiredRole, log)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))

			req := httptest.NewRequest("GET", "/", nil)
			if tt.userRole != "" {
				ctx := context.WithValue(req.Context(), roleKey, tt.userRole)
				req = req.WithContext(ctx)
			}

			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			if rr.Code != tt.expectedStatus {
				t.Errorf("handler returned wrong status code: got %v want %v",
					rr.Code, tt.expectedStatus)
			}
		})
	}
}

func TestRequireAnyRole(t *testing.T) {
	log := logger.Default()

	tests := []struct {
		name           string
		userRole       string
		requiredRoles  []string
		expectedStatus int
	}{
		{
			name:           "Admin Access",
			userRole:       models.RoleAdmin,
			requiredRoles:  []string{models.RoleUser, models.RoleAuditor},
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Allowed Role Access",
			userRole:       models.RoleUser,
			requiredRoles:  []string{models.RoleUser, models.RoleAuditor},
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Disallowed Role Access",
			userRole:       "guest",
			requiredRoles:  []string{models.RoleUser, models.RoleAuditor},
			expectedStatus: http.StatusForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := RequireAnyRole(tt.requiredRoles, log)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))

			req := httptest.NewRequest("GET", "/", nil)
			if tt.userRole != "" {
				ctx := context.WithValue(req.Context(), roleKey, tt.userRole)
				req = req.WithContext(ctx)
			}

			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			if rr.Code != tt.expectedStatus {
				t.Errorf("handler returned wrong status code: got %v want %v",
					rr.Code, tt.expectedStatus)
			}
		})
	}
}
