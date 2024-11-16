package redis

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type RedisCacheTestSuite struct {
	suite.Suite
	mr     *miniredis.Miniredis
	client *redis.Client
	cache  *Cache
	ctx    context.Context
}

func (s *RedisCacheTestSuite) SetupTest() {
	var err error
	s.mr, err = miniredis.Run()
	require.NoError(s.T(), err)

	s.client = redis.NewClient(&redis.Options{
		Addr: s.mr.Addr(),
	})
	s.cache = NewRedisCache(NewGoRedisAdapter(s.mr.Addr()))
	s.ctx = context.Background()
}

func (s *RedisCacheTestSuite) TearDownTest() {
	s.client.Close()
	s.mr.Close()
}

func TestRedisCacheSuite(t *testing.T) {
	suite.Run(t, new(RedisCacheTestSuite))
}

func (s *RedisCacheTestSuite) TestSetAndGet() {
	tests := []struct {
		name        string
		key         string
		value       interface{}
		ttl         time.Duration
		shouldError bool
	}{
		{
			name:        "simple string value",
			key:         "test1",
			value:       "hello world",
			ttl:         time.Minute,
			shouldError: false,
		},
		{
			name:        "empty string value",
			key:         "test2",
			value:       "",
			ttl:         time.Minute,
			shouldError: false,
		},
		{
			name:        "non-string value",
			key:         "test3",
			value:       123,
			ttl:         time.Minute,
			shouldError: true,
		},
		{
			name:        "zero TTL",
			key:         "test4",
			value:       "value",
			ttl:         0,
			shouldError: false,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			err := s.cache.Set(s.ctx, tt.key, tt.value, tt.ttl)
			if tt.shouldError {
				s.Error(err)
				return
			}

			s.NoError(err)

			value, exists, err := s.cache.Get(s.ctx, tt.key)
			s.NoError(err)
			s.True(exists)
			s.Equal(tt.value, value)

			ttl := s.mr.TTL(tt.key)
			if tt.ttl == 0 {
				s.Equal(time.Duration(0), ttl)
			} else {
				s.True(ttl > 0)
			}
		})
	}
}

func (s *RedisCacheTestSuite) TestGetNonExistent() {
	value, exists, err := s.cache.Get(s.ctx, "nonexistent")
	s.NoError(err)
	s.False(exists)
	s.Nil(value)
}

func (s *RedisCacheTestSuite) TestDelete() {

	err := s.cache.Set(s.ctx, "test", "value", time.Minute)
	s.NoError(err)

	exists := s.mr.Exists("test")
	s.True(exists)

	err = s.cache.Delete(s.ctx, "test")
	s.NoError(err)

	exists = s.mr.Exists("test")
	s.False(exists)
}

func (s *RedisCacheTestSuite) TestExpiration() {

	err := s.cache.Set(s.ctx, "test", "value", time.Second)
	s.NoError(err)

	value, exists, err := s.cache.Get(s.ctx, "test")
	s.NoError(err)
	s.True(exists)
	s.Equal("value", value)

	s.mr.FastForward(3 * time.Second)

	value, exists, err = s.cache.Get(s.ctx, "test")
	s.NoError(err)
	s.False(exists)
	s.Nil(value)
}

func (s *RedisCacheTestSuite) TestConcurrentAccess() {
	const goroutines = 10
	done := make(chan bool)

	for i := 0; i < goroutines; i++ {
		go func(id int) {
			key := fmt.Sprintf("key%d", id)
			value := fmt.Sprintf("value%d", id)

			err := s.cache.Set(s.ctx, key, value, time.Minute)
			s.NoError(err)

			val, exists, err := s.cache.Get(s.ctx, key)
			s.NoError(err)
			s.True(exists)
			s.Equal(value, val)

			done <- true
		}(i)
	}

	for i := 0; i < goroutines; i++ {
		<-done
	}
}

func (s *RedisCacheTestSuite) TestSetNil() {
	err := s.cache.Set(s.ctx, "niltest", nil, time.Minute)
	s.Error(err, "setting nil value should return error")
}

func (s *RedisCacheTestSuite) TestLargeValues() {
	largeValue := strings.Repeat("a", 1<<20)

	err := s.cache.Set(s.ctx, "large", largeValue, time.Minute)
	s.NoError(err)

	value, exists, err := s.cache.Get(s.ctx, "large")
	s.NoError(err)
	s.True(exists)
	s.Equal(largeValue, value)
}

func (s *RedisCacheTestSuite) TestContextCancellation() {
	ctx, cancel := context.WithCancel(context.Background())

	cancel()

	err := s.cache.Set(ctx, "test", "value", time.Minute)
	s.Error(err)

	_, _, err = s.cache.Get(ctx, "test")
	s.Error(err)

	err = s.cache.Delete(ctx, "test")
	s.Error(err)
}
