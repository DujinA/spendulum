# Exchange-rate provider for the first `RateProvider` adapter

Research for issue #5. Checked on 2026-11-06 against each provider's own docs, pricing pages and terms, and by calling the free APIs directly. Prices and limits can change, so re-check the linked pages before you rely on a number.

## Question

Which provider should the first `RateProvider` adapter use? It needs RSD, USD and EUR (ideally 150+ currencies), daily history back to at least September 2023, a free or cheap tier that suits lazy fetching (a handful of requests a month, one date at a time), terms that allow personal use and storing fetched rates, and a stable API.

## Recommendation

**Use Frankfurter v2 (`https://api.frankfurter.dev/v2`) with its default blended rates.**

- It needs no API key, has no monthly or daily quota, and covers RSD, USD and EUR plus about 160 other fiat currencies, with RSD history from 2000.
- Its FAQ says outright that storing rates in your own database is allowed and encouraged, and that blended rates are free to use, commercial use included.
- Its code is MIT-licensed and can be self-hosted, so if the public service ever goes away, the adapter only needs a new base URL.

Suggested adapter call, one request per missing date:

```
GET https://api.frankfurter.dev/v2/rates?date=2023-09-01&base=RSD&quotes=USD,EUR
-> [{"date":"2023-09-01","base":"RSD","quote":"EUR","rate":0.0085384},
    {"date":"2023-09-01","base":"RSD","quote":"USD","rate":0.0092706}]
```

Leave out `quotes` to get every currency for that date in a single request (197 rows on 2023-09-01, crypto and metals included).

If you later want the official National Bank of Serbia middle rate, add `&providers=nbs` to the same call. Read the NBS terms note below first.

### Base-currency behaviour

`base` can be any supported currency on every request, with no paid tier needed. If `base` is left out, the response uses a default base that the docs don't name, so always send `base` explicitly. RSD is a sensible base because it is Spendulum's primary currency. If you need a cross rate, such as EUR to USD, you can request it directly with `base=EUR`, or work it out from two RSD rates.

### Weekend and holiday behaviour

Tested against the live API:

| Requested date | Blended (default) returns | `providers=nbs` returns |
|---|---|---|
| 2023-09-02 (Sat) | `date: 2023-09-02` | — |
| 2023-09-03 (Sun) | `date: 2023-09-03` | `date: 2023-09-01` (previous Friday) |
| 2024-01-01 (New Year) | `date: 2024-01-01` | `date: 2023-12-29` |
| 2024-01-07 (Orthodox Christmas, Sun) | `date: 2024-01-07` | `date: 2024-01-05` |
| 2024-05-03 (Orthodox Good Friday) | `date: 2024-05-03` | `date: 2024-04-30` |
| 2026-12-25 (future) | `[]` (empty array) | — |

- **Blended rates usually return a row for the exact date asked for, weekends and holidays included.** Some of the source central banks publish on those days, and the blend uses each provider's latest observation (the `expand=providers` output shows contributors with earlier dates).
- **A single provider such as NBS returns its latest earlier publication** and sets `date` to that publication date. This already matches Spendulum's rule of using the latest earlier rate.
- **A date with no data yet returns an empty array**, not an error. The adapter should treat that as "not available yet" and should not store anything.
- **An unknown currency returns HTTP 422** with a body like `{"status":422,"message":"invalid currency: XYZ"}`.

For the adapter: store the rate under the date you asked for, and also keep the `date` the API returned, so you can tell when a rate was carried forward. Keep Spendulum's own "latest earlier rate" fallback in the domain for the empty-array case. Don't rely on the provider for it.

## Comparison

| Provider | RSD | Currencies | History back to Sept 2023 | Free tier | Key | Base on free tier | Storing rates | Verdict |
|---|---|---|---|---|---|---|---|---|
| **Frankfurter v2** | Yes (from 2000-01-03) | ~160 fiat (197 rows incl. crypto/metals) | Yes (back to 1948 overall) | No quotas; abuse rate limit only | No | Any | Explicitly allowed | **Recommended** |
| Frankfurter v1 (ECB only) | **No** | ~30 | Yes | No quotas | No | Any | Allowed | Rejected: no RSD |
| Open Exchange Rates | Yes | 174 | Yes (from 1999) | 1,000 req/month | Yes | **USD only** | Not addressed; website terms limit consumers to domestic and private use | Viable fallback |
| ExchangeRate-API | Yes (full list from 2021) | 160+ | **Paid plans only** | 1,500 req/month, no history | Yes | — | Allowed for your own use | Rejected: history needs Pro ($10/month) |
| exchangerate.host (APILayer) | Not verified | ~168 | Listed as included on the free plan | **100 req/month**, non-commercial | Yes | **Source switching only on paid plans** | Not checked | Rejected: tight limit, base locked |
| fawazahmed0 currency-api | Yes | 200+ (701 keys incl. crypto) | **No: dated data starts 2024-03-02** | No limits (CDN) | No | Any | CC0 (public domain) | Rejected: no 2023 history |
| National Bank of Serbia (direct) | Yes (pivot currency) | 29 | Yes (from 2002) | Free | **Registration: SOAP auth header needs UserName, Password, LicenceID** | RSD | **Written consent needed to save/reproduce** | Later adapter, with care |

### Frankfurter

- v2 blends rates from many central banks and official sources ("108 central banks and official sources"). The `/v2/currencies` endpoint lists RSD with `start_date` 2000-01-03, and 161 entries have `type: "currency"`. Source: https://frankfurter.dev/ and https://api.frankfurter.dev/v2/currencies
- 22 of its providers publish RSD, NBS among them (`pivot_currency: RSD`, `rate_type: "official middle rate"`, `start_date: 2002-05-15`). Source: https://api.frankfurter.dev/v2/providers
- Limits: "There are no quotas. Requests are rate-limited to prevent abuse, but there are no monthly or daily caps." Source: https://frankfurter.dev/docs/
- Storing: "Can I store or cache rates in my own database? Yes. Storing daily observations locally for caching or historical analytics is encouraged to keep API traffic light." Source: https://frankfurter.dev/ (FAQ)
- Terms: "Blended rates are Frankfurter's own calculation and free to use. Direct queries to individual provider endpoints remain subject to each provider's terms." Source: https://frankfurter.dev/ (FAQ) and https://frankfurter.dev/license/
- Licence: code, docs and Docker images are MIT and can be self-hosted. Source: https://frankfurter.dev/license/ and https://github.com/lineofflight/frankfurter (MIT, actively maintained, last push 2026-11-06)
- Stability: "The v1 API is deprecated in favor of v2, but remains available indefinitely." Build against v2. Source: https://frankfurter.dev/docs/
- v1 is ECB only and has no RSD. `GET /v1/currencies` has no RSD, and `GET /v1/2023-09-02?base=EUR&symbols=USD,RSD` returns only USD (dated 2023-09-01).
- Accuracy: blended rates "may shift as new data comes in" in the last decimal places. For compliance-grade figures, the FAQ suggests filtering to a single provider. Source: https://frankfurter.dev/ (FAQ)
- Responses carry `cache-control: public, max-age=86400`.

### Open Exchange Rates

- Free plan: up to 1,000 requests/month, hourly updates, USD base only. Paid plans start at Developer, $12/month (10,000 requests, any base). Source: https://openexchangerates.org/signup
- "All accounts have access to the Open Exchange Rates API, with live and historical rates for over 200 currencies." Source: https://openexchangerates.org/signup
- `historical/*.json` goes back to 1999-01-01. A historical rate is the last value published for that UTC day. Changing `base` and `symbols` needs Developer or higher. Source: https://docs.openexchangerates.org/reference/historical-json
- RSD is supported (174 currencies). Source: https://docs.openexchangerates.org/reference/supported-currencies
- Terms: the old licence was replaced by website terms. These grant a "non-transferable, non-exclusive, revocable, limited right to use", say that for consumers "we only provide our site for domestic and private use", and don't directly address storing data. Source: https://openexchangerates.org/license and https://openexchangerates.org/terms
- It works for personal use, but it needs an API key and always uses a USD base, and its terms on storing rates are unclear. It is the best fallback if Frankfurter disappears.

### ExchangeRate-API

- Free plan: 1.5K requests/month, daily updates. Pro is $10/month for 30K requests. Source: https://www.exchangerate-api.com/
- Historical data is "only available to users on our Pro, Business or Volume plans". The full currency list is available from 2021-01-01, and RSD is not in the smaller 1990-2020 list. Source: https://www.exchangerate-api.com/docs/historical-data-requests
- Terms: "Users are given permission to store & re-use any data retrieved from our API." Free plans "can be discontinued by ExchangeRate-API without notice". Redistribution is not allowed. Source: https://www.exchangerate-api.com/terms
- Rejected because historical rates need a paid plan.

### exchangerate.host

- Now run by APILayer. Free plan: "100 Requests / mo", "Daily Updates", "Historical Rates", "Non-Commercial Use". Source currency switching is listed only on the paid plans, which start at $14.99/month for 10,000 requests. Source: https://exchangerate.host/product and https://exchangerate.host/
- The API documentation sits behind an APILayer login (https://docs.apilayer.com/exchangerate/docs/api-documentation), so RSD coverage and how far back history goes were not checked against a primary source.
- Rejected: the limit is very low, the base is locked on the free plan, and the documentation is behind a login.

### fawazahmed0 currency-api

- URL: `https://cdn.jsdelivr.net/npm/@fawazahmed0/currency-api@{date}/v1/currencies/{code}.json`, with fallback `https://{date}.currency-api.pages.dev/v1/...`. "No Rate limits", CC0-1.0 licence, "200+ Currencies". The README doesn't name its upstream sources. Source: https://github.com/fawazahmed0/exchange-api
- Checked directly: `@2024-03-01` returns 404 and `@2024-03-02` returns 200, on both jsDelivr and pages.dev. The old `gh/fawazahmed0/currency-api@1/...` URLs also return 404 for 2023 dates. See https://github.com/fawazahmed0/exchange-api/blob/main/MIGRATION.md
- Rejected: no history before March 2024, and the data sources are not stated.

### National Bank of Serbia

- The official RSD middle rate is published every business day. Public web app: https://webappcenter.nbs.rs/ExchangeRateWebApp/ExchangeRate/CurrentMiddleRate
- The machine API is a SOAP 1.1 service. Operations include `GetExchangeRateByDate`, `GetExchangeRateByCurrency` and `GetExchangeRateRsdEurByPeriod`. Every operation needs an `AuthenticationHeader` with `UserName`, `Password` and `LicenceID`, so you must register. Source: https://webservices.nbs.rs/CommunicationOfficeService1_0/ExchangeRateService.asmx
- It covers 29 currencies, which is enough for RSD, USD and EUR but well short of 150. Source: https://api.frankfurter.dev/v2/providers (NBS entry)
- Terms: "Files published on the NBS website may be saved, reproduced, distributed and used for commercial purposes only subject to prior written consent of the NBS." Source: https://www.nbs.rs/en/copyright/index.html. This wording can be read as requiring consent even to save the data. For a single-user personal tool the practical risk is small, but it is not the explicit permission that Frankfurter's blended rates have. Frankfurter's `providers=nbs` route also falls under these terms.
- As a later adapter it would give authoritative Serbian rates, but it means SOAP, registration and unclear storage terms. Frankfurter's `providers=nbs` gives you the same numbers through a simple REST call if you ever want them.

## Open points

- RSD coverage on exchangerate.host is unverified because its docs need a login. It is rejected on other grounds, so this doesn't change the recommendation.
- Frankfurter's abuse rate limit has no published number. Lazy per-date fetching is far below any plausible limit.
