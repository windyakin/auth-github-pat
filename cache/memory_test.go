package cache

import (
	"context"
	"testing"
	"time"
)

func TestMemoryCache_SetAndGet(t *testing.T) {
	mc := NewMemoryCache()
	defer mc.Stop()

	ctx := context.Background()

	entry := Entry{Allowed: true, Username: "testuser"}
	mc.Set(ctx, "key1", entry, 5*time.Minute)

	got, ok := mc.Get(ctx, "key1")
	if !ok {
		t.Fatal("expected cache hit")
	}
	if got.Username != "testuser" || !got.Allowed {
		t.Fatalf("unexpected entry: %+v", got)
	}
}

func TestMemoryCache_Miss(t *testing.T) {
	mc := NewMemoryCache()
	defer mc.Stop()

	_, ok := mc.Get(context.Background(), "nonexistent")
	if ok {
		t.Fatal("expected cache miss")
	}
}

func TestMemoryCache_Expiry(t *testing.T) {
	mc := NewMemoryCache()
	defer mc.Stop()

	ctx := context.Background()
	mc.Set(ctx, "key1", Entry{Allowed: true, Username: "testuser"}, 50*time.Millisecond)

	time.Sleep(100 * time.Millisecond)

	_, ok := mc.Get(ctx, "key1")
	if ok {
		t.Fatal("expected cache miss after TTL expiry")
	}
}

func TestMemoryCache_ZeroTTL(t *testing.T) {
	mc := NewMemoryCache()
	defer mc.Stop()

	ctx := context.Background()
	mc.Set(ctx, "key1", Entry{Allowed: false, Username: "blockeduser"}, 0)

	_, ok := mc.Get(ctx, "key1")
	if ok {
		t.Fatal("expected cache miss for zero TTL")
	}
}

func TestMemoryCache_DeniedEntry(t *testing.T) {
	mc := NewMemoryCache()
	defer mc.Stop()

	ctx := context.Background()
	mc.Set(ctx, "key1", Entry{Allowed: false, Username: "blockeduser"}, 5*time.Minute)

	got, ok := mc.Get(ctx, "key1")
	if !ok {
		t.Fatal("expected cache hit")
	}
	if got.Allowed {
		t.Fatal("expected Allowed to be false")
	}
}
