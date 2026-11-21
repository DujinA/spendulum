package fx_test

import (
	"context"
	"slices"
	"sync"
	"time"

	"github.com/DujinA/spendulum/backend/internal/fx"
)

type fixture struct {
	provider *fakeProvider
	repo     *fakeRepo
	entries  *fakeEntries
	svc      *fx.Service
}

func newFixture() *fixture {
	f := &fixture{
		provider: &fakeProvider{currencies: []string{"EUR", "JPY", "KWD", "RSD", "USD"}},
		repo:     &fakeRepo{currencies: map[string]fx.Currency{}, rates: map[rateKey]fx.Rate{}},
		entries:  &fakeEntries{},
	}
	f.svc = fx.NewService(fx.Deps{Provider: f.provider, Repo: f.repo, Entries: f.entries})

	return f
}

type request struct{ from, to time.Time }

type fakeProvider struct {
	mu         sync.Mutex
	currencies []string
	rates      []fx.Rate
	err        error
	requests   []request
	calls      []time.Time
	// The first rates request whose range holds this date fails.
	failOnceOn time.Time
}

func (p *fakeProvider) Currencies(context.Context) ([]string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.calls = append(p.calls, time.Now().UTC())

	if p.err != nil {
		return nil, p.err
	}

	return slices.Clone(p.currencies), nil
}

func (p *fakeProvider) Rates(_ context.Context, from, to time.Time) ([]fx.Rate, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.requests = append(p.requests, request{from, to})

	if p.err != nil {
		return nil, p.err
	}

	if !p.failOnceOn.IsZero() && !from.After(p.failOnceOn) && !to.Before(p.failOnceOn) {
		p.failOnceOn = time.Time{}

		return nil, errProviderDown
	}

	var out []fx.Rate

	for _, r := range p.rates {
		if !r.Date.Before(from) && !r.Date.After(to) {
			out = append(out, r)
		}
	}

	return out, nil
}

func (p *fakeProvider) fail(err error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.err = err
}

func (p *fakeProvider) madeRequests() []request {
	p.mu.Lock()
	defer p.mu.Unlock()

	return slices.Clone(p.requests)
}

func (p *fakeProvider) callTimes() []time.Time {
	p.mu.Lock()
	defer p.mu.Unlock()

	return slices.Clone(p.calls)
}

type rateKey struct {
	currency string
	date     time.Time
}

type fakeRepo struct {
	mu         sync.Mutex
	currencies map[string]fx.Currency
	rates      map[rateKey]fx.Rate
}

func (r *fakeRepo) SaveCurrencies(_ context.Context, cs []fx.Currency) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, c := range cs {
		r.currencies[c.Code] = c
	}

	return nil
}

func (r *fakeRepo) Currencies(context.Context) ([]fx.Currency, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	out := make([]fx.Currency, 0, len(r.currencies))
	for _, c := range r.currencies {
		out = append(out, c)
	}

	slices.SortFunc(out, func(a, b fx.Currency) int { return compare(a.Code, b.Code) })

	return out, nil
}

func (r *fakeRepo) SaveRates(_ context.Context, rates []fx.Rate) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, rt := range rates {
		if _, ok := r.currencies[rt.Currency]; !ok {
			panic("rate for a currency that is not stored: " + rt.Currency)
		}

		r.rates[rateKey{rt.Currency, rt.Date}] = rt
	}

	return nil
}

func (r *fakeRepo) LatestRates(_ context.Context, currencies []string, on time.Time) ([]fx.Rate, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	latest := map[string]fx.Rate{}

	for k, rt := range r.rates {
		if slices.Contains(currencies, k.currency) && !k.date.After(on) && k.date.After(latest[k.currency].Date) {
			latest[k.currency] = rt
		}
	}

	out := make([]fx.Rate, 0, len(latest))
	for _, rt := range latest {
		out = append(out, rt)
	}

	return out, nil
}

func (r *fakeRepo) LastRateDate(context.Context) (time.Time, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	var last time.Time

	for k := range r.rates {
		if k.date.After(last) {
			last = k.date
		}
	}

	return last, !last.IsZero(), nil
}

func (r *fakeRepo) RateDates(_ context.Context, from, to time.Time) ([]time.Time, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	var out []time.Time

	for k := range r.rates {
		if !k.date.Before(from) && !k.date.After(to) && !slices.ContainsFunc(out, k.date.Equal) {
			out = append(out, k.date)
		}
	}

	slices.SortFunc(out, time.Time.Compare)

	return out, nil
}

func (r *fakeRepo) has(currency string, date time.Time) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	_, ok := r.rates[rateKey{currency, date}]

	return ok
}

type fakeEntries struct {
	earliest time.Time
}

func (e *fakeEntries) EarliestEntryDate(context.Context) (time.Time, bool, error) {
	return e.earliest, !e.earliest.IsZero(), nil
}

func compare(a, b string) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}
