package handlers

import (
	"encoding/base64"
	"encoding/json"
	"strings"
)

func (h *Handler) extractUserIDFromToken(authHeader string) string {
	if authHeader == "" {
		return ""
	}
	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || parts[0] != "Bearer" {
		return ""
	}
	token := parts[1]
	tokenParts := strings.Split(token, ".")
	if len(tokenParts) != 3 {
		return ""
	}

	// JWT payload is the second part
	payloadSegment := tokenParts[1]

	// Add padding if needed (standard base64) or use RawURLEncoding
	// JWT uses RawURLEncoding
	payload, err := base64.RawURLEncoding.DecodeString(payloadSegment)
	if err != nil {
		h.logger.Error("Failed to decode token payload", map[string]interface{}{"error": err.Error()})
		return ""
	}

	var claims map[string]interface{}
	if err := json.Unmarshal(payload, &claims); err != nil {
		h.logger.Error("Failed to unmarshal token claims", map[string]interface{}{"error": err.Error()})
		return ""
	}

	// Try standard claims
	if sub, ok := claims["sub"].(string); ok {
		return sub
	}
	if id, ok := claims["id"].(string); ok {
		return id
	}
	if uid, ok := claims["user_id"].(string); ok {
		return uid
	}

	return ""
}
