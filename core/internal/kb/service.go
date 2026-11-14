package kb

import (
	"context"
	"fmt"
	"strings"
	"time"

	"ai-buddy/core/internal/platform/reqctx"
	"ai-buddy/core/internal/rbac"
)

const staleAfter = 180 * 24 * time.Hour

type Item struct {
	DocumentID string    `json:"document_id"`
	Title      string    `json:"title"`
	Summary    string    `json:"summary"`
	URL        string    `json:"url"`
	Team       string    `json:"team"`
	Owner      string    `json:"owner"`
	UpdatedAt  time.Time `json:"updated_at"`
	Stale      bool      `json:"stale"`
	Score      float64   `json:"score"`
	Chunk      string    `json:"chunk"`
}

type SearchResult struct {
	Items []Item `json:"items"`
	Total int    `json:"total"`
}

type Document struct {
	DocumentID string    `json:"document_id"`
	Title      string    `json:"title"`
	Summary    string    `json:"summary"`
	Content    string    `json:"content"`
	URL        string    `json:"url"`
	Team       string    `json:"team"`
	TeamID     string    `json:"team_id"`
	Owner      string    `json:"owner"`
	UpdatedAt  time.Time `json:"updated_at"`
	Stale      bool      `json:"stale"`
	Visibility string    `json:"visibility"`
	Tags       []string  `json:"tags"`
}

type Access struct {
	Public bool
	Team   bool
	HR     bool
	TeamID string
	All    bool
}

type SearchQuery struct {
	Pattern string
	Limit   int
	Access  Access
}

type Store interface {
	Search(ctx context.Context, query SearchQuery) ([]Item, error)
	Document(ctx context.Context, documentID string) (Document, error)
}

type Service struct {
	store Store
	now   func() time.Time
}

func NewService(store Store) *Service {
	return &Service{store: store, now: time.Now}
}

func (s *Service) Search(ctx context.Context, principal reqctx.Principal, query string, limit int) (SearchResult, error) {
	if s == nil || s.store == nil {
		return SearchResult{}, fmt.Errorf("kb: store is not initialised")
	}

	query = strings.TrimSpace(query)
	if query == "" {
		return SearchResult{}, ErrEmptyQuery
	}

	if limit <= 0 {
		limit = 10
	}

	if limit > 20 {
		limit = 20
	}

	items, err := s.store.Search(ctx, SearchQuery{
		Pattern: "%" + query + "%",
		Limit:   limit,
		Access:  accessFor(principal),
	})
	if err != nil {
		return SearchResult{}, err
	}

	if items == nil {
		items = []Item{}
	}

	now := s.now()
	for index := range items {
		items[index].Stale = now.Sub(items[index].UpdatedAt) > staleAfter
	}

	return SearchResult{Items: items, Total: len(items)}, nil
}

func (s *Service) Document(ctx context.Context, principal reqctx.Principal, documentID string) (Document, error) {
	if s == nil || s.store == nil {
		return Document{}, fmt.Errorf("kb: store is not initialised")
	}

	documentID = strings.TrimSpace(documentID)
	if documentID == "" {
		return Document{}, ErrDocumentNotFound
	}

	document, err := s.store.Document(ctx, documentID)
	if err != nil {
		return Document{}, err
	}

	if !accessFor(principal).Allows(document.Visibility, document.TeamID) {
		return Document{}, ErrDocumentForbidden
	}
	document.Stale = s.now().Sub(document.UpdatedAt) > staleAfter

	return document, nil
}

func accessFor(principal reqctx.Principal) Access {
	return Access{
		Public: rbac.HasScope(principal.Scopes, rbac.ScopeKBRead),
		Team:   rbac.HasScope(principal.Scopes, rbac.ScopeKBReadTeam),
		HR:     rbac.HasScope(principal.Scopes, rbac.ScopeKBReadHR),
		TeamID: principal.TeamID,
		All:    rbac.HasScope(principal.Scopes, rbac.ScopeAll),
	}
}

func (a Access) Allows(visibility, teamID string) bool {
	if a.All {
		return true
	}

	switch visibility {
	case "public":
		return a.Public
	case "hr":
		return a.HR
	case "team":
		if !a.Team {
			return false
		}
		return teamID == "" || teamID == a.TeamID
	default:
		return false
	}
}

func (a Access) Any() bool {
	return a.All || a.Public || a.Team || a.HR
}
