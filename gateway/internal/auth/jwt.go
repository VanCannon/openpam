package auth

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// Claims represents the JWT claims
type Claims struct {
	UserID      string `json:"user_id"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
	jwt.RegisteredClaims
}

// TokenManager handles JWT token creation and validation
type TokenManager struct {
	secret     []byte
	expiration time.Duration
}

// NewTokenManager creates a new token manager
func NewTokenManager(secret string, expiration time.Duration) *TokenManager {
	return &TokenManager{
		secret:     []byte(secret),
		expiration: expiration,
	}
}

// GenerateToken creates a new JWT token for the user
func (tm *TokenManager) GenerateToken(userID, email, displayName string) (string, error) {
	now := time.Now()

	claims := Claims{
		UserID:      userID,
		Email:       email,
		DisplayName: displayName,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(tm.expiration)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    "openpam",
			Subject:   userID,
			ID:        uuid.New().String(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	tokenString, err := token.SignedString(tm.secret)
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}

	return tokenString, nil
}

// ValidateToken validates a JWT token and returns the claims
func (tm *TokenManager) ValidateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		// Verify signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return tm.secret, nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	if !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	claims, ok := token.Claims.(*Claims)
	if !ok {
		return nil, fmt.Errorf("invalid token claims")
	}

	return claims, nil
}

// RefreshToken creates a new token with extended expiration
func (tm *TokenManager) RefreshToken(oldToken string) (string, error) {
	claims, err := tm.ValidateToken(oldToken)
	if err != nil {
		return "", fmt.Errorf("cannot refresh invalid token: %w", err)
	}

	return tm.GenerateToken(claims.UserID, claims.Email, claims.DisplayName)
}
