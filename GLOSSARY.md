# Spendulum

A personal budget tracker for a single user: it records expenses and income, sets targets per category, and reports how actual spending compares to them.

## Money

**Primary currency**:
The one currency every amount, total and chart is shown in by default, and the currency new expenses and income start in.
_Avoid_: Base currency, home currency, default currency

**Conversion**:
An optional view setting that shows every amount in one chosen currency, converting each amount at the exchange rate of the day it happened.
_Avoid_: Display currency, FX mode

**Expense**:
Money the user spent: a dated amount in one currency, in exactly one expense category.
_Avoid_: Spending, cost, outflow

**Income**:
Money the user received: a dated amount in one currency, in exactly one income category.
_Avoid_: Earnings, inflow, revenue

**Refund**:
An expense with a negative amount, recording money returned for something spent; it lowers its category's total.
_Avoid_: Return, reimbursement, payback, negative income
