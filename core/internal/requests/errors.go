package requests

import "errors"

var (
	ErrEmptyMessage    = errors.New("requests: message is required")
	ErrInvalidDates    = errors.New("requests: invalid absence dates")
	ErrInvalidType     = errors.New("requests: invalid absence type")
	ErrCommentTooLong  = errors.New("requests: comment is too long")
	ErrMissingToken    = errors.New("requests: confirmation token is required")
	ErrInvalidToken    = errors.New("requests: confirmation token is invalid or expired")
	ErrPreviewNotFound = errors.New("requests: preview not found")
	ErrMissingKey      = errors.New("requests: idempotency key is required")
	ErrRequestNotFound = errors.New("requests: request not found")
	ErrForbidden       = errors.New("requests: request belongs to another user")
	ErrUpstream        = errors.New("requests: corporate service is unavailable")
	ErrBundleNotFound  = errors.New("requests: access bundle not found")
)
