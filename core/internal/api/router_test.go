package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"ai-buddy/core/internal/auth"
	"ai-buddy/core/internal/chat"
	"ai-buddy/core/internal/onboarding"
	"ai-buddy/core/internal/platform/ratelimit"

	"github.com/stretchr/testify/require"
)

type stubUserStore struct {
	user auth.User
}

func (s *stubUserStore) UserByEmail(_ context.Context, email string) (auth.User, error) {
	if !strings.EqualFold(s.user.Email, email) {
		return auth.User{}, auth.ErrUserNotFound
	}
	return s.user, nil
}

func (s *stubUserStore) UserByID(_ context.Context, id string) (auth.User, error) {
	if s.user.ID != id {
		return auth.User{}, auth.ErrUserNotFound
	}
	return s.user, nil
}

type stubPlanStore struct {
	plan onboarding.Plan
}

func (s *stubPlanStore) PlanByUserID(_ context.Context, _ string) (onboarding.Plan, error) {
	return s.plan, nil
}

func (s *stubPlanStore) ItemByID(_ context.Context, _ string) (onboarding.Item, error) {
	return onboarding.Item{}, onboarding.ErrItemNotFound
}

func (s *stubPlanStore) UpdateItemStatus(_ context.Context, _, _, _ string) (onboarding.Item, error) {
	return onboarding.Item{}, onboarding.ErrItemNotFound
}

func newTestRouter(t *testing.T, aiURL string) (http.Handler, *auth.TokenManager) {
	t.Helper()

	tokens, err := auth.NewTokenManager("secret", time.Hour, "ai-buddy-core")
	require.NoError(t, err)

	user := auth.User{
		ID:       "user-1",
		Email:    "dasha@demo.local",
		FullName: "Дарья Соколова",
		Role:     "intern",
	}

	authHandler := auth.NewHandler(auth.NewService(&stubUserStore{user: user}, tokens), tokens, nil, nil)

	plan := onboarding.Plan{
		ID:        "plan-1",
		Title:     "Адаптация в команде Search",
		StartsOn:  time.Now(),
		Stages:    []onboarding.Stage{},
		Progress:  onboarding.Progress{Done: 1, Total: 2, Percent: 50},
		NextStep:  &onboarding.Item{ID: "item-1", Title: "Получить доступ к Jira"},
		UpdatedAt: time.Now(),
	}
	onboardingHandler := onboarding.NewHandler(
		onboarding.NewService(&stubPlanStore{plan: plan}, nil, nil),
		nil,
	)

	chatHandler := chat.NewHandler(
		chat.NewService(chat.Config{
			AIURL:   aiURL,
			AIPath:  "/api/v1/chat/stream",
			Timeout: 5 * time.Second,
			Limiter: ratelimit.New(nil, 0),
		}, chat.NewClient(), nil),
		nil,
		nil,
	)

	return New(Deps{
		Auth:       authHandler,
		Onboarding: onboardingHandler,
		Chat:       chatHandler,
		Tokens:     tokens,
	}), tokens
}

func TestRouterServesLoginAndProtectedRoutes(t *testing.T) {
	t.Parallel()

	router, _ := newTestRouter(t, "http://127.0.0.1:1")

	loginReq := httptest.NewRequest(http.MethodPost, "/auth/login",
		strings.NewReader(`{"email":"dasha@demo.local"}`))
	loginRec := httptest.NewRecorder()

	router.ServeHTTP(loginRec, loginReq)

	require.Equal(t, http.StatusOK, loginRec.Code)

	var session auth.Session
	require.NoError(t, json.Unmarshal(loginRec.Body.Bytes(), &session))
	require.NotEmpty(t, session.Token)

	meReq := httptest.NewRequest(http.MethodGet, "/me", nil)
	meReq.Header.Set("Authorization", "Bearer "+session.Token)
	meRec := httptest.NewRecorder()

	router.ServeHTTP(meRec, meReq)

	require.Equal(t, http.StatusOK, meRec.Code)

	planReq := httptest.NewRequest(http.MethodGet, "/onboarding/plan", nil)
	planReq.Header.Set("Authorization", "Bearer "+session.Token)
	planRec := httptest.NewRecorder()

	router.ServeHTTP(planRec, planReq)

	require.Equal(t, http.StatusOK, planRec.Code)

	var plan onboarding.Plan
	require.NoError(t, json.Unmarshal(planRec.Body.Bytes(), &plan))
	require.Equal(t, "Адаптация в команде Search", plan.Title)
}

func TestRouterRejectsAnonymousRequests(t *testing.T) {
	t.Parallel()

	router, _ := newTestRouter(t, "http://127.0.0.1:1")

	for _, path := range []string{"/me", "/onboarding/plan"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusUnauthorized, rec.Code, path)
	}
}

func TestRouterRejectsInvalidToken(t *testing.T) {
	t.Parallel()

	router, _ := newTestRouter(t, "http://127.0.0.1:1")

	req := httptest.NewRequest(http.MethodGet, "/me", nil)
	req.Header.Set("Authorization", "Bearer broken")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestRouterReturnsNotFoundForUnknownRoute(t *testing.T) {
	t.Parallel()

	router, _ := newTestRouter(t, "http://127.0.0.1:1")

	req := httptest.NewRequest(http.MethodGet, "/unknown", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusNotFound, rec.Code)
}

func TestStubStoreReturnsErrors(t *testing.T) {
	t.Parallel()

	users := &stubUserStore{user: auth.User{Email: "dasha@demo.local"}}

	_, err := users.UserByEmail(context.Background(), "other@demo.local")
	require.True(t, errors.Is(err, auth.ErrUserNotFound))

	_, err = users.UserByID(context.Background(), "other")
	require.True(t, errors.Is(err, auth.ErrUserNotFound))
}
