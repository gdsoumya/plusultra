import { fireEvent, screen } from "@testing-library/react-native";
import { renderThemed, snapshots } from "../../test-utils";
import { LineChart } from "../LineChart";

const points = [
  { x: 0, y: 84.4 },
  { x: 3, y: 84.0 },
  { x: 7, y: 83.1 },
];
const trend = [
  { x: 0, y: 84.6 },
  { x: 3, y: 84.3 },
  { x: 7, y: 83.6 },
];

const chart = (
  <LineChart
    width={320}
    points={points}
    trend={trend}
    xDomain={[0, 7]}
    xLabels={["Nov 1", "Nov 8"]}
    format={(v) => v.toFixed(1)}
    legend={["Weigh-ins", "7-day average"]}
    summary="3 weigh-ins, trending down"
    table={{ columns: ["Date", "kg"], rows: [["Nov 1", "84.4"]] }}
  />
);

describe("LineChart", () => {
  it("draws a dot per point and labels the series", async () => {
    await renderThemed(chart);
    expect(screen.getByLabelText("3 weigh-ins, trending down")).toBeOnTheScreen();
    expect(screen.getAllByTestId("dot")).toHaveLength(3);
    expect(screen.getByText("Weigh-ins")).toBeOnTheScreen();
    expect(screen.getByText("7-day average")).toBeOnTheScreen();
    expect(screen.getByText("Nov 8")).toBeOnTheScreen();
  });

  it("toggles to a table and back", async () => {
    await renderThemed(chart);
    await fireEvent.press(screen.getByRole("button", { name: "Show table" }));
    expect(screen.getByText("84.4")).toBeOnTheScreen();
    expect(screen.queryByLabelText("3 weigh-ins, trending down")).toBeNull();
    await fireEvent.press(screen.getByRole("button", { name: "Show chart" }));
    expect(screen.getByLabelText("3 weigh-ins, trending down")).toBeOnTheScreen();
  });

  snapshots(() => chart);
});
