import { useEffect, useMemo, useRef, useState } from "react";

import { workoutRecordsRepo } from "../../data/repositories/workoutRecords";
import { useStore } from "../../data/StoreProvider";
import { useSync } from "../../data/useSync";
import type { WorkoutRecord } from "../../domain/types";

// The record lives in the store; every change is saved through the repository so sync sees it.
// The save happens here, not in a state updater: React may run updaters during render, and the
// store's listeners set state of their own.
export function useWorkoutRecord(id: string) {
  const store = useStore();
  const repo = useMemo(() => workoutRecordsRepo(store), [store]);
  const [record, setRecord] = useState(() => repo.get(id));
  const latest = useRef(record);
  // A sync may land the record (a deep link before the first pull) or a newer copy of it; the
  // store already holds every local edit, so re-reading loses nothing. It may also merge the
  // record into another device's record of the same planned day, which the screen then shows
  // (P2-R41).
  const { lastSyncedAt } = useSync();
  useEffect(() => {
    const was = latest.current;
    latest.current =
      repo.get(id) ??
      (was?.programId
        ? repo
            .byDate(was.date)
            .find((r) => r.programId === was.programId && r.week === was.week && r.day === was.day)
        : undefined);
    setRecord(latest.current);
  }, [lastSyncedAt, repo, id]);
  return {
    record,
    update(edit: (r: WorkoutRecord) => WorkoutRecord) {
      if (!latest.current) return;
      latest.current = repo.save(edit(latest.current));
      setRecord(latest.current);
    },
  };
}
