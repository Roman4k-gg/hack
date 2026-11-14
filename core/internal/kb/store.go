package kb

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresStore struct {
	pool *pgxpool.Pool
}

func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{pool: pool}
}

func (s *PostgresStore) Search(ctx context.Context, query SearchQuery) ([]Item, error) {
	if s == nil || s.pool == nil {
		return nil, fmt.Errorf("kb: postgres pool is not initialised")
	}

	rows, err := s.pool.Query(ctx, `
		WITH hits AS (
			SELECT document_id, count(*) AS chunk_hits
			  FROM doc_chunks
			 WHERE content ILIKE $1
			 GROUP BY document_id
		)
		SELECT d.id::text,
		       d.title,
		       d.summary,
		       d.url,
		       COALESCE(t.name, ''),
		       d.owner_email,
		       d.updated_at,
		       CASE
		         WHEN d.title ILIKE $1 THEN 1.0
		         WHEN d.summary ILIKE $1 THEN 0.85
		         WHEN h.document_id IS NOT NULL THEN 0.7
		         ELSE 0.5
		       END AS score,
		       COALESCE(
		         (SELECT c.content FROM doc_chunks c
		           WHERE c.document_id = d.id AND c.content ILIKE $1
		           ORDER BY c.chunk_index LIMIT 1),
		         d.content
		       ) AS chunk
		  FROM kb_documents d
		  LEFT JOIN teams t ON t.id = d.team_id
		  LEFT JOIN hits h ON h.document_id = d.id
		 WHERE (d.title ILIKE $1
		     OR d.summary ILIKE $1
		     OR d.content ILIKE $1
		     OR h.document_id IS NOT NULL)
		   AND (
		        $2
		     OR ($3 AND d.visibility = 'public')
		     OR ($4 AND d.visibility = 'team' AND (d.team_id::text = $5 OR d.team_id IS NULL))
		     OR ($6 AND d.visibility = 'hr')
		   )
		 ORDER BY score DESC, d.updated_at DESC
		 LIMIT $7`,
		query.Pattern,
		query.Access.All,
		query.Access.Public,
		query.Access.Team,
		query.Access.TeamID,
		query.Access.HR,
		query.Limit,
	)
	if err != nil {
		return nil, fmt.Errorf("kb: search documents: %w", err)
	}
	defer rows.Close()

	var items []Item

	for rows.Next() {
		var item Item

		if err := rows.Scan(
			&item.DocumentID,
			&item.Title,
			&item.Summary,
			&item.URL,
			&item.Team,
			&item.Owner,
			&item.UpdatedAt,
			&item.Score,
			&item.Chunk,
		); err != nil {
			return nil, fmt.Errorf("kb: scan document: %w", err)
		}

		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("kb: iterate documents: %w", err)
	}

	return items, nil
}

func (s *PostgresStore) Document(ctx context.Context, documentID string) (Document, error) {
	if s == nil || s.pool == nil {
		return Document{}, fmt.Errorf("kb: postgres pool is not initialised")
	}

	var document Document
	var teamID string

	err := s.pool.QueryRow(ctx, `
		SELECT d.id::text, d.title, d.summary, d.content, d.url,
		       COALESCE(t.name, ''), d.owner_email, d.updated_at, d.visibility, d.tags,
		       COALESCE(d.team_id::text, '')
		  FROM kb_documents d
		  LEFT JOIN teams t ON t.id = d.team_id
		 WHERE d.id::text = $1`,
		documentID,
	).Scan(
		&document.DocumentID,
		&document.Title,
		&document.Summary,
		&document.Content,
		&document.URL,
		&document.Team,
		&document.Owner,
		&document.UpdatedAt,
		&document.Visibility,
		&document.Tags,
		&teamID,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Document{}, ErrDocumentNotFound
		}
		return Document{}, fmt.Errorf("kb: load document: %w", err)
	}

	document.TeamID = teamID

	return document, nil
}
