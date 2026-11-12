import { fireEvent, screen } from "@testing-library/react-native";

import { renderThemed, snapshots } from "../../test-utils";
import { BarChart } from "../BarChart";

const chart = (
  <BarChart
    bars={[
      { label: "Calves", value: 10 },
      { label: "Quadriceps", value: 9 },
      { label: "Glutes", value: 3 },
    ]}
    columns={["Muscle", "Sets"]}
    summary="22 working sets"
    testID="bars"
  />
);

describe("BarChart", () => {
  it("labels every bar with its value and sizes it against the largest", async () => {
    await renderThemed(chart);
    expect(screen.getByLabelText("22 working sets")).toBeOnTheScreen();
    expect(screen.getByText("Quadriceps")).toBeOnTheScreen();
    expect(screen.getByText("9")).toBeOnTheScreen();
    expect(screen.getByTestId("bar-Calves")).toHaveStyle({ width: "100%" });
    expect(screen.getByTestId("bar-Glutes")).toHaveStyle({ width: "30%" });
  });

  it("toggles to a table", async () => {
    await renderThemed(chart);
    await fireEvent.press(screen.getByRole("button", { name: "Show table" }));
    expect(screen.getByText("Muscle")).toBeOnTheScreen();
    expect(screen.getByText("3")).toBeOnTheScreen();
  });

  snapshots(() => chart);
});
