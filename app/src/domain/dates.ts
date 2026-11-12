// en-CA formats as YYYY-MM-DD; the device zone when none is given.
export function localDate(date: Date, timeZone?: string): string {
  return new Intl.DateTimeFormat("en-CA", { timeZone }).format(date);
}

export const today = () => localDate(new Date());

// Calendar arithmetic on YYYY-MM-DD dates, done in UTC so no time zone shifts the day.
const parse = (date: string) => new Date(`${date}T00:00:00Z`);
const format = (d: Date) => d.toISOString().slice(0, 10);

export function addDays(date: string, n: number): string {
  const d = parse(date);
  d.setUTCDate(d.getUTCDate() + n);
  return format(d);
}

export const daysBetween = (from: string, to: string) =>
  Math.round((parse(to).getTime() - parse(from).getTime()) / 86_400_000);

// 1 is Monday, 7 is Sunday.
export const weekday = (date: string) => ((parse(date).getUTCDay() + 6) % 7) + 1;

export const mondayOf = (date: string) => addDays(date, 1 - weekday(date));

// Months are YYYY-MM.
export const monthOf = (date: string) => date.slice(0, 7);

export function addMonths(month: string, n: number): string {
  const d = parse(`${month}-01`);
  d.setUTCMonth(d.getUTCMonth() + n);
  return monthOf(format(d));
}

// Six Monday-first weeks, padded with the neighbouring months' dates, so every month has the same height.
export function monthGrid(month: string): string[][] {
  const first = mondayOf(`${month}-01`);
  return Array.from({ length: 6 }, (_, w) =>
    Array.from({ length: 7 }, (_, d) => addDays(first, w * 7 + d)),
  );
}

export const monthLabel = (month: string) =>
  new Intl.DateTimeFormat("en", { month: "long", year: "numeric", timeZone: "UTC" }).format(
    parse(`${month}-01`),
  );

export const dateLabel = (date: string) =>
  new Intl.DateTimeFormat("en-GB", {
    weekday: "long",
    day: "numeric",
    month: "long",
    timeZone: "UTC",
  }).format(parse(date));

export const nextMonday = (date: string) => addDays(date, 8 - weekday(date));

export const isDate = (s: string) =>
  /^\d{4}-\d{2}-\d{2}$/.test(s) && !Number.isNaN(parse(s).getTime()) && format(parse(s)) === s;

export const weekdayName = (date: string) =>
  new Intl.DateTimeFormat("en-US", { weekday: "long", timeZone: "UTC" }).format(parse(date));

export const formatDate = (date: string) =>
  new Intl.DateTimeFormat("en-GB", {
    day: "numeric",
    month: "short",
    year: "numeric",
    timeZone: "UTC",
  }).format(parse(date));

export const formatDateTime = (d: Date, timeZone?: string) =>
  new Intl.DateTimeFormat("en-GB", { dateStyle: "medium", timeStyle: "short", timeZone }).format(d);
// Day and short month in the device locale, e.g. "Oct 14".
export const shortDate = (date: string) =>
  parse(date).toLocaleDateString(undefined, { day: "numeric", month: "short", timeZone: "UTC" });

// Seconds as m:ss, for timers.
export const clock = (s: number) => `${Math.floor(s / 60)}:${String(s % 60).padStart(2, "0")}`;
