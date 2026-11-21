import { addDays, dayLabel, todayIn } from "@/lib/dates";

describe("todayIn", () => {
  it("takes the date in the given time zone, not the device's", () => {
    const now = new Date("2026-03-01T23:30:00Z");

    expect(todayIn("Europe/Belgrade", now)).toBe("2026-03-02");
    expect(todayIn("America/Los_Angeles", now)).toBe("2026-03-01");
  });
});

describe("addDays", () => {
  it.each([
    ["2026-03-01", -1, "2026-02-28"],
    ["2024-03-01", -1, "2024-02-29"],
    ["2026-01-01", -2, "2025-12-30"],
    ["2026-03-29", 1, "2026-03-30"],
  ])("%s %i days is %s", (date, days, want) => {
    expect(addDays(date, days)).toBe(want);
  });
});

describe("dayLabel", () => {
  it.each([
    ["2026-03-02", "Today"],
    ["2026-03-01", "Yesterday"],
    ["2026-02-28", "Sat 28 Feb"],
    ["2025-12-24", "Wed 24 Dec 2025"],
  ])("labels %s as %s when today is 2026-03-02", (date, want) => {
    expect(dayLabel(date, "2026-03-02")).toBe(want);
  });
});
