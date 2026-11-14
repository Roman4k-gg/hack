package analytics

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresStore struct {
	pool *pgxpool.Pool
}

func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{pool: pool}
}

func (s *PostgresStore) TopQuestions(ctx context.Context, limit int) ([]TopQuestion, error) {
	if s == nil || s.pool == nil {
		return nil, fmt.Errorf("analytics: postgres pool is not initialised")
	}

	rows, err := s.pool.Query(ctx, `
		SELECT payload->>'question' AS question, count(*)::int AS hits
		  FROM events
		 WHERE name = 'chat_stream_started'
		   AND COALESCE(payload->>'question', '') <> ''
		 GROUP BY payload->>'question'
		 ORDER BY hits DESC, question
		 LIMIT $1`,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("analytics: load top questions: %w", err)
	}
	defer rows.Close()

	questions := []TopQuestion{}

	for rows.Next() {
		var question TopQuestion
		if err := rows.Scan(&question.Question, &question.Count); err != nil {
			return nil, fmt.Errorf("analytics: scan top question: %w", err)
		}

		questions = append(questions, question)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("analytics: iterate top questions: %w", err)
	}

	return questions, nil
}

func (s *PostgresStore) ContentGaps(ctx context.Context, limit int) ([]ContentGap, error) {
	if s == nil || s.pool == nil {
		return nil, fmt.Errorf("analytics: postgres pool is not initialised")
	}

	questions, err := s.TopQuestions(ctx, limit)
	if err != nil {
		return nil, err
	}

	gaps := []ContentGap{}

	for _, question := range questions {
		_, updatedAt, found, err := s.DocumentMatch(ctx, question.Question)
		if err != nil {
			return nil, err
		}

		switch {
		case !found:
			gaps = append(gaps, ContentGap{
				Topic:     question.Question,
				Questions: question.Count,
				Reason:    "missing",
			})
		case time.Since(updatedAt) > staleAfter:
			gaps = append(gaps, ContentGap{
				Topic:     question.Question,
				Questions: question.Count,
				Reason:    "outdated",
			})
		}
	}

	return gaps, nil
}

func (s *PostgresStore) Overview(ctx context.Context) (Overview, error) {
	if s == nil || s.pool == nil {
		return Overview{}, fmt.Errorf("analytics: postgres pool is not initialised")
	}

	overview := Overview{TopQuestions: []TopQuestion{}}

	err := s.pool.QueryRow(ctx, `
		SELECT
			(SELECT count(*)::int FROM users WHERE role = 'intern'),
			(SELECT count(*)::int FROM events WHERE name = 'chat_stream_started'),
			(SELECT count(*)::int FROM requests),
			(SELECT count(*)::int FROM requests WHERE status = 'pending'),
			(SELECT count(*)::int FROM onboarding_items WHERE status = 'blocked')`,
	).Scan(
		&overview.Interns,
		&overview.Questions,
		&overview.Requests,
		&overview.Pending,
		&overview.BlockedItems,
	)
	if err != nil {
		return Overview{}, fmt.Errorf("analytics: load overview: %w", err)
	}

	questions, err := s.TopQuestions(ctx, 5)
	if err != nil {
		return Overview{}, err
	}

	overview.TopQuestions = questions

	return overview, nil
}

func (s *PostgresStore) DocumentMatch(ctx context.Context, query string) (string, time.Time, bool, error) {
	if s == nil || s.pool == nil {
		return "", time.Time{}, false, fmt.Errorf("analytics: postgres pool is not initialised")
	}

	query = normalizeQuestion(query)
	if query == "" {
		return "", time.Time{}, false, nil
	}

	words := keywordsOf(query)
	if len(words) == 0 {
		return "", time.Time{}, false, nil
	}

	conditions := make([]string, 0, len(words))
	scores := make([]string, 0, len(words))
	args := make([]any, 0, len(words))

	for index, word := range words {
		conditions = append(conditions, fmt.Sprintf(
			"(title ILIKE $%d OR summary ILIKE $%d OR content ILIKE $%d)", index+1, index+1, index+1))
		scores = append(scores, fmt.Sprintf(
			"(CASE WHEN title ILIKE $%d THEN 2 ELSE 0 END + CASE WHEN summary ILIKE $%d THEN 1 ELSE 0 END + CASE WHEN content ILIKE $%d THEN 1 ELSE 0 END)",
			index+1, index+1, index+1))
		args = append(args, "%"+word+"%")
	}

	var title string
	var updatedAt time.Time

	err := s.pool.QueryRow(ctx, `
		SELECT title, updated_at
		  FROM kb_documents
		 WHERE `+strings.Join(conditions, " OR ")+`
		 ORDER BY `+strings.Join(scores, " + ")+` DESC, updated_at DESC
		 LIMIT 1`,
		args...,
	).Scan(&title, &updatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", time.Time{}, false, nil
		}

		return "", time.Time{}, false, fmt.Errorf("analytics: match document: %w", err)
	}

	return title, updatedAt, true, nil
}

func keywordsOf(query string) []string {
	fields := strings.Fields(strings.ToLower(query))
	words := []string{}

	for _, field := range fields {
		word := strings.Trim(field, "?!.,:;")
		if len(word) < 5 || stopWords[word] {
			continue
		}

		runes := []rune(word)
		if len(runes) > keywordLength {
			word = string(runes[:keywordLength])
		}

		words = append(words, word)
	}

	return words
}

const keywordLength = 6

var stopWords = map[string]bool{
	"где": true, "как": true, "что": true, "такое": true, "какой": true,
	"какая": true, "какие": true, "когда": true, "почему": true, "зачем": true,
	"нужно": true, "надо": true, "можно": true, "чтобы": true, "если": true,
	"этот": true, "эта": true, "эти": true, "того": true, "ещё": true,
	"еще": true, "уже": true, "только": true, "очень": true, "тоже": true,
	"затем": true, "сейчас": true, "потом": true, "сразу": true, "там": true,
	"тут": true, "вот": true, "тебе": true, "мне": true, "нам": true,
	"вам": true, "него": true, "нее": true, "них": true, "раз": true,
	"два": true, "три": true, "свой": true, "свои": true,
}
