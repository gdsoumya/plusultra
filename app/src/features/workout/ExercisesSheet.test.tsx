import { fireEvent, screen, within } from "@testing-library/react-native";
import { GestureHandlerRootView } from "react-native-gesture-handler";

import type { ExerciseEntry } from "../../domain/types";
import { renderThemed } from "../../ui/test-utils";
import { ExercisesSheet } from "./ExercisesSheet";

jest.mock("../../api/useApi", () => ({ useApi: () => ({}) }));
jest.mock("../../data/exerciseInfo", () => ({ useExerciseInfo: () => ({}) }));

const entry = (i: number): ExerciseEntry => ({
  fromProgram: true,
  name: `Exercise ${i + 1}`,
  slug: `exercise-${i + 1}`,
  equipment: "barbell",
  label: "",
  alternates: [],
  current: 1,
  desc: "",
  superset: "",
  variation: 0,
  sets: [],
  promptValues: {},
});

describe("ExercisesSheet", () => {
  it("lists many exercises inside a scrolling body and removes one by its X", async () => {
    const entries = Array.from({ length: 15 }, (_, i) => entry(i));
    const onRemove = jest.fn();
    await renderThemed(
      <GestureHandlerRootView>
        <ExercisesSheet
          open
          entries={entries}
          onMove={jest.fn()}
          onRemove={onRemove}
          onClose={jest.fn()}
        />
      </GestureHandlerRootView>,
    );
    const scroll = within(screen.getByTestId("exercises-scroll"));
    for (const i of entries.keys()) {
      expect(scroll.getByText(`Exercise ${i + 1}`)).toBeOnTheScreen();
    }
    expect(scroll.getByTestId("exercises-14")).toBeOnTheScreen();
    await fireEvent.press(scroll.getByLabelText("Remove Exercise 15"));
    expect(onRemove).toHaveBeenCalledWith(14);
  });
});
