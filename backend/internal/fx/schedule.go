package fx

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"
)

const (
	syncHour   = 17
	firstRetry = time.Minute
	maxRetry   = time.Hour
)

// Scheduler backfills and syncs in-process: at startup, then daily at 17:00 Europe/Berlin.
type Scheduler struct {
	svc    *Service
	logger *slog.Logger
	berlin *time.Location
}

func NewScheduler(svc *Service, logger *slog.Logger) (*Scheduler, error) {
	berlin, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		return nil, fmt.Errorf("fx: load time zone: %w", err)
	}

	return &Scheduler{svc: svc, logger: logger, berlin: berlin}, nil
}

func (s *Scheduler) Run(ctx context.Context) {
	next := s.nextRun(time.Now())

	s.untilDone(ctx, "startup", next, s.backfillAndSync)

	for sleepUntil(ctx, next) {
		after := s.nextRun(next)
		s.untilDone(ctx, "daily", after, s.backfillAndSync)
		next = after
	}
}

// Every run backfills, so missing history is filled at the next run, and then syncs even if the backfill failed.
func (s *Scheduler) backfillAndSync(ctx context.Context, today time.Time) error {
	backfillErr := s.svc.BackfillFromEarliestEntry(ctx, today)
	if ctx.Err() != nil {
		return backfillErr
	}

	return errors.Join(backfillErr, s.svc.Sync(ctx, today))
}

// untilDone retries a failed run with doubling waits, and leaves the rest to the run scheduled at deadline.
func (s *Scheduler) untilDone(ctx context.Context, run string, deadline time.Time, fn func(ctx context.Context, today time.Time) error) {
	wait := firstRetry

	for {
		err := fn(ctx, s.today())
		if ctx.Err() != nil {
			return
		}

		if err == nil {
			s.logger.InfoContext(ctx, "exchange rates synced", slog.String("run", run))

			return
		}

		retryAt := time.Now().Add(wait)
		if !retryAt.Before(deadline) {
			retryAt = deadline
		}

		s.logger.ErrorContext(ctx, "exchange-rate sync failed", slog.String("run", run), slog.Any("error", err), slog.Time("next_attempt", retryAt))

		if retryAt.Equal(deadline) || !sleepUntil(ctx, retryAt) {
			return
		}

		wait = min(2*wait, maxRetry)
	}
}

func (s *Scheduler) nextRun(after time.Time) time.Time {
	y, m, d := after.In(s.berlin).Date()

	next := time.Date(y, m, d, syncHour, 0, 0, 0, s.berlin)
	if !next.After(after) {
		next = time.Date(y, m, d+1, syncHour, 0, 0, 0, s.berlin)
	}

	return next
}

// today is the Berlin calendar day, the day the provider's daily rates are published for.
func (s *Scheduler) today() time.Time {
	return civil(time.Now().In(s.berlin))
}

func sleepUntil(ctx context.Context, t time.Time) bool {
	timer := time.NewTimer(time.Until(t))
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
