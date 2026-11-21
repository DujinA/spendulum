// Until the currencies table (#35) gives each currency its minor units, every one has RSD's 2.
const minorUnitsByCurrency: Partial<Record<string, number>> = {};

function minorUnits(currency: string) {
  return minorUnitsByCurrency[currency] ?? 2;
}

const symbols: Partial<Record<string, string>> = { RSD: "din" };

const maxIntegerDigits = 12;

export function currencyLabel(currency: string) {
  return symbols[currency] ?? currency;
}

function groupThousands(digits: string) {
  return digits.replace(/\B(?=(\d{3})+(?!\d))/g, ",");
}

// Amounts are integer minor units, so this works on digits, never on floats.
export function formatMoney(amount: number, currency: string) {
  const units = minorUnits(currency);
  const digits = String(Math.abs(amount)).padStart(units + 1, "0");
  const whole = groupThousands(digits.slice(0, digits.length - units));
  const fraction = units > 0 ? `.${digits.slice(digits.length - units)}` : "";
  return `${amount < 0 ? "−" : ""}${whole}${fraction} ${currencyLabel(currency)}`;
}

export function isAmountKey(key: string) {
  return /^[0-9.,]$/.test(key) || key === "Backspace";
}

// The amount as typed on the keypad: digits, then at most one "." and the currency's minor units.
export function pressKey(typed: string, key: string, currency: string) {
  if (key === "Backspace") return typed.slice(0, -1);

  const units = minorUnits(currency);
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

export function toMinorUnits(typed: string, currency: string) {
  const units = minorUnits(currency);
  const [whole = "", fraction = ""] = typed.split(".");
  return Number(`${whole}${fraction.padEnd(units, "0")}`);
}

export function formatTyped(typed: string) {
  const [whole = "", fraction] = typed.split(".");
  const grouped = groupThousands(whole || "0");
  return fraction === undefined ? grouped : `${grouped}.${fraction}`;
}
