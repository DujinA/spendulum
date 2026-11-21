package fx_test

import (
	"reflect"
	"testing"
	"time"

	"github.com/DujinA/spendulum/backend/internal/fx"
	"github.com/DujinA/spendulum/backend/internal/platform/db/dbtest"
)

func TestMain(m *testing.M) { dbtest.Main(m) }

func TestCurrenciesAreSavedOnceAndListedByCode(t *testing.T) {
	repo := fx.NewPostgres(dbtest.Open(t).Queries())

	saveCurrencies(t, repo, fx.Currency{Code: "RSD", MinorUnits: 2}, fx.Currency{Code: "JPY", MinorUnits: 0}, fx.Currency{Code: "KWD", MinorUnits: 3})
	saveCurrencies(t, repo, fx.Currency{Code: "RSD", MinorUnits: 2}, fx.Currency{Code: "EUR", MinorUnits: 2})

	got, err := repo.Currencies(t.Context())
	if err != nil {
		t.Fatalf("Currencies: %v", err)
	}

	want := []fx.Currency{{Code: "EUR", MinorUnits: 2}, {Code: "JPY", MinorUnits: 0}, {Code: "KWD", MinorUnits: 3}, {Code: "RSD", MinorUnits: 2}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Currencies() = %v, want %v", got, want)
	}
}

func TestStoredRatesKeepFullPrecision(t *testing.T) {
	repo := fx.NewPostgres(dbtest.Open(t).Queries())
	saveCurrencies(t, repo, fx.Currency{Code: "IRR"}, fx.Currency{Code: "KWD", MinorUnits: 3}, fx.Currency{Code: "RSD"})

	want := []fx.Rate{
		rate(t, "IRR", "2026-01-05", "2127444"),
		rate(t, "KWD", "2026-01-05", "0.000017751"),
		rate(t, "RSD", "2026-01-05", "117.771234567890123456789"),
	}
	saveRates(t, repo, want...)

	got := latestRates(t, repo, []string{"IRR", "KWD", "RSD"}, "2026-01-05")
	assertRates(t, got, want)
}

func TestLatestRatesAreTheNewestOnOrBeforeTheDatePerCurrency(t *testing.T) {
	repo := fx.NewPostgres(dbtest.Open(t).Queries())
	saveCurrencies(t, repo, fx.Currency{Code: "JPY"}, fx.Currency{Code: "RSD"}, fx.Currency{Code: "USD"})
	saveRates(t, repo,
		rate(t, "RSD", "2026-01-02", "117.1"),
		rate(t, "USD", "2026-01-02", "1.04"),
		rate(t, "RSD", "2026-01-05", "117.5"),
		rate(t, "JPY", "2026-01-05", "160"),
	)

	assertRates(t, latestRates(t, repo, []string{"RSD", "USD", "JPY"}, "2026-01-04"), []fx.Rate{
		rate(t, "RSD", "2026-01-02", "117.1"),
		rate(t, "USD", "2026-01-02", "1.04"),
	})
	assertRates(t, latestRates(t, repo, []string{"RSD", "USD"}, "2026-01-05"), []fx.Rate{
		rate(t, "RSD", "2026-01-05", "117.5"),
		rate(t, "USD", "2026-01-02", "1.04"),
	})
}

func TestSavingARateAgainReplacesIt(t *testing.T) {
	repo := fx.NewPostgres(dbtest.Open(t).Queries())
	saveCurrencies(t, repo, fx.Currency{Code: "RSD"})
	saveRates(t, repo, rate(t, "RSD", "2026-01-05", "117.5"))
	saveRates(t, repo, rate(t, "RSD", "2026-01-05", "117.62"))

	assertRates(t, latestRates(t, repo, []string{"RSD"}, "2026-01-05"), []fx.Rate{rate(t, "RSD", "2026-01-05", "117.62")})
}

func TestLastRateDateAndTheDatesThatHaveRates(t *testing.T) {
	repo := fx.NewPostgres(dbtest.Open(t).Queries())

	if _, ok, err := repo.LastRateDate(t.Context()); ok || err != nil {
		t.Errorf("LastRateDate with no rates = %v, %v; want none", ok, err)
	}

	saveCurrencies(t, repo, fx.Currency{Code: "RSD"}, fx.Currency{Code: "USD"})
	saveRates(t, repo,
		rate(t, "RSD", "2026-01-02", "117.1"),
		rate(t, "USD", "2026-01-02", "1.04"),
		rate(t, "RSD", "2026-01-05", "117.5"),
	)

	if last, ok, err := repo.LastRateDate(t.Context()); !ok || err != nil || !last.Equal(day(t, "2026-01-05")) {
		t.Errorf("LastRateDate = %s, %v, %v; want 2026-01-05", last, ok, err)
	}

	got, err := repo.RateDates(t.Context(), day(t, "2026-01-01"), day(t, "2026-01-04"))
	if err != nil || !reflect.DeepEqual(got, []time.Time{day(t, "2026-01-02")}) {
		t.Errorf("RateDates(01-01, 01-04) = %v, %v; want [2026-01-02]", got, err)
	}
}

func saveCurrencies(t *testing.T, repo *fx.Postgres, cs ...fx.Currency) {
	t.Helper()

	if err := repo.SaveCurrencies(t.Context(), cs); err != nil {
		t.Fatalf("SaveCurrencies: %v", err)
	}
}

func saveRates(t *testing.T, repo *fx.Postgres, rates ...fx.Rate) {
	t.Helper()

	if err := repo.SaveRates(t.Context(), rates); err != nil {
		t.Fatalf("SaveRates: %v", err)
	}
}

func latestRates(t *testing.T, repo *fx.Postgres, currencies []string, on string) []fx.Rate {
	t.Helper()

	got, err := repo.LatestRates(t.Context(), currencies, day(t, on))
	if err != nil {
		t.Fatalf("LatestRates: %v", err)
	}

	return got
}

// assertRates compares rates by value, in currency order.
func assertRates(t *testing.T, got, want []fx.Rate) {
	t.Helper()

	same := len(got) == len(want)
	for i := 0; same && i < len(got); i++ {
		same = got[i].Currency == want[i].Currency && got[i].Date.Equal(want[i].Date) && got[i].PerEUR.Cmp(want[i].PerEUR) == 0
	}

	if !same {
		t.Errorf("rates = %v, want %v", got, want)
	}
}
