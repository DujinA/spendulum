# Rate-provider stub

Stands in for Frankfurter v2 in end-to-end tests, so the app reaches no network. `compose.e2e.yaml`
serves this directory as `http://rates:8080` with busybox `httpd`, and sets the app's
`SPENDULUM_RATE_PROVIDER_URL` to `http://rates:8080/v2`.

- `v2/currencies` answers `GET /v2/currencies`: a few fiat currencies and one crypto one, which the
  app ignores.
- `v2/rates` answers `GET /v2/rates`: fixed fake rates against EUR for 2026-01-02 and 2026-01-05.
  `httpd` ignores the query string, so every date range gets these same rows.

The files follow Frankfurter's JSON shape. `TestTheE2EStubServesFixedRatesTheFrankfurterAdapterReads`
in `backend/internal/fx` reads them through the real adapter.
