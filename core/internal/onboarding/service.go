package onboarding

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"ai-buddy/core/internal/audit"
	"ai-buddy/core/internal/platform/reqctx"
	"ai-buddy/core/internal/rbac"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrPlanNotFound  = errors.New("onboarding: plan not found")
	ErrItemNotFound  = errors.New("onboarding: item not found")
	ErrForbidden     = errors.New("onboarding: access denied")
	ErrInvalidStatus = errors.New("onboarding: invalid status")
	ErrMissingReason = errors.New("onboarding: blocked reason is required")
)

var itemStatuses = map[string]struct{}{
	"pending":     {},
	"in_progress": {},
	"done":        {},
	"blocked":     {},
	"skipped":     {},
}

type Progress struct {
	Done    int `json:"done"`
	Total   int `json:"total"`
	Percent int `json:"percent"`
}

type Item struct {
	ID            string     `json:"id"`
	StageID       string     `json:"stage_id"`
	Title         string     `json:"title"`
	Description   string     `json:"description"`
	Kind          string     `json:"kind"`
	Status        string     `json:"status"`
	BlockedReason string     `json:"blocked_reason,omitempty"`
	DueOn         *time.Time `json:"due_on,omitempty"`
	Position      int        `json:"position"`
	CompletedAt   *time.Time `json:"completed_at,omitempty"`
	OwnerID       string     `json:"-"`
}

type Stage struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Position    int      `json:"position"`
	Items       []Item   `json:"items"`
	Progress    Progress `json:"progress"`
}

type Plan struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	StartsOn  time.Time `json:"starts_on"`
	Stages    []Stage   `json:"stages"`
	Progress  Progress  `json:"progress"`
	NextStep  *Item     `json:"next_step,omitempty"`
	UpdatedAt time.Time `json:"updated_at"`
}

type PlanStore interface {
	PlanByUserID(ctx context.Context, userID string) (Plan, error)
	ItemByID(ctx context.Context, itemID string) (Item, error)
	UpdateItemStatus(ctx context.Context, itemID, status, blockedReason string) (Item, error)
}

type PostgresPlanStore struct {
	pool *pgxpool.Pool
}

func NewPostgresPlanStore(pool *pgxpool.Pool) *PostgresPlanStore {
	return &PostgresPlanStore{pool: pool}
}

func (s *PostgresPlanStore) PlanByUserID(ctx context.Context, userID string) (Plan, error) {
	if s == nil || s.pool == nil {
		return Plan{}, fmt.Errorf("onboarding: postgres pool is not initialised")
	}

	var plan Plan
	err := s.pool.QueryRow(ctx, `
		SELECT id::text, title, starts_on, updated_at
		  FROM onboarding_plans
		 WHERE user_id::text = $1`,
		userID,
	).Scan(&plan.ID, &plan.Title, &plan.StartsOn, &plan.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Plan{}, ErrPlanNotFound
	}
	if err != nil {
		return Plan{}, fmt.Errorf("onboarding: load plan: %w", err)
	}

	stages, err := s.stages(ctx, plan.ID)
	if err != nil {
		return Plan{}, err
	}
	items, err := s.items(ctx, plan.ID)
	if err != nil {
		return Plan{}, err
	}

	byStage := map[string][]Item{}
	for _, item := range items {
		byStage[item.StageID] = append(byStage[item.StageID], item)
	}

	plan.Stages = make([]Stage, 0, len(stages))
	for _, stage := range stages {
		stage.Items = byStage[stage.ID]
		if stage.Items == nil {
			stage.Items = []Item{}
		}
		stage.Progress = progressOf(stage.Items)
		plan.Stages = append(plan.Stages, stage)
	}

	plan.Progress = progressOf(items)
	plan.NextStep = nextStepOf(plan.Stages)

	return plan, nil
}

func (s *PostgresPlanStore) stages(ctx context.Context, planID string) ([]Stage, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id::text, title, description, position
		  FROM onboarding_stages
		 WHERE plan_id::text = $1
		 ORDER BY position`,
		planID,
	)
	if err != nil {
		return nil, fmt.Errorf("onboarding: load stages: %w", err)
	}
	defer rows.Close()

	var stages []Stage
	for rows.Next() {
		var stage Stage
		if err := rows.Scan(&stage.ID, &stage.Title, &stage.Description, &stage.Position); err != nil {
			return nil, fmt.Errorf("onboarding: scan stage: %w", err)
		}
		stages = append(stages, stage)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("onboarding: iterate stages: %w", err)
	}
	return stages, nil
}

func (s *PostgresPlanStore) items(ctx context.Context, planID string) ([]Item, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id::text, stage_id::text, title, description, kind, status,
		       COALESCE(blocked_reason, ''), due_on, position, completed_at
		  FROM onboarding_items
		 WHERE plan_id::text = $1
		 ORDER BY position`,
		planID,
	)
	if err != nil {
		return nil, fmt.Errorf("onboarding: load items: %w", err)
	}
	defer rows.Close()

	var items []Item
	for rows.Next() {
		item, err := scanItem(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("onboarding: iterate items: %w", err)
	}
	return items, nil
}

func (s *PostgresPlanStore) ItemByID(ctx context.Context, itemID string) (Item, error) {
	if s == nil || s.pool == nil {
		return Item{}, fmt.Errorf("onboarding: postgres pool is not initialised")
	}

	row := s.pool.QueryRow(ctx, `
		SELECT i.id::text, i.stage_id::text, i.title, i.description, i.kind, i.status,
		       COALESCE(i.blocked_reason, ''), i.due_on, i.position, i.completed_at, p.user_id::text
		  FROM onboarding_items i
		  JOIN onboarding_plans p ON p.id = i.plan_id
		 WHERE i.id::text = $1`,
		itemID,
	)

	item, err := scanItemWithOwner(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return Item{}, ErrItemNotFound
	}
	if err != nil {
		return Item{}, err
	}
	return item, nil
}

func (s *PostgresPlanStore) UpdateItemStatus(ctx context.Context, itemID, status, blockedReason string) (Item, error) {
	if s == nil || s.pool == nil {
		return Item{}, fmt.Errorf("onboarding: postgres pool is not initialised")
	}

	row := s.pool.QueryRow(ctx, `
		UPDATE onboarding_items
		   SET status = $2,
		       blocked_reason = $3,
		       completed_at = CASE WHEN $2 = 'done' THEN COALESCE(completed_at, now()) ELSE NULL END
		 WHERE id::text = $1
		RETURNING id::text, stage_id::text, title, description, kind, status,
		          COALESCE(blocked_reason, ''), due_on, position, completed_at`,
		itemID, status, blockedReason,
	)

	item, err := scanItem(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return Item{}, ErrItemNotFound
	}
	if err != nil {
		return Item{}, err
	}
	return item, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanItem(row rowScanner) (Item, error) {
	var item Item
	err := row.Scan(
		&item.ID,
		&item.StageID,
		&item.Title,
		&item.Description,
		&item.Kind,
		&item.Status,
		&item.BlockedReason,
		&item.DueOn,
		&item.Position,
		&item.CompletedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Item{}, err
		}
		return Item{}, fmt.Errorf("onboarding: scan item: %w", err)
	}
	return item, nil
}

func scanItemWithOwner(row rowScanner) (Item, error) {
	var item Item
	err := row.Scan(
		&item.ID,
		&item.StageID,
		&item.Title,
		&item.Description,
		&item.Kind,
		&item.Status,
		&item.BlockedReason,
		&item.DueOn,
		&item.Position,
		&item.CompletedAt,
		&item.OwnerID,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Item{}, err
		}
		return Item{}, fmt.Errorf("onboarding: scan item: %w", err)
	}
	return item, nil
}

func progressOf(items []Item) Progress {
	progress := Progress{Total: len(items)}
	for _, item := range items {
		if item.Status == "done" {
			progress.Done++
		}
	}
	if progress.Total > 0 {
		progress.Percent = progress.Done * 100 / progress.Total
	}
	return progress
}

func nextStepOf(stages []Stage) *Item {
	for _, stage := range stages {
		for _, item := range stage.Items {
			if item.Status == "done" || item.Status == "skipped" {
				continue
			}
			current := item
			return &current
		}
	}
	return nil
}

type UpdateItemInput struct {
	Status        string `json:"status"`
	BlockedReason string `json:"blocked_reason"`
}

type Service struct {
	store    PlanStore
	recorder *audit.Recorder
	logger   *slog.Logger
}

func NewService(store PlanStore, recorder *audit.Recorder, logger *slog.Logger) *Service {
	return &Service{store: store, recorder: recorder, logger: logger}
}

func (s *Service) Plan(ctx context.Context, userID string) (Plan, error) {
	if userID == "" {
		return Plan{}, ErrPlanNotFound
	}
	return s.store.PlanByUserID(ctx, userID)
}

func (s *Service) UpdateItem(ctx context.Context, principal reqctx.Principal, itemID string, input UpdateItemInput) (Item, error) {
	status := normalizeStatus(input.Status)
	if status == "" {
		return Item{}, ErrInvalidStatus
	}

	blockedReason := input.BlockedReason
	if status == "blocked" && blockedReason == "" {
		return Item{}, ErrMissingReason
	}
	if status != "blocked" {
		blockedReason = ""
	}

	current, err := s.store.ItemByID(ctx, itemID)
	if err != nil {
		return Item{}, err
	}

	if !canEdit(principal, current.OwnerID) {
		return Item{}, ErrForbidden
	}

	updated, err := s.store.UpdateItemStatus(ctx, itemID, status, blockedReason)
	if err != nil {
		return Item{}, err
	}

	if s.recorder != nil {
		traceID := reqctx.TraceID(ctx)
		if err := s.recorder.Record(ctx, audit.Entry{
			ActorID:    principal.UserID,
			Action:     "onboarding.item_updated",
			EntityType: "onboarding_item",
			EntityID:   updated.ID,
			Payload: map[string]any{
				"from":   current.Status,
				"to":     updated.Status,
				"reason": updated.BlockedReason,
			},
			TraceID: traceID,
		}); err != nil {
			s.logWarn("cannot write audit log", err)
		}
		if err := s.recorder.Emit(ctx, audit.Event{
			UserID:     principal.UserID,
			Name:       "onboarding_item_updated",
			EntityType: "onboarding_item",
			EntityID:   updated.ID,
			Payload: map[string]any{
				"status": updated.Status,
			},
			TraceID: traceID,
		}); err != nil {
			s.logWarn("cannot write event", err)
		}
	}

	return updated, nil
}

func canEdit(principal reqctx.Principal, ownerID string) bool {
	if principal.UserID == ownerID {
		return true
	}
	switch principal.Role {
	case rbac.RoleAdmin, rbac.RoleMentor:
		return true
	default:
		return false
	}
}

func (s *Service) logWarn(message string, err error) {
	if s.logger == nil {
		return
	}
	s.logger.Warn(message, "error", err.Error())
}

func normalizeStatus(status string) string {
	normalized := strings.ToLower(strings.TrimSpace(status))
	if _, ok := itemStatuses[normalized]; !ok {
		return ""
	}
	return normalized
}
