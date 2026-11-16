package analytics

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"ai-buddy/core/internal/platform/reqctx"
	"ai-buddy/core/internal/rbac"

	"github.com/stretchr/testify/require"
)

type failingStore struct{}

func (failingStore) TopQuestions(_ context.Context, _ int) ([]TopQuestion, error) {
	return nil, errors.New("db down")
}

func (failingStore) ContentGaps(_ context.Context, _ int) ([]ContentGap, error) {
	return nil, errors.New("db down")
}

func (failingStore) Overview(_ context.Context) (Overview, error) {
	return Overview{}, errors.New("db down")
}

func serve(t *testing.T, handler *Handler, admin bool, path string, withPrincipal bool) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, path, nil)
	if withPrincipal {
		req = req.WithContext(reqctx.WithPrincipal(req.Context(), reqctx.Principal{
			UserID: "mentor-1",
			Role:   rbac.RoleMentor,
			Scopes: []string{rbac.ScopeAnalyticsTeam},
		}))
	}
	rec := httptest.NewRecorder()

	router := handler.Router()
	if admin {
		router = handler.AdminRouter()
	}
	router.ServeHTTP(rec, req)

	return rec
}

func TestHandlerServesAnalytics(t *testing.T) {
	t.Parallel()

	store := &fakeStore{
		questions: []TopQuestion{{Question: "где vpn", Count: 3}},
		gaps:      []ContentGap{{Topic: "wi-fi", Questions: 2}},
	}
	handler := NewHandler(NewService(store), nil)

	for _, path := range []string{"/top-questions?limit=5", "/top-questions?limit=abc", "/content-gaps"} {
		rec := serve(t, handler, false, path, true)
		require.Equal(t, http.StatusOK, rec.Code, path)
		require.Contains(t, rec.Header().Get("Content-Type"), "application/json")
	}

	rec := serve(t, handler, true, "/overview", true)
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestHandlerRejectsAndReportsErrors(t *testing.T) {
	t.Parallel()

	handler := NewHandler(NewService(failingStore{}), nil)

	cases := []struct {
		admin bool
		path  string
	}{
		{false, "/top-questions"},
		{false, "/content-gaps"},
		{true, "/overview"},
	}

	for _, item := range cases {
		rec := serve(t, handler, item.admin, item.path, true)
		require.Equal(t, http.StatusInternalServerError, rec.Code, item.path)

		rec = serve(t, handler, item.admin, item.path, false)
		require.NotEqual(t, http.StatusOK, rec.Code, item.path)
	}
}
