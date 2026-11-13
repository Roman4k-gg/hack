package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"ai-buddy/core/internal/platform/errors"

	"github.com/stretchr/testify/require"
)

func newTestHandler(t *testing.T) (*Handler, *TokenManager) {
	t.Helper()

	manager, err := NewTokenManager("secret", time.Hour, "ai-buddy-core")
	require.NoError(t, err)

	service := NewService(newFakeUserStore(demoUser()), manager)
	return NewHandler(service, manager, nil, nil), manager
}

func doLogin(t *testing.T, handler *Handler, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.Login()(rec, req)

	return rec
}

func TestLoginReturnsTokenAndUser(t *testing.T) {
	t.Parallel()

	handler, _ := newTestHandler(t)

	rec := doLogin(t, handler, `{"email":"dasha@demo.local"}`)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "application/json; charset=utf-8", rec.Header().Get("Content-Type"))

	var session Session
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &session))
	require.NotEmpty(t, session.Token)
	require.Equal(t, "Bearer", session.TokenType)
	require.Equal(t, "dasha@demo.local", session.User.Email)
	require.Equal(t, "intern", session.User.Role)
	require.Equal(t, "Search", session.User.TeamName)
}

func TestLoginWithoutEmailReturnsValidationError(t *testing.T) {
	t.Parallel()

	handler, _ := newTestHandler(t)

	rec := doLogin(t, handler, `{"email":""}`)

	require.Equal(t, http.StatusBadRequest, rec.Code)

	var envelope errors.Envelope
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope))
	require.Equal(t, errors.CodeValidation, envelope.Error.Code)
}

func TestLoginWithBrokenBodyReturnsValidationError(t *testing.T) {
	t.Parallel()

	handler, _ := newTestHandler(t)

	rec := doLogin(t, handler, `{`)

	require.Equal(t, http.StatusBadRequest, rec.Code)

	var envelope errors.Envelope
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope))
	require.Equal(t, errors.CodeValidation, envelope.Error.Code)
}

func TestLoginWithUnknownUserReturnsUnauthorized(t *testing.T) {
	t.Parallel()

	handler, _ := newTestHandler(t)

	rec := doLogin(t, handler, `{"email":"nobody@demo.local"}`)

	require.Equal(t, http.StatusUnauthorized, rec.Code)

	var envelope errors.Envelope
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope))
	require.Equal(t, errors.CodeUnauthorized, envelope.Error.Code)
}

func TestMeRequiresAuthorization(t *testing.T) {
	t.Parallel()

	handler, manager := newTestHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	rec := httptest.NewRecorder()

	RequireAuth(manager)(http.HandlerFunc(handler.Me())).ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)

	var envelope errors.Envelope
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope))
	require.Equal(t, errors.CodeUnauthorized, envelope.Error.Code)
}

func TestMeRejectsInvalidToken(t *testing.T) {
	t.Parallel()

	handler, manager := newTestHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	req.Header.Set("Authorization", "Bearer broken-token")
	rec := httptest.NewRecorder()

	RequireAuth(manager)(http.HandlerFunc(handler.Me())).ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)

	var envelope errors.Envelope
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope))
	require.Equal(t, errors.CodeUnauthorized, envelope.Error.Code)
}

func TestMeReturnsProfile(t *testing.T) {
	t.Parallel()

	handler, manager := newTestHandler(t)

	token, _, err := manager.Issue(demoUser().ID, demoUser().Email, demoUser().Role, demoUser().TeamID, []string{"onboarding:plan:read"})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	RequireAuth(manager)(http.HandlerFunc(handler.Me())).ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var user User
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &user))
	require.Equal(t, demoUser().ID, user.ID)
	require.Equal(t, "Дарья Соколова", user.FullName)
	require.Equal(t, "intern", user.Role)
	require.Equal(t, "search", user.TeamSlug)
	require.Equal(t, "Пётр Иванов", user.MentorName)
}
