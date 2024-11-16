package redis

import (
	"context"
	"fmt"
	"time"
)

type Cache struct {
	client Client
}
type Client interface {
	Get(ctx context.Context, key string) (any, error)
	Set(ctx context.Context, key string, value any, ttl time.Duration) error
	Del(ctx context.Context, key string) error
}

func NewRedisCache(client Client) *Cache {
	return &Cache{
		client: client,
	}
}

func (c *Cache) Get(ctx context.Context, key string) (any, bool, error) {
	value, err := c.client.Get(ctx, key)
	if err != nil {
		return nil, false, err
	}
	if value == nil {
		return nil, false, nil
	}
	return value, true, nil
}

func (c *Cache) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	strValue, ok := value.(string)
	if !ok {
		return fmt.Errorf("redis cache only supports string values")
	}
	return c.client.Set(ctx, key, strValue, ttl)
}

func (c *Cache) Delete(ctx context.Context, key string) error {
	return c.client.Del(ctx, key)
}
