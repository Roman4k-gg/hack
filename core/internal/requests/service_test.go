package requests

import (
	"context"
	"errors"
	"testing"
	"time"

	"ai-buddy/core/internal/platform/reqctx"
	"ai-buddy/core/internal/rbac"

	"github.com/stretchr/testify/require"
)

type fakeStore struct {
	created  []Record
	records  map[string]Record
	byKey    map[string]Record
	events   map[string][]Event
	createdN int
}

func newFakeStore() *fakeStore {
	return &fakeStore{
		records: map[string]Record{},
		byKey:   map[string]Record{},
		events:  map[string][]Event{},
	}
}

func (s *fakeStore) Create(_ context.Context, record Record) (Record, error) {
	s.createdN++
	record.ID = "request-" + string(rune('0'+s.createdN))
	record.CreatedAt = time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC)
	record.UpdatedAt = record.CreatedAt
	s.created = append(s.created, record)
	s.records[record.ID] = record

	if record.IdempotencyKey != "" {
		s.byKey[record.IdempotencyKey] = record
	}

	return record, nil
}

func (s *fakeStore) ByID(_ context.Context, requestID string) (Record, error) {
	record, ok := s.records[requestID]
	if !ok {
		return Record{}, ErrRequestNotFound
	}

	return record, nil
}

func (s *fakeStore) ByIdempotencyKey(_ context.Context, _, _, key string) (Record, error) {
	record, ok := s.byKey[key]
	if !ok {
		return Record{}, nil
	}

	return record, nil
}

func (s *fakeStore) ListByUser(_ context.Context, userID string) ([]Record, error) {
	var records []Record

	for _, record := range s.records {
		if record.UserID == userID {
			records = append(records, record)
		}
	}

	return records, nil
}

func (s *fakeStore) Events(_ context.Context, requestID string) ([]Event, error) {
	return s.events[requestID], nil
}

func (s *fakeStore) AppendEvent(_ context.Context, requestID, from, to, comment string) error {
	s.events[requestID] = append(s.events[requestID], Event{From: from, To: to, Comment: comment})

	return nil
}

type fakeTokens struct {
	confirmations map[string]PreviewPayload
	idempotency   map[string]string
}

func newFakeTokens() *fakeTokens {
	return &fakeTokens{
		confirmations: map[string]PreviewPayload{},
		idempotency:   map[string]string{},
	}
}

func (t *fakeTokens) PutConfirmation(_ context.Context, token string, payload PreviewPayload, _ time.Duration) error {
	t.confirmations[token] = payload

	return nil
}

func (t *fakeTokens) TakeConfirmation(_ context.Context, token string) (PreviewPayload, bool, error) {
	payload, ok := t.confirmations[token]
	if !ok {
		return PreviewPayload{}, false, nil
	}

	delete(t.confirmations, token)

	return payload, true, nil
}

func (t *fakeTokens) PutIdempotency(_ context.Context, _, _, key, requestID string, _ time.Duration) error {
	t.idempotency[key] = requestID

	return nil
}

func (t *fakeTokens) FindIdempotency(_ context.Context, _, _, key string) (string, bool, error) {
	requestID, ok := t.idempotency[key]
	if !ok {
		return "", false, nil
	}

	return requestID, true, nil
}

type fakeAbsence struct {
	notes   []string
	result  AbsenceResult
	failing bool
}

func (a *fakeAbsence) Policy(_ context.Context, _ AbsenceInput) ([]string, error) {
	if a.failing {
		return nil, errors.New("absence service is down")
	}

	return a.notes, nil
}

func (a *fakeAbsence) Create(_ context.Context, _ AbsenceInput) (AbsenceResult, error) {
	if a.failing {
		return AbsenceResult{}, errors.New("absence service is down")
	}

	return a.result, nil
}

func intern() reqctx.Principal {
	return reqctx.Principal{
		UserID: "user-1",
		Role:   rbac.RoleIntern,
		TeamID: "team-1",
		Scopes: []string{rbac.ScopeAbsenceSelf, rbac.ScopeAbsenceSelfWrite, rbac.ScopeRequestSelf},
	}
}

func newService(t *testing.T) (*Service, *fakeStore, *fakeTokens, *fakeAbsence) {
	t.Helper()

	store := newFakeStore()
	tokens := newFakeTokens()
	absence := &fakeAbsence{
		notes:  []string{"Отгул согласует руководитель"},
		result: AbsenceResult{ExternalID: "ABS-1024", Status: StatusPending},
	}

	service := NewService(store, tokens, absence, nil, nil)
	service.now = func() time.Time { return time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC) }

	return service, store, tokens, absence
}

func TestPreviewAbsenceReturnsTokenAndNotes(t *testing.T) {
	t.Parallel()

	service, _, tokens, _ := newService(t)

	preview, err := service.PreviewAbsence(context.Background(), intern(), AbsenceInput{
		DateFrom:    "2026-11-20",
		DateTo:      "2026-11-20",
		AbsenceType: "day_off",
		Comment:     "семейные обстоятельства",
	})
	require.NoError(t, err)
	require.NotEmpty(t, preview.PreviewID)
	require.NotEmpty(t, preview.ConfirmationToken)
	require.Equal(t, "Отгул согласует руководитель", preview.PolicyNotes[0])
	require.Len(t, tokens.confirmations, 1)
	require.Equal(t, "2026-11-14T12:10:00Z", preview.ExpiresAt.Format(time.RFC3339))
}

func TestPreviewAbsenceValidatesInput(t *testing.T) {
	t.Parallel()

	service, _, _, _ := newService(t)

	cases := []struct {
		name  string
		input AbsenceInput
		err   error
	}{
		{"без дат", AbsenceInput{AbsenceType: "day_off"}, ErrInvalidDates},
		{"дата назад", AbsenceInput{DateFrom: "2026-11-20", DateTo: "2026-11-19", AbsenceType: "day_off"}, ErrInvalidDates},
		{"кривая дата", AbsenceInput{DateFrom: "20.11.2026", DateTo: "2026-11-20", AbsenceType: "day_off"}, ErrInvalidDates},
		{"неизвестный тип", AbsenceInput{DateFrom: "2026-11-20", DateTo: "2026-11-20", AbsenceType: "sick"}, ErrInvalidType},
		{"длинный комментарий", AbsenceInput{DateFrom: "2026-11-20", DateTo: "2026-11-20", AbsenceType: "day_off", Comment: longText(600)}, ErrCommentTooLong},
	}

	for _, testCase := range cases {
		_, err := service.PreviewAbsence(context.Background(), intern(), testCase.input)
		require.ErrorIs(t, err, testCase.err, testCase.name)
	}
}

func TestCreateAbsenceCreatesRequest(t *testing.T) {
	t.Parallel()

	service, store, _, _ := newService(t)

	preview, err := service.PreviewAbsence(context.Background(), intern(), AbsenceInput{
		DateFrom: "2026-11-20", DateTo: "2026-11-20", AbsenceType: "day_off",
	})
	require.NoError(t, err)

	detail, err := service.CreateAbsence(context.Background(), intern(),
		preview.PreviewID, preview.ConfirmationToken, "key-1")
	require.NoError(t, err)
	require.Equal(t, StatusPending, detail.Status)
	require.Equal(t, "ABS-1024", detail.ExternalID)
	require.Equal(t, "Отгул 20.11.2026", detail.Title)
	require.NotEmpty(t, detail.RequestID)
	require.Len(t, store.created, 1)
	require.Len(t, store.events[detail.RequestID], 1)
	require.Equal(t, StatusPending, store.events[detail.RequestID][0].To)
}

func TestCreateAbsenceIsIdempotent(t *testing.T) {
	t.Parallel()

	service, store, _, _ := newService(t)

	preview, err := service.PreviewAbsence(context.Background(), intern(), AbsenceInput{
		DateFrom: "2026-11-20", DateTo: "2026-11-20", AbsenceType: "day_off",
	})
	require.NoError(t, err)

	first, err := service.CreateAbsence(context.Background(), intern(),
		preview.PreviewID, preview.ConfirmationToken, "key-1")
	require.NoError(t, err)

	second, err := service.CreateAbsence(context.Background(), intern(),
		preview.PreviewID, preview.ConfirmationToken, "key-1")
	require.NoError(t, err)

	require.Equal(t, first.RequestID, second.RequestID)
	require.Len(t, store.created, 1)
}

func TestCreateAbsenceRejectsMissingToken(t *testing.T) {
	t.Parallel()

	service, _, _, _ := newService(t)

	_, err := service.CreateAbsence(context.Background(), intern(), "preview-1", "", "key-1")
	require.ErrorIs(t, err, ErrMissingToken)
}

func TestCreateAbsenceRejectsSpentToken(t *testing.T) {
	t.Parallel()

	service, _, _, _ := newService(t)

	preview, err := service.PreviewAbsence(context.Background(), intern(), AbsenceInput{
		DateFrom: "2026-11-20", DateTo: "2026-11-20", AbsenceType: "day_off",
	})
	require.NoError(t, err)

	_, err = service.CreateAbsence(context.Background(), intern(),
		preview.PreviewID, preview.ConfirmationToken, "key-1")
	require.NoError(t, err)

	_, err = service.CreateAbsence(context.Background(), intern(),
		preview.PreviewID, preview.ConfirmationToken, "key-2")
	require.ErrorIs(t, err, ErrInvalidToken)
}

func TestCreateAbsenceRejectsForeignToken(t *testing.T) {
	t.Parallel()

	service, _, _, _ := newService(t)

	preview, err := service.PreviewAbsence(context.Background(), intern(), AbsenceInput{
		DateFrom: "2026-11-20", DateTo: "2026-11-20", AbsenceType: "day_off",
	})
	require.NoError(t, err)

	other := intern()
	other.UserID = "user-2"

	_, err = service.CreateAbsence(context.Background(), other,
		preview.PreviewID, preview.ConfirmationToken, "key-1")
	require.ErrorIs(t, err, ErrForbidden)
}

func TestCreateAbsenceRequiresIdempotencyKey(t *testing.T) {
	t.Parallel()

	service, _, _, _ := newService(t)

	preview, err := service.PreviewAbsence(context.Background(), intern(), AbsenceInput{
		DateFrom: "2026-11-20", DateTo: "2026-11-20", AbsenceType: "day_off",
	})
	require.NoError(t, err)

	_, err = service.CreateAbsence(context.Background(), intern(),
		preview.PreviewID, preview.ConfirmationToken, "")
	require.ErrorIs(t, err, ErrMissingKey)
}

func TestListReturnsOwnRequests(t *testing.T) {
	t.Parallel()

	service, store, _, _ := newService(t)

	preview, err := service.PreviewAbsence(context.Background(), intern(), AbsenceInput{
		DateFrom: "2026-11-20", DateTo: "2026-11-20", AbsenceType: "day_off",
	})
	require.NoError(t, err)

	_, err = service.CreateAbsence(context.Background(), intern(),
		preview.PreviewID, preview.ConfirmationToken, "key-1")
	require.NoError(t, err)

	list, err := service.List(context.Background(), intern())
	require.NoError(t, err)
	require.Len(t, list.Items, 1)
	require.Equal(t, "Отгул 20.11.2026", list.Items[0].Title)

	other := intern()
	other.UserID = "user-2"

	empty, err := service.List(context.Background(), other)
	require.NoError(t, err)
	require.Empty(t, empty.Items)

	_ = store
}

func TestGetRequestOfAnotherUserIsForbidden(t *testing.T) {
	t.Parallel()

	service, _, _, _ := newService(t)

	preview, err := service.PreviewAbsence(context.Background(), intern(), AbsenceInput{
		DateFrom: "2026-11-20", DateTo: "2026-11-20", AbsenceType: "day_off",
	})
	require.NoError(t, err)

	detail, err := service.CreateAbsence(context.Background(), intern(),
		preview.PreviewID, preview.ConfirmationToken, "key-1")
	require.NoError(t, err)

	other := intern()
	other.UserID = "user-2"

	_, err = service.Get(context.Background(), other, detail.RequestID)
	require.ErrorIs(t, err, ErrForbidden)

	manager := intern()
	manager.UserID = "user-3"
	manager.Scopes = append(manager.Scopes, rbac.ScopeAbsenceApproveTeam)

	visible, err := service.Get(context.Background(), manager, detail.RequestID)
	require.NoError(t, err)
	require.Equal(t, detail.RequestID, visible.RequestID)
}

func TestServiceWithoutStoreFails(t *testing.T) {
	t.Parallel()

	service := NewService(nil, nil, nil, nil, nil)

	_, err := service.PreviewAbsence(context.Background(), intern(), AbsenceInput{
		DateFrom: "2026-11-20", DateTo: "2026-11-20", AbsenceType: "day_off",
	})
	require.Error(t, err)
}

func longText(size int) string {
	value := make([]byte, size)
	for index := range value {
		value[index] = 'a'
	}

	return string(value)
}
