import { formatMoney, formatTyped, pressKey, toMinorUnits } from "@/lib/money";

function typeKeys(keys: string) {
  let typed = "";
  for (const key of keys) typed = pressKey(typed, key === "<" ? "Backspace" : key, "RSD");
  return typed;
}

describe("formatMoney", () => {
  it.each([
    [178080, "RSD", "1,780.80 din"],
    [-199000, "RSD", "−1,990.00 din"],
    [5, "RSD", "0.05 din"],
    [123456789012, "RSD", "1,234,567,890.12 din"],
    [1050, "EUR", "10.50 EUR"],
  ])("formats %i %s as %s", (amount, currency, want) => {
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
  ])("typing %s shows %s and is %i minor units", (keys, want, minor) => {
    const typed = typeKeys(keys);

    expect(typed).toBe(want);
    expect(toMinorUnits(typed, "RSD")).toBe(minor);
  });

  it("groups thousands while typing and shows 0 when empty", () => {
    expect(formatTyped("1780.8")).toBe("1,780.8");
    expect(formatTyped("1234567")).toBe("1,234,567");
    expect(formatTyped("")).toBe("0");
  });
});
