package auth_test

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/DujinA/spendulum/backend/internal/auth"
)

func t0() time.Time { return time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC) }

func TestLimiterRefusesTheSixthAttemptWithin15Minutes(t *testing.T) {
	l := auth.NewLimiter()

	for i := range 5 {
		if wait := l.Attempt("k", t0().Add(time.Duration(i)*time.Minute)); wait != 0 {
			t.Fatalf("attempt %d refused for %s, want allowed", i+1, wait)
		}
	}

	// The oldest attempt, at t0, leaves the window at t0+15m.
	if wait := l.Attempt("k", t0().Add(5*time.Minute)); wait != 10*time.Minute {
		t.Errorf("6th attempt: wait = %s, want 10m", wait)
	}

	if wait := l.Attempt("k", t0().Add(15*time.Minute)); wait != 0 {
		t.Errorf("after the oldest attempt left the window: wait = %s, want 0", wait)
	}
}

func TestLimiterForgetsAttemptsOlderThan15Minutes(t *testing.T) {
	l := auth.NewLimiter()

	for i := range 5 {
		l.Attempt("k", t0().Add(time.Duration(i)*4*time.Minute))
	}

	// At t0+16m the attempt at t0 is outside the window: 4 remain.
	if wait := l.Attempt("k", t0().Add(16*time.Minute)); wait != 0 {
		t.Errorf("wait = %s, want 0", wait)
	}
}

func TestLimiterKeysAreIndependentAndReleaseUncountsOneAttempt(t *testing.T) {
	l := auth.NewLimiter()

	for i := range 5 {
		l.Attempt("a", t0().Add(time.Duration(i)*time.Second))
		l.Attempt("b", t0())
	}

	if wait := l.Attempt("c", t0()); wait != 0 {
		t.Errorf("an untouched key waits %s", wait)
	}

	l.Release("a", t0().Add(4*time.Second))

	if wait := l.Attempt("a", t0().Add(5*time.Second)); wait != 0 {
		t.Errorf("after releasing one of 5 attempts: wait %s, want 0", wait)
	}

	if wait := l.Attempt("a", t0().Add(6*time.Second)); wait == 0 {
		t.Error("releasing one attempt forgot the others")
	}

	if wait := l.Attempt("b", t0()); wait == 0 {
		t.Error("releasing one key's attempt allowed another key")
	}
}

func TestLimiterAllowsOnlyFiveOfManyConcurrentAttempts(t *testing.T) {
	l := auth.NewLimiter()

	var (
		allowed atomic.Int32
		wg      sync.WaitGroup
	)

	for range 50 {
		wg.Go(func() {
			if l.Attempt("k", t0()) == 0 {
				allowed.Add(1)
			}
		})
	}

	wg.Wait()

	if got := allowed.Load(); got != 5 {
		t.Errorf("%d of 50 concurrent attempts allowed, want 5", got)
	}
}
