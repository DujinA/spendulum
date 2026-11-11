package auth

import (
	"sync"
	"time"
)

const (
	maxFailedLogins = 5
	failedLoginTTL  = 15 * time.Minute
)

// Limiter counts failed logins per key in memory: one replica runs, and a restart only forgets failures.
type Limiter struct {
	mu        sync.Mutex
	failures  map[string][]time.Time
	lastSweep time.Time
}

func NewLimiter() *Limiter {
	return &Limiter{failures: make(map[string][]time.Time)}
}

// RetryAfter is 0 when key may try again, else the wait until its oldest counted failure expires.
func (l *Limiter) RetryAfter(key string, now time.Time) time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()

	recent := l.prune(key, now)
	if len(recent) < maxFailedLogins {
		return 0
	}

	return recent[len(recent)-maxFailedLogins].Add(failedLoginTTL).Sub(now)
}

func (l *Limiter) Fail(key string, now time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.sweep(now)
	l.failures[key] = append(l.prune(key, now), now)
}

func (l *Limiter) Reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	delete(l.failures, key)
}

func (l *Limiter) prune(key string, now time.Time) []time.Time {
	times := l.failures[key]

	i := 0
	for i < len(times) && !times[i].Add(failedLoginTTL).After(now) {
		i++
	}

	if i == len(times) {
		delete(l.failures, key)

		return nil
	}

	l.failures[key] = times[i:]

	return times[i:]
}

// Drops keys whose failures all expired, so attempts against many emails cannot grow the map without bound.
func (l *Limiter) sweep(now time.Time) {
	if now.Sub(l.lastSweep) < failedLoginTTL {
		return
	}

	l.lastSweep = now

	for key := range l.failures {
		l.prune(key, now)
	}
}
