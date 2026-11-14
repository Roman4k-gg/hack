package flags

import "errors"

var (
	ErrKeyRequired  = errors.New("flags: key is required")
	ErrRolloutRange = errors.New("flags: rollout must be between 0 and 100")
	ErrFlagNotFound = errors.New("flags: flag not found")
)
