package reqctx

import (
	"context"

	"github.com/google/uuid"
)

type traceKey struct{}
type userKey struct{}

func WithTraceID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, traceKey{}, id)
}

func TraceID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	id, _ := ctx.Value(traceKey{}).(string)
	return id
}

func WithUserID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, userKey{}, id)
}

func UserID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	id, _ := ctx.Value(userKey{}).(string)
	return id
}

func NewTraceID() string {
	return uuid.NewString()
}
