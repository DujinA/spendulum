package auth

import (
	"sync"
	"time"
)

const (
	maxLoginAttempts = 5
	loginWindow      = 15 * time.Minute
)

// Limiter counts login attempts per key in memory: one replica runs, and a restart only forgets them.
type Limiter struct {
	mu        sync.Mutex
	attempts  map[string][]time.Time
	lastSweep time.Time
}

func NewLimiter() *Limiter {
	return &Limiter{attempts: make(map[string][]time.Time)}
}

// Attempt counts before the check runs, so concurrent guesses cannot pass the limit. 0 means go ahead.
func (l *Limiter) Attempt(key string, now time.Time) time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.sweep(now)

	recent := l.prune(key, now)
	if len(recent) >= maxLoginAttempts {
		return recent[len(recent)-maxLoginAttempts].Add(loginWindow).Sub(now)
	}

	l.attempts[key] = append(recent, now)

	return 0
}

func (l *Limiter) Reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	delete(l.attempts, key)
}

func (l *Limiter) prune(key string, now time.Time) []time.Time {
	times := l.attempts[key]

	i := 0
	for i < len(times) && !times[i].Add(loginWindow).After(now) {
		i++
	}

	if i == len(times) {
		delete(l.attempts, key)

		return nil
	}

	l.attempts[key] = times[i:]

	return times[i:]
}

// Keys whose attempts all expired go away, so guesses at many emails cannot grow the map without bound.
func (l *Limiter) sweep(now time.Time) {
	if now.Sub(l.lastSweep) < loginWindow {
		return
	}

	l.lastSweep = now

	for key := range l.attempts {
		l.prune(key, now)
	}
}
