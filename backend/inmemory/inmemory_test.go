package inmemory

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInMemoryCache(t *testing.T) {
	cache := NewInMemoryCache()
	ctx := context.Background()

	t.Run("set and get value", func(t *testing.T) {
		err := cache.Set(ctx, "test", "value", time.Minute)
		require.NoError(t, err)

		value, exists, err := cache.Get(ctx, "test")
		require.NoError(t, err)
		assert.True(t, exists)
		assert.Equal(t, "value", value)
	})

	t.Run("get non-existent value", func(t *testing.T) {
		value, exists, err := cache.Get(ctx, "nonexistent")
		require.NoError(t, err)
		assert.False(t, exists)
		assert.Nil(t, value)
	})

	t.Run("delete value", func(t *testing.T) {
		err := cache.Set(ctx, "test", "value", time.Minute)
		require.NoError(t, err)

		err = cache.Delete(ctx, "test")
		require.NoError(t, err)

		value, exists, err := cache.Get(ctx, "test")
		require.NoError(t, err)
		assert.False(t, exists)
		assert.Nil(t, value)
	})

	t.Run("expired value", func(t *testing.T) {
		err := cache.Set(ctx, "test", "value", time.Millisecond)
		require.NoError(t, err)

		time.Sleep(time.Millisecond * 2)

		value, exists, err := cache.Get(ctx, "test")
		require.NoError(t, err)
		assert.False(t, exists)
		assert.Nil(t, value)
	})
}
