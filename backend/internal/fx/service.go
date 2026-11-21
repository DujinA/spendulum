package fx

import (
	"context"
	"fmt"
	"time"
)

// A request for about three months of rates answers with roughly 1 MB.
const maxDaysPerRequest = 92

type RateProvider interface {
	// Currencies lists the ISO codes of the currencies the provider has rates for.
	Currencies(ctx context.Context) ([]string, error)
	// Rates returns rates against EUR, each dated by the day it was published for.
	Rates(ctx context.Context, from, to time.Time) ([]Rate, error)
}

type Repository interface {
	SaveCurrencies(ctx context.Context, cs []Currency) error
	Currencies(ctx context.Context) ([]Currency, error)
	// SaveRates replaces a stored rate of the same currency and date.
	SaveRates(ctx context.Context, rates []Rate) error
	// LatestRates returns, for each currency that has one, its newest rate on or before the date.
	LatestRates(ctx context.Context, currencies []string, on time.Time) ([]Rate, error)
	LastRateDate(ctx context.Context) (time.Time, bool, error)
	// RateDates returns the distinct dates in [from, to] that have at least one stored rate.
	RateDates(ctx context.Context, from, to time.Time) ([]time.Time, error)
}

type EntryDates interface {
	// EarliestEntryDate is the earliest entry date of any user; false when there is no entry.
	EarliestEntryDate(ctx context.Context) (time.Time, bool, error)
}

type Deps struct {
	Provider RateProvider
	Repo     Repository
	Entries  EntryDates
}

type Service struct {
	d Deps
}

func NewService(d Deps) *Service {
	return &Service{d: d}
}

func (s *Service) Currencies(ctx context.Context) ([]Currency, error) {
	cs, err := s.d.Repo.Currencies(ctx)
	if err != nil {
		return nil, fmt.Errorf("fx: currencies: %w", err)
	}

	return cs, nil
}

// Sync stores the provider's currencies and the rates from the last synced date through today.
func (s *Service) Sync(ctx context.Context, today time.Time) error {
	today = civil(today)
	from := today

	last, ok, err := s.d.Repo.LastRateDate(ctx)
	if err != nil {
		return fmt.Errorf("fx: last rate date: %w", err)
	}

	if ok && last.Before(today) {
		from = last
	}

	return s.fetch(ctx, []dateRange{{from: from, to: today}})
}

// Backfill runs through today, never to an earlier end, so the lookback never hides a hole; import apply passes its earliest entry date.
func (s *Service) Backfill(ctx context.Context, from, today time.Time) error {
	from, today = civil(from), civil(today)

	have, err := s.d.Repo.RateDates(ctx, from.AddDate(0, 0, -lookbackDays), today)
	if err != nil {
		return fmt.Errorf("fx: stored rate dates: %w", err)
	}

	gaps := missing(have, from, today)
	if len(gaps) == 0 {
		return nil
	}

	return s.fetch(ctx, gaps)
}

// BackfillFromEarliestEntry runs at startup, so every entry date through today has a rate.
func (s *Service) BackfillFromEarliestEntry(ctx context.Context, today time.Time) error {
	today = civil(today)
	from := today

	earliest, ok, err := s.d.Entries.EarliestEntryDate(ctx)
	if err != nil {
		return fmt.Errorf("fx: earliest entry date: %w", err)
	}

	if ok && civil(earliest).Before(today) {
		from = civil(earliest)
	}

	return s.Backfill(ctx, from, today)
}

func (s *Service) Convert(ctx context.Context, amount int64, from, to string, on time.Time) (Conversion, error) {
	if from == to {
		return NewRateBook(nil, nil, time.Time{}).Convert(amount, from, to, on)
	}

	currencies, err := s.d.Repo.Currencies(ctx)
	if err != nil {
		return Conversion{}, fmt.Errorf("fx: currencies: %w", err)
	}

	rates, err := s.d.Repo.LatestRates(ctx, []string{from, to}, civil(on))
	if err != nil {
		return Conversion{}, fmt.Errorf("fx: latest rates: %w", err)
	}

	last, _, err := s.d.Repo.LastRateDate(ctx)
	if err != nil {
		return Conversion{}, fmt.Errorf("fx: last rate date: %w", err)
	}

	return NewRateBook(currencies, rates, last).Convert(amount, from, to, on)
}

func (s *Service) fetch(ctx context.Context, ranges []dateRange) error {
	known, err := s.syncCurrencies(ctx)
	if err != nil {
		return err
	}

	for _, r := range ranges {
		for _, c := range r.chunks(maxDaysPerRequest) {
			rates, err := s.d.Provider.Rates(ctx, c.from, c.to)
			if err != nil {
				return fmt.Errorf("fx: fetch rates %s: %w", c, err)
			}

			keep := make([]Rate, 0, len(rates))

			for _, rt := range rates {
				if known[rt.Currency] && rt.Currency != EUR {
					keep = append(keep, rt)
				}
			}

			if len(keep) == 0 {
				continue
			}

			if err := s.d.Repo.SaveRates(ctx, keep); err != nil {
				return fmt.Errorf("fx: save rates %s: %w", c, err)
			}
		}
	}

	return nil
}

func (s *Service) syncCurrencies(ctx context.Context) (map[string]bool, error) {
	codes, err := s.d.Provider.Currencies(ctx)
	if err != nil {
		return nil, fmt.Errorf("fx: fetch currencies: %w", err)
	}

	cs := make([]Currency, len(codes))
	known := make(map[string]bool, len(codes))

	for i, code := range codes {
		cs[i] = Currency{Code: code, MinorUnits: minorUnits(code)}
		known[code] = true
	}

	if err := s.d.Repo.SaveCurrencies(ctx, cs); err != nil {
		return nil, fmt.Errorf("fx: save currencies: %w", err)
	}

	return known, nil
}
