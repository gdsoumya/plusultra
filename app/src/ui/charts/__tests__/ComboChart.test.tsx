import { fireEvent, screen } from "@testing-library/react-native";

import { renderThemed, snapshots } from "../../test-utils";
import { ComboChart } from "../ComboChart";

const chart = (
  <ComboChart
    width={320}
    labels={["6 Oct", "13 Oct", "20 Oct", "27 Oct"]}
    bars={[2400, undefined, 2250, 2300]}
    line={[84.2, 83.9, undefined, 83.1]}
    format={(v) => String(Math.round(v))}
    lineFormat={(v) => v.toFixed(1)}
    lineMinSpan={2}
    legend={["kcal", "kg"]}
    summary="4 weeks of intake and weight"
    table={{ columns: ["Week", "kcal", "kg"], rows: [["6 Oct", "2400", "84.2"]] }}
    testID="combo"
  />
);

describe("ComboChart", () => {
  it("draws a bar and a dot per value, leaving gaps where a week has none", async () => {
    await renderThemed(chart);
    expect(screen.getByLabelText("4 weeks of intake and weight")).toBeOnTheScreen();
    expect(screen.getAllByTestId("bar")).toHaveLength(3);
    expect(screen.getAllByTestId("dot")).toHaveLength(3);
    expect(screen.getByText("kcal")).toBeOnTheScreen();
    expect(screen.getByText("kg")).toBeOnTheScreen();
  });

  it("toggles to a table", async () => {
    await renderThemed(chart);
    await fireEvent.press(screen.getByRole("button", { name: "Show table" }));
    expect(screen.getByText("Week")).toBeOnTheScreen();
    expect(screen.getByText("2400")).toBeOnTheScreen();
  });

  it("draws bars alone without a legend", async () => {
    await renderThemed(
      <ComboChart
        width={320}
        labels={["6 Oct", "13 Oct"]}
        bars={[12, 0]}
        format={String}
        legend={["Sets", "none"]}
        summary="12 sets"
        table={{ columns: ["Week", "Sets"], rows: [] }}
      />,
    );
    expect(screen.getAllByTestId("bar")).toHaveLength(2);
    expect(screen.queryAllByTestId("dot")).toHaveLength(0);
    expect(screen.queryByText("Sets")).toBeNull();
  });

  snapshots(() => chart);
});
