package fx_test

import (
	"errors"
	"math"
	"math/big"
	"testing"

	"pgregory.net/rapid"

	"github.com/DujinA/spendulum/backend/internal/fx"
)

func TestRoundRoundsHalfAwayFromZero(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want int64
	}{
		{"0", 0},
		{"7", 7},
		{"2.4999", 2},
		{"2.5", 3},
		{"2.5001", 3},
		{"-2.4999", -2},
		{"-2.5", -3},
		{"-2.5001", -3},
		{"1/3", 0},
		{"-2/3", -1},
		{"1068181.8181", 1068182},
		{"-0.5", -1},
	} {
		got, err := fx.Round(rat(t, tc.in))
		if err != nil || got != tc.want {
			t.Errorf("Round(%s) = %d, %v; want %d", tc.in, got, err, tc.want)
		}
	}
}

func TestRoundOfAFigureBeyondInt64IsAnError(t *testing.T) {
	huge := new(big.Rat).SetFrac(new(big.Int).Lsh(big.NewInt(1), 70), big.NewInt(1))

	if _, err := fx.Round(huge); !errors.Is(err, fx.ErrOutOfRange) {
		t.Errorf("Round(2^70) error = %v, want ErrOutOfRange", err)
	}
}

func TestOnlyTheDisplayedFigureIsRoundedToTheTargetCurrencysMinorUnits(t *testing.T) {
	book := fx.NewRateBook(testCurrencies(), []fx.Rate{
		rate(t, "JPY", "2026-01-05", "160.505"),
		rate(t, "KWD", "2026-01-05", "0.33555"),
		rate(t, "RSD", "2026-01-05", "117.5"),
	}, day(t, "2026-01-05"))

	for to, want := range map[string]int64{"JPY": 161, "KWD": 336, "RSD": 11750} {
		c, err := book.Convert(100, "EUR", to, day(t, "2026-01-05"))
		if err != nil {
			t.Fatal(err)
		}

		if got, err := fx.Round(c.Amount); err != nil || got != want {
			t.Errorf("1.00 EUR shown in %s = %d minor units, %v; want %d", to, got, err, want)
		}
	}

	total := new(big.Rat)

	for range 3 {
		c, _ := book.Convert(1, "EUR", "RSD", day(t, "2026-01-05"))
		total.Add(total, c.Amount)
	}

	if got, _ := fx.Round(total); got != 353 {
		t.Errorf("three times 0.01 EUR shown in RSD = %d, want 353 (352.5 rounded once, not 3 × 118)", got)
	}
}

func TestRoundIsWithinHalfAUnitAndSymmetricAroundZero(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		x := genRat(t, "x")

		got, err := fx.Round(x)
		if err != nil {
			t.Fatalf("Round(%s): %v", x.RatString(), err)
		}

		diff := new(big.Rat).Sub(new(big.Rat).SetInt64(got), x)
		if diff.Abs(diff).Cmp(big.NewRat(1, 2)) > 0 {
			t.Fatalf("Round(%s) = %d, more than half a unit away", x.RatString(), got)
		}

		neg, _ := fx.Round(new(big.Rat).Neg(x))
		if neg != -got {
			t.Fatalf("Round(-%s) = %d, want %d", x.RatString(), neg, -got)
		}
	})
}

func TestRoundOfAHalfMovesAwayFromZero(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		n := rapid.Int64Range(0, math.MaxInt64/4).Draw(t, "n")
		half := new(big.Rat).Add(new(big.Rat).SetInt64(n), big.NewRat(1, 2))

		if got, _ := fx.Round(half); got != n+1 {
			t.Fatalf("Round(%d + 1/2) = %d, want %d", n, got, n+1)
		}

		if got, _ := fx.Round(new(big.Rat).Neg(half)); got != -n-1 {
			t.Fatalf("Round(-%d - 1/2) = %d, want %d", n, got, -n-1)
		}
	})
}

// genRat draws a fraction whose rounded value fits in int64.
func genRat(t *rapid.T, label string) *big.Rat {
	num := rapid.Int64Range(-1<<50, 1<<50).Draw(t, label+" numerator")
	den := rapid.Int64Range(1, 1<<20).Draw(t, label+" denominator")

	return big.NewRat(num, den)
}

func rat(tb testing.TB, s string) *big.Rat {
	tb.Helper()

	r, ok := new(big.Rat).SetString(s)
	if !ok {
		tb.Fatalf("not a number: %q", s)
	}

	return r
}
