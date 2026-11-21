package fx

import (
	"errors"
	"fmt"
	"math/big"
	"slices"
	"time"
)

// EUR is the currency every rate is stored against (ADR 0007), so its own rate is 1 on every date.
const EUR = "EUR"

var (
	ErrUnknownCurrency = errors.New("fx: the rate provider does not support this currency")
	ErrNoRate          = errors.New("fx: no exchange rate on or before this date")
	ErrOutOfRange      = errors.New("fx: the rounded figure does not fit in int64")
)

type Currency struct {
	Code       string
	MinorUnits int
}

// A Rate is how many units of Currency one euro buys on Date.
type Rate struct {
	Currency string
	Date     time.Time
	PerEUR   *big.Rat
}

type Conversion struct {
	// Minor units of Currency at full precision; Round it only for display.
	Amount   *big.Rat
	Currency string
	// The date of the rate used; for a cross rate, the older of its two rates. Zero when nothing was converted.
	RateDate    time.Time
	Provisional bool
}

// A RateBook converts with the rates it was given and never fetches any.
type RateBook struct {
	currencies map[string]Currency
	rates      map[string][]Rate
	lastSynced time.Time
}

func NewRateBook(currencies []Currency, rates []Rate, lastSynced time.Time) *RateBook {
	b := &RateBook{
		currencies: make(map[string]Currency, len(currencies)),
		rates:      make(map[string][]Rate),
		lastSynced: civil(lastSynced),
	}

	for _, c := range currencies {
		b.currencies[c.Code] = c
	}

	for _, r := range rates {
		r.Date = civil(r.Date)
		b.rates[r.Currency] = append(b.rates[r.Currency], r)
	}

	for _, rs := range b.rates {
		slices.SortFunc(rs, func(x, y Rate) int { return x.Date.Compare(y.Date) })
	}

	return b
}

// Convert uses each currency's latest rate on or before the entry date; past the last synced date it is provisional.
func (b *RateBook) Convert(amount int64, from, to string, on time.Time) (Conversion, error) {
	if from == to {
		return Conversion{Amount: new(big.Rat).SetInt64(amount), Currency: to}, nil
	}

	on = civil(on)

	fromCur, fromRate, err := b.rateOn(from, on)
	if err != nil {
		return Conversion{}, err
	}

	toCur, toRate, err := b.rateOn(to, on)
	if err != nil {
		return Conversion{}, err
	}

	v := new(big.Rat).SetInt64(amount)
	v.Mul(v, toRate.PerEUR)
	v.Quo(v, fromRate.PerEUR)
	v.Mul(v, pow10(toCur.MinorUnits-fromCur.MinorUnits))

	return Conversion{
		Amount:      v,
		Currency:    to,
		RateDate:    olderRateDate(fromRate, toRate),
		Provisional: on.After(b.lastSynced),
	}, nil
}

func (b *RateBook) rateOn(code string, on time.Time) (Currency, Rate, error) {
	c, ok := b.currencies[code]
	if !ok {
		return Currency{}, Rate{}, fmt.Errorf("%w: %s", ErrUnknownCurrency, code)
	}

	if code == EUR {
		return c, Rate{Currency: EUR, PerEUR: big.NewRat(1, 1)}, nil
	}

	rs := b.rates[code]

	i, found := slices.BinarySearchFunc(rs, on, func(r Rate, d time.Time) int { return r.Date.Compare(d) })
	if found {
		return c, rs[i], nil
	}

	if i == 0 {
		return Currency{}, Rate{}, fmt.Errorf("%w: %s on %s", ErrNoRate, code, on.Format(time.DateOnly))
	}

	return c, rs[i-1], nil
}

// EUR's implicit rate has no date, so the other rate dates the conversion.
func olderRateDate(a, b Rate) time.Time {
	switch {
	case a.Date.IsZero():
		return b.Date
	case b.Date.IsZero() || a.Date.Before(b.Date):
		return a.Date
	default:
		return b.Date
	}
}

func pow10(n int) *big.Rat {
	p := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(max(n, -n))), nil)
	if n < 0 {
		return new(big.Rat).SetFrac(big.NewInt(1), p)
	}

	return new(big.Rat).SetInt(p)
}

// From ISO 4217 list one, published 2026-09-17. Codes it does not list, and XDR (minor unit "N.A."), get 2.
func minorUnits(code string) int {
	switch code {
	case "BIF", "CLP", "DJF", "GNF", "ISK", "JPY", "KMF", "KRW", "PYG", "RWF", "UGX", "UYI", "VND", "VUV", "XAF", "XOF", "XPF":
		return 0
	case "BHD", "IQD", "JOD", "KWD", "LYD", "OMR", "TND":
		return 3
	case "CLF", "UYW":
		return 4
	default:
		return 2
	}
}

// A dateRange is the calendar days from through to, both included.
type dateRange struct {
	from, to time.Time
}

func (r dateRange) String() string {
	return r.from.Format(time.DateOnly) + " to " + r.to.Format(time.DateOnly)
}

func (r dateRange) chunks(maxDays int) []dateRange {
	var out []dateRange

	for from := r.from; !from.After(r.to); from = from.AddDate(0, 0, maxDays) {
		to := from.AddDate(0, 0, maxDays-1)
		if to.After(r.to) {
			to = r.to
		}

		out = append(out, dateRange{from: from, to: to})
	}

	return out
}

// A stored rate covers the days after it up to lookbackDays, past any weekend or holiday (the longest ECB closure is 4 days).
const lookbackDays = 7

// missing returns the runs of days in [from, to] that no rate in have covers, each starting lookbackDays early so the fetch also brings the rate before it.
func missing(have []time.Time, from, to time.Time) []dateRange {
	stored := make(map[time.Time]bool, len(have))
	for _, d := range have {
		stored[civil(d)] = true
	}

	var (
		out    []dateRange
		latest time.Time
	)

	for d := from.AddDate(0, 0, -lookbackDays); !d.After(to); d = d.AddDate(0, 0, 1) {
		if stored[d] {
			latest = d
		}

		switch n := len(out); {
		case d.Before(from), !latest.IsZero() && !d.After(latest.AddDate(0, 0, lookbackDays)):
		case n > 0 && out[n-1].to.Equal(d.AddDate(0, 0, -1)):
			out[n-1].to = d
		default:
			out = append(out, dateRange{from: d.AddDate(0, 0, -lookbackDays), to: d})
		}
	}

	return out
}

// civil drops the time of day: rates and entry dates are calendar days.
func civil(t time.Time) time.Time {
	y, m, d := t.Date()

	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// Round rounds a full-precision amount in minor units half away from zero; only a displayed figure is rounded (#10).
func Round(minor *big.Rat) (int64, error) {
	q, r := new(big.Int).QuoRem(minor.Num(), minor.Denom(), new(big.Int))

	if twice := new(big.Int).Lsh(r.Abs(r), 1); twice.Cmp(minor.Denom()) >= 0 {
		q.Add(q, big.NewInt(int64(minor.Sign())))
	}

	if !q.IsInt64() {
		return 0, ErrOutOfRange
	}

	return q.Int64(), nil
}
