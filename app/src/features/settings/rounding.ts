import { create } from "@bufbuild/protobuf";

import { RoundingSchema, type Rounding } from "../../gen/plusultra/v1/common_pb";

export const roundingKinds = ["plates", "fixed", "stack", "bodyweight"] as const;

// A rounding as typed into the editor; numbers stay text until saved.
export interface RoundingDraft {
  kind: string;
  bar: string;
  increment: string;
  plates: string;
}

export const draftOf = (r?: Rounding): RoundingDraft => ({
  kind: r?.kind || "plates",
  bar: r?.bar ? String(r.bar) : "",
  increment: r?.increment ? String(r.increment) : "",
  plates: r?.plates.join(", ") ?? "",
});

const number = (text: string, label: string, min: number) => {
  const n = Number(text.trim());
  if (!text.trim() || !Number.isFinite(n) || n < min) {
    throw new Error(min > 0 ? `${label} must be above 0` : `${label} must be a number`);
  }
  return n;
};

const numbers = (text: string, label: string) => {
  const parts = text.split(/[\s,]+/).filter(Boolean);
  const values = parts.map(Number);
  if (!values.length || values.some((n) => !Number.isFinite(n) || n <= 0)) {
    throw new Error(`${label} must be numbers above 0`);
  }
  return values;
};

// Throws a message for the editor when a number cannot be read.
export function parseRounding(d: RoundingDraft): Rounding {
  switch (d.kind) {
    case "plates":
      return create(RoundingSchema, {
        kind: d.kind,
        bar: number(d.bar, "Bar", 0),
        plates: numbers(d.plates, "Plates"),
      });
    case "fixed":
      return create(RoundingSchema, { kind: d.kind, plates: numbers(d.plates, "Weights") });
    case "stack":
      return create(RoundingSchema, {
        kind: d.kind,
        increment: number(d.increment, "Increment", Number.MIN_VALUE),
      });
    default:
      return create(RoundingSchema, { kind: "bodyweight" });
  }
}

export function describeRounding(r?: Rounding): string {
  switch (r?.kind) {
    case "plates":
      return `Bar ${r.bar}, plates ${r.plates.join("/")}`;
    case "fixed":
      return `${r.plates.length} weights, ${Math.min(...r.plates)} to ${Math.max(...r.plates)}`;
    case "stack":
      return `Steps of ${r.increment}`;
    case "bodyweight":
      return "No rounding";
    default:
      return "Equipment rounding";
  }
}
