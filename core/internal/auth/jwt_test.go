package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
)

func TestNewTokenManagerValidatesInput(t *testing.T) {
	t.Parallel()

	_, err := NewTokenManager("", time.Hour, "ai-buddy-core")
	require.Error(t, err)

	_, err = NewTokenManager("secret", 0, "ai-buddy-core")
	require.Error(t, err)

	manager, err := NewTokenManager("secret", time.Hour, "  ")
	require.NoError(t, err)
	require.Equal(t, defaultIssuer, manager.issuer)
	require.Equal(t, time.Hour, manager.TTL())
}

func TestTokenManagerIssueAndParse(t *testing.T) {
	t.Parallel()

	manager, err := NewTokenManager("secret", time.Hour, "ai-buddy-core")
	require.NoError(t, err)

	token, expiresAt, err := manager.Issue("user-1", "dasha@demo.local", "intern", "team-1")
	require.NoError(t, err)
	require.NotEmpty(t, token)
	require.WithinDuration(t, time.Now().Add(time.Hour), expiresAt, time.Minute)

	claims, err := manager.Parse(token)
	require.NoError(t, err)
	require.Equal(t, "user-1", claims.Subject)
	require.Equal(t, "dasha@demo.local", claims.Email)
	require.Equal(t, "intern", claims.Role)
	require.Equal(t, "team-1", claims.TeamID)
	require.Equal(t, "ai-buddy-core", claims.Issuer)
}

func TestTokenManagerIssueRequiresUserID(t *testing.T) {
	t.Parallel()

	manager, err := NewTokenManager("secret", time.Hour, "ai-buddy-core")
	require.NoError(t, err)

	_, _, err = manager.Issue("", "dasha@demo.local", "intern", "team-1")
	require.Error(t, err)
}

func TestTokenManagerParseRejectsForeignSecret(t *testing.T) {
	t.Parallel()

	issuer, err := NewTokenManager("secret", time.Hour, "ai-buddy-core")
	require.NoError(t, err)
	other, err := NewTokenManager("another-secret", time.Hour, "ai-buddy-core")
	require.NoError(t, err)

	token, _, err := issuer.Issue("user-1", "dasha@demo.local", "intern", "team-1")
	require.NoError(t, err)

	_, err = other.Parse(token)
	require.Error(t, err)
}

func TestTokenManagerParseRejectsExpiredToken(t *testing.T) {
	t.Parallel()

	manager, err := NewTokenManager("secret", time.Hour, "ai-buddy-core")
	require.NoError(t, err)

	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "user-1",
			Issuer:    "ai-buddy-core",
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour)),
		},
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte("secret"))
	require.NoError(t, err)

	_, err = manager.Parse(token)
	require.Error(t, err)
}

func TestTokenManagerParseRejectsMalformedToken(t *testing.T) {
	t.Parallel()

	manager, err := NewTokenManager("secret", time.Hour, "ai-buddy-core")
	require.NoError(t, err)

	_, err = manager.Parse("not-a-token")
	require.Error(t, err)

	_, err = manager.Parse("")
	require.Error(t, err)
}
