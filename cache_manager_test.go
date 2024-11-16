package cachemanager

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockBackend struct {
	data map[string]interface{}
}

func newMockBackend() *mockBackend {
	return &mockBackend{
		data: make(map[string]interface{}),
	}
}

func (m *mockBackend) Get(ctx context.Context, key string) (interface{}, bool, error) {
	value, exists := m.data[key]
	return value, exists, nil
}

func (m *mockBackend) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	m.data[key] = value
	return nil
}

func (m *mockBackend) Delete(ctx context.Context, key string) error {
	delete(m.data, key)
	return nil
}

func TestCacheManager_Get(t *testing.T) {
	tests := []struct {
		name          string
		setupBackends func() []CacheConfig
		key           string
		value         interface{}
		expectedError bool
	}{
		{
			name: "value found in first backend",
			setupBackends: func() []CacheConfig {
				backend1 := newMockBackend()
				backend1.data["test"] = "value1"
				return []CacheConfig{
					{Backend: backend1, TTL: time.Minute},
					{Backend: newMockBackend(), TTL: time.Minute},
				}
			},
			key:           "test",
			value:         "value1",
			expectedError: false,
		},
		{
			name: "value found in second backend",
			setupBackends: func() []CacheConfig {
				backend1 := newMockBackend()
				backend2 := newMockBackend()
				backend2.data["test"] = "value2"
				return []CacheConfig{
					{Backend: backend1, TTL: time.Minute},
					{Backend: backend2, TTL: time.Minute},
				}
			},
			key:           "test",
			value:         "value2",
			expectedError: false,
		},
		{
			name: "value not found in any backend",
			setupBackends: func() []CacheConfig {
				return []CacheConfig{
					{Backend: newMockBackend(), TTL: time.Minute},
					{Backend: newMockBackend(), TTL: time.Minute},
				}
			},
			key:           "test",
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cm := NewCacheManager(tt.setupBackends()...)
			ctx := context.Background()

			value, err := cm.Get(ctx, tt.key)
			if tt.expectedError {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.value, value)
		})
	}
}

func TestCacheManager_Set(t *testing.T) {
	ctx := context.Background()
	backend1 := newMockBackend()
	backend2 := newMockBackend()

	cm := NewCacheManager(
		CacheConfig{Backend: backend1, TTL: time.Minute},
		CacheConfig{Backend: backend2, TTL: time.Minute},
	)

	err := cm.Set(ctx, "test", "value")
	require.NoError(t, err)

	value1, exists1, err := backend1.Get(ctx, "test")
	require.NoError(t, err)
	assert.True(t, exists1)
	assert.Equal(t, "value", value1)

	value2, exists2, err := backend2.Get(ctx, "test")
	require.NoError(t, err)
	assert.True(t, exists2)
	assert.Equal(t, "value", value2)
}

func TestCacheManager_Delete(t *testing.T) {
	ctx := context.Background()
	backend1 := newMockBackend()
	backend2 := newMockBackend()

	backend1.data["test"] = "value"
	backend2.data["test"] = "value"

	cm := NewCacheManager(
		CacheConfig{Backend: backend1, TTL: time.Minute},
		CacheConfig{Backend: backend2, TTL: time.Minute},
	)

	err := cm.Delete(ctx, "test")
	require.NoError(t, err)

	_, exists1, err := backend1.Get(ctx, "test")
	require.NoError(t, err)
	assert.False(t, exists1)

	_, exists2, err := backend2.Get(ctx, "test")
	require.NoError(t, err)
	assert.False(t, exists2)
}
