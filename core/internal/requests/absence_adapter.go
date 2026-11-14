package requests

import (
	"context"

	"ai-buddy/core/internal/integrations"
)

type AbsenceAdapter struct {
	service integrations.AbsenceService
}

func NewAbsenceAdapter(service integrations.AbsenceService) *AbsenceAdapter {
	return &AbsenceAdapter{service: service}
}

func (a *AbsenceAdapter) Policy(ctx context.Context, input AbsenceInput) ([]string, error) {
	if a == nil || a.service == nil {
		return nil, nil
	}

	return a.service.Policy(ctx, integrations.AbsenceInput(input))
}

func (a *AbsenceAdapter) Create(ctx context.Context, input AbsenceInput) (AbsenceResult, error) {
	if a == nil || a.service == nil {
		return AbsenceResult{ExternalID: "ABS-0000", Status: StatusPending}, nil
	}

	result, err := a.service.Create(ctx, integrations.AbsenceInput(input))
	if err != nil {
		return AbsenceResult{}, err
	}

	return AbsenceResult{ExternalID: result.ExternalID, Status: result.Status}, nil
}
