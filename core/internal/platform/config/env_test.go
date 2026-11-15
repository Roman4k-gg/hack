package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestEnvInt(t *testing.T) {
	t.Setenv("TEST_INT", "")
	require.Equal(t, 7, envInt("TEST_INT", 7))

	t.Setenv("TEST_INT", "12")
	require.Equal(t, 12, envInt("TEST_INT", 7))

	t.Setenv("TEST_INT", "not-a-number")
	require.Equal(t, 7, envInt("TEST_INT", 7))

	t.Setenv("TEST_INT", "  15  ")
	require.Equal(t, 15, envInt("TEST_INT", 7))
}

func TestEnvBool(t *testing.T) {
	t.Setenv("TEST_BOOL", "")
	require.True(t, envBool("TEST_BOOL", true))

	t.Setenv("TEST_BOOL", "false")
	require.False(t, envBool("TEST_BOOL", true))

	t.Setenv("TEST_BOOL", "yes-please")
	require.True(t, envBool("TEST_BOOL", true))
}

func TestEnvList(t *testing.T) {
	t.Setenv("TEST_LIST", "")
	require.Equal(t, []string{"a"}, envList("TEST_LIST", []string{"a"}))

	t.Setenv("TEST_LIST", "x, y ,z")
	require.Equal(t, []string{"x", "y", "z"}, envList("TEST_LIST", nil))

	t.Setenv("TEST_LIST", " , , ")
	require.Equal(t, []string{"a"}, envList("TEST_LIST", []string{"a"}))
}

func TestEnvDuration(t *testing.T) {
	t.Setenv("TEST_DURATION", "")
	value, err := envDuration("TEST_DURATION", time.Minute)
	require.NoError(t, err)
	require.Equal(t, time.Minute, value)

	t.Setenv("TEST_DURATION", "90s")
	value, err = envDuration("TEST_DURATION", time.Minute)
	require.NoError(t, err)
	require.Equal(t, 90*time.Second, value)

	t.Setenv("TEST_DURATION", "broken")
	_, err = envDuration("TEST_DURATION", time.Minute)
	require.Error(t, err)
}

func TestEnvSeconds(t *testing.T) {
	t.Setenv("TEST_SECONDS", "")
	value, err := envSeconds("TEST_SECONDS", 30)
	require.NoError(t, err)
	require.Equal(t, 30*time.Second, value)

	t.Setenv("TEST_SECONDS", "45")
	value, err = envSeconds("TEST_SECONDS", 30)
	require.NoError(t, err)
	require.Equal(t, 45*time.Second, value)

	t.Setenv("TEST_SECONDS", "broken")
	_, err = envSeconds("TEST_SECONDS", 30)
	require.Error(t, err)
}

func TestEnvFallsBackToDefault(t *testing.T) {
	t.Setenv("TEST_STRING", "")
	require.Equal(t, "fallback", env("TEST_STRING", "fallback"))

	t.Setenv("TEST_STRING", "value")
	require.Equal(t, "value", env("TEST_STRING", "fallback"))
}

func TestValidateRequiresSecrets(t *testing.T) {
	t.Parallel()

	cfg := Config{
		PostgresDSN: "postgres://buddy:buddy@localhost:5432/buddy",
		RedisAddr:   "localhost:6379",
		JWTSecret:   "",
	}

	err := cfg.Validate()
	require.Error(t, err)
	require.Contains(t, err.Error(), "JWT_SECRET")
}

func TestParseFlagsIgnoresBrokenPairs(t *testing.T) {
	t.Parallel()

	flags := parseFlags("good=true,bad,other=false")
	require.Equal(t, map[string]bool{"good": true, "other": false}, flags)

	require.Empty(t, parseFlags("   "))
}
