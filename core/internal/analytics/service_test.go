package analytics

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type fakeStore struct {
	questions []TopQuestion
	gaps      []ContentGap
	overview  Overview
}

func (s *fakeStore) TopQuestions(_ context.Context, _ int) ([]TopQuestion, error) {
	return s.questions, nil
}

func (s *fakeStore) ContentGaps(_ context.Context, _ int) ([]ContentGap, error) {
	return s.gaps, nil
}

func (s *fakeStore) Overview(_ context.Context) (Overview, error) {
	return s.overview, nil
}

func TestTopQuestionsReturnsList(t *testing.T) {
	t.Parallel()

	service := NewService(&fakeStore{questions: []TopQuestion{
		{Question: "где взять доступ к Jira?", Count: 4},
	}})

	questions, err := service.TopQuestions(context.Background(), 0)
	require.NoError(t, err)
	require.Len(t, questions, 1)
	require.Equal(t, 4, questions[0].Count)
}

func TestTopQuestionsReturnsEmptyListNotNil(t *testing.T) {
	t.Parallel()

	service := NewService(&fakeStore{})

	questions, err := service.TopQuestions(context.Background(), 0)
	require.NoError(t, err)
	require.NotNil(t, questions)
	require.Empty(t, questions)
}

func TestContentGapsReturnsList(t *testing.T) {
	t.Parallel()

	service := NewService(&fakeStore{gaps: []ContentGap{
		{Topic: "командировка", Questions: 3, Reason: "missing"},
		{Topic: "секреты", Questions: 2, Reason: "outdated"},
	}})

	gaps, err := service.ContentGaps(context.Background(), 0)
	require.NoError(t, err)
	require.Len(t, gaps, 2)
	require.Equal(t, "missing", gaps[0].Reason)
	require.Equal(t, "outdated", gaps[1].Reason)
}

func TestOverviewReturnsMetrics(t *testing.T) {
	t.Parallel()

	service := NewService(&fakeStore{overview: Overview{Interns: 6, Questions: 12, Requests: 3, Pending: 2}})

	overview, err := service.Overview(context.Background())
	require.NoError(t, err)
	require.Equal(t, 6, overview.Interns)
	require.Equal(t, 12, overview.Questions)
	require.Equal(t, 2, overview.Pending)
}

func TestKeywordsOf(t *testing.T) {
	t.Parallel()

	words := keywordsOf("Где взять доступ к Jira?")
	require.Contains(t, words, "взять")
	require.Contains(t, words, "доступ")
	require.NotContains(t, words, "к")
	require.NotContains(t, words, "jira")
	require.NotContains(t, words, "что")
	require.NotContains(t, words, "такое")
}

func TestStaleThreshold(t *testing.T) {
	t.Parallel()

	fresh := time.Now().AddDate(0, 0, -10)
	old := time.Now().AddDate(0, 0, -200)

	require.False(t, time.Since(fresh) > staleAfter)
	require.True(t, time.Since(old) > staleAfter)
}

func TestServiceWithoutStoreFails(t *testing.T) {
	t.Parallel()

	service := NewService(nil)

	_, err := service.TopQuestions(context.Background(), 0)
	require.Error(t, err)
}
