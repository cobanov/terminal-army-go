package svc

import (
	"sync"
	"time"
)

// PresenceTracker keeps an in-memory "user last seen" cache used to render
// online dots on galaxy/leaderboard views. The Python port also keeps this in
// memory; persistence is not worth the write cost for a 5-minute window.
type PresenceTracker struct {
	mu     sync.RWMutex
	seen   map[int64]time.Time
	window time.Duration
}

func NewPresenceTracker() *PresenceTracker {
	return &PresenceTracker{
		seen:   make(map[int64]time.Time),
		window: 5 * time.Minute,
	}
}

func (p *PresenceTracker) Touch(uid int64) {
	p.mu.Lock()
	p.seen[uid] = time.Now()
	p.mu.Unlock()
}

func (p *PresenceTracker) Online(uid int64) bool {
	p.mu.RLock()
	t, ok := p.seen[uid]
	p.mu.RUnlock()
	return ok && time.Since(t) <= p.window
}

func (p *PresenceTracker) Count() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	cutoff := time.Now().Add(-p.window)
	n := 0
	for _, t := range p.seen {
		if t.After(cutoff) {
			n++
		}
	}
	return n
}
