import { screen } from "@testing-library/react-native";

import { renderThemed } from "../../ui/test-utils";
import { MacroBars } from "./MacroBars";

const totals = { kcal: 3289, protein: 150, carbs: 300, fat: 90, fiber: 0, waterMl: 0 };

describe("MacroBars", () => {
  it("prints grams after the macros and wraps a long day total instead of cutting it", async () => {
    await renderThemed(
      <MacroBars totals={totals} target={{ ...totals, kcal: 2900, protein: 180 }} />,
    );
    expect(screen.getByText("3289 / 2900")).toBeOnTheScreen();
    expect(screen.getByText("150 / 180 g")).toBeOnTheScreen();
    expect(screen.getByText("3289 / 2900").props.numberOfLines).toBeUndefined();
  });

  it("prints grams without a target", async () => {
    await renderThemed(<MacroBars totals={totals} />);
    expect(screen.getByText("3289")).toBeOnTheScreen();
    expect(screen.getByText("150 g")).toBeOnTheScreen();
  });
});
