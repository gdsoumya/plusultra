import type { ResolvedSet } from "../../domain/types";
import type { ColorRole } from "../../ui";

export type SetGroup = { label: string; color: ColorRole };

// The colour-coded group a set row shows: warmups, the plan's own label ("Backoff", "Drop"), or
// plain work.
export const setGroup = (p: ResolvedSet): SetGroup =>
  p.warmup
    ? { label: "Warmup", color: "warmup" }
    : p.label
      ? { label: p.label, color: "backoff" }
      : { label: "Work", color: "accent" };
