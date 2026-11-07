# Spendulum

A personal budget tracker for a single user: it records expenses and income, sets targets per category, and reports how actual spending compares to them.

## Money

**Primary currency**:
The user's main currency: new entries start in it, and totals, charts and budget progress are shown in it unless another display currency is chosen.
_Avoid_: Base currency, home currency, default currency

**Display currency**:
The currency totals, charts and budget progress are currently shown in; the primary currency unless the user switches it.
_Avoid_: Conversion currency, reporting currency, view currency

**Conversion**:
Turning an amount into another currency at the exchange rate of its entry date, never today's rate.
_Avoid_: FX mode, repricing

**Provisional**:
A converted amount that uses an earlier day's rate because its own date's rate is not available yet.
_Avoid_: Estimated, approximate

**Expense**:
Money the user spent: a dated amount in one currency, in exactly one expense category.
_Avoid_: Spending, cost, outflow

**Income**:
Money the user received: a dated amount in one currency, in exactly one income category.
_Avoid_: Earnings, inflow, revenue

**Refund**:
An expense with a negative amount, recording money returned for something spent; it lowers its category's total.
_Avoid_: Return, reimbursement, payback, negative income

## Entries

**Entry**:
An expense or an income: the one thing the user records.
_Avoid_: Transaction, record, item

**Entry date**:
The calendar day an entry happened, in the user's time zone, with no time of day.
_Avoid_: Timestamp, transaction date

**Vendor**:
Who an expense was paid to or an income came from; optional on an entry, and labelled "Source" on income.
_Avoid_: Merchant, payee, shop, payer

**Category**:
A named group of one kind, expense or income; every entry belongs to exactly one category of its own kind.
_Avoid_: Tag, bucket, group

**Archived**:
A vendor or category that is hidden from pickers but stays on the entries that already use it.
_Avoid_: Disabled, hidden, inactive

**Merge**:
Folding one vendor into another: the survivor takes all of its entries and the merged vendor stops existing.
_Avoid_: Combine, dedupe

**Deleted entry**:
An entry the user removed; it no longer appears in any list or report.
_Avoid_: Trashed, voided
