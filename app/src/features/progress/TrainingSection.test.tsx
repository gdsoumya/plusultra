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
  it("draws up to four weeks as calendar rows under the weekday initials, hollow outside the range", async () => {
    await mount();
    expect(screen.getByLabelText(/^2 training days from /)).toBeOnTheScreen();
    expect(screen.getAllByTestId("trained")).toHaveLength(2);
    // Two weeks of 7 hold the 8 days of the range.
    const outside = screen.getAllByTestId("outside");
    expect(outside).toHaveLength(6);
    expect(outside[0].props.style.width).toBe(28);
    expect(screen.getByText("M")).toBeOnTheScreen();
    expect(screen.getAllByText("T")).toHaveLength(2);
    expect(screen.getAllByText("S")).toHaveLength(2);
  });

  it("draws longer ranges as week columns that fit the width, a year included", async () => {
    const quarter = await mount(storeWith([]), rangeOf("2026-08-13", today));
    expect(screen.getAllByTestId("outside")[0].props.style.width).toBe(16);
    await quarter.unmount();
    await mount(storeWith([]), rangeOf("2025-11-11", today));
    const size = screen.getAllByTestId("outside")[0].props.style.width;
    expect(size).toBeGreaterThanOrEqual(4);
    // 53 columns with 1 px gaps fit the width beside the label column.
    expect(53 * (size + 1) + 16).toBeLessThanOrEqual(358);
  });

  it("shows workouts per week over the range, with a decimal only when not whole", async () => {
    // Two finished workouts over the eight days from Nov 4 to Nov 11.
    const first = await mount(storeWith(["2026-11-04", "2026-11-09"]));
    expect(screen.getByLabelText("Workouts per week: 1.8")).toBeOnTheScreen();
    expect(screen.getByLabelText("Week streak: 2")).toBeOnTheScreen();
    await first.unmount();
    await mount(storeWith(["2026-11-05", "2026-11-09"]), rangeOf("2026-11-05", today));
    expect(screen.getByLabelText("Workouts per week: 2")).toBeOnTheScreen();
  });

  it("counts the weeks from the first workout when the range starts before it", async () => {
    // Signed up a week ago: two workouts in the last 7 days of a 30-day range.
    await mount(storeWith(["2026-11-05", "2026-11-09"]), rangeOf("2026-10-13", today));
    expect(screen.getByLabelText("Workouts per week: 2")).toBeOnTheScreen();
  });
});
