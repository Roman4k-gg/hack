package requests

import (
	"context"
	"testing"
	"time"

	"ai-buddy/core/internal/platform/reqctx"
	"ai-buddy/core/internal/rbac"

	"github.com/stretchr/testify/require"
)

type fakeBundles struct {
	items map[string][]AccessItem
}

func (b *fakeBundles) Bundle(_ context.Context, bundle string) ([]AccessItem, error) {
	return b.items[bundle], nil
}

func newAccessService(t *testing.T) (*Service, *fakeStore, *fakeTokens) {
	t.Helper()

	store := newFakeStore()
	tokens := newFakeTokens()
	absence := &fakeAbsence{result: AbsenceResult{ExternalID: "ABS-1", Status: StatusPending}}

	bundles := &fakeBundles{items: map[string][]AccessItem{
		"intern": {
			{System: "VPN", Reason: "Доступ в сеть", Approver: "IT-отдел"},
			{System: "Jira", Reason: "Задачи команды", Approver: "Наставник"},
		},
	}}

	service := NewService(store, tokens, absence, bundles, nil, nil)
	service.now = func() time.Time { return time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC) }

	return service, store, tokens
}

func accessIntern() reqctx.Principal {
	return reqctx.Principal{
		UserID: "user-1",
		Role:   rbac.RoleIntern,
		Scopes: []string{rbac.ScopeAccessSelf, rbac.ScopeAccessSelfWrite},
	}
}

func TestPreviewAccessReturnsBundle(t *testing.T) {
	t.Parallel()

	service, _, _ := newAccessService(t)

	preview, err := service.PreviewAccess(context.Background(), accessIntern(), "intern")
	require.NoError(t, err)
	require.NotEmpty(t, preview.ConfirmationToken)
	require.Equal(t, "intern", preview.Bundle)
	require.Len(t, preview.Items, 2)
	require.Equal(t, "VPN", preview.Items[0].System)
}

func TestPreviewAccessWithUnknownBundleFails(t *testing.T) {
	t.Parallel()

	service, _, _ := newAccessService(t)

	_, err := service.PreviewAccess(context.Background(), accessIntern(), "unknown")
	require.ErrorIs(t, err, ErrBundleNotFound)
}

func TestCreateAccessCreatesRequestsPerSystem(t *testing.T) {
	t.Parallel()

	service, store, _ := newAccessService(t)

	preview, err := service.PreviewAccess(context.Background(), accessIntern(), "intern")
	require.NoError(t, err)

	result, err := service.CreateAccess(context.Background(), accessIntern(),
		preview.PreviewID, preview.ConfirmationToken, "access-key-1")
	require.NoError(t, err)
	require.Len(t, result.Requests, 2)
	require.Equal(t, "VPN", result.Requests[0].System)
	require.NotEmpty(t, result.Requests[0].RequestID)
	require.NotEmpty(t, result.NextStep)
	require.Len(t, store.created, 2)
}

func TestCreateAccessIsIdempotent(t *testing.T) {
	t.Parallel()

	service, store, _ := newAccessService(t)

	preview, err := service.PreviewAccess(context.Background(), accessIntern(), "intern")
	require.NoError(t, err)

	first, err := service.CreateAccess(context.Background(), accessIntern(),
		preview.PreviewID, preview.ConfirmationToken, "access-key-1")
	require.NoError(t, err)

	second, err := service.CreateAccess(context.Background(), accessIntern(),
		preview.PreviewID, preview.ConfirmationToken, "access-key-1")
	require.NoError(t, err)

	require.Len(t, first.Requests, 2)
	require.Len(t, second.Requests, 2)
	require.Equal(t, first.Requests[0].RequestID, second.Requests[0].RequestID)
	require.Len(t, store.created, 2)
}

func TestCreateAccessRequiresToken(t *testing.T) {
	t.Parallel()

	service, _, _ := newAccessService(t)

	_, err := service.CreateAccess(context.Background(), accessIntern(), "preview", "", "access-key-1")
	require.ErrorIs(t, err, ErrMissingToken)

	_, err = service.CreateAccess(context.Background(), accessIntern(), "preview", "token", "")
	require.ErrorIs(t, err, ErrMissingKey)
}

func TestCreateAccessRejectsSpentToken(t *testing.T) {
	t.Parallel()

	service, _, _ := newAccessService(t)

	preview, err := service.PreviewAccess(context.Background(), accessIntern(), "intern")
	require.NoError(t, err)

	_, err = service.CreateAccess(context.Background(), accessIntern(),
		preview.PreviewID, preview.ConfirmationToken, "access-key-1")
	require.NoError(t, err)

	_, err = service.CreateAccess(context.Background(), accessIntern(),
		preview.PreviewID, preview.ConfirmationToken, "access-key-2")
	require.ErrorIs(t, err, ErrInvalidToken)
}
