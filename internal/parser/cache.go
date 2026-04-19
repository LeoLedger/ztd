package parser

import (
	"context"
	"fmt"
	"time"

	"golang.org/x/sync/singleflight"
	"github.com/redis/go-redis/v9"
)

type Cache struct {
	client *redis.Client
	sfGroup singleflight.Group
}

func NewCache(redisURL string) (*Cache, error) {
	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse redis url: %w", err)
	}

	client := redis.NewClient(opt)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to redis: %w", err)
	}

	return &Cache{client: client}, nil
}

func (c *Cache) Get(ctx context.Context, key string) (string, error) {
	return c.client.Get(ctx, key).Result()
}

func (c *Cache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	return c.client.Set(ctx, key, value, ttl).Err()
}

func (c *Cache) GetOrSet(ctx context.Context, key string, ttl time.Duration, fn func() (string, error)) (string, error) {
	val, err := c.Get(ctx, key)
	if err == nil {
		return val, nil
	}
	if err != redis.Nil {
		return "", err
	}

	v, err, _ := c.sfGroup.Do(key, func() (interface{}, error) {
		result, err := fn()
		if err != nil {
			return nil, err
		}
		if cacheErr := c.Set(ctx, key, result, ttl); cacheErr != nil {
			return result, nil
		}
		return result, nil
	})

	if err != nil {
		return "", err
	}
	return v.(string), nil
}

func (c *Cache) Close() error {
	return c.client.Close()
}

func (c *Cache) Ping(ctx context.Context) error {
	return c.client.Ping(ctx).Err()
}
