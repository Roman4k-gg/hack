package httpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

const readinessCheckTimeout = 2 * time.Second

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "ok",
		"service": "core",
		"version": Version,
		"time":    time.Now().UTC().Format(time.RFC3339),
	})
}

func (s *Server) handleReadyz(w http.ResponseWriter, r *http.Request) {
	checks := map[string]string{}
	var mu sync.Mutex
	var wg sync.WaitGroup

	check := func(name string, ping func(context.Context) error) {
		defer wg.Done()
		ctx, cancel := context.WithTimeout(r.Context(), readinessCheckTimeout)
		defer cancel()

		err := ping(ctx)
		mu.Lock()
		defer mu.Unlock()
		if err != nil {
			checks[name] = err.Error()
			return
		}
		checks[name] = "ok"
	}

	wg.Add(2)
	go check("postgres", s.store.Ping)
	go check("redis", s.cache.Ping)
	wg.Wait()

	ready := len(checks) == 2
	for _, status := range checks {
		if status != "ok" {
			ready = false
		}
	}

	status := "ok"
	code := http.StatusOK
	if !ready {
		status = "unavailable"
		code = http.StatusServiceUnavailable
	}

	writeJSON(w, code, map[string]any{
		"status":  status,
		"service": "core",
		"checks":  checks,
		"time":    time.Now().UTC().Format(time.RFC3339),
	})
}

func writeJSON(w http.ResponseWriter, status int, payload map[string]any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
