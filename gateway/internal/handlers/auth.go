package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/bvanc/openpam/gateway/internal/auth"
	"github.com/bvanc/openpam/gateway/internal/logger"
	"github.com/bvanc/openpam/gateway/internal/repository"
	"github.com/google/uuid"
)

// AuthHandler handles authentication-related requests
type AuthHandler struct {
	entraID      *auth.EntraIDClient
	tokenManager *auth.TokenManager
	sessionStore auth.SessionStore
	stateStore   auth.StateStore
	userRepo     *repository.UserRepository
	logger       *logger.Logger
	devMode      bool
}

// NewAuthHandler creates a new authentication handler
func NewAuthHandler(
	entraID *auth.EntraIDClient,
	tokenManager *auth.TokenManager,
	sessionStore auth.SessionStore,
	stateStore auth.StateStore,
	userRepo *repository.UserRepository,
	log *logger.Logger,
	devMode bool,
) *AuthHandler {
	return &AuthHandler{
		entraID:      entraID,
		tokenManager: tokenManager,
		sessionStore: sessionStore,
		stateStore:   stateStore,
		userRepo:     userRepo,
		logger:       log,
		devMode:      devMode,
	}
}

// HandleLogin initiates the OAuth2 login flow
func (h *AuthHandler) HandleLogin() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// In dev mode, auto-login as test user
		if h.devMode {
			h.handleDevLogin(w, r)
			return
		}

		// Generate state parameter for CSRF protection
		state, err := auth.GenerateState()
		if err != nil {
			h.logger.Error("Failed to generate state", map[string]interface{}{
				"error": err.Error(),
			})
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		// Store state with expiration
		ctx := r.Context()
		if err := h.stateStore.Create(ctx, state, time.Now().Add(10*time.Minute)); err != nil {
			h.logger.Error("Failed to store state", map[string]interface{}{
				"error": err.Error(),
			})
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		// Get authorization URL
		authURL := h.entraID.GetAuthURL(state)

		h.logger.Info("Redirecting to EntraID login", map[string]interface{}{
			"state": state,
		})

		// Redirect to EntraID
		http.Redirect(w, r, authURL, http.StatusFound)
	}
}

// HandleCallback handles the OAuth2 callback
func (h *AuthHandler) HandleCallback() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// Get code and state from query parameters
		code := r.URL.Query().Get("code")
		state := r.URL.Query().Get("state")

		if code == "" || state == "" {
			h.logger.Warn("Missing code or state in callback")
			http.Error(w, "Missing code or state", http.StatusBadRequest)
			return
		}

		// Validate state
		valid, err := h.stateStore.Validate(ctx, state)
		if err != nil {
			h.logger.Error("Failed to validate state", map[string]interface{}{
				"error": err.Error(),
			})
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		if !valid {
			h.logger.Warn("Invalid state parameter")
			http.Error(w, "Invalid state parameter", http.StatusBadRequest)
			return
		}

		// Delete state after validation
		h.stateStore.Delete(ctx, state)

		// Exchange code for token
		token, err := h.entraID.ExchangeCode(ctx, code)
		if err != nil {
			h.logger.Error("Failed to exchange code", map[string]interface{}{
				"error": err.Error(),
			})
			http.Error(w, "Failed to authenticate", http.StatusUnauthorized)
			return
		}

		// Get user info from EntraID
		userInfo, err := h.entraID.GetUserInfo(ctx, token)
		if err != nil {
			h.logger.Error("Failed to get user info", map[string]interface{}{
				"error": err.Error(),
			})
			http.Error(w, "Failed to get user info", http.StatusInternalServerError)
			return
		}

		h.logger.Info("User authenticated", map[string]interface{}{
			"email":   userInfo.Email,
			"user_id": userInfo.ID,
		})

		// Get or create user in database
		user, err := h.userRepo.GetOrCreate(ctx, userInfo.ID, userInfo.Email, userInfo.DisplayName)
		if err != nil {
			h.logger.Error("Failed to get or create user", map[string]interface{}{
				"error": err.Error(),
			})
			http.Error(w, "Failed to create user", http.StatusInternalServerError)
			return
		}

		// Check if user is enabled
		if !user.Enabled {
			h.logger.Warn("Disabled user attempted login", map[string]interface{}{
				"user_id": user.ID.String(),
				"email":   user.Email,
			})
			http.Error(w, "Account disabled", http.StatusForbidden)
			return
		}

		// Generate JWT token
		jwtToken, err := h.tokenManager.GenerateToken(
			user.ID.String(),
			user.Email,
			user.DisplayName,
			user.Role,
		)
		if err != nil {
			h.logger.Error("Failed to generate token", map[string]interface{}{
				"error": err.Error(),
			})
			http.Error(w, "Failed to generate token", http.StatusInternalServerError)
			return
		}

		// Create session
		sessionID, err := auth.GenerateSessionID()
		if err != nil {
			h.logger.Error("Failed to generate session ID", map[string]interface{}{
				"error": err.Error(),
			})
			http.Error(w, "Failed to create session", http.StatusInternalServerError)
			return
		}

		session := &auth.Session{
			ID:          sessionID,
			UserID:      user.ID.String(),
			Email:       user.Email,
			DisplayName: user.DisplayName,
			CreatedAt:   time.Now(),
			ExpiresAt:   time.Now().Add(24 * time.Hour),
			Data:        make(map[string]interface{}),
		}

		if err := h.sessionStore.Create(ctx, session); err != nil {
			h.logger.Error("Failed to create session", map[string]interface{}{
				"error": err.Error(),
			})
			http.Error(w, "Failed to create session", http.StatusInternalServerError)
			return
		}

		// Set cookie with JWT token
		http.SetCookie(w, &http.Cookie{
			Name:     "openpam_token",
			Value:    jwtToken,
			Path:     "/",
			HttpOnly: true,
			Secure:   r.TLS != nil, // Only set Secure flag if using HTTPS
			SameSite: http.SameSiteLaxMode,
			MaxAge:   86400, // 24 hours
		})

		h.logger.Info("User logged in successfully", map[string]interface{}{
			"user_id": user.ID.String(),
			"email":   user.Email,
		})

		// Redirect to home page or return JSON
		response := map[string]interface{}{
			"success": true,
			"user": map[string]interface{}{
				"id":           user.ID.String(),
				"email":        user.Email,
				"display_name": user.DisplayName,
				"role":         user.Role,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}
}

// HandleLogout handles user logout
func (h *AuthHandler) HandleLogout() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// Get token from cookie
		cookie, err := r.Cookie("openpam_token")
		if err == nil && cookie.Value != "" {
			// Validate token to get user info
			claims, err := h.tokenManager.ValidateToken(cookie.Value)
			if err == nil {
				// Delete sessions for this user
				h.sessionStore.DeleteByUserID(ctx, claims.UserID)

				h.logger.Info("User logged out", map[string]interface{}{
					"user_id": claims.UserID,
				})
			}
		}

		// Clear cookie
		http.SetCookie(w, &http.Cookie{
			Name:     "openpam_token",
			Value:    "",
			Path:     "/",
			HttpOnly: true,
			Secure:   r.TLS != nil,
			SameSite: http.SameSiteLaxMode,
			MaxAge:   -1, // Delete cookie
		})

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"message": "Logged out successfully",
		})
	}
}

// HandleMe returns the current user's information
func (h *AuthHandler) HandleMe() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// Get token from cookie or Authorization header
		var token string

		// Try cookie first
		cookie, err := r.Cookie("openpam_token")
		if err == nil && cookie.Value != "" {
			token = cookie.Value
		} else {
			// Try Authorization header
			authHeader := r.Header.Get("Authorization")
			if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
				token = authHeader[7:]
			}
		}

		if token == "" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Validate token
		claims, err := h.tokenManager.ValidateToken(token)
		if err != nil {
			h.logger.Warn("Invalid token", map[string]interface{}{
				"error": err.Error(),
			})
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Get user from database
		userID, err := parseUUID(claims.UserID)
		if err != nil {
			http.Error(w, "Invalid user ID", http.StatusBadRequest)
			return
		}

		user, err := h.userRepo.GetByID(ctx, userID)
		if err != nil {
			h.logger.Error("Failed to get user", map[string]interface{}{
				"error":   err.Error(),
				"user_id": claims.UserID,
			})
			http.Error(w, "User not found", http.StatusNotFound)
			return
		}

		// Return user info
		response := map[string]interface{}{
			"id":           user.ID.String(),
			"email":        user.Email,
			"display_name": user.DisplayName,
			"enabled":      user.Enabled,
			"role":         user.Role,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}
}

// parseUUID is a helper to parse UUID strings
func parseUUID(s string) (uuid.UUID, error) {
	return uuid.Parse(s)
}

// handleDevLogin handles automatic login in development mode
func (h *AuthHandler) handleDevLogin(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	h.logger.Warn("Development mode: auto-logging in as test user", nil)

	// Get or create dev test user
	user, err := h.userRepo.GetOrCreate(
		ctx,
		"dev-user-123",
		"dev@example.com",
		"Development User",
	)
	if err != nil {
		h.logger.Error("Failed to get or create dev user", map[string]interface{}{
			"error": err.Error(),
		})
		http.Error(w, "Failed to create dev user", http.StatusInternalServerError)
		return
	}

	// Generate JWT token
	jwtToken, err := h.tokenManager.GenerateToken(
		user.ID.String(),
		user.Email,
		user.DisplayName,
		user.Role,
	)
	if err != nil {
		h.logger.Error("Failed to generate token", map[string]interface{}{
			"error": err.Error(),
		})
		http.Error(w, "Failed to generate token", http.StatusInternalServerError)
		return
	}

	// Create session
	sessionID, err := auth.GenerateSessionID()
	if err != nil {
		h.logger.Error("Failed to generate session ID", map[string]interface{}{
			"error": err.Error(),
		})
		http.Error(w, "Failed to create session", http.StatusInternalServerError)
		return
	}

	session := &auth.Session{
		ID:          sessionID,
		UserID:      user.ID.String(),
		Email:       user.Email,
		DisplayName: user.DisplayName,
		CreatedAt:   time.Now(),
		ExpiresAt:   time.Now().Add(24 * time.Hour),
		Data:        make(map[string]interface{}),
	}

	if err := h.sessionStore.Create(ctx, session); err != nil {
		h.logger.Error("Failed to create session", map[string]interface{}{
			"error": err.Error(),
		})
		http.Error(w, "Failed to create session", http.StatusInternalServerError)
		return
	}

	// Set cookie with JWT token
	http.SetCookie(w, &http.Cookie{
		Name:     "openpam_token",
		Value:    jwtToken,
		Path:     "/",
		HttpOnly: true,
		Secure:   false, // Dev mode typically doesn't use HTTPS
		SameSite: http.SameSiteLaxMode,
		MaxAge:   86400, // 24 hours
	})

	h.logger.Info("Dev user logged in", map[string]interface{}{
		"user_id": user.ID.String(),
		"email":   user.Email,
	})

	// Redirect back to frontend with token in query (for frontend to pick up)
	redirectURL := fmt.Sprintf("http://localhost:3000/auth/callback?token=%s", jwtToken)
	http.Redirect(w, r, redirectURL, http.StatusFound)
}
