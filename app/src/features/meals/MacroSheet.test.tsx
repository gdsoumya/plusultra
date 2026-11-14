import { screen } from "@testing-library/react-native";

import type { Macros } from "../../domain/types";
import { renderThemed } from "../../ui/test-utils";
import { MacroSheet } from "./MacroSheet";

const macros = (kcal: number, protein: number): Macros => ({
  kcal,
  protein,
  carbs: 300,
  fat: 100,
  fiber: 0,
  waterMl: 0,
});

describe("MacroSheet", () => {
  it("shows each macro's gap to the target and the verdict", async () => {
    await renderThemed(
      <MacroSheet
        open
        title="Day total"
        totals={macros(3705, 180)}
        target={macros(3550, 183)}
        onTarget={false}
        onClose={jest.fn()}
      />,
    );
    expect(screen.getByText("Day total")).toBeOnTheScreen();
    expect(screen.getByLabelText("Off target")).toBeOnTheScreen();
    expect(screen.getByText("Target 3550 kcal")).toBeOnTheScreen();
    expect(screen.getByText("155 kcal over")).toBeOnTheScreen();
    expect(screen.getByText("3 g short")).toBeOnTheScreen();
    expect(screen.getAllByText("on target")).toHaveLength(2);
    expect(screen.queryByText("Fiber")).toBeNull();
  });

  it("shows plain amounts without a target", async () => {
    await renderThemed(
      <MacroSheet open title="Lunch" totals={macros(800, 40)} onClose={jest.fn()} />,
    );
    expect(screen.getByText("800 kcal")).toBeOnTheScreen();
    expect(screen.queryByLabelText(/target/)).toBeNull();
  });
});
