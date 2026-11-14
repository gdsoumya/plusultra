import { fireEvent, screen } from "@testing-library/react-native";

import type { ExerciseEntry } from "../../domain/types";
import { renderThemed } from "../../ui/test-utils";
import { SetList } from "./SetList";

const entry = (over: Partial<ExerciseEntry>): ExerciseEntry => ({
  name: "Bench Press",
  slug: "",
  equipment: "",
  label: "",
  alternates: ["Bench Press"],
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
const render = async (e: ExerciseEntry, onChange = jest.fn()) => {
  await renderThemed(
    <SetList
      entry={e}
      index={0}
      unit="kg"
      onDone={noop}
      onEdit={noop}
      onRemoveSet={noop}
      onSetValues={noop}
      onChange={onChange}
    />,
  );
  return onChange;
};
// The entry the last onChange edit produces.
const edited = (onChange: jest.Mock, e: ExerciseEntry) =>
  (onChange.mock.lastCall?.[0] as (e: ExerciseEntry) => ExerciseEntry)(e);

describe("SetList note", () => {
  it("offers Add note while there is none", async () => {
    await render(entry({}));
    expect(screen.getByRole("button", { name: "Add note" })).toBeOnTheScreen();
    expect(screen.queryByTestId("exercise-0-note")).toBeNull();
  });

  it("shows the note with Edit note", async () => {
    await render(entry({ notes: "Felt heavy" }));
    expect(screen.getByRole("button", { name: "Edit note" })).toBeOnTheScreen();
    expect(screen.getByTestId("exercise-0-note")).toHaveTextContent("Felt heavy");
  });

  it("saves the trimmed note through onChange", async () => {
    const e = entry({});
    const onChange = await render(e);
    await fireEvent.press(screen.getByRole("button", { name: "Add note" }));
    await fireEvent.changeText(screen.getByLabelText("Note"), "  Left knee twinge on set 3 \n");
    await fireEvent.press(screen.getByRole("button", { name: "Save" }));
    expect(edited(onChange, e).notes).toBe("Left knee twinge on set 3");
  });

  it("removes the note", async () => {
    const e = entry({ notes: "Felt heavy" });
    const onChange = await render(e);
    await fireEvent.press(screen.getByRole("button", { name: "Edit note" }));
    await fireEvent.press(screen.getByRole("button", { name: "Remove note" }));
    expect(edited(onChange, e).notes).toBeUndefined();
  });
});
