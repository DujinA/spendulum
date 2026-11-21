package fx

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/DujinA/spendulum/backend/internal/platform/db/gen"
)

type Postgres struct {
	q *gen.Queries
}

func NewPostgres(q *gen.Queries) *Postgres {
	return &Postgres{q: q}
}

func (p *Postgres) SaveCurrencies(ctx context.Context, cs []Currency) error {
	arg := gen.SaveCurrenciesParams{Codes: make([]string, len(cs)), MinorUnits: make([]int16, len(cs))}
	for i, c := range cs {
		arg.Codes[i] = c.Code
		arg.MinorUnits[i] = int16(c.MinorUnits) //nolint:gosec // ISO 4217 minor units are 0 to 4
	}

	if err := p.q.SaveCurrencies(ctx, arg); err != nil {
		return fmt.Errorf("fx: upsert currencies: %w", err)
	}

	return nil
}

func (p *Postgres) Currencies(ctx context.Context) ([]Currency, error) {
	rows, err := p.q.Currencies(ctx)
	if err != nil {
		return nil, fmt.Errorf("fx: select currencies: %w", err)
	}

	out := make([]Currency, len(rows))
	for i, r := range rows {
		out[i] = Currency{Code: r.Code, MinorUnits: int(r.MinorUnits)}
	}

	return out, nil
}

func (p *Postgres) SaveRates(ctx context.Context, rates []Rate) error {
	arg := gen.SaveRatesParams{
		Currencies: make([]string, len(rates)),
		RateDates:  make([]time.Time, len(rates)),
		PerEur:     make([]string, len(rates)),
	}

	for i, r := range rates {
		perEUR, err := decimalText(r.PerEUR)
		if err != nil {
			return err
		}

		arg.Currencies[i], arg.RateDates[i], arg.PerEur[i] = r.Currency, civil(r.Date), perEUR
	}

	if err := p.q.SaveRates(ctx, arg); err != nil {
		return fmt.Errorf("fx: upsert rates: %w", err)
	}

	return nil
}

func (p *Postgres) LatestRates(ctx context.Context, currencies []string, on time.Time) ([]Rate, error) {
	rows, err := p.q.LatestRates(ctx, gen.LatestRatesParams{Currencies: currencies, OnDate: civil(on)})
	if err != nil {
		return nil, fmt.Errorf("fx: select latest rates: %w", err)
	}

	out := make([]Rate, len(rows))

	for i, r := range rows {
		perEUR, ok := new(big.Rat).SetString(r.PerEur)
		if !ok {
			return nil, fmt.Errorf("fx: stored rate %q of %s is not a number", r.PerEur, r.Currency)
		}

		out[i] = Rate{Currency: r.Currency, Date: r.RateDate, PerEUR: perEUR}
	}

	return out, nil
}

func (p *Postgres) LastRateDate(ctx context.Context) (time.Time, bool, error) {
	d, err := p.q.LastRateDate(ctx)

	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return time.Time{}, false, nil
	case err != nil:
		return time.Time{}, false, fmt.Errorf("fx: select last rate date: %w", err)
	}

	return d, true, nil
}

func (p *Postgres) RateDates(ctx context.Context, from, to time.Time) ([]time.Time, error) {
	dates, err := p.q.RateDates(ctx, gen.RateDatesParams{FromDate: civil(from), ToDate: civil(to)})
	if err != nil {
		return nil, fmt.Errorf("fx: select rate dates: %w", err)
	}

	return dates, nil
}

// decimalText writes a rate without losing a digit; rates arrive as decimals, so their denominator is 2^a·5^b.
func decimalText(r *big.Rat) (string, error) {
	d := new(big.Int).Set(r.Denom())
	twos := d.TrailingZeroBits()
	d.Rsh(d, twos)

	fives := uint(0)
	five, rem := big.NewInt(5), new(big.Int)

	for {
		q, m := new(big.Int).QuoRem(d, five, rem)
		if m.Sign() != 0 {
			break
		}

		d, fives = q, fives+1
	}

	if d.Cmp(big.NewInt(1)) != 0 {
		return "", fmt.Errorf("fx: rate %s is not a finite decimal", r.RatString())
	}

	return r.FloatString(int(max(twos, fives))), nil
}
