-- Currencies and exchange rates are shared by every user, so they have no user_id (ADR 0009).

-- +goose Up
CREATE TABLE currencies (
    code char(3) PRIMARY KEY,
    minor_units smallint NOT NULL CHECK (minor_units BETWEEN 0 AND 4)
);

-- Units of the currency that one euro buys on rate_date (ADR 0007); EUR itself is always 1 and is not stored.
CREATE TABLE exchange_rates (
    currency char(3) NOT NULL REFERENCES currencies (code),
    rate_date date NOT NULL,
    per_eur numeric NOT NULL CHECK (per_eur > 0),
    PRIMARY KEY (currency, rate_date)
);

CREATE INDEX exchange_rates_rate_date_idx ON exchange_rates (rate_date);
