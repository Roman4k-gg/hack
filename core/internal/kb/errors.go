package kb

import "errors"

var (
	ErrEmptyQuery        = errors.New("kb: query is required")
	ErrDocumentNotFound  = errors.New("kb: document not found")
	ErrDocumentForbidden = errors.New("kb: document is not available for the role")
)
