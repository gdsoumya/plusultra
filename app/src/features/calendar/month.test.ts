import { checkinsRepo } from "../../data/repositories/checkins";
import { dayLogsRepo } from "../../data/repositories/dayLogs";
import { resolvedDaysRepo } from "../../data/repositories/resolvedDays";
import { buildMonth } from "./month";
import { makeFloating, seedStore } from "../../data/seedStore";

const today = "2026-11-11";

describe("buildMonth", () => {
  it("plans future days only for the anchored program", async () => {
    const store = await seedStore(today);
    const repo = resolvedDaysRepo(store);
    const [training, ...rest] = repo.range("2026-11-12", "2026-11-30");
    const last = rest[rest.length - 1];
    repo.upsertFromServer([{ ...last, payload: { ...last.payload, kind: "rest", exercises: [] } }]);
    const { days, next } = buildMonth(store, "2026-11", today);
    expect(days[training.date!]?.status).toBe("planned");
    expect(days[last.date!]?.status).toBe("rest");
    expect(next).toBeUndefined();

    makeFloating(store);
    const floating = buildMonth(store, "2026-11", today);
    expect(floating.days["2026-11-12"]).toBeUndefined();
    expect(floating.next).toEqual({ week: 2, day: 1, name: "GZCLP A1" });
  });

  it("marks logged days by their status and today's open workout as partial", async () => {
    const store = await seedStore(today);
    const { days } = buildMonth(store, "2026-11", today);
    for (const log of dayLogsRepo(store).range("2026-11-01", "2026-11-10")) {
      expect(days[log.date]?.status).toBe(log.status);
    }
    expect(days[today]).toMatchObject({ status: "partial", checkin: false });
    const [weighed] = checkinsRepo(store).range("2026-11-01", "2026-11-10");
    expect(days[weighed.date]?.checkin).toBe(true);
  });

  it("marks a past planned day without a result as skipped, as adherence counts it", async () => {
    const store = await seedStore(today);
    const date = "2026-11-02";
    dayLogsRepo(store)
      .byDate(date)
      .forEach((l) => store.delete("dayLogs", l.id));
    store
      .list("workoutRecords", { dateTo: date })
      .forEach((r) => store.delete("workoutRecords", r.id));
    store.setKv("calendarKinds:2026-11", { [date]: "training", [today]: "training" });
    const { days } = buildMonth(store, "2026-11", today);
    expect(days[date]?.status).toBe("skipped");
    expect(days[today]?.status).toBe("partial");
  });

  it("lays out six Monday-first weeks around the month", async () => {
    const { weeks } = buildMonth(await seedStore(today), "2026-11", today);
    expect(weeks[0][0]).toBe("2026-10-26");
    expect(weeks).toHaveLength(6);
  });
});
