package requests

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"ai-buddy/core/internal/audit"
	"ai-buddy/core/internal/platform/reqctx"
	"ai-buddy/core/internal/rbac"

	"github.com/google/uuid"
)

const (
	KindAbsence = "absence"

	StatusPending   = "pending"
	StatusApproved  = "approved"
	StatusRejected  = "rejected"
	StatusCancelled = "cancelled"

	confirmationTTL = 10 * time.Minute
	idempotencyTTL  = 24 * time.Hour

	maxCommentLength = 500
)

var absenceTypes = map[string]bool{
	"vacation":   true,
	"day_off":    true,
	"sick_leave": true,
}

type PreviewPayload struct {
	PreviewID string       `json:"preview_id"`
	UserID    string       `json:"user_id"`
	Draft     AbsenceDraft `json:"draft"`
}

type Service struct {
	store    Store
	tokens   TokenStore
	absence  AbsenceService
	recorder *audit.Recorder
	logger   *slog.Logger
	now      func() time.Time
}

func NewService(store Store, tokens TokenStore, absence AbsenceService, recorder *audit.Recorder, logger *slog.Logger) *Service {
	return &Service{
		store:    store,
		tokens:   tokens,
		absence:  absence,
		recorder: recorder,
		logger:   logger,
		now:      time.Now,
	}
}

func (s *Service) PreviewAbsence(ctx context.Context, principal reqctx.Principal, input AbsenceInput) (Preview, error) {
	if s == nil || s.store == nil || s.tokens == nil {
		return Preview{}, fmt.Errorf("requests: service is not initialised")
	}

	draft, err := validateAbsence(input)
	if err != nil {
		return Preview{}, err
	}

	notes := []string{}
	if s.absence != nil {
		notes, err = s.absence.Policy(ctx, AbsenceInput(draft))
		if err != nil {
			return Preview{}, err
		}
	}

	if notes == nil {
		notes = []string{}
	}

	previewID := uuid.NewString()
	token := uuid.NewString()

	payload := PreviewPayload{PreviewID: previewID, UserID: principal.UserID, Draft: draft}

	if err := s.tokens.PutConfirmation(ctx, token, payload, confirmationTTL); err != nil {
		return Preview{}, err
	}

	return Preview{
		PreviewID:         previewID,
		ConfirmationToken: token,
		ExpiresAt:         s.now().Add(confirmationTTL),
		Draft:             draft,
		PolicyNotes:       notes,
	}, nil
}

func (s *Service) CreateAbsence(ctx context.Context, principal reqctx.Principal, previewID, token, idempotencyKey string) (Detail, error) {
	if s == nil || s.store == nil || s.tokens == nil {
		return Detail{}, fmt.Errorf("requests: service is not initialised")
	}

	if strings.TrimSpace(token) == "" {
		return Detail{}, ErrMissingToken
	}

	if strings.TrimSpace(idempotencyKey) == "" {
		return Detail{}, ErrMissingKey
	}

	if existing, err := s.store.ByIdempotencyKey(ctx, principal.UserID, KindAbsence, idempotencyKey); err == nil && existing.ID != "" {
		return s.detail(ctx, principal, existing)
	}

	payload, ok, err := s.tokens.TakeConfirmation(ctx, token)
	if err != nil {
		return Detail{}, err
	}

	if !ok {
		return Detail{}, ErrInvalidToken
	}

	if payload.UserID != principal.UserID {
		return Detail{}, ErrForbidden
	}

	if previewID != "" && payload.PreviewID != previewID {
		return Detail{}, ErrInvalidToken
	}

	result := AbsenceResult{ExternalID: "ABS-" + uuid.NewString()[:6], Status: StatusPending}

	if s.absence != nil {
		result, err = s.absence.Create(ctx, AbsenceInput(payload.Draft))
		if err != nil {
			return Detail{}, err
		}
	}

	record, err := s.store.Create(ctx, Record{
		UserID:         principal.UserID,
		Kind:           KindAbsence,
		Status:         StatusPending,
		Title:          absenceTitle(payload.Draft),
		Payload:        payload.Draft,
		ExternalID:     result.ExternalID,
		PreviewID:      payload.PreviewID,
		IdempotencyKey: idempotencyKey,
	})
	if err != nil {
		return Detail{}, err
	}

	if err := s.store.AppendEvent(ctx, record.ID, "", StatusPending, "Создана стажёром"); err != nil {
		return Detail{}, err
	}

	if err := s.tokens.PutIdempotency(ctx, principal.UserID, KindAbsence, idempotencyKey, record.ID, idempotencyTTL); err != nil {
		return Detail{}, err
	}

	s.record(ctx, principal, record, "absence.request_created", "absence_request_created")

	return s.detail(ctx, principal, record)
}

func (s *Service) List(ctx context.Context, principal reqctx.Principal) (List, error) {
	if s == nil || s.store == nil {
		return List{}, fmt.Errorf("requests: service is not initialised")
	}

	records, err := s.store.ListByUser(ctx, principal.UserID)
	if err != nil {
		return List{}, err
	}

	items := make([]Item, 0, len(records))
	for _, record := range records {
		items = append(items, Item{
			RequestID:  record.ID,
			ID:         record.ID,
			Kind:       record.Kind,
			Type:       record.Kind,
			Title:      record.Title,
			Status:     record.Status,
			ExternalID: record.ExternalID,
			Number:     record.ExternalID,
			Approver:   record.Approver,
			CreatedAt:  record.CreatedAt,
			UpdatedAt:  record.UpdatedAt,
		})
	}

	return List{Items: items}, nil
}

func (s *Service) Get(ctx context.Context, principal reqctx.Principal, requestID string) (Detail, error) {
	if s == nil || s.store == nil {
		return Detail{}, fmt.Errorf("requests: service is not initialised")
	}

	record, err := s.store.ByID(ctx, strings.TrimSpace(requestID))
	if err != nil {
		return Detail{}, err
	}

	return s.detail(ctx, principal, record)
}

func (s *Service) detail(ctx context.Context, principal reqctx.Principal, record Record) (Detail, error) {
	if record.UserID != principal.UserID && !canApprove(principal) {
		return Detail{}, ErrForbidden
	}

	history := []Event{}
	if s.store != nil {
		events, err := s.store.Events(ctx, record.ID)
		if err != nil {
			return Detail{}, err
		}
		history = events
	}

	return Detail{
		RequestID:  record.ID,
		ID:         record.ID,
		Kind:       record.Kind,
		Type:       record.Kind,
		Title:      record.Title,
		Status:     record.Status,
		ExternalID: record.ExternalID,
		Number:     record.ExternalID,
		Payload:    record.Payload,
		Approver:   record.Approver,
		NextStep:   nextStepOf(record.Status),
		CreatedAt:  record.CreatedAt,
		UpdatedAt:  record.UpdatedAt,
		History:    history,
	}, nil
}

func (s *Service) record(ctx context.Context, principal reqctx.Principal, record Record, action, event string) {
	if s.recorder == nil {
		return
	}

	payload := map[string]any{
		"kind":        record.Kind,
		"status":      record.Status,
		"external_id": record.ExternalID,
		"date_from":   record.Payload.DateFrom,
		"date_to":     record.Payload.DateTo,
	}

	if err := s.recorder.Record(ctx, audit.Entry{
		ActorID:    principal.UserID,
		Action:     action,
		EntityType: "request",
		EntityID:   record.ID,
		Payload:    payload,
		TraceID:    reqctx.TraceID(ctx),
	}); err != nil && s.logger != nil {
		s.logger.Warn("cannot write audit log", "error", err.Error())
	}

	if err := s.recorder.Emit(ctx, audit.Event{
		UserID:     principal.UserID,
		Name:       event,
		EntityType: "request",
		EntityID:   record.ID,
		Payload:    payload,
		TraceID:    reqctx.TraceID(ctx),
	}); err != nil && s.logger != nil {
		s.logger.Warn("cannot write event", "error", err.Error())
	}
}

func validateAbsence(input AbsenceInput) (AbsenceDraft, error) {
	dateFrom := strings.TrimSpace(input.DateFrom)
	dateTo := strings.TrimSpace(input.DateTo)

	if dateFrom == "" || dateTo == "" {
		return AbsenceDraft{}, ErrInvalidDates
	}

	from, err := time.Parse(time.DateOnly, dateFrom)
	if err != nil {
		return AbsenceDraft{}, ErrInvalidDates
	}

	to, err := time.Parse(time.DateOnly, dateTo)
	if err != nil {
		return AbsenceDraft{}, ErrInvalidDates
	}

	if to.Before(from) {
		return AbsenceDraft{}, ErrInvalidDates
	}

	absenceType := strings.TrimSpace(input.AbsenceType)
	if !absenceTypes[absenceType] {
		return AbsenceDraft{}, ErrInvalidType
	}

	comment := strings.TrimSpace(input.Comment)
	if len(comment) > maxCommentLength {
		return AbsenceDraft{}, ErrCommentTooLong
	}

	return AbsenceDraft{
		DateFrom:    dateFrom,
		DateTo:      dateTo,
		AbsenceType: absenceType,
		Comment:     comment,
	}, nil
}

func absenceTitle(draft AbsenceDraft) string {
	if draft.DateFrom == draft.DateTo {
		return "Отгул " + formatDay(draft.DateFrom)
	}

	return "Отгул с " + formatDay(draft.DateFrom)
}

func formatDay(value string) string {
	parsed, err := time.Parse(time.DateOnly, value)
	if err != nil {
		return value
	}

	return parsed.Format("02.01.2006")
}

func nextStepOf(status string) string {
	switch status {
	case StatusApproved:
		return "Отгул согласован руководителем"
	case StatusRejected:
		return "Заявку отклонили, уточните причину у руководителя"
	case StatusCancelled:
		return "Заявка отменена"
	default:
		return "Заявка уйдёт руководителю; статус можно проверить в разделе «Заявки»"
	}
}

func canApprove(principal reqctx.Principal) bool {
	return rbac.HasScope(principal.Scopes, rbac.ScopeAll) ||
		rbac.HasScope(principal.Scopes, rbac.ScopeAbsenceApproveTeam)
}

func marshalPreview(payload PreviewPayload) (string, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("requests: marshal preview: %w", err)
	}

	return string(raw), nil
}

func unmarshalPreview(raw string) (PreviewPayload, error) {
	var payload PreviewPayload

	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return PreviewPayload{}, fmt.Errorf("requests: unmarshal preview: %w", err)
	}

	return payload, nil
}
