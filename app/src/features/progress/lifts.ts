import type { Store } from "../../data/store";
import type { Range } from "./shared";

type Lift = { slug: string; name: string };

// Exercises with completed loaded sets in the range, most often trained first.
export function loadedLifts(store: Store, { from, to }: Range): Lift[] {
  const seen = new Map<string, { name: string; count: number }>();
  for (const record of store.list("workoutRecords", { dateFrom: from, dateTo: to })) {
    for (const e of record.entries) {
      if (!e.slug || !e.sets.some((s) => s.completed && s.weight.value > 0)) continue;
      const lift = seen.get(e.slug) ?? { name: e.name, count: 0 };
      seen.set(e.slug, { ...lift, count: lift.count + 1 });
    }
  }
  return [...seen]
    .sort((a, b) => b[1].count - a[1].count)
    .map(([slug, { name }]) => ({ slug, name }));
}
