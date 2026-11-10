package errors

import (
	"context"
	"encoding/json"
	"net/http"

	"ai-buddy/core/internal/platform/reqctx"
)

type Code string

const (
	CodeInternal       Code = "INTERNAL_ERROR"
	CodeValidation     Code = "VALIDATION_ERROR"
	CodeUnauthorized   Code = "UNAUTHORIZED"
	CodeForbidden      Code = "FORBIDDEN"
	CodeNotFound       Code = "NOT_FOUND"
	CodeConflict       Code = "CONFLICT"
	CodeUpstreamError  Code = "UPSTREAM_ERROR"
	CodeTimeout        Code = "TIMEOUT"
	CodeUnavailable    Code = "SERVICE_UNAVAILABLE"
	CodeInvalidRequest Code = "INVALID_REQUEST"
)

type APIError struct {
	Code    Code           `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`
	TraceID string         `json:"trace_id"`
}

type Envelope struct {
	Error APIError `json:"error"`
}

func New(code Code, message string) *APIError {
	return &APIError{Code: code, Message: message}
}

func (e *APIError) WithDetails(details map[string]any) *APIError {
	e.Details = details
	return e
}

func (e *APIError) WithDetail(key string, value any) *APIError {
	if e.Details == nil {
		e.Details = map[string]any{}
	}
	e.Details[key] = value
	return e
}

func Write(w http.ResponseWriter, ctx context.Context, status int, e *APIError) {
	if e == nil {
		e = New(CodeInternal, "Внутренняя ошибка сервера")
	}
	if e.TraceID == "" && ctx != nil {
		e.TraceID = reqctx.TraceID(ctx)
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(Envelope{Error: *e})
}
