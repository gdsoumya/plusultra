import { screen } from "@testing-library/react-native";

import { createClients } from "../../api/clients";
import { createMockTransport } from "../../api/mockTransport";
import type { ExerciseEntry } from "../../domain/types";
import { renderThemed } from "../../ui/test-utils";
import { ExerciseCarousel } from "./ExerciseCarousel";

const mockApi = createClients(createMockTransport({ today: "2026-11-11" }).transport);
jest.mock("../../api/useApi", () => ({ useApi: () => mockApi }));

const entry = (name: string): ExerciseEntry => ({
  name,
  slug: "",
  equipment: "",
  label: "",
  alternates: [name],
  current: 1,
  desc: "",
  superset: "",
  fromProgram: false,
  variation: 0,
  promptValues: {},
  sets: [],
});
const noop = () => {};
const render = () =>
  renderThemed(
    <ExerciseCarousel
      entries={[entry("Squat"), entry("Bench Press")]}
      shown={0}
      onShow={noop}
      onList={noop}
    />,
  );
// The strip lays out one item per child, side by side; the first tile's item has them all as siblings.
const items = () => screen.getByTestId("tile-0").parent!.parent!.children;

describe("ExerciseCarousel", () => {
  it("lays every exercise tile out as its own item in one line, and nothing else", async () => {
    await render();
    expect(items()).toHaveLength(2);
    expect(new Set(["tile-0", "tile-1"].map((id) => screen.getByTestId(id).parent)).size).toBe(2);
    expect(screen.queryByTestId("add-exercise-tile")).toBeNull();
  });
});
