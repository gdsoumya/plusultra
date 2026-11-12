import {
  addDays,
  addMonths,
  dateLabel,
  daysBetween,
  formatDate,
  formatDateTime,
  isDate,
  localDate,
  mondayOf,
  monthGrid,
  monthLabel,
  monthOf,
  nextMonday,
  shortDate,
  weekday,
  weekdayDate,
  weekdayName,
} from "./dates";

describe("localDate", () => {
  it("formats the calendar date in the given time zone", () => {
    const instant = new Date("2026-11-10T20:00:00Z");
    expect(localDate(instant, "Asia/Kolkata")).toBe("2026-11-11");
    expect(localDate(instant, "America/Los_Angeles")).toBe("2026-11-10");
  });
});

describe("day arithmetic", () => {
  it("adds days across month and year ends", () => {
    expect(addDays("2026-11-30", 1)).toBe("2026-12-01");
    expect(addDays("2027-01-01", -1)).toBe("2026-12-31");
  });

  it("numbers weekdays from Monday and finds the week's Monday", () => {
    expect(weekday("2026-11-09")).toBe(1);
    expect(weekday("2026-11-15")).toBe(7);
    expect(mondayOf("2026-11-11")).toBe("2026-11-09");
    expect(mondayOf("2026-11-09")).toBe("2026-11-09");
  });
});

describe("months", () => {
  it("names a date's month and steps months across years", () => {
    expect(monthOf("2026-11-11")).toBe("2026-11");
    expect(addMonths("2026-11", 2)).toBe("2027-01");
    expect(addMonths("2026-01", -1)).toBe("2025-12");
  });

  it("lays a month out in Monday-first weeks padded with neighbouring dates", () => {
    const weeks = monthGrid("2026-11");
    expect(weeks).toHaveLength(6);
    expect(weeks[0]).toEqual([
      "2026-10-26",
      "2026-10-27",
      "2026-10-28",
      "2026-10-29",
      "2026-10-30",
      "2026-10-31",
      "2026-11-01",
    ]);
    expect(weeks[5][0]).toBe("2026-11-30");
    expect(weeks[5][6]).toBe("2026-12-06");
    // A month starting on Monday has no leading padding.
    expect(monthGrid("2026-06")[0][0]).toBe("2026-06-01");
  });

  it("labels a month and a date for display", () => {
    expect(monthLabel("2026-11")).toBe("November 2026");
    expect(dateLabel("2026-11-11")).toBe("Wednesday 11 November");
    expect(weekdayDate("2026-11-10")).toBe("Tue 10 Nov");
  });
});

describe("calendar arithmetic", () => {
  it("adds days across month ends", () => {
    expect(addDays("2026-10-30", 3)).toBe("2026-11-02");
    expect(addDays("2026-11-02", -3)).toBe("2026-10-30");
  });

  it("numbers weekdays from Monday", () => {
    expect(weekday("2026-11-09")).toBe(1);
    expect(weekday("2026-11-15")).toBe(7);
    expect(weekdayName("2026-11-11")).toBe("Wednesday");
  });

  it("finds the Monday after a date, a week away on a Monday", () => {
    expect(nextMonday("2026-11-11")).toBe("2026-11-16");
    expect(nextMonday("2026-11-15")).toBe("2026-11-16");
    expect(nextMonday("2026-11-16")).toBe("2026-11-23");
  });

  it("accepts only real YYYY-MM-DD dates", () => {
    expect(isDate("2026-11-11")).toBe(true);
    expect(isDate("2026-02-30")).toBe(false);
    expect(isDate("11/11/2026")).toBe(false);
    expect(isDate("")).toBe(false);
  });

  it("formats a date for display", () => {
    expect(formatDate("2026-10-26")).toBe("26 Oct 2026");
    expect(formatDateTime(new Date("2026-11-10T20:00:00Z"), "Asia/Kolkata")).toBe(
      "11 Nov 2026, 01:30",
    );
  });

  it("adds days across month and year ends", () => {
    expect(addDays("2026-11-11", -28)).toBe("2026-10-14");
    expect(addDays("2026-12-31", 1)).toBe("2027-01-01");
  });

  it("counts the days between two dates", () => {
    expect(daysBetween("2026-10-14", "2026-11-11")).toBe(28);
  });

  it("finds the Monday of the week", () => {
    expect(mondayOf("2026-11-11")).toBe("2026-11-09");
    expect(mondayOf("2026-11-15")).toBe("2026-11-09");
    expect(mondayOf("2026-11-09")).toBe("2026-11-09");
  });

  it("shortens a date to day and month", () => {
    expect(shortDate("2026-10-14")).toMatch(/14/);
    expect(shortDate("2026-10-14")).toMatch(/Oct/);
  });
});
