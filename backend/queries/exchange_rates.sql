-- name: SaveCurrencies :exec
INSERT INTO currencies (code, minor_units)
SELECT unnest(@codes::text[]), unnest(@minor_units::smallint[])
ON CONFLICT (code) DO UPDATE SET minor_units = excluded.minor_units;

-- name: Currencies :many
SELECT code, minor_units FROM currencies ORDER BY code;

-- Rates travel as text so that numeric keeps every digit.
-- name: SaveRates :exec
INSERT INTO exchange_rates (currency, rate_date, per_eur)
SELECT unnest(@currencies::text[]), unnest(@rate_dates::date[]), unnest(@per_eur::text[])::numeric
ON CONFLICT (currency, rate_date) DO UPDATE SET per_eur = excluded.per_eur;

-- name: LatestRates :many
SELECT DISTINCT ON (currency) currency, rate_date, per_eur::text AS per_eur
FROM exchange_rates
WHERE currency = ANY(CAST(@currencies::text[] AS bpchar[])) AND rate_date <= @on_date::date
ORDER BY currency, rate_date DESC;

-- name: LastRateDate :one
SELECT max(rate_date)::date AS rate_date FROM exchange_rates;

-- name: RateDates :many
SELECT DISTINCT rate_date FROM exchange_rates
WHERE rate_date BETWEEN @from_date::date AND @to_date::date
ORDER BY rate_date;
