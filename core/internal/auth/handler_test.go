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
	return NewHandler(service, manager), manager
}

func TestLoginReturnsTokenAndUser(t *testing.T) {
	t.Parallel()

	handler, _ := newTestHandler(t)

	body := strings.NewReader(`{"email":"dasha@demo.local"}`)
	req := httptest.NewRequest(http.MethodPost, "/auth/login", body)
	rec := httptest.NewRecorder()

	handler.Router().ServeHTTP(rec, req)

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

	req := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(`{"email":""}`))
	rec := httptest.NewRecorder()

	handler.Router().ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)

	var envelope errors.Envelope
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope))
	require.Equal(t, errors.CodeValidation, envelope.Error.Code)
}

func TestLoginWithUnknownUserReturnsUnauthorized(t *testing.T) {
	t.Parallel()

	handler, _ := newTestHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(`{"email":"nobody@demo.local"}`))
	rec := httptest.NewRecorder()

	handler.Router().ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)

	var envelope errors.Envelope
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope))
	require.Equal(t, errors.CodeUnauthorized, envelope.Error.Code)
}

func TestMeRequiresAuthorization(t *testing.T) {
	t.Parallel()

	handler, _ := newTestHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/me", nil)
	rec := httptest.NewRecorder()

	handler.Router().ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)

	var envelope errors.Envelope
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope))
	require.Equal(t, errors.CodeUnauthorized, envelope.Error.Code)
}

func TestMeRejectsInvalidToken(t *testing.T) {
	t.Parallel()

	handler, _ := newTestHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/me", nil)
	req.Header.Set("Authorization", "Bearer broken-token")
	rec := httptest.NewRecorder()

	handler.Router().ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)

	var envelope errors.Envelope
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope))
	require.Equal(t, errors.CodeUnauthorized, envelope.Error.Code)
}

func TestMeReturnsProfile(t *testing.T) {
	t.Parallel()

	handler, manager := newTestHandler(t)

	token, _, err := manager.Issue(demoUser().ID, demoUser().Email, demoUser().Role, demoUser().TeamID)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	handler.Router().ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var user User
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &user))
	require.Equal(t, demoUser().ID, user.ID)
	require.Equal(t, "Дарья Соколова", user.FullName)
	require.Equal(t, "intern", user.Role)
	require.Equal(t, "search", user.TeamSlug)
	require.Equal(t, "Пётр Иванов", user.MentorName)
}
