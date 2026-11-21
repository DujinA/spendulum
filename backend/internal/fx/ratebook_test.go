package fx_test

import (
	"errors"
	"math/big"
	"testing"
	"time"

	"pgregory.net/rapid"

	"github.com/DujinA/spendulum/backend/internal/fx"
)

func testCurrencies() []fx.Currency {
	return []fx.Currency{
		{Code: "EUR", MinorUnits: 2},
		{Code: "JPY", MinorUnits: 0},
		{Code: "KWD", MinorUnits: 3},
		{Code: "RSD", MinorUnits: 2},
		{Code: "USD", MinorUnits: 2},
	}
}

func TestConvertUsesTheEntryDatesRateAtFullPrecision(t *testing.T) {
	book := fx.NewRateBook(testCurrencies(), []fx.Rate{
		rate(t, "RSD", "2026-01-05", "117.5"),
		rate(t, "USD", "2026-01-05", "1.1"),
		rate(t, "RSD", "2026-01-06", "118"),
		rate(t, "USD", "2026-01-06", "1.1"),
	}, day(t, "2026-01-06"))

	for on, want := range map[string]*big.Rat{
		"2026-01-05": big.NewRat(1175000, 11),
		"2026-01-06": big.NewRat(1180000, 11),
	} {
		got, err := book.Convert(1000, "USD", "RSD", day(t, on))
		if err != nil {
			t.Fatalf("Convert on %s: %v", on, err)
		}

		if got.Amount.Cmp(want) != 0 || got.Currency != "RSD" || !got.RateDate.Equal(day(t, on)) || got.Provisional {
			t.Errorf("10.00 USD in RSD on %s = %+v (%s), want %s RSD minor units at that date's rate, not provisional",
				on, got, got.Amount.RatString(), want.RatString())
		}
	}
}

func TestAnyPairIsACrossRateOfTheirRatesAgainstEUR(t *testing.T) {
	book := fx.NewRateBook(testCurrencies(), []fx.Rate{
		rate(t, "JPY", "2026-01-05", "160"),
		rate(t, "KWD", "2026-01-05", "0.32"),
		rate(t, "RSD", "2026-01-05", "117.5"),
	}, day(t, "2026-01-05"))

	for _, tc := range []struct {
		amount   int64
		from, to string
		want     *big.Rat
	}{
		{100, "EUR", "RSD", big.NewRat(11750, 1)},
		{11750, "RSD", "EUR", big.NewRat(100, 1)},
		{1000, "JPY", "KWD", big.NewRat(2000, 1)},
		{2000, "KWD", "JPY", big.NewRat(1000, 1)},
		{100, "RSD", "JPY", big.NewRat(64, 47)},
	} {
		got, err := book.Convert(tc.amount, tc.from, tc.to, day(t, "2026-01-05"))
		if err != nil || got.Amount.Cmp(tc.want) != 0 {
			t.Errorf("Convert(%d %s to %s) = %v, %v; want %s", tc.amount, tc.from, tc.to, got.Amount, err, tc.want.RatString())
		}
	}
}

func TestAnAmountAlreadyInTheTargetCurrencyIsNotConverted(t *testing.T) {
	book := fx.NewRateBook(nil, nil, time.Time{})

	got, err := book.Convert(-12345, "RSD", "RSD", day(t, "2030-01-01"))
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}

	if got.Amount.Cmp(big.NewRat(-12345, 1)) != 0 || got.Currency != "RSD" || got.Provisional || !got.RateDate.IsZero() {
		t.Errorf("Convert(-12345 RSD to RSD) = %+v, want the amount itself, no rate, not provisional", got)
	}
}

func TestADateWithNoRateOnOrBeforeTheLastSyncedDateUsesTheLatestEarlierRateAndIsNotProvisional(t *testing.T) {
	book := fx.NewRateBook(testCurrencies(), []fx.Rate{
		rate(t, "RSD", "2026-01-02", "117.1"),
		rate(t, "RSD", "2026-01-05", "117.5"),
	}, day(t, "2026-01-05"))

	for _, saturday := range []string{"2026-01-03", "2026-01-04"} {
		got, err := book.Convert(100, "EUR", "RSD", day(t, saturday))
		if err != nil {
			t.Fatalf("Convert: %v", err)
		}

		if got.Amount.Cmp(big.NewRat(11710, 1)) != 0 || !got.RateDate.Equal(day(t, "2026-01-02")) || got.Provisional {
			t.Errorf("1.00 EUR in RSD on %s = %+v, want 11710 at the rate of 2026-01-02, not provisional", saturday, got)
		}
	}
}

func TestADateAfterTheLastSyncedDateUsesTheLatestEarlierRateAndIsProvisional(t *testing.T) {
	book := fx.NewRateBook(testCurrencies(), []fx.Rate{
		rate(t, "RSD", "2026-01-02", "117.1"),
		rate(t, "USD", "2026-01-02", "1.04"),
		rate(t, "RSD", "2026-01-05", "117.5"),
	}, day(t, "2026-01-05"))

	got, err := book.Convert(100, "EUR", "RSD", day(t, "2026-01-07"))
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}

	if got.Amount.Cmp(big.NewRat(11750, 1)) != 0 || !got.RateDate.Equal(day(t, "2026-01-05")) || !got.Provisional {
		t.Errorf("1.00 EUR in RSD on 2026-01-07 = %+v, want 11750 at the rate of 2026-01-05, provisional", got)
	}

	cross, err := book.Convert(104, "USD", "RSD", day(t, "2026-01-07"))
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}

	if cross.Amount.Cmp(big.NewRat(11750, 1)) != 0 || !cross.RateDate.Equal(day(t, "2026-01-02")) || !cross.Provisional {
		t.Errorf("1.04 USD in RSD on 2026-01-07 = %+v, want 11750, provisional, dated by its older rate 2026-01-02", cross)
	}
}

func TestConvertWithoutARateOrWithAnUnknownCurrencyFails(t *testing.T) {
	book := fx.NewRateBook(testCurrencies(), []fx.Rate{rate(t, "RSD", "2026-01-05", "117.5")}, day(t, "2026-01-05"))

	if _, err := book.Convert(100, "EUR", "RSD", day(t, "2026-01-04")); !errors.Is(err, fx.ErrNoRate) {
		t.Errorf("Convert before the first rate = %v, want ErrNoRate", err)
	}

	if _, err := book.Convert(100, "EUR", "USD", day(t, "2026-01-05")); !errors.Is(err, fx.ErrNoRate) {
		t.Errorf("Convert to a currency without rates = %v, want ErrNoRate", err)
	}

	if _, err := book.Convert(100, "EUR", "XYZ", day(t, "2026-01-05")); !errors.Is(err, fx.ErrUnknownCurrency) {
		t.Errorf("Convert to XYZ = %v, want ErrUnknownCurrency", err)
	}
}

func TestConvertingThereAndBackAtOneDateIsExact(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		book, on := genBook(t)
		from := rapid.SampledFrom(testCurrencies()).Draw(t, "from").Code
		to := rapid.SampledFrom(testCurrencies()).Draw(t, "to").Code
		a := rapid.Int64Range(-1e12, 1e12).Draw(t, "a")
		b := rapid.Int64Range(-1e12, 1e12).Draw(t, "b")

		there, err := book.Convert(a, from, to, on)
		if err != nil {
			t.Fatalf("Convert: %v", err)
		}

		back, err := book.Convert(b, to, from, on)
		if err != nil {
			t.Fatalf("Convert back: %v", err)
		}

		want := new(big.Rat).SetInt(new(big.Int).Mul(big.NewInt(a), big.NewInt(b)))
		if product := new(big.Rat).Mul(there.Amount, back.Amount); product.Cmp(want) != 0 {
			t.Fatalf("%d %s→%s times %d back = %s, want exactly %s", a, from, to, b, product.RatString(), want.RatString())
		}
	})
}

func TestConvertingASumEqualsSummingTheConversions(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		book, on := genBook(t)
		from := rapid.SampledFrom(testCurrencies()).Draw(t, "from").Code
		to := rapid.SampledFrom(testCurrencies()).Draw(t, "to").Code
		a := rapid.Int64Range(-1e15, 1e15).Draw(t, "a")
		b := rapid.Int64Range(-1e15, 1e15).Draw(t, "b")

		sum, _ := book.Convert(a+b, from, to, on)
		ca, _ := book.Convert(a, from, to, on)
		cb, _ := book.Convert(b, from, to, on)

		if added := new(big.Rat).Add(ca.Amount, cb.Amount); added.Cmp(sum.Amount) != 0 {
			t.Fatalf("convert(%d)+convert(%d) = %s, convert(%d) = %s", a, b, added.RatString(), a+b, sum.Amount.RatString())
		}
	})
}

func TestConvertIsProvisionalExactlyAfterTheLastSyncedDate(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		book, lastSynced := genBook(t)
		on := lastSynced.AddDate(0, 0, rapid.IntRange(-30, 30).Draw(t, "days from the last synced date"))

		got, err := book.Convert(100, "EUR", "RSD", on)
		if err != nil {
			t.Fatalf("Convert on %s: %v", on, err)
		}

		if got.Provisional != on.After(lastSynced) {
			t.Fatalf("Convert on %s with last synced %s: provisional = %v", on, lastSynced, got.Provisional)
		}

		if got.RateDate.After(on) {
			t.Fatalf("Convert on %s used a later rate, of %s", on, got.RateDate)
		}
	})
}

// genBook draws daily rates for every test currency over 60 days and returns the last one as the last synced date.
func genBook(t *rapid.T) (*fx.RateBook, time.Time) {
	first := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	currencies := testCurrencies()
	rates := make([]fx.Rate, 0, 60*len(currencies))

	for d := range 60 {
		for _, c := range currencies[1:] {
			micros := rapid.Int64Range(1, 1e12).Draw(t, c.Code+" rate in millionths")
			rates = append(rates, fx.Rate{Currency: c.Code, Date: first.AddDate(0, 0, d), PerEUR: big.NewRat(micros, 1e6)})
		}
	}

	return fx.NewRateBook(testCurrencies(), rates, first.AddDate(0, 0, 59)), first.AddDate(0, 0, 59)
}

func rate(tb testing.TB, currency, date, perEUR string) fx.Rate {
	tb.Helper()

	return fx.Rate{Currency: currency, Date: day(tb, date), PerEUR: rat(tb, perEUR)}
}

func day(tb testing.TB, s string) time.Time {
	tb.Helper()

	d, err := time.Parse(time.DateOnly, s)
	if err != nil {
		tb.Fatal(err)
	}

	return d
}
