package auth

import (
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const defaultIssuer = "ai-buddy-core"

type Claims struct {
	jwt.RegisteredClaims
	Email  string   `json:"email"`
	Role   string   `json:"role"`
	TeamID string   `json:"team_id,omitempty"`
	Scopes []string `json:"scopes,omitempty"`
}

type TokenManager struct {
	secret []byte
	ttl    time.Duration
	issuer string
}

func NewTokenManager(secret string, ttl time.Duration, issuer string) (*TokenManager, error) {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return nil, fmt.Errorf("auth: jwt secret is required")
	}
	if ttl <= 0 {
		return nil, fmt.Errorf("auth: jwt ttl must be positive")
	}
	if strings.TrimSpace(issuer) == "" {
		issuer = defaultIssuer
	}

	return &TokenManager{
		secret: []byte(secret),
		ttl:    ttl,
		issuer: strings.TrimSpace(issuer),
	}, nil
}

func (m *TokenManager) TTL() time.Duration {
	return m.ttl
}

func (m *TokenManager) Issue(userID, email, role, teamID string, scopes []string) (string, time.Time, error) {
	if strings.TrimSpace(userID) == "" {
		return "", time.Time{}, fmt.Errorf("auth: user id is required")
	}

	now := time.Now().UTC()
	expiresAt := now.Add(m.ttl)

	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   strings.TrimSpace(userID),
			Issuer:    m.issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
		Email:  strings.TrimSpace(email),
		Role:   strings.TrimSpace(role),
		TeamID: strings.TrimSpace(teamID),
		Scopes: scopes,
	}

	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.secret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("auth: sign token: %w", err)
	}
	return signed, expiresAt, nil
}

func (m *TokenManager) Parse(raw string) (*Claims, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("auth: token is empty")
	}

	claims := &Claims{}
	token, err := jwt.ParseWithClaims(raw, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("auth: unexpected signing method %v", t.Header["alg"])
		}
		return m.secret, nil
	}, jwt.WithIssuer(m.issuer), jwt.WithExpirationRequired())
	if err != nil {
		return nil, fmt.Errorf("auth: parse token: %w", err)
	}
	if !token.Valid {
		return nil, fmt.Errorf("auth: token is not valid")
	}
	if claims.Subject == "" {
		return nil, fmt.Errorf("auth: token has no subject")
	}

	return claims, nil
}
