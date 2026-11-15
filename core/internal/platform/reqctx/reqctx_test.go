package reqctx

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPrincipalRoundTrip(t *testing.T) {
	t.Parallel()

	principal := Principal{
		UserID: "user-1",
		Email:  "dasha@demo.local",
		Role:   "intern",
		TeamID: "team-1",
		Scopes: []string{"onboarding:plan:read"},
	}

	ctx := WithPrincipal(context.Background(), principal)

	loaded, ok := PrincipalFrom(ctx)
	require.True(t, ok)
	require.Equal(t, principal, loaded)
}

func TestPrincipalFromWithoutValue(t *testing.T) {
	t.Parallel()

	_, ok := PrincipalFrom(context.Background())
	require.False(t, ok)

	_, ok = PrincipalFrom(nil)
	require.False(t, ok)
}

func TestTraceIDRoundTrip(t *testing.T) {
	t.Parallel()

	ctx := WithTraceID(context.Background(), "trace-1")

	require.Equal(t, "trace-1", TraceID(ctx))
	require.Equal(t, "", TraceID(context.Background()))
	require.Equal(t, "", TraceID(nil))
}

func TestUserIDRoundTrip(t *testing.T) {
	t.Parallel()

	ctx := WithUserID(context.Background(), "user-2")

	require.Equal(t, "user-2", UserID(ctx))
	require.Equal(t, "", UserID(context.Background()))
	require.Equal(t, "", UserID(nil))
}

func TestNewTraceIDIsUnique(t *testing.T) {
	t.Parallel()

	first := NewTraceID()
	second := NewTraceID()

	require.NotEmpty(t, first)
	require.NotEqual(t, first, second)
}
