package redis

import (
	"context"
	"fmt"

	goredis "github.com/redis/go-redis/v9"
)

type Client struct {
	client *goredis.Client
}

func New(_ context.Context, addr, password string, db int) (*Client, error) {
	if addr == "" {
		return nil, fmt.Errorf("redis: address is required")
	}
	client := goredis.NewClient(&goredis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})
	return &Client{client: client}, nil
}

func (c *Client) Client() *goredis.Client {
	if c == nil {
		return nil
	}
	return c.client
}

func (c *Client) Ping(ctx context.Context) error {
	if c == nil || c.client == nil {
		return fmt.Errorf("redis: client is not initialised")
	}
	if err := c.client.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("redis: ping: %w", err)
	}
	return nil
}

func (c *Client) Close() error {
	if c == nil || c.client == nil {
		return nil
	}
	return c.client.Close()
}
