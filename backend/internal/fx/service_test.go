package fx_test

import (
	"errors"
	"math/big"
	"reflect"
	"testing"
	"time"

	"github.com/DujinA/spendulum/backend/internal/fx"
)

func TestSyncStoresEveryProviderCurrencyWithItsISO4217MinorUnits(t *testing.T) {
	f := newFixture()
	f.provider.currencies = []string{"USD", "RSD", "JPY", "KWD", "EUR", "CLF", "GGP", "XDR"}

	if err := f.svc.Sync(t.Context(), day(t, "2026-01-05")); err != nil {
		t.Fatalf("Sync: %v", err)
	}

	got, err := f.svc.Currencies(t.Context())
	if err != nil {
		t.Fatalf("Currencies: %v", err)
	}

	want := []fx.Currency{
		{Code: "CLF", MinorUnits: 4},
		{Code: "EUR", MinorUnits: 2},
		{Code: "GGP", MinorUnits: 2},
		{Code: "JPY", MinorUnits: 0},
		{Code: "KWD", MinorUnits: 3},
		{Code: "RSD", MinorUnits: 2},
		{Code: "USD", MinorUnits: 2},
		{Code: "XDR", MinorUnits: 2},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Currencies() = %v, want %v", got, want)
	}
}

func TestSyncStoresTheNewestRatesAgainstEURAndConvertsAnyPairAsACrossRate(t *testing.T) {
	f := newFixture()
	f.provider.rates = []fx.Rate{
		rate(t, "EUR", "2026-01-05", "1"),
		rate(t, "RSD", "2026-01-05", "117.5"),
		rate(t, "USD", "2026-01-05", "1.1"),
		rate(t, "BTC", "2026-01-05", "0.0000106"),
	}

	if err := f.svc.Sync(t.Context(), day(t, "2026-01-05")); err != nil {
		t.Fatalf("Sync: %v", err)
	}

	got, err := f.svc.Convert(t.Context(), 1000, "USD", "RSD", day(t, "2026-01-05"))
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}

	if got.Amount.Cmp(big.NewRat(1175000, 11)) != 0 || got.Provisional || !got.RateDate.Equal(day(t, "2026-01-05")) {
		t.Errorf("10.00 USD in RSD = %+v (%s), want 1175000/11 at the rate of 2026-01-05", got, got.Amount.RatString())
	}

	if f.repo.has("BTC", day(t, "2026-01-05")) || f.repo.has("EUR", day(t, "2026-01-05")) {
		t.Error("stored a rate for BTC, which is not a provider currency, or for EUR itself")
	}
}

func TestSyncFetchesFromTheLastSyncedDateThroughToday(t *testing.T) {
	f := newFixture()
	f.provider.rates = []fx.Rate{rate(t, "RSD", "2026-01-02", "117.1")}

	if err := f.svc.Sync(t.Context(), day(t, "2026-01-02")); err != nil {
		t.Fatalf("first Sync: %v", err)
	}

	if err := f.svc.Sync(t.Context(), day(t, "2026-01-06")); err != nil {
		t.Fatalf("second Sync: %v", err)
	}

	want := []request{
		{day(t, "2026-01-02"), day(t, "2026-01-02")},
		{day(t, "2026-01-02"), day(t, "2026-01-06")},
	}
	if got := f.provider.madeRequests(); !reflect.DeepEqual(got, want) {
		t.Errorf("rate requests = %v, want %v", got, want)
	}
}

func TestStartupBackfillFetchesWhatIsMissingFromTheEarliestEntryDateToToday(t *testing.T) {
	f := newFixture()
	f.entries.earliest = day(t, "2026-01-01")
	f.storeRSDDaily(t, "2026-01-04", "2026-01-06")
	f.provider.rates = []fx.Rate{rate(t, "RSD", "2026-01-01", "117.3")}
	f.provider.requests = nil

	if err := f.svc.BackfillFromEarliestEntry(t.Context(), day(t, "2026-01-10")); err != nil {
		t.Fatalf("BackfillFromEarliestEntry: %v", err)
	}

	// 01-07 to 01-10 fall within 7 days of the rate of 01-06, so only 01-01 to 01-03 are missing.
	want := []request{{day(t, "2025-12-25"), day(t, "2026-01-03")}}
	if got := f.provider.madeRequests(); !reflect.DeepEqual(got, want) {
		t.Errorf("rate requests = %v, want %v", got, want)
	}

	if !f.repo.has("RSD", day(t, "2026-01-01")) {
		t.Error("the backfilled rate of 2026-01-01 was not stored")
	}
}

func TestStartupBackfillWithNoEntryCoversToday(t *testing.T) {
	f := newFixture()

	if err := f.svc.BackfillFromEarliestEntry(t.Context(), day(t, "2026-01-10")); err != nil {
		t.Fatalf("BackfillFromEarliestEntry: %v", err)
	}

	want := []request{{day(t, "2026-01-03"), day(t, "2026-01-10")}}
	if got := f.provider.madeRequests(); !reflect.DeepEqual(got, want) {
		t.Errorf("rate requests = %v, want %v", got, want)
	}
}

func TestStartupBackfillAsksTheProviderNothingWhenStoredRatesCoverTheRange(t *testing.T) {
	f := newFixture()
	f.entries.earliest = day(t, "2026-01-04")
	f.storeRSDDaily(t, "2026-01-04", "2026-01-06")
	f.provider.fail(errProviderDown)
	before := len(f.provider.callTimes()) + len(f.provider.madeRequests())

	if err := f.svc.BackfillFromEarliestEntry(t.Context(), day(t, "2026-01-06")); err != nil {
		t.Fatalf("BackfillFromEarliestEntry: %v", err)
	}

	if calls := len(f.provider.callTimes()) + len(f.provider.madeRequests()) - before; calls != 0 {
		t.Errorf("the provider was asked %d times, want 0", calls)
	}
}

func TestBackfillOfARangeFetchesItInRequestsOfAtMost92Days(t *testing.T) {
	f := newFixture()

	if err := f.svc.Backfill(t.Context(), day(t, "2025-01-01"), day(t, "2025-12-31")); err != nil {
		t.Fatalf("Backfill: %v", err)
	}

	want := []request{
		{day(t, "2024-12-25"), day(t, "2025-03-26")},
		{day(t, "2025-03-27"), day(t, "2025-06-26")},
		{day(t, "2025-06-27"), day(t, "2025-09-26")},
		{day(t, "2025-09-27"), day(t, "2025-12-27")},
		{day(t, "2025-12-28"), day(t, "2025-12-31")},
	}
	if got := f.provider.madeRequests(); !reflect.DeepEqual(got, want) {
		t.Errorf("rate requests = %v, want %v", got, want)
	}
}

func TestBackfillStartingOnAWeekendStoresTheRateBeforeIt(t *testing.T) {
	f := newFixture()
	f.provider.rates = []fx.Rate{rate(t, "RSD", "2026-01-02", "117.1"), rate(t, "RSD", "2026-01-05", "117.5")}

	if err := f.svc.Backfill(t.Context(), day(t, "2026-01-03"), day(t, "2026-01-05")); err != nil {
		t.Fatalf("Backfill: %v", err)
	}

	got, err := f.svc.Convert(t.Context(), 100, "EUR", "RSD", day(t, "2026-01-03"))
	if err != nil {
		t.Fatalf("Convert on Saturday 2026-01-03: %v", err)
	}

	if got.Amount.Cmp(big.NewRat(11710, 1)) != 0 || !got.RateDate.Equal(day(t, "2026-01-02")) || got.Provisional {
		t.Errorf("1.00 EUR in RSD on Saturday = %+v, want 11710 at Friday's rate, not provisional", got)
	}
}

func TestBackfillOverWeekendsItAlreadyStoredAsksTheProviderNothing(t *testing.T) {
	f := newFixture()

	for d := day(t, "2026-01-01"); d.Before(day(t, "2026-02-01")); d = d.AddDate(0, 0, 1) {
		if d.Weekday() != time.Saturday && d.Weekday() != time.Sunday {
			f.provider.rates = append(f.provider.rates, fx.Rate{Currency: "RSD", Date: d, PerEUR: big.NewRat(1175, 10)})
		}
	}

	if err := f.svc.Backfill(t.Context(), day(t, "2026-01-01"), day(t, "2026-01-31")); err != nil {
		t.Fatalf("first Backfill: %v", err)
	}

	before := len(f.provider.callTimes()) + len(f.provider.madeRequests())

	if err := f.svc.Backfill(t.Context(), day(t, "2026-01-01"), day(t, "2026-01-31")); err != nil {
		t.Fatalf("second Backfill: %v", err)
	}

	if calls := len(f.provider.callTimes()) + len(f.provider.madeRequests()) - before; calls != 0 {
		t.Errorf("the second backfill asked the provider %d times, want 0", calls)
	}
}

func TestConvertKeepsUsingStoredRatesWhileTheProviderFails(t *testing.T) {
	f := newFixture()
	f.provider.rates = []fx.Rate{rate(t, "RSD", "2026-01-05", "117.5")}

	if err := f.svc.Sync(t.Context(), day(t, "2026-01-05")); err != nil {
		t.Fatalf("Sync: %v", err)
	}

	f.provider.fail(errProviderDown)

	if err := f.svc.Sync(t.Context(), day(t, "2026-01-06")); !errors.Is(err, errProviderDown) {
		t.Fatalf("Sync with the provider down = %v, want its error", err)
	}

	got, err := f.svc.Convert(t.Context(), 100, "EUR", "RSD", day(t, "2026-01-06"))
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}

	if got.Amount.Cmp(big.NewRat(11750, 1)) != 0 || !got.Provisional || !got.RateDate.Equal(day(t, "2026-01-05")) {
		t.Errorf("1.00 EUR in RSD on 2026-01-06 = %+v, want 11750 at the stored rate of 2026-01-05, provisional", got)
	}
}

var errProviderDown = errors.New("provider is down")

// storeRSDDaily stores one RSD rate a day for from through to.
func (f *fixture) storeRSDDaily(t *testing.T, from, to string) {
	t.Helper()

	for d := day(t, from); !d.After(day(t, to)); d = d.AddDate(0, 0, 1) {
		f.provider.rates = append(f.provider.rates, fx.Rate{Currency: "RSD", Date: d, PerEUR: big.NewRat(1175, 10)})
	}

	if err := f.svc.Backfill(t.Context(), day(t, from), day(t, to)); err != nil {
		t.Fatalf("store RSD rates: %v", err)
	}
}
