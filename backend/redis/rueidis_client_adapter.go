package redis

import (
	"context"
	"errors"
	"time"

	"github.com/redis/rueidis"
)

type rueidisClient struct {
	client rueidis.Client
}

func NewRueidisAdapter(addr string, opts ...Option) (Client, error) {
	options := &redisOptions{
		Password: "",
		DB:       0,
	}

	for _, opt := range opts {
		opt(options)
	}

	client, err := rueidis.NewClient(rueidis.ClientOption{
		InitAddress: []string{addr},
		Password:    options.Password,
		SelectDB:    options.DB,
		ClientTrackingOptions: []string{
			"OPTIN",  // Only track keys that are explicitly requested
			"BCAST",  // Broadcast mode - all clients will receive invalidation messages
			"NOLOOP", // Don't receive invalidation messages for our own modifications
		},
	})
	if err != nil {
		return nil, err
	}

	return &rueidisClient{
		client: client,
	}, nil
}

func (c *rueidisClient) Get(ctx context.Context, key string) (any, error) {
	// Enable client tracking for this key
	cmd := c.client.B().ClientTracking().On().Optin().Build()
	err := c.client.Do(ctx, cmd).Error()
	if err != nil {
		return nil, err
	}

	// Get the value with client caching
	cmd = c.client.B().Get().Key(key).Build()
	resp := c.client.Do(ctx, cmd)
	if resp.Error() == rueidis.Nil {
		return nil, nil
	}
	if resp.Error() != nil {
		return nil, resp.Error()
	}
	return resp.ToString()
}

func (c *rueidisClient) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	strValue, ok := value.(string)
	if !ok {
		return errors.New("redis cache only supports string values")
	}

	var cmd rueidis.Completed
	if ttl > 0 {
		cmd = c.client.B().Set().Key(key).Value(strValue).Px(ttl).Build()
	} else {
		cmd = c.client.B().Set().Key(key).Value(strValue).Build()
	}
	return c.client.Do(ctx, cmd).Error()
}

func (c *rueidisClient) Del(ctx context.Context, key string) error {
	cmd := c.client.B().Del().Key(key).Build()
	return c.client.Do(ctx, cmd).Error()
}

// Close closes the client connection
func (c *rueidisClient) Close() error {
	c.client.Close()
	return nil
}

// StartInvalidationListener starts listening for cache invalidation messages
// It returns a channel that will receive invalidated keys
func (c *rueidisClient) StartInvalidationListener(ctx context.Context) (<-chan string, error) {
	invalidatedKeys := make(chan string, 100)

	// Start the invalidation message handler
	go func() {
		defer close(invalidatedKeys)

		c.client.Receive(ctx)
		for msg := range c.client.Receive(ctx) {
			// Handle invalidation message
			if msg.Error != nil {
				continue
			}
			if len(msg.Message) >= 2 {
				switch msg.Message[0] {
				case "invalidate":
					// Message format: ["invalidate", key]
					invalidatedKeys <- msg.Message[1]
				case "switched":
					// Reconnection happened, need to re-enable tracking
					cmd := c.client.B().ClientTracking().On().Optin().Build()
					_ = c.client.Do(ctx, cmd).Error()
				}
			}
		}
	}()

	return invalidatedKeys, nil
}
