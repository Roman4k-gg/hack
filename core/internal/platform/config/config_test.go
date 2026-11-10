package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestLoadRequiresPostgresDSN(t *testing.T) {
	t.Setenv("POSTGRES_DSN", "")

	_, err := Load()
	require.Error(t, err)
	require.Contains(t, err.Error(), "POSTGRES_DSN")
}

func TestLoadUsesDefaults(t *testing.T) {
	t.Setenv("POSTGRES_DSN", "postgres://buddy:buddy@localhost:5432/buddy")
	t.Setenv("REDIS_ADDR", "")

	cfg, err := Load()
	require.NoError(t, err)

	require.Equal(t, ":8081", cfg.HTTPAddr)
	require.Equal(t, "localhost:6379", cfg.RedisAddr)
	require.Equal(t, "mock", cfg.IntegrationsMode)
	require.Equal(t, 5, cfg.MaxToolIterations)
	require.Equal(t, 5, cfg.RateLimitRPS)
	require.Equal(t, 300*time.Second, cfg.ConfirmationTTL)
	require.Equal(t, 8*time.Hour, cfg.JWTTTL)
}

func TestLoadParsesOverrides(t *testing.T) {
	t.Setenv("POSTGRES_DSN", "postgres://buddy:buddy@localhost:5432/buddy")
	t.Setenv("JWT_TTL", "1h")
	t.Setenv("CONFIRMATION_TTL", "60")
	t.Setenv("FEATURE_FLAGS_OVERRIDE", "absence_action=false,chat_enabled=true,broken")

	cfg, err := Load()
	require.NoError(t, err)

	require.Equal(t, time.Hour, cfg.JWTTTL)
	require.Equal(t, time.Minute, cfg.ConfirmationTTL)
	require.Equal(t, map[string]bool{"absence_action": false, "chat_enabled": true}, cfg.FeatureFlagsOverride)
}

func TestLoadRejectsUnknownIntegrationsMode(t *testing.T) {
	t.Setenv("POSTGRES_DSN", "postgres://buddy:buddy@localhost:5432/buddy")
	t.Setenv("INTEGRATIONS_MODE", "production")

	_, err := Load()
	require.Error(t, err)
	require.Contains(t, err.Error(), "INTEGRATIONS_MODE")
}

func TestAllowsAnyOrigin(t *testing.T) {
	require.True(t, Config{CORSAllowedOrigins: []string{"*"}}.AllowsAnyOrigin())
	require.False(t, Config{CORSAllowedOrigins: []string{"http://localhost:3000"}}.AllowsAnyOrigin())
}
