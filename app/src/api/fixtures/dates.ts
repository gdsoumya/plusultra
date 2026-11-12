import { timestampFromDate } from "@bufbuild/protobuf/wkt";

import { addDays } from "../../domain/dates";

export { addDays, daysBetween, mondayOf, weekday } from "../../domain/dates";

export function* datesIn(from: string, to: string) {
  for (let d = from; d <= to; d = addDays(d, 1)) yield d;
}

// A wall-clock time on a date in the fixture user's zone, Asia/Kolkata.
export const at = (date: string, time: string) =>
  new Date(`${date}T${time}:00+05:30`).toISOString();

export const stampAt = (date: string, time: string) => timestampFromDate(new Date(at(date, time)));
