import type { components } from "@/api/schema.gen";

export type Currency = components["schemas"]["Currency"];

// Before the first rate sync the list is empty, so a missing code takes 2, the most common minor units.
export function findCurrency(currencies: readonly Currency[], code: string): Currency {
  return currencies.find((c) => c.code === code) ?? { code, minorUnits: 2 };
}

const symbols: Partial<Record<string, string>> = { RSD: "din" };

const maxIntegerDigits = 12;

export function currencyLabel(currency: string) {
  return symbols[currency] ?? currency;
}

function groupThousands(digits: string) {
  return digits.replace(/\B(?=(\d{3})+(?!\d))/g, ",");
}

// Amounts are integer minor units, so this works on digits, never on floats: 178080 in RSD is "1780" and "80".
function splitDigits(amount: number, units: number) {
  const digits = String(Math.abs(amount)).padStart(units + 1, "0");
  return { whole: digits.slice(0, digits.length - units), fraction: digits.slice(digits.length - units) };
}

export function formatMoney(amount: number, currency: Currency) {
  const { whole, fraction } = splitDigits(amount, currency.minorUnits);
  const decimals = currency.minorUnits > 0 ? `.${fraction}` : "";
  return `${amount < 0 ? "−" : ""}${groupThousands(whole)}${decimals} ${currencyLabel(currency.code)}`;
}

export function isAmountKey(key: string) {
  return /^[0-9.,]$/.test(key) || key === "Backspace";
}

// The amount as typed on the keypad: digits, then at most one "." and the currency's minor units.
export function pressKey(typed: string, key: string, currency: Currency) {
  if (key === "Backspace") return typed.slice(0, -1);

  const units = currency.minorUnits;
  const [whole = "", fraction] = typed.split(".");

  if (key === "." || key === ",") {
    if (fraction !== undefined || units === 0) return typed;
    return `${whole || "0"}.`;
  }

  if (!/^[0-9]$/.test(key)) return typed;
  if (fraction !== undefined) return fraction.length < units ? typed + key : typed;
  if (whole === "0") return key;
  return whole.length < maxIntegerDigits ? typed + key : typed;
}

export function toMinorUnits(typed: string, currency: Currency) {
  const units = currency.minorUnits;
  const [whole = "", fraction = ""] = typed.split(".");
  return Number(`${whole}${fraction.padEnd(units, "0")}`);
}

export function formatTyped(typed: string) {
  const [whole = "", fraction] = typed.split(".");
  const grouped = groupThousands(whole || "0");
  return fraction === undefined ? grouped : `${grouped}.${fraction}`;
}

// Every minor digit, so the keypad and the edit form start from the stored amount: 178080 in RSD is "1780.80".
export function toTyped(amount: number, currency: Currency) {
  const { whole, fraction } = splitDigits(amount, currency.minorUnits);
  return currency.minorUnits > 0 ? `${whole}.${fraction}` : whole;
}

// Minor units of a typed amount, with "." or "," before at most the currency's minor digits; undefined when it is not one.
const typedAmount = new RegExp(`^(\\d{1,${String(maxIntegerDigits)}})(?:[.,](\\d*))?$`);

export function parseAmount(text: string, currency: Currency) {
  const match = typedAmount.exec(text.trim());
  if (!match) return undefined;
  const [, whole = "", fraction = ""] = match;
  if (fraction.length > currency.minorUnits) return undefined;
  return Number(whole + fraction.padEnd(currency.minorUnits, "0"));
}
