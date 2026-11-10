package errors

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"ai-buddy/core/internal/platform/reqctx"

	"github.com/stretchr/testify/require"
)

func TestWriteUsesUnifiedEnvelope(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	ctx := reqctx.WithTraceID(context.Background(), "trace-1")

	err := New(CodeForbidden, "Недостаточно прав для этого действия").
		WithDetail("scope", "absence:self:write")

	Write(rec, ctx, http.StatusForbidden, err)

	require.Equal(t, http.StatusForbidden, rec.Code)
	require.Equal(t, "application/json; charset=utf-8", rec.Header().Get("Content-Type"))

	var env Envelope
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &env))

	require.Equal(t, CodeForbidden, env.Error.Code)
	require.Equal(t, "Недостаточно прав для этого действия", env.Error.Message)
	require.Equal(t, "trace-1", env.Error.TraceID)
	require.Equal(t, "absence:self:write", env.Error.Details["scope"])
}

func TestWriteFillsTraceIDFromContext(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	Write(rec, context.Background(), http.StatusInternalServerError, New(CodeInternal, "boom"))

	var env Envelope
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &env))
	require.Equal(t, CodeInternal, env.Error.Code)
}

func TestWriteHandlesNilError(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	Write(rec, context.Background(), http.StatusInternalServerError, nil)

	var env Envelope
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &env))
	require.Equal(t, CodeInternal, env.Error.Code)
}
