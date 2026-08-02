package service

import (
	"context"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// JWTClaims represents the JWT claims structure with UserID and optional SessionID.
// SessionID (sid) ties the token to a row in sessions; when present, the auth
// middleware MUST consult sessions.revoked_at on every request.
type JWTClaims struct {
	Email     string `json:"email"`
	UserID    string `json:"user_id"`
	SessionID string `json:"sid,omitempty"`
	jwt.RegisteredClaims
}

// JWTManager handles JWT token generation and validation
type JWTManager struct {
	secretKey []byte
	issuer    string
	duration  time.Duration
}

// NewJWTManager creates a new JWT manager
func NewJWTManager(secretKey string, issuer string, duration time.Duration) *JWTManager {
	return &JWTManager{
		secretKey: []byte(secretKey),
		issuer:    issuer,
		duration:  duration,
	}
}

// Generate creates a new JWT token for the given email and userID (no session binding).
func (m *JWTManager) Generate(ctx context.Context, email, userID string) (string, error) {
	return m.GenerateWithSession(ctx, email, userID, "")
}

// GenerateWithSession issues a token bound to a sessions.id row. Spec 059 FR-009.
func (m *JWTManager) GenerateWithSession(_ context.Context, email, userID, sessionID string) (string, error) {
	now := time.Now()
	claims := JWTClaims{
		Email:     email,
		UserID:    userID,
		SessionID: sessionID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(m.duration)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    m.issuer,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(m.secretKey)
}

// Validate validates a JWT token and returns the claims
func (m *JWTManager) Validate(tokenString string) (*JWTClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		// Validate signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("invalid signing method")
		}
		return m.secretKey, nil
	})

	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*JWTClaims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token claims")
	}

	return claims, nil
}
