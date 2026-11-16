package redis

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewRequiresAddress(t *testing.T) {
	t.Parallel()

	_, err := New(context.Background(), "", "", 0)
	require.Error(t, err)
}

func TestNewBuildsClient(t *testing.T) {
	t.Parallel()

	client, err := New(context.Background(), "127.0.0.1:6379", "", 0)
	require.NoError(t, err)
	require.NotNil(t, client)
	require.NotNil(t, client.Client())

	require.NoError(t, client.Close())
}

func TestPingWithRunningRedis(t *testing.T) {
	addr := os.Getenv("REDIS_TEST_ADDR")
	if addr == "" {
		t.Skip("REDIS_TEST_ADDR is not set")
	}

	client, err := New(context.Background(), addr, "", 0)
	require.NoError(t, err)

	t.Cleanup(func() { _ = client.Close() })

	require.NoError(t, client.Ping(context.Background()))
}

func TestNilClientFails(t *testing.T) {
	t.Parallel()

	var client *Client

	require.Nil(t, client.Client())
	require.Error(t, client.Ping(context.Background()))
}
