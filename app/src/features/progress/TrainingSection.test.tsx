import { screen } from "@testing-library/react-native";

import { createMemoryStore } from "../../data/memoryStore";
import { renderThemed } from "../../ui/test-utils";
import { rangeOf } from "./shared";
import { TrainingSection } from "./TrainingSection";

const today = "2026-11-11";

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
) => renderThemed(<TrainingSection store={store} range={range} today={today} width={358} />);

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

  it("shows workouts per week over the range, with a decimal only when not whole", async () => {
    // Two finished workouts over the eight days from Nov 4 to Nov 11.
    const first = await mount();
    expect(screen.getByLabelText("Workouts per week: 1.8")).toBeOnTheScreen();
    expect(screen.getByLabelText("Week streak: 2")).toBeOnTheScreen();
    first.unmount();
    await mount(storeWith(["2026-11-05", "2026-11-09"]), rangeOf("2026-11-05", today));
    expect(screen.getByLabelText("Workouts per week: 2")).toBeOnTheScreen();
  });
});
