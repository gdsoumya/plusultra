import { fireEvent, screen } from "@testing-library/react-native";

import { createClients } from "../../api/clients";
import { sets } from "../../api/fixtures/build";
import { createMockTransport } from "../../api/mockTransport";
import type { ExerciseEntry } from "../../domain/types";
import { renderThemed } from "../../ui/test-utils";
import { ExercisePane } from "./ExercisePane";

const mockApi = createClients(createMockTransport({ today: "2026-11-11" }).transport);
jest.mock("../../api/useApi", () => ({ useApi: () => mockApi }));
jest.mock("expo-router", () => ({
  useRouter: () => ({ push: jest.fn() }),
  useSegments: () => ["(tabs)", "(today)", "workout", "[id]"],
}));

const entry = (over: Partial<ExerciseEntry>): ExerciseEntry => ({
  name: "Jump Rope",
  slug: "",
  equipment: "",
  label: "",
  alternates: ["Jump Rope"],
  current: 1,
  desc: "",
  superset: "",
  fromProgram: true,
  variation: 0,
  promptValues: {},
  sets: [],
  ...over,
});
const noop = () => {};
const render = (e: ExerciseEntry) =>
  renderThemed(
    <ExercisePane
      entry={e}
      record={{ id: "r1", date: "2026-11-11" }}
      index={0}
      count={1}
      unit="kg"
      finished={false}
      onDone={noop}
      onEdit={noop}
      onRemoveSet={noop}
      onSetValues={noop}
      onChange={noop}
      onSwitch={noop}
      onMove={noop}
      onRemove={noop}
      onNext={noop}
    />,
  );

describe("ExercisePane", () => {
  it("sums a timed exercise as sets and time, not reps", async () => {
    const timed = sets(3, 1, 0, { work: 60 }).map((planned, i) => ({
      planned,
      completed: i === 0,
      reps: i === 0 ? 1 : 0,
      weight: planned.weight,
      rpe: 0,
      workTime: i === 0 ? 50 : undefined,
    }));
    await render(entry({ sets: timed }));
    expect(screen.getByTestId("exercise-0-summary")).toHaveTextContent("1 / 3 sets · 0:50 / 3:00", {
      exact: true,
    });
  });

  it("lets a long tonnage line wrap instead of cutting the unit off", async () => {
    const heavy = sets(3, 5, 100).map((planned) => ({
      planned,
      completed: false,
      reps: 0,
      weight: planned.weight,
      rpe: 0,
    }));
    await render(entry({ sets: heavy }));
    expect(screen.getByTestId("exercise-0-summary").props.numberOfLines).toBe(2);
  });

  it("folds a long coach note to three lines behind More", async () => {
    const desc = "Keep the elbows tucked. ".repeat(12).trim();
    await render(entry({ desc }));
    expect(screen.getByText(desc).props.numberOfLines).toBe(3);
    await fireEvent.press(screen.getByRole("button", { name: "More" }));
    expect(screen.getByText(desc).props.numberOfLines).toBeUndefined();
    await fireEvent.press(screen.getByRole("button", { name: "Less" }));
    expect(screen.getByText(desc).props.numberOfLines).toBe(3);
  });

  it("shows a short note whole, with nothing to toggle", async () => {
    await render(entry({ desc: "Elbows tucked." }));
    expect(screen.getByText("Elbows tucked.").props.numberOfLines).toBeUndefined();
    expect(screen.queryByRole("button", { name: "More" })).toBeNull();
  });
});
