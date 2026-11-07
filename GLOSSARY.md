# Spendulum

A personal budget tracker: each user records their own expenses and income, sets targets per category, and sees how actual spending compares to them.

## People

**User**:
A person who signs in and owns their own entries, vendors, categories, budget targets and settings.
_Avoid_: Tenant, member, customer

**Account**:
A user's sign-in identity together with all the data they own; the unit that is exported or deleted.
_Avoid_: Profile, tenant

**Operator**:
The person who runs the instance and its command-line tools; never a role inside the app.
_Avoid_: Admin, superuser

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

**Import**:
A one-off load of one user's past entries, categories and budget targets from another tool's export; it can be undone as a whole.
_Avoid_: Upload, migration, sync

## Budgets and reports

**Budget target**:
A monthly amount, in its own currency, that one category is expected to stay within (expense) or reach (income), from a start month until it is changed or ended.
_Avoid_: Budget, limit, goal, allowance

**Period**:
The date range a report covers: a preset or a custom range.
_Avoid_: Timeframe, window

**Completed month**:
A calendar month that has fully passed in the user's time zone.
_Avoid_: Closed month, past month

**Monthly average**:
A period's total over its completed months, divided by the number of those months.
_Avoid_: Mean, run rate

**Variance**:
Actual minus target, as an amount and as a percent of the target.
_Avoid_: Deviation, delta, difference

**Untargeted spending**:
Expenses in categories that have no budget target for that month.
_Avoid_: Unbudgeted, other spending

**Savings**:
Income minus expenses over a period.
_Avoid_: Net, profit, surplus

**Savings rate**:
Savings divided by income over a period.
_Avoid_: Savings ratio, margin
