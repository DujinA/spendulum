package auth

import (
	"slices"
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

// Attempt counts first, so concurrent guesses cannot pass the limit; 0 means go ahead, and a success calls Release.
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

func (l *Limiter) Release(key string, at time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()

	times := l.attempts[key]
	for i, t := range slices.Backward(times) {
		if t.Equal(at) {
			l.attempts[key] = slices.Delete(slices.Clone(times), i, i+1)

			return
		}
	}
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
