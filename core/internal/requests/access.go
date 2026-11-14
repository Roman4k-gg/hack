package requests

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"ai-buddy/core/internal/platform/reqctx"

	"github.com/google/uuid"
)

const KindAccess = "access"

type AccessBundleStore interface {
	Bundle(ctx context.Context, bundle string) ([]AccessItem, error)
}

type AccessService interface {
	Preview(ctx context.Context, bundle string) ([]AccessItem, error)
	Create(ctx context.Context, items []AccessItem) ([]AccessCreated, error)
}

func (s *Service) PreviewAccess(ctx context.Context, principal reqctx.Principal, bundle string) (AccessPreview, error) {
	if s == nil || s.store == nil || s.tokens == nil {
		return AccessPreview{}, fmt.Errorf("requests: service is not initialised")
	}

	bundle = strings.TrimSpace(bundle)
	if bundle == "" {
		bundle = "intern"
	}

	if s.bundles != nil {
		items, err := s.bundles.Bundle(ctx, bundle)
		if err != nil {
			return AccessPreview{}, err
		}

		if items == nil {
			items = []AccessItem{}
		}

		if len(items) == 0 {
			return AccessPreview{}, ErrBundleNotFound
		}

		previewID := uuid.NewString()
		token := uuid.NewString()

		payload := PreviewPayload{
			PreviewID: previewID,
			UserID:    principal.UserID,
			Draft:     AbsenceDraft{AbsenceType: KindAccess, Comment: bundle},
		}

		if err := s.tokens.PutConfirmation(ctx, token, payload, confirmationTTL); err != nil {
			return AccessPreview{}, err
		}

		return AccessPreview{
			PreviewID:         previewID,
			ConfirmationToken: token,
			ExpiresAt:         s.now().Add(confirmationTTL),
			Bundle:            bundle,
			Items:             items,
		}, nil
	}

	return AccessPreview{}, ErrBundleNotFound
}

func (s *Service) CreateAccess(ctx context.Context, principal reqctx.Principal, previewID, token, idempotencyKey string) (AccessResult, error) {
	if s == nil || s.store == nil || s.tokens == nil {
		return AccessResult{}, fmt.Errorf("requests: service is not initialised")
	}

	if strings.TrimSpace(token) == "" {
		return AccessResult{}, ErrMissingToken
	}

	if strings.TrimSpace(idempotencyKey) == "" {
		return AccessResult{}, ErrMissingKey
	}

	if existing, err := s.store.ByIdempotencyKey(ctx, principal.UserID, KindAccess, idempotencyKey); err == nil && existing.ID != "" {
		records, err := s.store.ListByUser(ctx, principal.UserID)
		if err != nil {
			return AccessResult{}, err
		}

		sort.SliceStable(records, func(i, j int) bool {
			return records[i].Position < records[j].Position
		})

		created := []AccessCreated{}

		for _, record := range records {
			if record.Kind != KindAccess || record.IdempotencyKey != idempotencyKey {
				continue
			}

			created = append(created, AccessCreated{
				RequestID:  record.ID,
				System:     record.Payload.Comment,
				Status:     record.Status,
				ExternalID: record.ExternalID,
			})
		}

		return AccessResult{Requests: created, NextStep: accessNextStep}, nil
	}

	payload, ok, err := s.tokens.TakeConfirmation(ctx, token)
	if err != nil {
		return AccessResult{}, err
	}

	if !ok {
		return AccessResult{}, ErrInvalidToken
	}

	if payload.UserID != principal.UserID {
		return AccessResult{}, ErrForbidden
	}

	if previewID != "" && payload.PreviewID != previewID {
		return AccessResult{}, ErrInvalidToken
	}

	bundle := payload.Draft.Comment
	items := []AccessItem{}

	if s.bundles != nil {
		items, err = s.bundles.Bundle(ctx, bundle)
		if err != nil {
			return AccessResult{}, err
		}
	}

	if len(items) == 0 {
		return AccessResult{}, ErrBundleNotFound
	}

	created := []AccessCreated{}

	for position, item := range items {
		record, err := s.store.Create(ctx, Record{
			UserID:         principal.UserID,
			Kind:           KindAccess,
			Status:         StatusPending,
			Title:          "Доступ: " + item.System,
			Payload:        AbsenceDraft{AbsenceType: KindAccess, Comment: item.System, DateFrom: item.Reason},
			ExternalID:     "ACC-" + strings.ToUpper(uuid.NewString()[:4]),
			PreviewID:      payload.PreviewID,
			IdempotencyKey: idempotencyKey,
			Position:       position,
		})
		if err != nil {
			return AccessResult{}, err
		}

		if err := s.store.AppendEvent(ctx, record.ID, "", StatusPending, "Заявка на доступ создана"); err != nil {
			return AccessResult{}, err
		}

		s.record(ctx, principal, record, "access.request_created", "access_request_created")

		created = append(created, AccessCreated{
			RequestID:  record.ID,
			System:     item.System,
			Status:     record.Status,
			ExternalID: record.ExternalID,
		})
	}

	if err := s.tokens.PutIdempotency(ctx, principal.UserID, KindAccess, idempotencyKey, created[0].RequestID, idempotencyTTL); err != nil {
		return AccessResult{}, err
	}

	return AccessResult{Requests: created, NextStep: accessNextStep}, nil
}

const accessNextStep = "Заявки ушли наставнику и в IT-отдел; статус можно проверить в разделе «Заявки»"
