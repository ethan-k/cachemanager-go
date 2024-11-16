package inmemory

import (
	"container/list"
	"context"
	"sync"
	"time"
)

// Cache represents an in-memory cache
type Cache struct {
	mu              sync.RWMutex
	data            map[string]cacheEntry
	ageList         *list.List
	ageElements     map[string]*list.Element
	cleanupTicker   *time.Ticker
	stopCleanup     chan struct{}
	cleanupInterval time.Duration
	maxEntries      int
}

type ageEntry struct {
	key       string
	createdAt time.Time
}

type cacheEntry struct {
	value     any
	expiresAt time.Time
	createdAt time.Time
}

// Option defines the functional option type for configuring the cache
type Option func(*Cache)

// WithCleanupInterval sets the interval for cleanup of expired entries
func WithCleanupInterval(interval time.Duration) Option {
	return func(c *Cache) {
		if interval > 0 {
			c.cleanupInterval = interval
		}
	}
}

// WithMaxEntries sets the maximum number of entries in the cache
// Use -1 for unlimited entries
func WithMaxEntries(max int) Option {
	return func(c *Cache) {
		c.maxEntries = max
	}
}

func NewInMemoryCache(opts ...Option) *Cache {
	cache := &Cache{
		data:            make(map[string]cacheEntry),
		ageList:         list.New(),
		ageElements:     make(map[string]*list.Element),
		cleanupInterval: 5 * time.Minute,
		maxEntries:      -1,
		stopCleanup:     make(chan struct{}),
	}

	for _, opt := range opts {
		opt(cache)
	}

	cache.startCleanup()

	return cache
}

func (c *Cache) Get(_ context.Context, key string) (any, bool, error) {
	entry, exists := c.data[key]
	if !exists {
		return nil, false, nil
	}

	if time.Now().After(entry.expiresAt) {
		delete(c.data, key)
		return nil, false, nil
	}

	return entry.value, true, nil
}

func (c *Cache) Set(_ context.Context, key string, value any, ttl time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()

	if elem, exists := c.ageElements[key]; exists {
		c.ageList.Remove(elem)
		delete(c.ageElements, key)
	} else if c.maxEntries > 0 && len(c.data) >= c.maxEntries {
		if oldest := c.ageList.Front(); oldest != nil {
			oldestKey := oldest.Value.(ageEntry).key
			c.ageList.Remove(oldest)
			delete(c.ageElements, oldestKey)
			delete(c.data, oldestKey)
		}
	}

	c.data[key] = cacheEntry{
		value:     value,
		expiresAt: now.Add(ttl),
		createdAt: now,
	}

	elem := c.ageList.PushBack(ageEntry{
		key:       key,
		createdAt: now,
	})
	c.ageElements[key] = elem

	return nil
}

func (c *Cache) Stop() {
	close(c.stopCleanup)
}

func (c *Cache) Delete(_ context.Context, key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.data, key)

	if elem, exists := c.ageElements[key]; exists {
		c.ageList.Remove(elem)
		delete(c.ageElements, key)
	}

	return nil
}

func (c *Cache) startCleanup() {
	c.cleanupTicker = time.NewTicker(c.cleanupInterval)

	go func() {
		for {
			select {
			case <-c.cleanupTicker.C:
				c.cleanup()
			case <-c.stopCleanup:
				c.cleanupTicker.Stop()
				return
			}
		}
	}()
}

func (c *Cache) cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	var expiredKeys []string

	for key, entry := range c.data {
		if now.After(entry.expiresAt) {
			expiredKeys = append(expiredKeys, key)
		}
	}

	for _, key := range expiredKeys {
		delete(c.data, key)
	}

	if c.maxEntries > 0 && len(c.data) > c.maxEntries {
		type keyAge struct {
			key       string
			createdAt time.Time
		}
		entries := make([]keyAge, 0, len(c.data))

		for key, entry := range c.data {
			entries = append(entries, keyAge{key, entry.createdAt})
		}

		for i := 0; i < len(entries)-1; i++ {
			for j := 0; j < len(entries)-i-1; j++ {
				if entries[j+1].createdAt.Before(entries[j].createdAt) {
					entries[j], entries[j+1] = entries[j+1], entries[j]
				}
			}
		}

		numToRemove := len(c.data) - c.maxEntries
		for i := 0; i < numToRemove && i < len(entries); i++ {
			delete(c.data, entries[i].key)
		}
	}
}
