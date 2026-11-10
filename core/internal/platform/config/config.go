package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	HTTPAddr             string
	PostgresDSN          string
	DBMaxConns           int32
	RedisAddr            string
	RedisPassword        string
	RedisDB              int
	JWTSecret            string
	JWTTTL               time.Duration
	AIURL                string
	MCPURL               string
	IntegrationsMode     string
	FeatureFlagsOverride map[string]bool
	MaxToolIterations    int
	ConfirmationTTL      time.Duration
	RateLimitRPS         int
	LogLevel             string
	SeedDemo             bool
	CORSAllowedOrigins   []string
	ShutdownTimeout      time.Duration
}

func Load() (Config, error) {
	cfg := Config{
		HTTPAddr:           env("HTTP_ADDR", ":8081"),
		PostgresDSN:        env("POSTGRES_DSN", ""),
		DBMaxConns:         int32(envInt("DB_MAX_CONNS", 10)),
		RedisAddr:          env("REDIS_ADDR", "localhost:6379"),
		RedisPassword:      env("REDIS_PASSWORD", ""),
		RedisDB:            envInt("REDIS_DB", 0),
		JWTSecret:          env("JWT_SECRET", "dev-secret-change-me"),
		AIURL:              env("AI_URL", "http://ai:8000"),
		MCPURL:             env("MCP_URL", "http://mcp:8001/mcp"),
		IntegrationsMode:   strings.ToLower(env("INTEGRATIONS_MODE", "mock")),
		LogLevel:           strings.ToLower(env("LOG_LEVEL", "info")),
		SeedDemo:           envBool("SEED_DEMO", true),
		CORSAllowedOrigins: envList("CORS_ALLOWED_ORIGINS", []string{"*"}),
		ShutdownTimeout:    15 * time.Second,
	}

	var err error
	if cfg.JWTTTL, err = envDuration("JWT_TTL", 8*time.Hour); err != nil {
		return Config{}, err
	}
	if cfg.ConfirmationTTL, err = envSeconds("CONFIRMATION_TTL", 300); err != nil {
		return Config{}, err
	}
	cfg.MaxToolIterations = envInt("MAX_TOOL_ITERATIONS", 5)
	cfg.RateLimitRPS = envInt("RATE_LIMIT_RPS", 5)
	cfg.FeatureFlagsOverride = parseFlags(env("FEATURE_FLAGS_OVERRIDE", ""))

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) Validate() error {
	if strings.TrimSpace(c.PostgresDSN) == "" {
		return fmt.Errorf("config: POSTGRES_DSN is required")
	}
	if strings.TrimSpace(c.RedisAddr) == "" {
		return fmt.Errorf("config: REDIS_ADDR is required")
	}
	switch c.IntegrationsMode {
	case "mock", "real":
	default:
		return fmt.Errorf("config: INTEGRATIONS_MODE must be \"mock\" or \"real\", got %q", c.IntegrationsMode)
	}
	return nil
}

func (c Config) AllowsAnyOrigin() bool {
	for _, o := range c.CORSAllowedOrigins {
		if o == "*" {
			return true
		}
	}
	return false
}

func env(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && strings.TrimSpace(v) != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	v, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(v) == "" {
		return def
	}
	n, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil {
		return def
	}
	return n
}

func envBool(key string, def bool) bool {
	v, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(v) == "" {
		return def
	}
	b, err := strconv.ParseBool(strings.TrimSpace(v))
	if err != nil {
		return def
	}
	return b
}

func envDuration(key string, def time.Duration) (time.Duration, error) {
	v, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(v) == "" {
		return def, nil
	}
	d, err := time.ParseDuration(strings.TrimSpace(v))
	if err != nil {
		return 0, fmt.Errorf("config: %s must be a duration: %w", key, err)
	}
	return d, nil
}

func envSeconds(key string, def int) (time.Duration, error) {
	v, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(v) == "" {
		return time.Duration(def) * time.Second, nil
	}
	n, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil {
		return 0, fmt.Errorf("config: %s must be an integer number of seconds: %w", key, err)
	}
	return time.Duration(n) * time.Second, nil
}

func envList(key string, def []string) []string {
	v, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(v) == "" {
		return def
	}
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return def
	}
	return out
}

func parseFlags(raw string) map[string]bool {
	flags := map[string]bool{}
	if strings.TrimSpace(raw) == "" {
		return flags
	}
	for _, pair := range strings.Split(raw, ",") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		key, value, found := strings.Cut(pair, "=")
		if !found {
			continue
		}
		key = strings.TrimSpace(key)
		b, err := strconv.ParseBool(strings.TrimSpace(value))
		if err != nil {
			continue
		}
		flags[key] = b
	}
	return flags
}
