import { fireEvent, render, screen, waitFor, within } from "@testing-library/react-native";

import { createClients, type Api } from "../../api/clients";
import { createMockTransport } from "../../api/mockTransport";

import { checkinsRepo } from "../../data/repositories/checkins";
import { dayLogsRepo } from "../../data/repositories/dayLogs";
import { resolvedDaysRepo } from "../../data/repositories/resolvedDays";
import type { Store } from "../../data/store";
import { StoreProvider } from "../../data/StoreProvider";
import { dateLabel } from "../../domain/dates";
import { ProgramStatus } from "../../gen/plusultra/v1/program_pb";
import { colors } from "../../ui/tokens";
import { ThemeProvider } from "../../ui";
import { pullToRefresh } from "../../ui/test-utils";
import CalendarScreen from "./CalendarScreen";
import { makeFloating, seedStore } from "../../data/seedStore";

const mockPush = jest.fn();
let mockApi: Api;
jest.mock("../../api/useApi", () => ({ useApi: () => mockApi }));
const mockSync = { status: "idle", pendingCount: 0, reasons: {}, syncNow: jest.fn(async () => {}) };
jest.mock("../../data/useSync", () => ({ useSync: () => mockSync }));
jest.mock("expo-router", () => ({
  useRouter: () => ({ push: mockPush }),
  useFocusEffect: jest.fn(),
}));

const today = "2026-11-11";
let store: Store;

beforeEach(async () => {
  mockPush.mockClear();
  store = await seedStore(today);
  mockApi = createClients(createMockTransport({ today }).transport);
});

const renderCalendar = () =>
  render(
    <ThemeProvider>
      <StoreProvider store={store}>
        <CalendarScreen today={today} />
      </StoreProvider>
    </ThemeProvider>,
  );

const cell = (date: string) =>
  screen.getByRole("button", { name: new RegExp(`^${dateLabel(date)}(,|$)`) });

describe("CalendarScreen", () => {
  it("marks logged days by status and outlines today", async () => {
    const logs = dayLogsRepo(store);
    const [rested] = logs.range("2026-11-01", "2026-11-01");
    store.put("dayLogs", { ...rested, status: "rest" });
    await renderCalendar();
    for (const status of ["done", "skipped", "partial", "rest"] as const) {
      const log = logs.range("2026-10-26", "2026-11-10").find((l) => l.status === status)!;
      expect(cell(log.date)).toHaveAccessibleName(new RegExp(`, ${status}`));
    }
    expect(cell(today)).toHaveAccessibleName(/, partial, today$/);
  });

  it("marks check-ins with their own blue dot, told apart from rest in the legend", async () => {
    await renderCalendar();
    const [{ date }] = checkinsRepo(store).range("2026-10-26", "2026-11-11");
    expect(cell(date)).toHaveAccessibleName(/check-in/);
    expect(within(cell(date)).getByTestId("checkin-marker")).toHaveStyle({
      backgroundColor: colors.info,
    });
    expect(screen.getByText("check-in")).toBeOnTheScreen();
  });

  it("marks past planned days from the server when the pull has not cached them", async () => {
    const past = resolvedDaysRepo(store)
      .range("2026-11-01", "2026-11-10")
      .find((d) => d.payload.exercises.length)!;
    // Nothing local says anything about the date: no log, record or cached plan.
    dayLogsRepo(store)
      .byDate(past.date!)
      .forEach((l) => store.delete("dayLogs", l.id));
    store
      .list("workoutRecords", { dateTo: "2026-11-10" })
      .forEach((r) => store.delete("workoutRecords", r.id));
    resolvedDaysRepo(store)
      .range("2026-10-01", "2026-11-10")
      .forEach((d) => store.delete("resolvedDays", d.id));
    await renderCalendar();
    await waitFor(() => expect(cell(past.date!)).toHaveAccessibleName(/, planned(,|$)/));
  });

  it("plans future days for an anchored program only", async () => {
    await renderCalendar();
    const [future] = resolvedDaysRepo(store).range("2026-11-12", "2026-11-30");
    expect(cell(future.date!)).toHaveAccessibleName(/, planned$/);
    expect(screen.queryByText(/Next up/)).toBeNull();
  });

  it("shows a floating program's next day as a hint instead of dates", async () => {
    makeFloating(store);
    // The mock server must agree, or its calendar plans the anchored program's dates.
    const { programs } = await mockApi.programs.listPrograms({});
    const active = programs.find((p) => p.status === ProgramStatus.ACTIVE)!;
    await mockApi.programs.finishProgram({ id: active.id });
    await mockApi.programs.activateProgram({ id: "program-gzclp" });
    await renderCalendar();
    expect(cell("2026-11-12")).toHaveAccessibleName(dateLabel("2026-11-12"));
    expect(screen.getByText(/Next up: GZCLP A1 \(week 2, day 1\)/)).toBeOnTheScreen();
  });

  it("syncs on pull-to-refresh and shows what the sync stored", async () => {
    const logs = dayLogsRepo(store);
    const [log] = logs.range("2026-11-01", "2026-11-01");
    mockSync.syncNow.mockImplementationOnce(async () => {
      store.put("dayLogs", { ...log, status: "rest" });
    });
    await renderCalendar();
    expect(cell(log.date)).not.toHaveAccessibleName(/, rest/);
    await pullToRefresh();
    expect(mockSync.syncNow).toHaveBeenCalledTimes(1);
    expect(cell(log.date)).toHaveAccessibleName(/, rest/);
  });

  it("opens a day on tap", async () => {
    await renderCalendar();
    await fireEvent.press(cell("2026-11-12"));
    expect(mockPush).toHaveBeenCalledWith("/day/2026-11-12");
  });

  it("moves between months", async () => {
    await renderCalendar();
    expect(screen.getByText("November 2026")).toBeOnTheScreen();
    await fireEvent.press(screen.getByRole("button", { name: "Next month" }));
    expect(screen.getByText("December 2026")).toBeOnTheScreen();
    await fireEvent.press(screen.getByRole("button", { name: "Previous month" }));
    await fireEvent.press(screen.getByRole("button", { name: "Previous month" }));
    expect(screen.getByText("October 2026")).toBeOnTheScreen();
  });
});
