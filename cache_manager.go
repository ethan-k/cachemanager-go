package cachemanager

import (
	"context"
	"fmt"
	"time"
)

type CacheBackend interface {
	Get(ctx context.Context, key string) (any, bool, error)
	Set(ctx context.Context, key string, value any, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
	Close() error
}

type CacheBackendWithInvalidationChannel interface {
	CacheBackend
	GetInvalidationChannel() <-chan string
}

// CacheConfig holds configuration for a single cache backend
type CacheConfig struct {
	Backend CacheBackend
	TTL     time.Duration
}

// CacheManager orchestrates multiple cache backends
type CacheManager struct {
	backends []CacheConfig
}

func NewCacheManager(configs ...CacheConfig) *CacheManager {
	cm := &CacheManager{
		backends: configs,
	}

	// Start listening for invalidation events from all backends
	for i, config := range configs {
		if cacheBackend, ok := config.Backend.(CacheBackendWithInvalidationChannel); ok {
			go cm.handleInvalidation(context.Background(), cacheBackend.GetInvalidationChannel(), i)
		}
	}

	return cm
}

// Get retrieves a value from the cache chain
func (cm *CacheManager) Get(ctx context.Context, key string) (any, error) {
	var lastErr error

	for i, config := range cm.backends {
		value, found, err := config.Backend.Get(ctx, key)
		if err != nil {
			lastErr = fmt.Errorf("error from backend %d: %w", i, err)
			continue
		}

		if found {
			go cm.populatePreviousBackends(ctx, key, value, i)
			return value, nil
		}
	}

	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("key %s not found in any backend", key)
}

// Set stores a value in all cache backends
func (cm *CacheManager) Set(ctx context.Context, key string, value any) error {
	var lastErr error

	for i, config := range cm.backends {
		err := config.Backend.Set(ctx, key, value, config.TTL)
		if err != nil {
			lastErr = fmt.Errorf("error setting in backend %d: %w", i, err)
		}
	}

	return lastErr
}

// Delete removes a value from all cache backends
func (cm *CacheManager) Delete(ctx context.Context, key string) error {
	var lastErr error

	for i, config := range cm.backends {
		if err := config.Backend.Delete(ctx, key); err != nil {
			lastErr = fmt.Errorf("error deleting from backend %d: %w", i, err)
		}
	}

	return lastErr
}

// populatePreviousBackends populates all backends before the hit index
func (cm *CacheManager) populatePreviousBackends(ctx context.Context, key string, value any, hitIndex int) {
	for i := 0; i < hitIndex; i++ {
		config := cm.backends[i]
		_ = config.Backend.Set(ctx, key, value, config.TTL)
	}
}

// handleInvalidation processes cache invalidation events from a backend
func (cm *CacheManager) handleInvalidation(ctx context.Context, invalidationChan <-chan string, sourceIndex int) {
	for key := range invalidationChan {
		// Delete from all other backends except the source
		for i, config := range cm.backends {
			if i != sourceIndex {
				// Use a new context for each delete operation
				deleteCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
				_ = config.Backend.Delete(deleteCtx, key)
				cancel()
			}
		}
	}
}

// Close closes all cache backends
func (cm *CacheManager) Close() error {
	var lastErr error
	for i, config := range cm.backends {
		if err := config.Backend.Close(); err != nil {
			lastErr = fmt.Errorf("error closing backend %d: %w", i, err)
		}
	}
	return lastErr
}
