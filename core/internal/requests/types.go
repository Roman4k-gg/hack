package requests

import (
	"context"
	"time"
)

type AbsenceDraft struct {
	DateFrom    string `json:"date_from"`
	DateTo      string `json:"date_to"`
	AbsenceType string `json:"absence_type"`
	Comment     string `json:"comment"`
}

type Preview struct {
	PreviewID         string       `json:"preview_id"`
	ConfirmationToken string       `json:"confirmation_token"`
	ExpiresAt         time.Time    `json:"expires_at"`
	Draft             AbsenceDraft `json:"draft"`
	PolicyNotes       []string     `json:"policy_notes"`
}

type Person struct {
	ID       string `json:"id"`
	FullName string `json:"full_name"`
}

type Item struct {
	RequestID  string    `json:"request_id"`
	Kind       string    `json:"kind"`
	Title      string    `json:"title"`
	Status     string    `json:"status"`
	ExternalID string    `json:"external_id"`
	Approver   *Person   `json:"approver"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type List struct {
	Items []Item `json:"items"`
}

type Event struct {
	From      string    `json:"from"`
	To        string    `json:"to"`
	Comment   string    `json:"comment"`
	CreatedAt time.Time `json:"created_at"`
}

type Detail struct {
	RequestID  string       `json:"request_id"`
	Kind       string       `json:"kind"`
	Title      string       `json:"title"`
	Status     string       `json:"status"`
	ExternalID string       `json:"external_id"`
	Payload    AbsenceDraft `json:"payload"`
	Approver   *Person      `json:"approver"`
	NextStep   string       `json:"next_step"`
	CreatedAt  time.Time    `json:"created_at"`
	UpdatedAt  time.Time    `json:"updated_at"`
	History    []Event      `json:"history"`
}

type Record struct {
	ID             string
	UserID         string
	Kind           string
	Status         string
	Title          string
	Payload        AbsenceDraft
	ExternalID     string
	PreviewID      string
	IdempotencyKey string
	Approver       *Person
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type Store interface {
	Create(ctx context.Context, record Record) (Record, error)
	ByID(ctx context.Context, requestID string) (Record, error)
	ByIdempotencyKey(ctx context.Context, userID, kind, key string) (Record, error)
	ListByUser(ctx context.Context, userID string) ([]Record, error)
	Events(ctx context.Context, requestID string) ([]Event, error)
	AppendEvent(ctx context.Context, requestID, from, to, comment string) error
}

type TokenStore interface {
	PutConfirmation(ctx context.Context, token string, payload PreviewPayload, ttl time.Duration) error
	TakeConfirmation(ctx context.Context, token string) (PreviewPayload, bool, error)
	PutIdempotency(ctx context.Context, userID, kind, key, requestID string, ttl time.Duration) error
	FindIdempotency(ctx context.Context, userID, kind, key string) (string, bool, error)
}

type AbsenceInput struct {
	DateFrom    string
	DateTo      string
	AbsenceType string
	Comment     string
}

type AbsenceResult struct {
	ExternalID string
	Status     string
}

type AbsenceService interface {
	Policy(ctx context.Context, input AbsenceInput) ([]string, error)
	Create(ctx context.Context, input AbsenceInput) (AbsenceResult, error)
}
