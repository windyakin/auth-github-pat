package cache

import (
	"context"
	"time"
)

type Entry struct {
	Allowed  bool
	Username string
}

type Cache interface {
	Get(ctx context.Context, key string) (Entry, bool)
	Set(ctx context.Context, key string, entry Entry, ttl time.Duration)
}
