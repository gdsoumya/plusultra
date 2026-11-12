import { createClients } from "../../api/clients";
import { fromProto } from "../../api/convert";
import { createMockTransport } from "../../api/mockTransport";
import { fireEvent, render, screen } from "@testing-library/react-native";

import { plannedDays } from "../../data/plans";
import { checkinsRepo } from "../../data/repositories/checkins";
import { dayLogsRepo } from "../../data/repositories/dayLogs";
import { mealLogsRepo } from "../../data/repositories/mealLogs";
import { programsRepo } from "../../data/repositories/programs";
import { resolvedDaysRepo } from "../../data/repositories/resolvedDays";
import { workoutRecordsRepo } from "../../data/repositories/workoutRecords";
import type { Store } from "../../data/store";
import { StoreProvider } from "../../data/StoreProvider";
import { ThemeProvider } from "../../ui";
import { pullToRefresh } from "../../ui/test-utils";
import DayScreen from "./DayScreen";
import { seedStore } from "../../data/seedStore";

const mockPush = jest.fn();
const mockSync = { status: "idle", pendingCount: 0, reasons: {}, syncNow: jest.fn(async () => {}) };
jest.mock("../../data/useSync", () => ({ useSync: () => mockSync }));
jest.mock("expo-router", () => ({
  useRouter: () => ({ push: mockPush, back: jest.fn(), canGoBack: () => true }),
  useFocusEffect: () => {},
}));
jest.mock("../../data/uuid", () => ({ uuid: () => "log-new" }));

const today = "2026-11-11";
const mockApi = createClients(createMockTransport({ today: "2026-11-11" }).transport);
jest.mock("../../api/useApi", () => ({ useApi: () => mockApi }));
let store: Store;

beforeEach(async () => {
  mockPush.mockClear();
  store = await seedStore(today);
});

const renderDay = (date: string) =>
  render(
    <ThemeProvider>
      <StoreProvider store={store}>
        <DayScreen date={date} today={today} unit="kg" userId="user-1" />
      </StoreProvider>
    </ThemeProvider>,
  );

const mealDay = {
  week: 3,
  day: 0,
  kind: "training" as const,
  name: "Training day",
  desc: "",
  target: { kcal: 2500, protein: 170, carbs: 300, fat: 70, fiber: 30, waterMl: 3500 },
  meals: [
    {
      name: "Breakfast",
      items: [
        {
          foodName: "Oats",
          qty: 80,
          unit: "g",
          planned: true,
          macros: { kcal: 300, protein: 10, carbs: 50, fat: 6, fiber: 8, waterMl: 0 },
        },
      ],
    },
  ],
};

const putMealDay = (date: string) =>
  store.put("mealDays", {
    id: `meals:${date}`,
    programId: "program-meals",
    week: 3,
    day: 0,
    date,
    payload: mealDay,
  });

// A past planned training day the fixture user skipped: a plan, a skipped log, no record.
const skippedPlannedDate = () =>
  dayLogsRepo(store)
    .range("2026-10-12", "2026-11-10")
    .find((l) => l.status === "skipped" && plannedDays(store, l.date, l.date)[0]?.exercises.length)!
    .date;

const noAddActions = () => {
  expect(screen.queryByRole("button", { name: "Add workout" })).toBeNull();
  expect(screen.queryByRole("button", { name: "Add meals" })).toBeNull();
  expect(screen.queryByRole("button", { name: "Save weight" })).toBeNull();
};

const press = async (name: string | RegExp) =>
  fireEvent.press(await screen.findByRole("button", { name }));

describe("DayScreen", () => {
  it("syncs on pull-to-refresh and shows the status the sync stored", async () => {
    const logs = dayLogsRepo(store);
    const [log] = logs.range("2026-11-01", "2026-11-01");
    workoutRecordsRepo(store)
      .byDate(log.date)
      .forEach((r) => store.delete("workoutRecords", r.id));
    mockSync.syncNow.mockImplementationOnce(async () => {
      store.put("dayLogs", { ...log, status: "rest" });
    });
    await renderDay(log.date);
    expect(screen.getByRole("checkbox", { name: "Rest" })).not.toBeChecked();
    await pullToRefresh();
    expect(mockSync.syncNow).toHaveBeenCalledTimes(1);
    expect(screen.getByRole("checkbox", { name: "Rest" })).toBeChecked();
  });

  it("shows a past day's logs and links to the workout and meals", async () => {
    const record = workoutRecordsRepo(store)
      .range("2026-11-01", "2026-11-10")
      .find((r) => r.finalized)!;
    const meal = mealLogsRepo(store).byDate(record.date)[0];
    await renderDay(record.date);
    const sets = record.entries.flatMap((e) => e.sets);
    const done = sets.filter((s) => s.completed).length;
    expect(screen.getByText(`${done} of ${sets.length} sets done · Done`)).toBeOnTheScreen();
    expect(
      screen.getByText(`${Math.round(meal.totals.kcal)} of ${meal.target!.kcal} kcal`),
    ).toBeOnTheScreen();

    await fireEvent.press(screen.getByRole("button", { name: new RegExp(`^${record.dayName}`) }));
    expect(mockPush).toHaveBeenCalledWith(`/day/${record.date}/workout/${record.id}`);
    await fireEvent.press(screen.getByRole("button", { name: /^Meals, / }));
    expect(mockPush).toHaveBeenCalledWith(`/day/${record.date}/meals`);
    expect(screen.queryByRole("checkbox", { name: "Skipped" })).toBeNull();
  });

  it("shows the plan for a future anchored day", async () => {
    const plan = resolvedDaysRepo(store)
      .range("2026-11-12", "2026-11-30")
      .find((d) => d.payload.kind === "training")!;
    putMealDay(plan.date!);
    await renderDay(plan.date!);
    expect(screen.getByText(plan.payload.name)).toBeOnTheScreen();
    const first = plan.payload.exercises[0];
    const work = first.sets.find((s) => !s.warmup)!;
    expect(screen.getByText(first.name)).toBeOnTheScreen();
    expect(
      screen.getByText(new RegExp(`^\\d+x${work.reps} ${work.weight.value} kg`)),
    ).toBeOnTheScreen();
    expect(screen.getAllByTestId("thumbnail-placeholder")).toHaveLength(
      plan.payload.exercises.length,
    );
    expect(screen.getByText("Target 2500 kcal")).toBeOnTheScreen();
    expect(screen.getByText("300 kcal")).toBeOnTheScreen();
    expect(screen.queryByRole("checkbox", { name: "Skipped" })).toBeNull();
  });

  it("shows an empty state for a future date with nothing on it", async () => {
    await renderDay("2027-01-05");
    expect(screen.getByText("Nothing on this day")).toBeOnTheScreen();
    noAddActions();
  });

  it("offers no add actions on a future planned day", async () => {
    const plan = resolvedDaysRepo(store)
      .range("2026-11-12", "2026-11-30")
      .find((d) => d.payload.kind === "training")!;
    putMealDay(plan.date!);
    await renderDay(plan.date!);
    noAddActions();
  });

  it("adds a past planned day's workout from the program and clears Skipped", async () => {
    const date = skippedPlannedDate();
    const plan = plannedDays(store, date, date)[0];
    await renderDay(date);
    await press("Add workout");
    expect(screen.getByRole("button", { name: "Ad hoc workout" })).toBeOnTheScreen();
    await press("From a program");
    // The date's own day is preselected: its program, week and day.
    expect(
      await screen.findByRole("button", { name: /Planned for this date/ }),
    ).toHaveAccessibleName(
      new RegExp(`^${plan.day}\\. ${plan.name}, Planned for this date · ${plan.exercises[0].name}`),
    );
    expect(screen.getByRole("checkbox", { name: `Week ${plan.week}` })).toBeChecked();
    expect(screen.queryByRole("button", { name: /Breakfast/ })).toBeNull();
    await press("Create");
    const [record] = workoutRecordsRepo(store).byDate(date);
    expect(record).toMatchObject({
      id: "log-new",
      userId: "user-1",
      date,
      programId: "program-strength16",
      week: plan.week,
      day: plan.day,
      dayName: plan.name,
      version: 0,
      finalized: false,
      finalizeRequested: false,
      dirty: true,
    });
    expect(record.startedAt).toBeTruthy();
    expect(record.entries.map((e) => e.name)).toEqual(plan.exercises.map((e) => e.name));
    expect(record.entries.flatMap((e) => e.sets).some((s) => s.completed)).toBe(false);
    expect(dayLogsRepo(store).byDate(date)[0]).toMatchObject({ status: "planned", dirty: true });
    expect(mockPush).toHaveBeenCalledWith(`/day/${date}/workout/log-new`);
  });

  it("fetches the plan of a past date the pull never sent", async () => {
    const date = skippedPlannedDate();
    const plan = plannedDays(store, date, date)[0];
    for (const { id } of resolvedDaysRepo(store).byDate(date)) store.delete("resolvedDays", id);
    await renderDay(date);
    expect(await screen.findByText(plan.name)).toBeOnTheScreen();
    expect(plannedDays(store, date, date)).toHaveLength(1);
  });

  it("picks any program's week and day for a date without a plan", async () => {
    const draft = { ...programsRepo(store).get("program-gzclp")!, status: "draft" as const };
    programsRepo(store).upsertFromServer([draft]);
    await renderDay("2026-01-05");
    await press("Add workout");
    await press("From a program");
    await press(new RegExp(`^${draft.name}`));
    expect(await screen.findByRole("checkbox", { name: "Week 2" })).not.toBeChecked();
    expect(screen.queryByRole("button", { name: /Planned for this date/ })).toBeNull();
    expect(screen.getByRole("button", { name: "Create" })).toBeDisabled();
    await fireEvent.press(screen.getByRole("checkbox", { name: "Week 2" }));
    await press(/^2\. /);
    await press("Create");
    const [record] = workoutRecordsRepo(store).byDate("2026-01-05");
    expect(record).toMatchObject({ programId: draft.id, week: 2, day: 2, dirty: true });
    expect(record.entries.length).toBeGreaterThan(0);
    expect(mockPush).toHaveBeenCalledWith("/day/2026-01-05/workout/log-new");
  });

  it("says a workout finished here is finished before the server finalizes it", async () => {
    const record = workoutRecordsRepo(store)
      .range("2026-11-01", "2026-11-10")
      .find((r) => r.finalized)!;
    workoutRecordsRepo(store).save({ ...record, finalized: false });
    await renderDay(record.date);
    expect(screen.getByRole("button", { name: new RegExp(`^${record.dayName}, .*Finished$`) }));
  });

  it("adds an ad hoc workout on a past unplanned day", async () => {
    await renderDay("2026-01-05");
    await press("Add workout");
    await press("Ad hoc workout");
    const [record] = workoutRecordsRepo(store).byDate("2026-01-05");
    expect(record).toMatchObject({
      id: "log-new",
      userId: "user-1",
      date: "2026-01-05",
      entries: [],
      version: 0,
      finalized: false,
      dirty: true,
    });
    expect(record.programId).toBeUndefined();
    expect(record.startedAt).toBeTruthy();
    expect(mockPush).toHaveBeenCalledWith("/day/2026-01-05/workout/log-new");
    expect(screen.getByRole("button", { name: /^Workout, / })).toBeOnTheScreen();
  });

  it("adds a past day's meals from the program's planned day with every meal disabled", async () => {
    const date = skippedPlannedDate();
    putMealDay(date);
    for (const { id } of mealLogsRepo(store).byDate(date)) store.delete("mealLogs", id);
    await renderDay(date);
    // The plan's meals are not shown until the user picks them.
    expect(screen.getByText("Nothing logged")).toBeOnTheScreen();
    expect(screen.queryByText("Breakfast")).toBeNull();
    await press("Add meals");
    expect(screen.getByRole("button", { name: "Ad hoc meals" })).toBeOnTheScreen();
    await press("From a program");
    expect(
      await screen.findByRole("button", { name: /^Training day, Planned for this date · / }),
    ).toBeOnTheScreen();
    expect(screen.getByRole("checkbox", { name: "Week 3" })).toBeChecked();
    await press("Create");
    const { mealDays } = await mockApi.programs.getResolvedDays({ programId: "program-meals" });
    const planned = mealDays
      .map((d) => fromProto.mealDay(d).payload)
      .find((d) => d.week === 3 && d.kind === "training")!;
    const [log] = mealLogsRepo(store).byDate(date);
    expect(log).toMatchObject({
      id: "log-new",
      userId: "user-1",
      date,
      programId: "program-meals",
      target: planned.target,
      version: 0,
      dirty: true,
    });
    expect(log.meals.map((m) => m.name)).toEqual(planned.meals.map((m) => m.name));
    expect(log.meals.flatMap((m) => m.items).every((i) => i.qty === 0)).toBe(true);
    expect(log.totals.kcal).toBe(0);
    expect(dayLogsRepo(store).byDate(date)[0]).toMatchObject({ status: "planned" });
    expect(mockPush).toHaveBeenCalledWith(`/day/${date}/meals`);
  });

  it("offers a draft meal program's simulated days", async () => {
    const meals = programsRepo(store).get("program-meals")!;
    programsRepo(store).upsertFromServer([{ ...meals, status: "draft", mode: undefined }]);
    for (const { id } of mealLogsRepo(store).byDate("2026-01-05")) store.delete("mealLogs", id);
    await renderDay("2026-01-05");
    await press("Add meals");
    await press("From a program");
    await press(new RegExp(`^${meals.name}, Draft`));
    await fireEvent.press(await screen.findByRole("checkbox", { name: "Week 2" }));
    await press(/^Rest day, /);
    await press("Create");
    const [log] = mealLogsRepo(store).byDate("2026-01-05");
    expect(log).toMatchObject({ programId: meals.id, dirty: true });
    expect(log.meals.length).toBeGreaterThan(0);
    expect(log.target?.kcal).toBe(2200);
  });

  it("adds an empty meal log on a day without a plan and clears Skipped", async () => {
    await renderDay("2026-01-05");
    await fireEvent.press(screen.getByRole("checkbox", { name: "Skipped" }));
    await press("Add meals");
    await press("Ad hoc meals");
    expect(mealLogsRepo(store).byDate("2026-01-05")[0]).toMatchObject({
      id: "log-new",
      date: "2026-01-05",
      meals: [],
      version: 0,
      dirty: true,
    });
    expect(mealLogsRepo(store).byDate("2026-01-05")[0].target).toBeUndefined();
    expect(dayLogsRepo(store).byDate("2026-01-05")[0]).toMatchObject({ status: "planned" });
    expect(screen.queryByRole("checkbox", { name: "Skipped" })).not.toBeChecked();
    expect(mockPush).toHaveBeenCalledWith("/day/2026-01-05/meals");
  });

  it("resets a past day's workout as a tombstone and offers the add options again", async () => {
    const record = workoutRecordsRepo(store).range("2026-11-01", "2026-11-10")[0];
    store.put("workoutRecords", { ...record, finalized: false });
    await renderDay(record.date);
    await press("Workout options");
    await press("Reset day");
    expect(screen.getByText(/Removes this day's workout/)).toBeOnTheScreen();
    await press("Reset");
    expect(store.get("workoutRecords", record.id)).toMatchObject({
      deletedAt: expect.any(String),
      dirty: true,
    });
    expect(workoutRecordsRepo(store).byDate(record.date)).toEqual([]);
    expect(screen.getByRole("button", { name: "Add workout" })).toBeOnTheScreen();
  });

  it("resets a finalized workout too and says the progression stays", async () => {
    const record = workoutRecordsRepo(store)
      .range("2026-11-01", "2026-11-10")
      .find((r) => r.finalized)!;
    await renderDay(record.date);
    await press("Workout options");
    await press("Reset day");
    expect(screen.getByText(/progression it already applied stays/)).toBeOnTheScreen();
    await press("Reset");
    expect(store.get("workoutRecords", record.id)).toMatchObject({
      deletedAt: expect.any(String),
      dirty: true,
    });
    expect(screen.getByRole("button", { name: "Add workout" })).toBeOnTheScreen();
  });

  it("resets a past day's meals as a tombstone", async () => {
    const meal = mealLogsRepo(store).range("2026-11-01", "2026-11-10")[0];
    await renderDay(meal.date);
    await press("Meals options");
    await press("Reset day");
    expect(screen.getByText(/Removes this day's meal log/)).toBeOnTheScreen();
    await press("Reset");
    expect(store.get("mealLogs", meal.id)).toMatchObject({
      deletedAt: expect.any(String),
      dirty: true,
    });
    expect(screen.getByRole("button", { name: "Add meals" })).toBeOnTheScreen();
  });

  it("saves a check-in dated to the day", async () => {
    await renderDay("2026-01-05");
    await fireEvent.changeText(screen.getByLabelText("Weight (kg)"), "80.5");
    await fireEvent.press(screen.getByRole("button", { name: "Save weight" }));
    expect(checkinsRepo(store).byDate("2026-01-05")[0]).toMatchObject({
      id: "log-new",
      userId: "user-1",
      date: "2026-01-05",
      weight: { value: 80.5, unit: "kg" },
      version: 0,
      dirty: true,
    });
    expect(screen.getByText("80.5 kg")).toBeOnTheScreen();
  });

  it("names the day of an earlier weigh-in like the rest of the app", async () => {
    store.put("checkins", {
      id: "checkin-earlier",
      userId: "user-1",
      date: "2026-01-03",
      weight: { value: 80, unit: "kg" },
      measurements: {},
      version: 1,
      updatedAt: "2026-01-03T08:00:00Z",
      deviceId: "device-1",
      dirty: false,
    });
    await renderDay("2026-01-05");
    expect(screen.getByText("Last weighed Sat 3 Jan, not today")).toBeOnTheScreen();
  });

  it("sets a past day's status through the repository", async () => {
    await renderDay("2026-01-05");
    await fireEvent.press(screen.getByRole("checkbox", { name: "Skipped" }));
    expect(dayLogsRepo(store).byDate("2026-01-05")[0]).toMatchObject({
      id: "log-new",
      userId: "user-1",
      status: "skipped",
      version: 0,
      dirty: true,
    });
    expect(screen.getByRole("checkbox", { name: "Skipped" })).toBeChecked();

    await fireEvent.press(screen.getByRole("checkbox", { name: "Rest" }));
    const [log] = dayLogsRepo(store).byDate("2026-01-05");
    expect(log).toMatchObject({ id: "log-new", status: "rest" });
    expect(dayLogsRepo(store).byDate("2026-01-05")).toHaveLength(1);
  });

  it("keeps an existing log's id and notes when its status changes", async () => {
    const skipped = dayLogsRepo(store)
      .range("2026-11-01", "2026-11-10")
      .find((l) => l.status === "skipped")!;
    const log = { ...skipped, notes: "Tired" };
    store.put("dayLogs", log);
    await renderDay(log.date);
    await fireEvent.press(screen.getByRole("checkbox", { name: "Rest" }));
    expect(dayLogsRepo(store).byDate(log.date)[0]).toMatchObject({
      id: log.id,
      notes: "Tired",
      status: "rest",
      version: log.version,
    });
  });
});
