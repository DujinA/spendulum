package fx

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"time"
)

// Frankfurter is the RateProvider for the Frankfurter v2 API and its blended rates (#5).
type Frankfurter struct {
	baseURL string
	client  *http.Client
}

func NewFrankfurter(baseURL string, client *http.Client) *Frankfurter {
	return &Frankfurter{baseURL: baseURL, client: client}
}

func (f *Frankfurter) Currencies(ctx context.Context) ([]string, error) {
	var rows []struct {
		ISOCode string `json:"iso_code"`
		Type    string `json:"type"`
	}

	if err := f.get(ctx, "currencies", nil, &rows); err != nil {
		return nil, err
	}

	var codes []string

	for _, r := range rows {
		if r.Type == "currency" {
			codes = append(codes, r.ISOCode)
		}
	}

	return codes, nil
}

func (f *Frankfurter) Rates(ctx context.Context, from, to time.Time) ([]Rate, error) {
	var rows []struct {
		Date  string      `json:"date"`
		Base  string      `json:"base"`
		Quote string      `json:"quote"`
		Rate  json.Number `json:"rate"`
	}

	query := url.Values{"from": {from.Format(time.DateOnly)}, "to": {to.Format(time.DateOnly)}, "base": {EUR}}
	if err := f.get(ctx, "rates", query, &rows); err != nil {
		return nil, err
	}

	rates := make([]Rate, len(rows))

	for i, r := range rows {
		date, err := time.Parse(time.DateOnly, r.Date)
		if err != nil {
			return nil, fmt.Errorf("fx: frankfurter: date of %s: %w", r.Quote, err)
		}

		perEUR, ok := new(big.Rat).SetString(r.Rate.String())
		if r.Base != EUR || !ok || perEUR.Sign() <= 0 {
			return nil, fmt.Errorf("fx: frankfurter: %s on %s is not a positive rate against EUR: %s %s", r.Quote, r.Date, r.Base, r.Rate)
		}

		rates[i] = Rate{Currency: r.Quote, Date: date, PerEUR: perEUR}
	}

	return rates, nil
}

func (f *Frankfurter) get(ctx context.Context, path string, query url.Values, into any) error {
	u, err := url.JoinPath(f.baseURL, path)
	if err != nil {
		return fmt.Errorf("fx: frankfurter URL: %w", err)
	}

	if query != nil {
		u += "?" + query.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return fmt.Errorf("fx: frankfurter request: %w", err)
	}

	res, err := f.client.Do(req)
	if err != nil {
		return fmt.Errorf("fx: frankfurter GET %s: %w", path, err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		var problem struct {
			Message string `json:"message"`
		}

		_ = json.NewDecoder(io.LimitReader(res.Body, 4096)).Decode(&problem)

		return fmt.Errorf("fx: frankfurter GET %s: %s: %s", path, res.Status, problem.Message)
	}

	if err := json.NewDecoder(res.Body).Decode(into); err != nil {
		return fmt.Errorf("fx: frankfurter GET %s: decode: %w", path, err)
	}

	return nil
}
