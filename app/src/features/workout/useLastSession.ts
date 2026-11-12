import { useEffect, useMemo, useState } from "react";

import { fromProto } from "../../api/convert";
import { useApi } from "../../api/useApi";
import { useStore } from "../../data/StoreProvider";
import { localDate, today } from "../../domain/dates";
import { findLastSession, type LastSession } from "../../domain/lastSession";
import type { ExerciseEntry, WorkoutRecord } from "../../domain/types";

type Cached = { at: string; sessions: LastSession[] };

// How the exercise went the last time, from the store's records first. When the store has no
// earlier record, the server's lift history (best set and e1RM per date, no set list) fills in,
// fetched at most once a day per exercise and kept in kv so it shows offline.
export function useLastSession(
  entry: ExerciseEntry,
  current: Pick<WorkoutRecord, "id" | "date">,
): LastSession | undefined {
  const store = useStore();
  const api = useApi();
  const { slug, name } = entry;
  const alternates = entry.alternates.join();
  const key = `lastSession:${slug}`;
  const local = useMemo(
    () => findLastSession(store.list("workoutRecords", { dateTo: current.date }), entry, current),
    // Only the exercise's identity matters, not its sets as they are ticked.
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [store, slug, name, alternates, current.id, current.date],
  );
  const [, fetched] = useState(0);
  useEffect(() => {
    if (local || !slug || localDate(new Date(store.getKv<Cached>(key)?.at ?? 0)) === today())
      return;
    let live = true;
    api.insights.liftHistory({ slug }).then(
      ({ points }) => {
        const sessions = points.slice(-5).map((p) => ({
          date: p.date,
          sets: [],
          best: { weight: fromProto.weight(p.bestWeight), reps: p.bestReps },
          e1rm: fromProto.weight(p.e1rm),
        }));
        store.setKv(key, { at: new Date().toISOString(), sessions } satisfies Cached);
        if (live) fetched((n) => n + 1);
      },
      () => {},
    );
    return () => {
      live = false;
    };
  }, [api, store, key, slug, local]);
  if (local) return local;
  return store
    .getKv<Cached>(key)
    ?.sessions.filter((s) => s.date < current.date)
    .at(-1);
}
