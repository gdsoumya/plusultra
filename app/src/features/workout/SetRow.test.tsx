import { fireEvent, screen } from "@testing-library/react-native";
import type { ViewStyle } from "react-native";

import type { SetEntry } from "../../domain/types";
import { colors, control, minTouch, space } from "../../ui";
import { renderThemed } from "../../ui/test-utils";
import { SetRow } from "./SetRow";

const planned: SetEntry["planned"] = {
  reps: 12,
  minReps: 0,
  weight: { value: 140, unit: "kg" },
  rpe: 8,
  rest: 90,
  work: 45,
  amrap: false,
  ask: false,
  auto: true,
  countUp: false,
  warmup: false,
  label: "",
  logRpe: false,
};
const set = (p: Partial<SetEntry["planned"]> = {}, s: Partial<SetEntry> = {}): SetEntry => ({
  planned: { ...planned, ...p },
  completed: false,
  reps: 0,
  weight: planned.weight,
  rpe: 0,
  ...s,
});
const noop = () => {};
const row = (s: SetEntry, unilateral = false, props: Partial<Parameters<typeof SetRow>[0]> = {}) =>
  renderThemed(
    <SetRow
      set={s}
      number={1}
      unit="kg"
      unilateral={unilateral}
      active={false}
      running={false}
      editing={false}
      onDone={noop}
      onEdit={noop}
      onRemove={noop}
      onChange={noop}
      testID="set-0-0"
      {...props}
    />,
  );

const flat = (style: unknown): ViewStyle => Object.assign({}, ...[style].flat(Infinity));

describe("SetRow", () => {
  it.each([
    ["Warmup", { warmup: true }, colors.warmup],
    ["Work", {}, colors.accent],
    ["Backoff", { label: "Backoff" }, colors.backoff],
  ])("colours the %s group marker and label", async (label, plan, color) => {
    await row(set(plan));
    expect(screen.getByText(label)).toHaveStyle({ color });
    expect(flat(screen.getByTestId("set-0-0-group").props.style).backgroundColor).toBe(color);
  });

  it("always shows the planned rest, work, RPE, AMRAP and ask before anything is pressed", async () => {
    await row(set({ amrap: true, ask: true }));
    expect(screen.getByTestId("set-0-0-caption")).toHaveTextContent(
      "Work · work 0:45 · auto · rest 1:30 · RPE 8 · AMRAP · ask",
    );
    expect(screen.getByTestId("set-0-0-reps")).toHaveTextContent("12+");
    expect(screen.getByTestId("set-0-0-weight")).toHaveTextContent("?");
  });

  it("shows one number for a rep range and keeps the range in the caption", async () => {
    await row(set({ minReps: 12, reps: 15 }));
    expect(screen.getByTestId("set-0-0-reps")).toHaveTextContent(/^15$/);
    expect(screen.getByTestId("set-0-0-caption")).toHaveTextContent(/12-15 reps · aim 15/);
  });

  it("keeps an L/R row inside 390 px: the caption sits on its own line and nothing fixed exceeds the width", async () => {
    await row(set(), true);
    const line = screen.getByTestId("set-0-0-values");
    const fixed = [
      flat(screen.getByTestId("set-0-0-group").props.style).width as number,
      ...["repsLeft", "reps", "weight"].map(
        (id) => flat(screen.getByTestId(`set-0-0-${id}`).props.style).minWidth as number,
      ),
      2 * minTouch,
    ];
    expect(fixed).toEqual([3, control.sm, control.sm, control.sm, 2 * minTouch]);
    expect(fixed.reduce((a, b) => a + b) + 6 * space[2]).toBeLessThan(390 - 2 * space[4]);
    expect(line).not.toHaveTextContent(/rest/);
    expect(screen.getByTestId("set-0-0-repsLeft")).toHaveTextContent("L 12");
    expect(screen.getByTestId("set-0-0-reps")).toHaveTextContent("R 12");
  });

  it("dims a completed row and shows its check", async () => {
    await row(set({}, { completed: true, reps: 12 }));
    expect(screen.getByTestId("set-0-0-done")).toBeChecked();
    expect(flat(screen.getByTestId("set-0-0-body").props.style).opacity).toBeLessThan(1);
  });

  it("opens the editor from any bubble", async () => {
    const onEdit = jest.fn();
    await row(set(), false, { onEdit });
    await fireEvent.press(screen.getByTestId("set-0-0-weight"));
    expect(onEdit).toHaveBeenCalledTimes(1);
  });

  it("presses play, stop and check with Space, which the web gives only to buttons", async () => {
    const onDone = jest.fn();
    await row(set(), false, { onDone });
    const preventDefault = jest.fn();
    await fireEvent(screen.getByTestId("set-0-0-done"), "keyDown", {
      nativeEvent: { key: " " },
      preventDefault,
    });
    expect(onDone).toHaveBeenCalledTimes(1);
    expect(preventDefault).toHaveBeenCalled();
    await fireEvent(screen.getByTestId("set-0-0-done"), "keyDown", {
      nativeEvent: { key: "a" },
      preventDefault,
    });
    expect(onDone).toHaveBeenCalledTimes(1);
  });
});
