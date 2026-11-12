import { create } from "@bufbuild/protobuf";
import { Code, ConnectError } from "@connectrpc/connect";
import { fireEvent, render, screen, within } from "@testing-library/react-native";
import { Dimensions } from "react-native";

import { createClients, type Api } from "../../api/clients";
import { OFFLINE } from "../../api/errors";
import { fromProto } from "../../api/convert";
import { createMockTransport } from "../../api/mockTransport";
import { createMemoryStore } from "../../data/memoryStore";
import { workoutRecordsRepo } from "../../data/repositories/workoutRecords";
import type { Store } from "../../data/store";
import {
  AdherenceResponseSchema,
  LiftHistoryResponseSchema,
  WeeklySummaryResponseSchema,
  WeightTrendResponseSchema,
} from "../../gen/plusultra/v1/insight_pb";
import { maxWidth, space, ThemeProvider } from "../../ui";
import { pullToRefresh } from "../../ui/test-utils";
import ProgressScreen from "./ProgressScreen";

const today = "2026-11-11";
const mockSync = { status: "idle", pendingCount: 0, reasons: {}, syncNow: jest.fn(async () => {}) };
jest.mock("../../data/useSync", () => ({ useSync: () => mockSync }));
const offline = () => new ConnectError("offline", Code.Unavailable);

async function setup(store: Store = createMemoryStore()) {
  const mock = createMockTransport({ today });
  const api = createClients(mock.transport);
  const rows = fromProto.syncRows((await api.sync.pull({})).rows, "user-1");
  workoutRecordsRepo(store).upsertFromServer(rows.workoutRecords);
  return { mock, api, store };
}

const mount = (insights: Api["insights"], store: Store) =>
  render(
    <ThemeProvider>
      <ProgressScreen insights={insights} store={store} today={today} unit="kg" />
    </ThemeProvider>,
  );

const card = (testID: string) => within(screen.getByTestId(testID));

describe("ProgressScreen", () => {
  it("shows the weight trend with its 7-day average and weekly rate", async () => {
    const { api, store } = await setup();
    await mount(api.insights, store);
    const summary = await screen.findByLabelText(/23 weigh-ins/);
    expect(summary.props.accessibilityLabel).toMatch(/7-day average 83 kg, -0.4 kg a week/);
    const weight = card("progress-weight");
    expect(weight.getByText("83.0")).toBeOnTheScreen();
    expect(weight.getByText("-0.4")).toBeOnTheScreen();
    await fireEvent.press(weight.getByRole("button", { name: "Show table" }));
    expect(weight.getAllByText("84.4").length).toBeGreaterThan(0);
  });

  it("syncs on pull-to-refresh, then fetches the insights again", async () => {
    const { api, store } = await setup();
    const trend = jest.spyOn(api.insights, "weightTrend");
    await mount(api.insights, store);
    await screen.findByLabelText(/23 weigh-ins/);
    expect(trend).toHaveBeenCalledTimes(1);
    await pullToRefresh();
    expect(mockSync.syncNow).toHaveBeenCalledTimes(1);
    await screen.findByLabelText(/23 weigh-ins/);
    expect(trend).toHaveBeenCalledTimes(2);
  });

  it("keeps the charts inside the screen column on a wide window", async () => {
    const phone = { window: Dimensions.get("window"), screen: Dimensions.get("screen") };
    const desktop = { width: 1920, height: 1080, scale: 1, fontScale: 1 };
    Dimensions.set({ window: desktop, screen: desktop });
    try {
      const { api, store } = await setup();
      await mount(api.insights, store);
      await screen.findByLabelText(/23 weigh-ins/);
      const dots = card("progress-weight").getAllByTestId("dot");
      const right = Math.max(...dots.map((d) => Number(d.props.cx)));
      // The column holds a card, whose padding holds the chart.
      expect(right).toBeLessThanOrEqual(maxWidth.wide - 2 * space[4]);
    } finally {
      Dimensions.set(phone);
    }
  });

  it("switches the lift when a chip is pressed", async () => {
    const { api, store } = await setup();
    await mount(api.insights, store);
    const lifts = card("progress-lifts");
    // Chips are matched by label so the test holds across the Chip role change in app@829e9e0.
    expect(await lifts.findByLabelText("Deadlift")).toBeOnTheScreen();
    expect(lifts.queryByLabelText("Jump Rope")).toBeNull();
    await fireEvent.press(lifts.getByLabelText("Squat"));
    expect(await lifts.findByLabelText(/^Squat: /)).toBeOnTheScreen();
  });

  it("counts planned, done, partial and skipped days", async () => {
    const { api, store } = await setup();
    await mount(api.insights, store);
    const adherence = card("progress-adherence");
    expect(await adherence.findByText("22")).toBeOnTheScreen();
    for (const n of ["15", "4", "2"]) expect(adherence.getByText(n)).toBeOnTheScreen();
  });

  it("shows this week's sets per muscle, largest first", async () => {
    const { api, store } = await setup();
    await mount(api.insights, store);
    const volume = card("progress-volume");
    expect(await volume.findByLabelText(/38 working sets/)).toBeOnTheScreen();
    expect(volume.getByText("Calves")).toBeOnTheScreen();
    expect(volume.getByText(/^Glutes: glute max \d+$/)).toBeOnTheScreen();
    expect(volume.getByText("Lower back")).toBeOnTheScreen();
    expect(volume.queryByText("Forearms")).toBeNull();
  });

  it("keeps showing cached data with a note when the server is unreachable", async () => {
    const { mock, api, store } = await setup();
    const first = await mount(api.insights, store);
    await screen.findByLabelText(/23 weigh-ins/);
    await first.unmount();

    mock.failNext("insights.weightTrend", offline());
    await mount(api.insights, store);
    const weight = card("progress-weight");
    expect(await weight.findByText(/Offline · last updated/)).toBeOnTheScreen();
    expect(weight.getByLabelText(/23 weigh-ins/)).toBeOnTheScreen();
  });

  it("offers a retry when nothing is cached", async () => {
    const { mock, api, store } = await setup();
    mock.failNext("insights.weightTrend", offline());
    await mount(api.insights, store);
    const weight = card("progress-weight");
    expect(await weight.findByText("Couldn't load")).toBeOnTheScreen();
    expect(weight.getByText(OFFLINE)).toBeOnTheScreen();
    await fireEvent.press(weight.getByRole("button", { name: "Retry" }));
    expect(await weight.findByLabelText(/23 weigh-ins/)).toBeOnTheScreen();
  });

  describe("without data", () => {
    const insights: Api["insights"] = {
      weightTrend: jest.fn(async () => create(WeightTrendResponseSchema)),
      liftHistory: jest.fn(async () => create(LiftHistoryResponseSchema)),
      weeklySummary: jest.fn(async () => create(WeeklySummaryResponseSchema)),
      adherence: jest.fn(async () => create(AdherenceResponseSchema)),
    };

    it("shows an empty state per section", async () => {
      await mount(insights, createMemoryStore());
      expect(await screen.findByText("No weigh-ins yet")).toBeOnTheScreen();
      expect(screen.getByText("No lifts logged yet")).toBeOnTheScreen();
      expect(screen.getByText("No sets this week")).toBeOnTheScreen();
      expect(screen.getByText("Nothing planned yet")).toBeOnTheScreen();
      expect(insights.weeklySummary).toHaveBeenCalledWith({ weekStart: "2026-11-09" });
    });

    it("re-queries the range from the picker", async () => {
      await mount(insights, createMemoryStore());
      await screen.findByText("No weigh-ins yet");
      await fireEvent.press(screen.getByLabelText("3 months"));
      const dates = { from: "2026-08-12", to: today };
      expect(insights.weightTrend).toHaveBeenLastCalledWith({ dates });
      expect(insights.adherence).toHaveBeenLastCalledWith({ dates });
    });
  });
});
