import { create } from "@bufbuild/protobuf";
import { screen } from "@testing-library/react-native";

import { createMemoryStore } from "../../data/memoryStore";
import { WeeklyTrendResponseSchema } from "../../gen/plusultra/v1/insight_pb";
import { renderThemed } from "../../ui/test-utils";
import { rangeOf } from "./shared";
import { TrainingSection } from "./TrainingSection";
import type { Trend } from "./useWeeklyTrend";

const today = "2026-11-11";

const trend = (weeks: { weekStart: string; planned: number; done: number }[]): Trend => ({
  data: create(WeeklyTrendResponseSchema, {
    weeks: weeks.map(({ weekStart, planned, done }) => ({
      weekStart,
      adherence: { planned, done },
    })),
  }),
  loading: false,
  retry: () => {},
});

function storeWith(dates: string[]) {
  const store = createMemoryStore();
  for (const date of dates) {
    store.put("workoutRecords", {
      id: `w-${date}`,
      userId: "u",
      updatedAt: "",
      version: 1,
      deviceId: "d",
      dirty: false,
      date,
      week: 0,
      day: 0,
      dayName: "",
      dayDesc: "",
      entries: [],
      finishedAt: `${date}T08:00:00Z`,
      finalized: true,
      finalizeRequested: false,
    });
  }
  return store;
}

const mount = (
  store = storeWith(["2026-11-05", "2026-11-09"]),
  range = rangeOf("2026-11-04", today),
  weeks = [
    { weekStart: "2026-11-02", planned: 3, done: 3 },
    { weekStart: "2026-11-09", planned: 2, done: 2 },
  ],
) =>
  renderThemed(
    <TrainingSection store={store} range={range} today={today} width={358} trend={trend(weeks)} />,
  );

describe("TrainingSection", () => {
  it("draws the weeks as columns under weekday initials, hollow outside the range", async () => {
    await mount();
    expect(screen.getByLabelText(/^2 training days from /)).toBeOnTheScreen();
    expect(screen.getAllByTestId("trained")).toHaveLength(2);
    // Two weeks of 7 hold the 8 days of the range.
    expect(screen.getAllByTestId("outside")).toHaveLength(6);
    expect(screen.getByText("M")).toBeOnTheScreen();
    expect(screen.getAllByText("T")).toHaveLength(2);
    expect(screen.getAllByText("S")).toHaveLength(2);
  });

  it("fits a year of weeks in one row", async () => {
    await mount(storeWith([]), rangeOf("2025-11-11", today));
    const squares = screen.getAllByTestId("outside");
    const size = squares[0].props.style.width;
    expect(size).toBeGreaterThanOrEqual(4);
    // 53 columns with their gaps fit the width beside the label column.
    expect(53 * size + 52 * 2).toBeLessThanOrEqual(358 - 16);
  });

  it("shows workouts per week with a decimal only when not whole", async () => {
    const first = await mount();
    expect(screen.getByLabelText("Workouts per week: 2.5")).toBeOnTheScreen();
    expect(screen.getByLabelText("Week streak: 2")).toBeOnTheScreen();
    first.unmount();
    await mount(undefined, undefined, [
      { weekStart: "2026-11-02", planned: 3, done: 3 },
      { weekStart: "2026-11-09", planned: 3, done: 3 },
    ]);
    expect(screen.getByLabelText("Workouts per week: 3")).toBeOnTheScreen();
  });
});
