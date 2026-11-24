import { findCurrency, formatMoney, formatTyped, parseAmount, pressKey, toMinorUnits, toTyped } from "@/lib/money";

const rsd = { code: "RSD", minorUnits: 2 };
const jpy = { code: "JPY", minorUnits: 0 };
const kwd = { code: "KWD", minorUnits: 3 };

function typeKeys(keys: string, currency = rsd) {
  let typed = "";
  for (const key of keys) typed = pressKey(typed, key === "<" ? "Backspace" : key, currency);
  return typed;
}

describe("formatMoney", () => {
  it.each([
    [178080, rsd, "1,780.80 din"],
    [-199000, rsd, "−1,990.00 din"],
    [5, rsd, "0.05 din"],
    [123456789012, rsd, "1,234,567,890.12 din"],
    [1050, { code: "EUR", minorUnits: 2 }, "10.50 EUR"],
    [1780, jpy, "1,780 JPY"],
    [-5, jpy, "−5 JPY"],
    [1780800, kwd, "1,780.800 KWD"],
    [5, kwd, "0.005 KWD"],
  ])("formats %i %o as %s", (amount, currency, want) => {
    expect(formatMoney(amount, currency)).toBe(want);
  });
});

describe("keypad amount", () => {
  it.each([
    ["1780.80", "1780.80", 178080],
    ["1780,8", "1780.8", 178080],
    ["0012", "12", 1200],
    [".5", "0.5", 50],
    ["1.234", "1.23", 123],
    ["1..2", "1.2", 120],
    ["12<<3", "3", 300],
    ["1234567890123", "123456789012", 12345678901200],
    ["", "", 0],
  ])("typing %s in RSD shows %s and is %i minor units", (keys, want, minor) => {
    const typed = typeKeys(keys);

    expect(typed).toBe(want);
    expect(toMinorUnits(typed, rsd)).toBe(minor);
  });

  it.each([
    [jpy, "12.5", "125", 125],
    [jpy, "1,780", "1780", 1780],
    [kwd, "1.2345", "1.234", 1234],
    [kwd, "1.2", "1.2", 1200],
    [kwd, ".05", "0.05", 50],
  ])("typing in %o: %s shows %s and is %i minor units", (currency, keys, want, minor) => {
    const typed = typeKeys(keys, currency);

    expect(typed).toBe(want);
    expect(toMinorUnits(typed, currency)).toBe(minor);
  });

  it("groups thousands while typing and shows 0 when empty", () => {
    expect(formatTyped("1780.8")).toBe("1,780.8");
    expect(formatTyped("1234567")).toBe("1,234,567");
    expect(formatTyped("")).toBe("0");
  });
});

describe("findCurrency", () => {
  it("takes the minor units from the currencies list, and 2 for a code the list lacks", () => {
    expect(findCurrency([rsd, jpy, kwd], "KWD")).toEqual(kwd);
    expect(findCurrency([rsd], "JPY")).toEqual({ code: "JPY", minorUnits: 2 });
  });
});

describe("toTyped", () => {
  it.each([
    [178080, rsd, "1780.80"],
    [-199000, rsd, "1990.00"],
    [5, rsd, "0.05"],
    [1780, jpy, "1780"],
    [5, kwd, "0.005"],
  ])("writes %i %o as %s", (amount, currency, want) => {
    expect(toTyped(amount, currency)).toBe(want);
  });
});

describe("parseAmount", () => {
  it.each([
    ["1780.80", rsd, 178080],
    ["1780,8", rsd, 178080],
    [" 1990 ", rsd, 199000],
    ["1990.", rsd, 199000],
    ["0", rsd, 0],
    ["1780", jpy, 1780],
    ["1.234", kwd, 1234],
  ])("reads %j in %o as %i", (text, currency, want) => {
    expect(parseAmount(text, currency)).toBe(want);
  });

  it.each([
    ["", rsd],
    ["12.345", rsd],
    ["1.5", jpy],
    ["-5", rsd],
    ["1,780.80", rsd],
    [".5", rsd],
    ["1e3", rsd],
    ["1234567890123", rsd],
  ])("refuses %j in %o", (text, currency) => {
    expect(parseAmount(text, currency)).toBeUndefined();
  });
});
