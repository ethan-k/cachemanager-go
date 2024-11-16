package redis

import (
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

type Option func(*redisOptions)

type redisOptions struct {
	Password string
	DB       int
}

func WithPassword(password string) Option {
	return func(ro *redisOptions) {
		ro.Password = password
	}
}

func WithDB(db int) Option {
	return func(ro *redisOptions) {
		ro.DB = db
	}
}

type goRedisClient struct {
	client *redis.Client
}

func NewGoRedisAdapter(addr string, opts ...Option) Client {
	options := &redisOptions{
		Password: "",
		DB:       0,
	}

	for _, opt := range opts {
		opt(options)
	}

	rdb := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: options.Password,
		DB:       options.DB,
	})

	return &goRedisClient{
		client: rdb,
	}
}

func (g *goRedisClient) Get(ctx context.Context, key string) (any, error) {
	val, err := g.client.Get(ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return val, nil
}

func (g *goRedisClient) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	if ttl > 0 {
		return g.client.Set(ctx, key, value, ttl).Err()
	}
	return g.client.Set(ctx, key, value, 0).Err()
}

func (g *goRedisClient) Del(ctx context.Context, key string) error {
	return g.client.Del(ctx, key).Err()
}
