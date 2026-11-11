package auth_test

import (
	"testing"
	"time"

	"github.com/DujinA/spendulum/backend/internal/auth"
)

func t0() time.Time { return time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC) }

func TestLimiterBlocksAfterFiveFailuresWithin15Minutes(t *testing.T) {
	l := auth.NewLimiter()

	for i := range 5 {
		if wait := l.RetryAfter("k", t0().Add(time.Duration(i)*time.Minute)); wait != 0 {
			t.Fatalf("attempt %d blocked for %s, want allowed", i+1, wait)
		}

		l.Fail("k", t0().Add(time.Duration(i)*time.Minute))
	}

	// The oldest failure, at t0(), leaves the window at t0()+15m.
	if wait := l.RetryAfter("k", t0().Add(5*time.Minute)); wait != 10*time.Minute {
		t.Errorf("6th attempt: RetryAfter = %s, want 10m", wait)
	}

	if wait := l.RetryAfter("k", t0().Add(15*time.Minute)); wait != 0 {
		t.Errorf("after the oldest failure left the window: RetryAfter = %s, want 0", wait)
	}
}

func TestLimiterIgnoresFailuresOlderThan15Minutes(t *testing.T) {
	l := auth.NewLimiter()

	for i := range 5 {
		l.Fail("k", t0().Add(time.Duration(i)*4*time.Minute))
	}

	// At t0()+16m the failure at t0() is outside the window: 4 remain.
	if wait := l.RetryAfter("k", t0().Add(16*time.Minute)); wait != 0 {
		t.Errorf("RetryAfter = %s, want 0", wait)
	}
}

func TestLimiterKeysAreIndependentAndResetClearsOne(t *testing.T) {
	l := auth.NewLimiter()

	for range 5 {
		l.Fail("a", t0())
		l.Fail("b", t0())
	}

	if wait := l.RetryAfter("c", t0()); wait != 0 {
		t.Errorf("an untouched key is blocked for %s", wait)
	}

	l.Reset("a")

	if wait := l.RetryAfter("a", t0()); wait != 0 {
		t.Errorf("a reset key is blocked for %s", wait)
	}

	if wait := l.RetryAfter("b", t0()); wait == 0 {
		t.Error("resetting one key unblocked another")
	}
}
