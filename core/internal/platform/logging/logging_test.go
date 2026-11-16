package logging

import (
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseLevel(t *testing.T) {
	t.Parallel()

	require.Equal(t, slog.LevelDebug, ParseLevel("debug"))
	require.Equal(t, slog.LevelWarn, ParseLevel("warn"))
	require.Equal(t, slog.LevelWarn, ParseLevel("WARNING"))
	require.Equal(t, slog.LevelError, ParseLevel("error"))
	require.Equal(t, slog.LevelInfo, ParseLevel("info"))
	require.Equal(t, slog.LevelInfo, ParseLevel("  "))
	require.Equal(t, slog.LevelInfo, ParseLevel("unknown"))
}

func TestNewBuildsLogger(t *testing.T) {
	t.Parallel()

	logger := New("error", "core")
	require.NotNil(t, logger)
	require.True(t, logger.Enabled(context.Background(), slog.LevelError))
	require.False(t, logger.Enabled(context.Background(), slog.LevelDebug))
}

func TestReplaceAttrRenamesTimeAndDropsSource(t *testing.T) {
	t.Parallel()

	attr := replaceAttr(nil, slog.Attr{Key: slog.TimeKey, Value: slog.StringValue("now")})
	require.Equal(t, "ts", attr.Key)

	attr = replaceAttr(nil, slog.Attr{Key: slog.SourceKey, Value: slog.StringValue("file.go")})
	require.Equal(t, slog.Attr{}, attr)

	attr = replaceAttr([]string{"group"}, slog.Attr{Key: slog.TimeKey, Value: slog.StringValue("now")})
	require.Equal(t, slog.TimeKey, attr.Key)
}
