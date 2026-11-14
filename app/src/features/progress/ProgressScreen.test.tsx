import { create } from "@bufbuild/protobuf";
import { Code, ConnectError } from "@connectrpc/connect";
import { fireEvent, render, screen, within } from "@testing-library/react-native";
import { Dimensions } from "react-native";

import { createClients, type Api } from "../../api/clients";
import { OFFLINE } from "../../api/errors";
import { fromProto } from "../../api/convert";
import { addDays, mondayOf } from "../../domain/dates";
import { createMockTransport } from "../../api/mockTransport";
import { createMemoryStore } from "../../data/memoryStore";
import { checkinsRepo } from "../../data/repositories/checkins";
import { mealLogsRepo } from "../../data/repositories/mealLogs";
import { workoutRecordsRepo } from "../../data/repositories/workoutRecords";
import type { Store } from "../../data/store";
import {
  AdherenceResponseSchema,
  LiftHistoryResponseSchema,
  WeeklySummaryResponseSchema,
  WeeklyTrendResponseSchema,
  WeightTrendResponseSchema,
} from "../../gen/plusultra/v1/insight_pb";
import { maxWidth, space, ThemeProvider } from "../../ui";
import { pullToRefresh } from "../../ui/test-utils";
import ProgressScreen from "./ProgressScreen";

const today = "2026-11-11";
const mockSync = { status: "idle", pendingCount: 0, reasons: {}, syncNow: jest.fn(async () => {}) };
jest.mock("../../data/useSync", () => ({ useSync: () => mockSync }));
const mockRouter = { push: jest.fn() };
jest.mock("expo-router", () => ({ useRouter: () => mockRouter }));
const offline = () => new ConnectError("offline", Code.Unavailable);

async function setup(store: Store = createMemoryStore()) {
  const mock = createMockTransport({ today });
  const api = createClients(mock.transport);
  const rows = fromProto.syncRows((await api.sync.pull({})).rows, "user-1");
  workoutRecordsRepo(store).upsertFromServer(rows.workoutRecords);
  mealLogsRepo(store).upsertFromServer(rows.mealLogs);
  checkinsRepo(store).upsertFromServer(rows.checkins);
  return { mock, api, store };
}

const mount = (insights: Api["insights"], store: Store, on = today) =>
  render(
    <ThemeProvider>
      <ProgressScreen insights={insights} store={store} today={on} unit="kg" />
    </ThemeProvider>,
  );

const card = (testID: string) => within(screen.getByTestId(testID));

// Opens the range sheet; returns the queries scoped to it.
async function openRanges() {
  await fireEvent.press(screen.getByTestId("progress-range"));
  return within(await screen.findByTestId("progress-range-sheet"));
}

describe("ProgressScreen", () => {
  it("shows the weight trend with its 7-day average and weekly rate", async () => {
    const { api, store } = await setup();
    await mount(api.insights, store);
    const summary = await screen.findByLabelText(/22 weigh-ins/);
    expect(summary.props.accessibilityLabel).toMatch(/7-day average 83 kg, -0.5 kg a week/);
    const weight = card("progress-weight");
    expect(weight.getByText("83.0")).toBeOnTheScreen();
    expect(weight.getByText("-0.5")).toBeOnTheScreen();
    await fireEvent.press(weight.getByRole("button", { name: "Show table" }));
    expect(weight.getAllByText("84.4").length).toBeGreaterThan(0);
  });

  it("syncs on pull-to-refresh, then fetches the insights again", async () => {
    const { api, store } = await setup();
    const trend = jest.spyOn(api.insights, "weightTrend");
    await mount(api.insights, store);
    await screen.findByLabelText(/22 weigh-ins/);
    expect(trend).toHaveBeenCalledTimes(1);
    await pullToRefresh();
    expect(mockSync.syncNow).toHaveBeenCalledTimes(1);
    await screen.findByLabelText(/22 weigh-ins/);
    expect(trend).toHaveBeenCalledTimes(2);
  });

  it("keeps the charts inside the screen column on a wide window", async () => {
    const phone = { window: Dimensions.get("window"), screen: Dimensions.get("screen") };
    const desktop = { width: 1920, height: 1080, scale: 1, fontScale: 1 };
    Dimensions.set({ window: desktop, screen: desktop });
    try {
      const { api, store } = await setup();
      await mount(api.insights, store);
      await screen.findByLabelText(/22 weigh-ins/);
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

  it("charts intake against weight by day over four weeks, from the local logs", async () => {
    const { api, store } = await setup();
    await mount(api.insights, store);
    const nutrition = card("progress-nutrition");
    const from = addDays(today, -27);
    const logged = store.list("mealLogs", { dateFrom: from, dateTo: today });
    const weighIns = store.list("checkins", { dateFrom: from, dateTo: today });
    expect(logged.length).toBeGreaterThan(0);
    const summary = await nutrition.findByLabelText(/days with meals averaging/);
    expect(summary.props.accessibilityLabel).toMatch(
      new RegExp(
        `^${logged.length} days with meals averaging \\d+ kcal and \\d+ g protein a day; weight -?\\d+(\\.\\d)? kg across ${weighIns.length} weighings`,
      ),
    );
    expect(nutrition.getByLabelText(/^Protein a day: \d+ g$/)).toBeOnTheScreen();
    expect(nutrition.getByLabelText(/^Weight change: -?\d+(\.\d)? kg$/)).toBeOnTheScreen();
    // One bar per logged day; days without a log leave a gap.
    expect(nutrition.getAllByTestId("bar")).toHaveLength(logged.length);
    await fireEvent.press(nutrition.getByRole("button", { name: "Show table" }));
    expect(nutrition.getByText("Day")).toBeOnTheScreen();
  });

  it("buckets intake and weight by week beyond four weeks", async () => {
    const { api, store } = await setup();
    await mount(api.insights, store);
    const sheet = await openRanges();
    await fireEvent.press(sheet.getByLabelText("3 months"));
    const nutrition = card("progress-nutrition");
    await nutrition.findByLabelText(/days with meals averaging/);
    const weeks = new Set(
      store
        .list("mealLogs", { dateFrom: addDays(today, -91), dateTo: today })
        .map((l) => mondayOf(l.date)),
    );
    expect(nutrition.getAllByTestId("bar")).toHaveLength(weeks.size);
    await fireEvent.press(nutrition.getByRole("button", { name: "Show table" }));
    expect(nutrition.getByText("Week")).toBeOnTheScreen();
  });

  it("charts a muscle group's sets per week, all groups by default", async () => {
    const { api, store } = await setup();
    await mount(api.insights, store);
    const muscles = card("progress-muscles");
    const all = await muscles.findByLabelText(/^All: \d+ working sets over 5 weeks/);
    expect(all).toBeOnTheScreen();
    expect(muscles.getByLabelText("All")).toBeChecked();
    expect(muscles.queryByLabelText("Neck")).toBeNull();
    await fireEvent.press(muscles.getByLabelText("Quadriceps"));
    expect(muscles.getByLabelText(/^Quadriceps: \d+ working sets/)).toBeOnTheScreen();
    expect(muscles.getAllByTestId("bar")).toHaveLength(5);
  });

  it("shows the training days as a heatmap with the weekly rate and streak", async () => {
    const { api, store } = await setup();
    await mount(api.insights, store);
    const training = card("progress-training");
    const grid = await training.findByLabelText(/training days from/);
    const trained = training.getAllByTestId("trained").length;
    expect(trained).toBeGreaterThan(5);
    expect(grid.props.accessibilityLabel).toMatch(new RegExp(`^${trained} training days`));
    expect(await training.findByLabelText(/^Workouts per week: \d+(\.\d)?$/)).toBeOnTheScreen();
    // Every week of the last month has a finished workout; today's is still open.
    expect(training.getByLabelText(/^Week streak: [4-6]$/)).toBeOnTheScreen();
  });

  it("applies a custom range and remembers it", async () => {
    const { api, store } = await setup();
    const trend = jest.spyOn(api.insights, "weeklyTrend");
    const first = await mount(api.insights, store);
    await screen.findByLabelText(/22 weigh-ins/);
    let sheet = await openRanges();
    await fireEvent.press(sheet.getByLabelText("Custom"));
    await fireEvent.press(sheet.getByRole("button", { name: "Previous month" }));
    expect(sheet.getByText("October 2026")).toBeOnTheScreen();
    await fireEvent.press(sheet.getByRole("button", { name: "Thursday 15 October" }));
    // A tap before From restarts the span.
    await fireEvent.press(sheet.getByRole("button", { name: "Thursday 1 October" }));
    expect(sheet.getByText(/^From (Oct 1|1 Oct) – To …$/)).toBeOnTheScreen();
    expect(sheet.getByRole("button", { name: "Apply" })).toBeDisabled();
    await fireEvent.press(sheet.getByRole("button", { name: "Saturday 31 October" }));
    expect(sheet.getByText(/^From (Oct 1|1 Oct) – To (Oct 31|31 Oct)$/)).toBeOnTheScreen();
    await fireEvent.press(sheet.getByRole("button", { name: "Apply" }));
    const dates = { from: "2026-10-01", to: "2026-10-31" };
    expect(trend).toHaveBeenLastCalledWith({ dates });
    const label = /Oct 1 – Oct 31|1 Oct – 31 Oct/;
    expect(within(screen.getByTestId("progress-range")).getByText(label)).toBeOnTheScreen();
    await first.unmount();
    await mount(api.insights, store);
    expect(within(screen.getByTestId("progress-range")).getByText(label)).toBeOnTheScreen();
    sheet = await openRanges();
    expect(sheet.getByLabelText("Custom")).toBeChecked();
    expect(sheet.getByRole("button", { name: "Thursday 15 October" })).toBeSelected();
    await fireEvent.press(sheet.getByLabelText("4 weeks"));
    expect(trend).toHaveBeenLastCalledWith({ dates: { from: "2026-10-15", to: today } });
  });

  it("plots the lift's max weight, volume or reps instead of its e1RM", async () => {
    const { api, store } = await setup();
    await mount(api.insights, store);
    const lifts = card("progress-lifts");
    await fireEvent.press(await lifts.findByLabelText("Deadlift"));
    expect(
      await lifts.findByLabelText(/^Deadlift: \d+ sessions, e1RM [\d.]+ kg/),
    ).toBeOnTheScreen();
    await fireEvent.press(lifts.getByLabelText("Volume"));
    expect(lifts.getByLabelText(/^Deadlift: \d+ sessions, Volume [\d.]+ kg/)).toBeOnTheScreen();
    await fireEvent.press(lifts.getByLabelText("Max reps"));
    expect(lifts.getByLabelText(/^Deadlift: \d+ sessions, Max reps \d+ reps/)).toBeOnTheScreen();
    await fireEvent.press(lifts.getByRole("button", { name: "Show table" }));
    // The chip and the table's column.
    expect(lifts.getAllByText("Max reps")).toHaveLength(2);
  });

  it("shows the lift's all-time records and opens a tapped session", async () => {
    const { api, store } = await setup();
    const history = jest.spyOn(api.insights, "liftHistory");
    await mount(api.insights, store);
    const lifts = card("progress-lifts");
    await fireEvent.press(await lifts.findByLabelText("Deadlift"));
    expect(
      await lifts.findByLabelText(
        /^All-time e1RM \(kg\) · \w+ \d+|^All-time e1RM \(kg\) · \d+ \w+/,
      ),
    ).toBeOnTheScreen();
    expect(lifts.getByLabelText(/^All-time max \(kg\) · .*: [\d.]+ × \d+$/)).toBeOnTheScreen();
    expect(history).toHaveBeenCalledWith({ slug: "deadlift-barbell" });
    await fireEvent.press(lifts.getByTestId("lift-chart-plot"), {
      nativeEvent: { locationX: 300 },
    });
    await fireEvent.press(lifts.getByRole("button", { name: "Open workout" }));
    expect(mockRouter.push).toHaveBeenCalledWith(
      expect.stringMatching(/^\/session\/workout-2026-\d\d-\d\d$/),
    );
  });

  it("counts planned, done, partial and skipped days", async () => {
    const { api, store } = await setup();
    await mount(api.insights, store);
    const adherence = card("progress-adherence");
    const { counts } = await api.insights.adherence({
      dates: { from: addDays(today, -27), to: today },
    });
    expect(await adherence.findByText(String(counts!.planned))).toBeOnTheScreen();
    for (const n of [counts!.done, counts!.partial, counts!.skipped]) {
      expect(adherence.getAllByText(String(n)).length).toBeGreaterThan(0);
    }
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
    await screen.findByLabelText(/22 weigh-ins/);
    await first.unmount();

    mock.failNext("insights.weightTrend", offline());
    await mount(api.insights, store);
    const weight = card("progress-weight");
    expect(await weight.findByText(/Offline · last updated/)).toBeOnTheScreen();
    expect(weight.getByLabelText(/22 weigh-ins/)).toBeOnTheScreen();
  });

  it("offers a retry when nothing is cached", async () => {
    const { mock, api, store } = await setup();
    mock.failNext("insights.weightTrend", offline());
    await mount(api.insights, store);
    const weight = card("progress-weight");
    expect(await weight.findByText("Couldn't load")).toBeOnTheScreen();
    expect(weight.getByText(OFFLINE)).toBeOnTheScreen();
    await fireEvent.press(weight.getByRole("button", { name: "Retry" }));
    expect(await weight.findByLabelText(/22 weigh-ins/)).toBeOnTheScreen();
  });

  describe("without data", () => {
    const insights: Api["insights"] = {
      weightTrend: jest.fn(async () => create(WeightTrendResponseSchema)),
      liftHistory: jest.fn(async () => create(LiftHistoryResponseSchema)),
      weeklySummary: jest.fn(async () => create(WeeklySummaryResponseSchema)),
      adherence: jest.fn(async () => create(AdherenceResponseSchema)),
      weeklyTrend: jest.fn(async () => create(WeeklyTrendResponseSchema)),
    };

    it("shows an empty state per section", async () => {
      await mount(insights, createMemoryStore());
      expect(await screen.findByText("No weigh-ins yet")).toBeOnTheScreen();
      expect(screen.getByText("No lifts logged yet")).toBeOnTheScreen();
      expect(screen.getByText("No sets this week")).toBeOnTheScreen();
      expect(screen.getByText("Nothing planned yet")).toBeOnTheScreen();
      expect(screen.getByText("No meals or weigh-ins yet")).toBeOnTheScreen();
      expect(screen.getByText("No working sets in this range")).toBeOnTheScreen();
      expect(screen.getByLabelText(/^0 training days/)).toBeOnTheScreen();
      expect(screen.getByLabelText("Week streak: 0")).toBeOnTheScreen();
      expect(insights.weeklySummary).toHaveBeenCalledWith({ weekStart: "2026-11-09" });
    });

    it("says when the lifts fall outside the range", async () => {
      const { store } = await setup();
      await mount(insights, store, "2027-06-01");
      expect(await screen.findByText("No lifts in this range")).toBeOnTheScreen();
      expect(
        screen.getByText("Pick a longer range, or finish a workout with loaded sets."),
      ).toBeOnTheScreen();
    });

    it("re-queries the range from the picker", async () => {
      await mount(insights, createMemoryStore());
      await screen.findByText("No weigh-ins yet");
      const sheet = await openRanges();
      await fireEvent.press(sheet.getByLabelText("3 months"));
      const dates = { from: "2026-08-13", to: today };
      expect(insights.weightTrend).toHaveBeenLastCalledWith({ dates });
      expect(insights.adherence).toHaveBeenLastCalledWith({ dates });
      expect(insights.weeklyTrend).toHaveBeenLastCalledWith({ dates });
      expect(screen.queryByTestId("progress-range-sheet")).toBeNull();
    });
  });
});
