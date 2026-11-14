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

  it("marks the nearest point on a tap and captions it", async () => {
    const onOpen = jest.fn();
    await renderThemed(
      <LineChart
        width={320}
        points={points}
        xDomain={[0, 7]}
        xLabels={["Nov 1", "Nov 8"]}
        format={(v) => v.toFixed(1)}
        xLabel={(x) => `Day ${x}`}
        onOpen={onOpen}
        summary="3 weigh-ins"
        table={{ columns: ["Date", "kg"], rows: [] }}
        testID="chart"
      />,
    );
    expect(screen.queryByTestId("marker")).toBeNull();
    // The plot runs from x=40 to x=312; 150 is nearest the middle point.
    await fireEvent.press(screen.getByTestId("chart-plot"), { nativeEvent: { locationX: 150 } });
    expect(screen.getByTestId("marker")).toBeOnTheScreen();
    expect(screen.getByText("Day 3 · 84.0")).toBeOnTheScreen();
    await fireEvent.press(screen.getByRole("button", { name: "Open workout" }));
    expect(onOpen).toHaveBeenCalledWith(1);
    // Tapping the same point again clears it; so does the caption's close.
    await fireEvent.press(screen.getByTestId("chart-plot"), { nativeEvent: { locationX: 150 } });
    expect(screen.queryByTestId("marker")).toBeNull();
    await fireEvent.press(screen.getByTestId("chart-plot"), { nativeEvent: { locationX: 0 } });
    expect(screen.getByText("Day 0 · 84.4")).toBeOnTheScreen();
    await fireEvent.press(screen.getByRole("button", { name: "Clear selection" }));
    expect(screen.queryByTestId("marker")).toBeNull();
  });

  snapshots(() => chart);
});
