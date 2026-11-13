package reqctx

import (
	"context"

	"github.com/google/uuid"
)

type traceKey struct{}
type userKey struct{}
type principalKey struct{}

type Principal struct {
	UserID string
	Email  string
	Role   string
	TeamID string
	Scopes []string
}

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

func WithPrincipal(ctx context.Context, principal Principal) context.Context {
	return context.WithValue(ctx, principalKey{}, principal)
}

func PrincipalFrom(ctx context.Context) (Principal, bool) {
	if ctx == nil {
		return Principal{}, false
	}
	principal, ok := ctx.Value(principalKey{}).(Principal)
	return principal, ok
}

func NewTraceID() string {
	return uuid.NewString()
}
