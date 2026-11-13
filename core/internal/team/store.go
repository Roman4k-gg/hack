package team

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresStore struct {
	pool *pgxpool.Pool
}

func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{pool: pool}
}

func (s *PostgresStore) Snapshot(ctx context.Context, filter Filter) (Snapshot, error) {
	if s == nil || s.pool == nil {
		return Snapshot{}, fmt.Errorf("team: postgres pool is not initialised")
	}

	interns, err := s.interns(ctx, filter)
	if err != nil {
		return Snapshot{}, err
	}

	snapshot := Snapshot{Interns: interns, Activity: map[string]ActivityRecord{}}

	ids := make([]string, 0, len(interns))
	for _, intern := range interns {
		ids = append(ids, intern.ID)
	}

	if len(ids) == 0 {
		return snapshot, nil
	}

	items, err := s.items(ctx, ids)
	if err != nil {
		return Snapshot{}, err
	}
	snapshot.Items = items

	activity, err := s.activity(ctx, ids)
	if err != nil {
		return Snapshot{}, err
	}
	snapshot.Activity = activity

	return snapshot, nil
}

func (s *PostgresStore) interns(ctx context.Context, filter Filter) ([]InternRecord, error) {
	query := `
		SELECT u.id::text, u.full_name, u.email, u.position,
		       COALESCE(t.id::text, ''), COALESCE(t.name, ''), COALESCE(t.slug, ''),
		       COALESCE(m.id::text, ''), COALESCE(m.full_name, ''),
		       COALESCE(p.id::text, ''), COALESCE(p.title, ''),
		       COALESCE(p.starts_on, CURRENT_DATE)
		  FROM users u
		  LEFT JOIN teams t ON t.id = u.team_id
		  LEFT JOIN users m ON m.id = u.mentor_id
		  LEFT JOIN onboarding_plans p ON p.user_id = u.id
		 WHERE u.role = 'intern'`

	args := []any{}

	if !filter.All {
		query += ` AND (u.team_id::text = $1 OR u.mentor_id::text = $2)`
		args = append(args, filter.TeamID, filter.MentorID)
	}

	query += ` ORDER BY u.full_name`

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("team: load interns: %w", err)
	}
	defer rows.Close()

	var interns []InternRecord

	for rows.Next() {
		var intern InternRecord
		if err := rows.Scan(
			&intern.ID,
			&intern.FullName,
			&intern.Email,
			&intern.Position,
			&intern.TeamID,
			&intern.TeamName,
			&intern.TeamSlug,
			&intern.MentorID,
			&intern.MentorName,
			&intern.PlanID,
			&intern.PlanTitle,
			&intern.StartsOn,
		); err != nil {
			return nil, fmt.Errorf("team: scan intern: %w", err)
		}

		intern.HasPlan = intern.PlanID != ""
		interns = append(interns, intern)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("team: iterate interns: %w", err)
	}

	return interns, nil
}

func (s *PostgresStore) items(ctx context.Context, internIDs []string) ([]ItemRecord, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT p.user_id::text, i.id::text, s.title, s.position, i.position,
		       i.title, i.status, COALESCE(i.blocked_reason, ''), i.due_on
		  FROM onboarding_items i
		  JOIN onboarding_plans p ON p.id = i.plan_id
		  JOIN onboarding_stages s ON s.id = i.stage_id
		 WHERE p.user_id::text = ANY($1::text[])
		 ORDER BY s.position, i.position`,
		internIDs,
	)
	if err != nil {
		return nil, fmt.Errorf("team: load items: %w", err)
	}
	defer rows.Close()

	var items []ItemRecord

	for rows.Next() {
		var item ItemRecord
		if err := rows.Scan(
			&item.InternID,
			&item.ItemID,
			&item.StageTitle,
			&item.StagePosition,
			&item.Position,
			&item.Title,
			&item.Status,
			&item.BlockedReason,
			&item.DueOn,
		); err != nil {
			return nil, fmt.Errorf("team: scan item: %w", err)
		}

		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("team: iterate items: %w", err)
	}

	return items, nil
}

func (s *PostgresStore) activity(ctx context.Context, internIDs []string) (map[string]ActivityRecord, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT e.user_id::text,
		       count(*) FILTER (WHERE e.name LIKE 'chat%'),
		       max(e.created_at)
		  FROM events e
		 WHERE e.user_id::text = ANY($1::text[])
		 GROUP BY e.user_id`,
		internIDs,
	)
	if err != nil {
		return nil, fmt.Errorf("team: load activity: %w", err)
	}
	defer rows.Close()

	activity := map[string]ActivityRecord{}

	for rows.Next() {
		var record ActivityRecord
		if err := rows.Scan(&record.UserID, &record.Questions, &record.LastEventAt); err != nil {
			return nil, fmt.Errorf("team: scan activity: %w", err)
		}

		activity[record.UserID] = record
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("team: iterate activity: %w", err)
	}

	return activity, nil
}
