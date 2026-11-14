package analytics

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type TopQuestion struct {
	Question string `json:"question"`
	Count    int    `json:"count"`
}

type ContentGap struct {
	Topic     string `json:"topic"`
	Questions int    `json:"questions"`
	Reason    string `json:"reason,omitempty"`
}

type Overview struct {
	Interns      int           `json:"interns"`
	Questions    int           `json:"questions"`
	Requests     int           `json:"requests"`
	Pending      int           `json:"pending"`
	BlockedItems int           `json:"blocked_items"`
	TopQuestions []TopQuestion `json:"top_questions"`
}

type Store interface {
	TopQuestions(ctx context.Context, limit int) ([]TopQuestion, error)
	ContentGaps(ctx context.Context, limit int) ([]ContentGap, error)
	Overview(ctx context.Context) (Overview, error)
}

type Service struct {
	store Store
}

func NewService(store Store) *Service {
	return &Service{store: store}
}

func (s *Service) TopQuestions(ctx context.Context, limit int) ([]TopQuestion, error) {
	if s == nil || s.store == nil {
		return nil, fmt.Errorf("analytics: store is not initialised")
	}

	if limit <= 0 || limit > 50 {
		limit = 10
	}

	questions, err := s.store.TopQuestions(ctx, limit)
	if err != nil {
		return nil, err
	}

	if questions == nil {
		questions = []TopQuestion{}
	}

	return questions, nil
}

func (s *Service) ContentGaps(ctx context.Context, limit int) ([]ContentGap, error) {
	if s == nil || s.store == nil {
		return nil, fmt.Errorf("analytics: store is not initialised")
	}

	if limit <= 0 || limit > 50 {
		limit = 10
	}

	gaps, err := s.store.ContentGaps(ctx, limit)
	if err != nil {
		return nil, err
	}

	if gaps == nil {
		gaps = []ContentGap{}
	}

	return gaps, nil
}

func (s *Service) Overview(ctx context.Context) (Overview, error) {
	if s == nil || s.store == nil {
		return Overview{}, fmt.Errorf("analytics: store is not initialised")
	}

	return s.store.Overview(ctx)
}

const staleAfter = 180 * 24 * time.Hour

func normalizeQuestion(value string) string {
	return strings.TrimSpace(value)
}
