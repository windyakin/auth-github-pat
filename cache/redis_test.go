package cache

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
)

func TestRedisCache_SetAndGet(t *testing.T) {
	mr := miniredis.RunT(t)

	rc, err := NewRedisCache("redis://" + mr.Addr())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx := context.Background()
	entry := Entry{Allowed: true, Username: "testuser"}
	rc.Set(ctx, "key1", entry, 5*time.Minute)

	got, ok := rc.Get(ctx, "key1")
	if !ok {
		t.Fatal("expected cache hit")
	}
	if got.Username != "testuser" || !got.Allowed {
		t.Fatalf("unexpected entry: %+v", got)
	}
}

func TestRedisCache_Miss(t *testing.T) {
	mr := miniredis.RunT(t)

	rc, err := NewRedisCache("redis://" + mr.Addr())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, ok := rc.Get(context.Background(), "nonexistent")
	if ok {
		t.Fatal("expected cache miss")
	}
}

func TestRedisCache_Expiry(t *testing.T) {
	mr := miniredis.RunT(t)

	rc, err := NewRedisCache("redis://" + mr.Addr())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx := context.Background()
	rc.Set(ctx, "key1", Entry{Allowed: true, Username: "testuser"}, 1*time.Second)

	// Fast-forward time in miniredis
	mr.FastForward(2 * time.Second)

	_, ok := rc.Get(ctx, "key1")
	if ok {
		t.Fatal("expected cache miss after TTL expiry")
	}
}

func TestRedisCache_ZeroTTL(t *testing.T) {
	mr := miniredis.RunT(t)

	rc, err := NewRedisCache("redis://" + mr.Addr())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx := context.Background()
	rc.Set(ctx, "key1", Entry{Allowed: false, Username: "blockeduser"}, 0)

	_, ok := rc.Get(ctx, "key1")
	if ok {
		t.Fatal("expected cache miss for zero TTL")
	}
}
