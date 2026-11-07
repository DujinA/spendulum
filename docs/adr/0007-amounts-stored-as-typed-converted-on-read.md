# Amounts are stored as typed and converted on read at their own date's rate

Every expense and income stores only the amount the user typed (integer minor units) and its ISO currency; no converted amount is ever stored. Totals, charts, budget progress and exports convert on read into the display currency, each amount at the exchange rate of its own entry date, and the display currency is chosen while viewing (a "Show in" switcher, defaulting to the primary currency, remembered per device) rather than fixed in settings.

Storing a converted amount would freeze one reporting currency into the data: changing the primary currency (for example, moving from RSD to EUR) or viewing in USD would mean rewriting history or keeping a second converted column per currency. Converting at the entry date's rate, unlike the old sheet's today's-rate formula, keeps past months stable. Entry currencies are limited to those the rate provider supports, so every amount can always be converted.

## Consequences

- When a date's rate is not yet available, the latest earlier rate is used and the converted figure is marked provisional.
- Views in different display currencies are not proportional to one another, because each amount uses its own date's rate.
- Budget targets keep their own currency and convert like entries; which rate a monthly target uses is a report rule.
