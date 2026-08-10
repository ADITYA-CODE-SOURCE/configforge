package middleware

import (
	"context"
	"sync"
	"time"
)

// RateLimitResult reports the outcome of a rate-limit check.
type RateLimitResult struct {
	Allowed    bool
	Limit      int
	Remaining  int
	RetryAfter time.Duration
}

// RateLimitStorage stores per-key counters. A Redis implementation could
// satisfy this interface later without changing the middleware.
type RateLimitStorage interface {
	// Hit increments the counter for key within the window aligned to now.
	// It returns the updated count, whether the request is allowed (count <=
	// limit), and the time until the current window resets.
	Hit(ctx context.Context, key string, limit int, window time.Duration, now time.Time) (RateLimitResult, error)
}

// MemoryStorage is a concurrency-safe in-memory fixed-window rate-limit store.
// A background cleanup goroutine removes expired entries periodically.
type MemoryStorage struct {
	mu      sync.Mutex
	entries map[string]memoryEntry
	stop    chan struct{}
}

type memoryEntry struct {
	count       int
	windowStart time.Time
}

// NewMemoryStorage constructs an in-memory store and starts a cleanup goroutine
// that runs at the supplied interval. Call Close to stop the goroutine.
func NewMemoryStorage(cleanupInterval time.Duration) *MemoryStorage {
	if cleanupInterval <= 0 {
		cleanupInterval = time.Minute
	}
	m := &MemoryStorage{
		entries: make(map[string]memoryEntry),
		stop:    make(chan struct{}),
	}
	go m.cleanupLoop(cleanupInterval)
	return m
}

// Close stops the cleanup goroutine. It is safe to call multiple times.
func (m *MemoryStorage) Close() {
	select {
	case <-m.stop:
	default:
		close(m.stop)
	}
}

// Hit implements RateLimitStorage using a fixed-window algorithm keyed by the
// caller-provided key (route:user or route:ip).
func (m *MemoryStorage) Hit(ctx context.Context, key string, limit int, window time.Duration, now time.Time) (RateLimitResult, error) {
	// Respect context cancellation for blocking integration tests.
	select {
	case <-ctx.Done():
		return RateLimitResult{Allowed: false}, ctx.Err()
	default:
	}

	if window <= 0 {
		window = time.Minute
	}
	bucketStart := now.Truncate(window)

	m.mu.Lock()
	defer m.mu.Unlock()

	entry, ok := m.entries[key]
	if !ok || !entry.windowStart.Equal(bucketStart) {
		entry = memoryEntry{count: 0, windowStart: bucketStart}
	}
	entry.count++

	allowed := entry.count <= limit
	m.entries[key] = entry

	remaining := limit - entry.count
	if remaining < 0 {
		remaining = 0
	}
	retryAfter := window - now.Sub(bucketStart)
	if retryAfter < 0 {
		retryAfter = 0
	}

	return RateLimitResult{
		Allowed:    allowed,
		Limit:      limit,
		Remaining:  remaining,
		RetryAfter: retryAfter,
	}, nil
}

func (m *MemoryStorage) cleanupLoop(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-m.stop:
			return
		case now := <-ticker.C:
			m.cleanup(now)
		}
	}
}

func (m *MemoryStorage) cleanup(now time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for key, entry := range m.entries {
		if now.Sub(entry.windowStart) > 2*time.Hour {
			delete(m.entries, key)
		}
	}
}
