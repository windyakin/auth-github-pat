package cache

import (
	"context"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisCache struct {
	client *redis.Client
}

func NewRedisCache(url string) (*RedisCache, error) {
	opts, err := redis.ParseURL(url)
	if err != nil {
		return nil, err
	}

	client := redis.NewClient(opts)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, err
	}

	return &RedisCache{
		client: client,
	}, nil
}

func (rc *RedisCache) Get(ctx context.Context, key string) (Entry, bool) {
	val, err := rc.client.Get(ctx, key).Result()
	if err != nil {
		return Entry{}, false
	}

	var entry Entry
	if err := json.Unmarshal([]byte(val), &entry); err != nil {
		return Entry{}, false
	}

	return entry, true
}

func (rc *RedisCache) Set(ctx context.Context, key string, entry Entry, ttl time.Duration) {
	if ttl <= 0 {
		return
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return
	}

	rc.client.Set(ctx, key, string(data), ttl)
}
