package middleware

import (
	"sync"
	"time"

	apperrors "github.com/cng1985/dbgo/internal/platform/errors"
)

type bucket struct {
	window time.Time
	count  int
	seen   time.Time
}

// Limiter 是单进程固定窗口限流。多实例不共享计数，键数量有上限。
type Limiter struct {
	mu     sync.Mutex
	items  map[string]*bucket
	limit  int
	window time.Duration
}

func NewLimiter(limit int) *Limiter {
	if limit < 1 {
		limit = 1
	}
	return &Limiter{
		items:  map[string]*bucket{},
		limit:  limit,
		window: time.Minute,
	}
}

func (l *Limiter) Allow(key string) error {
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.items) > 10000 {
		l.evict(now)
	}
	item := l.items[key]
	if item == nil || now.Sub(item.window) >= l.window {
		l.items[key] = &bucket{window: now, count: 1, seen: now}
		return nil
	}
	item.seen = now
	item.count++
	if item.count > l.limit {
		return apperrors.RateLimited("")
	}
	return nil
}

func (l *Limiter) evict(now time.Time) {
	for key, item := range l.items {
		if now.Sub(item.seen) >= l.window {
			delete(l.items, key)
		}
	}
	if len(l.items) <= 10000 {
		return
	}
	for key := range l.items {
		delete(l.items, key)
		if len(l.items) <= 8000 {
			return
		}
	}
}
