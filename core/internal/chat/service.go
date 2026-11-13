package chat

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"ai-buddy/core/internal/platform/ratelimit"
	"ai-buddy/core/internal/platform/reqctx"
)

var (
	ErrEmptyMessage    = errors.New("chat: message is required")
	ErrRateLimited     = errors.New("chat: rate limit exceeded")
	ErrUpstream        = errors.New("chat: upstream is unavailable")
	ErrUpstreamStatus  = errors.New("chat: upstream returned error status")
	ErrUpstreamTimeout = errors.New("chat: upstream timeout")
)

type Config struct {
	AIURL   string
	AIPath  string
	Timeout time.Duration
	Limiter *ratelimit.Limiter
}

type Service struct {
	cfg    Config
	client *http.Client
	logger *slog.Logger
}

func NewService(cfg Config, client *http.Client, logger *slog.Logger) *Service {
	if client == nil {
		client = NewClient()
	}
	return &Service{cfg: cfg, client: client, logger: logger}
}

func NewClient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   10 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          100,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ResponseHeaderTimeout: 30 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
	}
}

type Stream struct {
	Body        io.ReadCloser
	ContentType string
	Cancel      context.CancelFunc
	Deadline    time.Time
}

func (s *Service) Stream(ctx context.Context, principal reqctx.Principal, authHeader, message string) (*Stream, error) {
	message = strings.TrimSpace(message)
	if message == "" {
		return nil, ErrEmptyMessage
	}

	if s.cfg.Limiter != nil {
		allowed, err := s.cfg.Limiter.Allow(ctx, "ratelimit:chat:"+principal.UserID)
		if err != nil {
			return nil, err
		}
		if !allowed {
			return nil, ErrRateLimited
		}
	}

	payload, err := json.Marshal(map[string]any{
		"message": message,
		"user_id": principal.UserID,
		"role":    principal.Role,
	})
	if err != nil {
		return nil, fmt.Errorf("chat: marshal request: %w", err)
	}

	timeout := s.cfg.Timeout
	if timeout <= 0 {
		timeout = 2 * time.Minute
	}
	streamCtx, cancel := context.WithTimeout(ctx, timeout)

	req, err := http.NewRequestWithContext(streamCtx, http.MethodPost, s.url(), bytes.NewReader(payload))
	if err != nil {
		cancel()
		return nil, fmt.Errorf("chat: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	if strings.TrimSpace(authHeader) != "" {
		req.Header.Set("Authorization", authHeader)
	}
	if traceID := reqctx.TraceID(ctx); traceID != "" {
		req.Header.Set("X-Request-ID", traceID)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		cancel()
		if errors.Is(streamCtx.Err(), context.DeadlineExceeded) {
			return nil, fmt.Errorf("%w: %v", ErrUpstreamTimeout, err)
		}
		return nil, fmt.Errorf("%w: %v", ErrUpstream, err)
	}

	if resp.StatusCode >= http.StatusBadRequest {
		defer resp.Body.Close()
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		cancel()
		return nil, fmt.Errorf("%w: status %d: %s", ErrUpstreamStatus, resp.StatusCode, strings.TrimSpace(string(snippet)))
	}

	contentType := resp.Header.Get("Content-Type")
	if strings.TrimSpace(contentType) == "" {
		contentType = "text/event-stream"
	}

	return &Stream{
		Body:        resp.Body,
		ContentType: contentType,
		Cancel:      cancel,
		Deadline:    time.Now().Add(timeout),
	}, nil
}

func (s *Service) url() string {
	base := strings.TrimRight(strings.TrimSpace(s.cfg.AIURL), "/")
	path := strings.TrimSpace(s.cfg.AIPath)
	if path == "" {
		path = "/api/v1/chat/stream"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return base + path
}
