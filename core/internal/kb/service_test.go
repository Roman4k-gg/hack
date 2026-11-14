package kb

import (
	"context"
	"testing"
	"time"

	"ai-buddy/core/internal/platform/reqctx"
	"ai-buddy/core/internal/rbac"

	"github.com/stretchr/testify/require"
)

type fakeStore struct {
	items    []Item
	document Document
	err      error
	query    SearchQuery
}

func (s *fakeStore) Search(_ context.Context, query SearchQuery) ([]Item, error) {
	s.query = query

	if s.err != nil {
		return nil, s.err
	}

	return s.items, nil
}

func (s *fakeStore) Document(_ context.Context, _ string) (Document, error) {
	if s.err != nil {
		return Document{}, s.err
	}

	return s.document, nil
}

func now() time.Time {
	return time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC)
}

func intern(teamID string) reqctx.Principal {
	return reqctx.Principal{
		UserID: "user-1",
		Role:   rbac.RoleIntern,
		TeamID: teamID,
		Scopes: []string{rbac.ScopeKBRead, rbac.ScopeKBReadTeam},
	}
}

func TestSearchBuildsPatternAndAccess(t *testing.T) {
	t.Parallel()

	store := &fakeStore{items: []Item{{DocumentID: "doc-1", Title: "Jira"}}}
	service := NewService(store)
	service.now = now

	result, err := service.Search(context.Background(), intern("team-search"), "доступ к Jira", 0)
	require.NoError(t, err)
	require.Equal(t, "%доступ к Jira%", store.query.Pattern)
	require.Equal(t, 10, store.query.Limit)
	require.True(t, store.query.Access.Public)
	require.True(t, store.query.Access.Team)
	require.False(t, store.query.Access.HR)
	require.Equal(t, "team-search", store.query.Access.TeamID)
	require.Equal(t, 1, result.Total)
	require.Len(t, result.Items, 1)
}

func TestSearchCapsLimit(t *testing.T) {
	t.Parallel()

	store := &fakeStore{}
	service := NewService(store)

	_, err := service.Search(context.Background(), intern("team-search"), "jira", 100)
	require.NoError(t, err)
	require.Equal(t, 20, store.query.Limit)
}

func TestSearchRequiresQuery(t *testing.T) {
	t.Parallel()

	store := &fakeStore{}
	service := NewService(store)

	_, err := service.Search(context.Background(), intern("team-search"), "   ", 0)
	require.ErrorIs(t, err, ErrEmptyQuery)
}

func TestSearchMarksStaleDocuments(t *testing.T) {
	t.Parallel()

	store := &fakeStore{items: []Item{
		{DocumentID: "fresh", UpdatedAt: now().AddDate(0, 0, -10)},
		{DocumentID: "old", UpdatedAt: now().AddDate(0, 0, -200)},
	}}
	service := NewService(store)
	service.now = now

	result, err := service.Search(context.Background(), intern("team-search"), "jira", 0)
	require.NoError(t, err)
	require.False(t, result.Items[0].Stale)
	require.True(t, result.Items[1].Stale)
}

func TestAccessAllows(t *testing.T) {
	t.Parallel()

	access := Access{Public: true, Team: true, TeamID: "team-search"}
	require.True(t, access.Allows("public", ""))
	require.True(t, access.Allows("team", "team-search"))
	require.False(t, access.Allows("team", "team-ads"))
	require.False(t, access.Allows("hr", ""))

	hr := Access{HR: true}
	require.True(t, hr.Allows("hr", ""))
	require.False(t, hr.Allows("team", "team-search"))

	admin := Access{All: true}
	require.True(t, admin.Allows("hr", "team-search"))
	require.True(t, admin.Allows("team", ""))
}

func TestDocumentRespectsAccess(t *testing.T) {
	t.Parallel()

	store := &fakeStore{document: Document{
		DocumentID: "doc-1",
		Title:      "Политика отпусков",
		Visibility: "hr",
		UpdatedAt:  now(),
	}}
	service := NewService(store)
	service.now = now

	_, err := service.Document(context.Background(), intern("team-search"), "doc-1")
	require.ErrorIs(t, err, ErrDocumentForbidden)

	hr := reqctx.Principal{UserID: "hr-1", Role: rbac.RoleHR, Scopes: []string{rbac.ScopeKBReadHR}}

	document, err := service.Document(context.Background(), hr, "doc-1")
	require.NoError(t, err)
	require.Equal(t, "Политика отпусков", document.Title)
	require.False(t, document.Stale)
}

func TestDocumentNotFound(t *testing.T) {
	t.Parallel()

	store := &fakeStore{err: ErrDocumentNotFound}
	service := NewService(store)

	_, err := service.Document(context.Background(), intern("team-search"), "missing")
	require.ErrorIs(t, err, ErrDocumentNotFound)
}

func TestServiceWithoutStoreFails(t *testing.T) {
	t.Parallel()

	service := NewService(nil)

	_, err := service.Search(context.Background(), intern("team-search"), "jira", 0)
	require.Error(t, err)
}
