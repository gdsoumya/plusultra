import { createClients } from "../../api/clients";
import { createMockTransport } from "../../api/mockTransport";
import { fireEvent, render, screen, within } from "@testing-library/react-native";

import { createMemoryStore } from "../../data/memoryStore";
import { checkinsRepo } from "../../data/repositories/checkins";
import { dayLogsRepo } from "../../data/repositories/dayLogs";
import { mealLogsRepo } from "../../data/repositories/mealLogs";
import { programsRepo } from "../../data/repositories/programs";
import { mealDaysRepo, resolvedDaysRepo } from "../../data/repositories/resolvedDays";
import { workoutRecordsRepo } from "../../data/repositories/workoutRecords";
import { seedStore } from "../../data/seedStore";
import type { Store } from "../../data/store";
import { StoreProvider } from "../../data/StoreProvider";
import { setsSummary } from "../../domain/sets";
import { dayTotals, disableMeal, mealEnabled, newMealLog } from "../../domain/totals";
import { ThemeProvider } from "../../ui";
import { pullToRefresh } from "../../ui/test-utils";
import { addExercise } from "../workout/workoutEdits";
import TodayScreen, { type TodayScreenProps } from "./TodayScreen";

jest.mock("../../data/uuid", () => {
  let n = 0;
  return { uuid: jest.fn(() => `uuid-${++n}`) };
});
const mockPush = jest.fn();
jest.mock("expo-router", () => ({ useRouter: () => ({ push: mockPush }) }));
const mockSync = { status: "idle", pendingCount: 0, reasons: {}, syncNow: jest.fn(async () => {}) };
jest.mock("../../data/useSync", () => ({ useSync: () => mockSync }));

const today = "2026-11-11";
const mockApi = createClients(createMockTransport({ today: "2026-11-11" }).transport);
jest.mock("../../api/useApi", () => ({ useApi: () => mockApi }));

let store: Store;
beforeEach(async () => {
  mockPush.mockClear();
  mockSync.status = "idle";
  store = await seedStore(today);
});

const activeWorkout = () =>
  programsRepo(store)
    .all()
    .find((p) => p.kind === "workout" && p.status === "active")!;
const todaysPlan = () => resolvedDaysRepo(store).byDate(today)[0];
const todaysRecord = () => workoutRecordsRepo(store).byDate(today)[0];
const todaysMealLog = () => mealLogsRepo(store).byDate(today)[0];
const todaysCheckin = () => checkinsRepo(store).byDate(today)[0];

function clearToday() {
  store.delete("workoutRecords", todaysRecord().id);
  store.delete("mealLogs", todaysMealLog().id);
}

const button = (name: string) => screen.getByRole("button", { name });
const noButton = (name: string) => expect(screen.queryByRole("button", { name })).toBeNull();

async function renderToday(over: Partial<TodayScreenProps> = {}) {
  await render(
    <ThemeProvider>
      <StoreProvider store={store}>
        <TodayScreen today={today} unit="kg" userId="user-1" {...over} />
      </StoreProvider>
    </ThemeProvider>,
  );
}

describe("Today", () => {
  it("syncs on pull-to-refresh and shows what the sync stored", async () => {
    mockSync.syncNow.mockImplementationOnce(async () => {
      checkinsRepo(store).upsertFromServer([
        {
          id: "checkin-sync",
          userId: "user-1",
          version: 1,
          updatedAt: "2026-11-11T08:00:00Z",
          deviceId: "tablet",
          date: today,
          weight: { value: 79.5, unit: "kg" },
          measurements: {},
        },
      ]);
    });
    await renderToday();
    await pullToRefresh();
    expect(mockSync.syncNow).toHaveBeenCalledTimes(1);
    expect(screen.getByText("79.5 kg")).toBeOnTheScreen();
  });

  it("shows the anchored day planned for today with weights in kilograms", async () => {
    const { payload } = todaysPlan();
    await renderToday();
    expect(screen.getByTestId("today-screen")).toBeOnTheScreen();
    expect(screen.getByText(`Week ${payload.week}, day ${payload.day}`)).toBeOnTheScreen();
    expect(screen.getByText(payload.name)).toBeOnTheScreen();
    const bench = payload.exercises[0];
    expect(screen.getByText(bench.name)).toBeOnTheScreen();
    expect(screen.getByText(setsSummary(bench.sets, "kg"))).toBeOnTheScreen();
    expect(screen.getAllByTestId("thumbnail-placeholder")).toHaveLength(payload.exercises.length);
    const split = payload.exercises.find((e) => e.unilateral)!;
    expect(split.name).toBe("Bulgarian Split Squat");
    expect(screen.getAllByText(/ · per side$/)).toHaveLength(1);
  });

  it("keeps long exercise names and many sets on one truncated line each", async () => {
    const row = todaysPlan();
    const name = "Single Arm Dumbbell Row With A Very Long Name Here"; // 45+ characters
    const base = row.payload.exercises[0].sets.find((s) => !s.warmup)!;
    const sets = Array.from({ length: 12 }, (_, i) => ({
      ...base,
      reps: 3 + i,
      weight: { value: 20 + 2.5 * i, unit: "kg" as const },
    }));
    store.put("resolvedDays", {
      ...row,
      payload: { ...row.payload, exercises: [{ ...row.payload.exercises[0], name, sets }] },
    });
    await renderToday();
    const line = screen.getByTestId("exercise-line-0");
    expect(line).toHaveStyle({ flex: 1 });
    const summary = setsSummary(sets, "kg");
    expect(summary.length).toBeGreaterThan(100);
    expect(within(line).getByText(name).props.numberOfLines).toBe(1);
    expect(within(line).getByText(summary).props.numberOfLines).toBe(1);
  });

  it("shows weights in pounds for a pound user", async () => {
    const bench = todaysPlan().payload.exercises[0];
    const kg = bench.sets.find((s) => !s.warmup)!.weight.value;
    await renderToday({ unit: "lb" });
    const lb = Math.round(kg * 2.2046226218 * 10) / 10;
    expect(screen.getAllByText(new RegExp(`${lb} lb`)).length).toBeGreaterThan(0);
  });

  it("labels a floating program's next day from its cursor", async () => {
    clearToday();
    const program = activeWorkout();
    programsRepo(store).upsertFromServer([
      {
        ...program,
        mode: "floating",
        startDate: undefined,
        endDate: undefined,
        nextWeek: 4,
        nextDay: 1,
      },
    ]);
    const next = resolvedDaysRepo(store).at(program.id, 4, 1)!.payload;
    await renderToday();
    expect(screen.getByText("Next: week 4 day 1")).toBeOnTheScreen();
    expect(screen.getByText(next.name)).toBeOnTheScreen();
    expect(screen.queryByText(todaysPlan().payload.name)).toBeNull();
  });

  it("hides quick fill on a rest day", async () => {
    clearToday();
    store.delete("resolvedDays", todaysPlan().id);
    await renderToday();
    expect(screen.getByText("Rest day")).toBeOnTheScreen();
    noButton("Start workout");
    noButton("Done as planned");
  });

  it("offers Programs when no program is active", async () => {
    store = createMemoryStore();
    await renderToday();
    expect(screen.getByText("No program")).toBeOnTheScreen();
    noButton("Start workout");
    noButton("Done as planned");
    noButton("Log as planned");
    await fireEvent.press(button("Browse programs"));
    expect(mockPush).toHaveBeenCalledWith("/programs");
  });

  it("starts a workout as a snapshot of the plan and opens it", async () => {
    clearToday();
    const { payload, programId } = todaysPlan();
    await renderToday();
    await fireEvent.press(button("Start workout"));
    const record = todaysRecord();
    expect(record).toMatchObject({ programId, week: payload.week, day: payload.day, dirty: true });
    expect(record.startedAt).toBeDefined();
    expect(record.entries.map((e) => e.name)).toEqual(payload.exercises.map((e) => e.name));
    const flat = record.entries.flatMap((e) => e.sets);
    expect(flat).toHaveLength(payload.exercises.flatMap((e) => e.sets).length);
    expect(flat.every((s) => !s.completed && s.planned.reps > 0)).toBe(true);
    expect(mockPush).toHaveBeenCalledWith(`/workout/${record.id}`);
  });

  it("continues a started workout instead of creating another", async () => {
    const started = todaysRecord();
    await renderToday();
    expect(screen.getByText(/In progress/)).toBeOnTheScreen();
    await fireEvent.press(button("Continue workout"));
    expect(mockPush).toHaveBeenCalledWith(`/workout/${started.id}`);
    expect(workoutRecordsRepo(store).byDate(today)).toHaveLength(1);
  });

  it("done as planned completes every set and asks the server to finalize", async () => {
    clearToday();
    await renderToday();
    await fireEvent.press(button("Done as planned"));
    const record = todaysRecord();
    expect(record).toMatchObject({ finalizeRequested: true, finalized: false, dirty: true });
    expect(record.finishedAt).toBeDefined();
    const flat = record.entries.flatMap((e) => e.sets);
    expect(flat.every((s) => s.completed && s.reps === s.planned.reps)).toBe(true);
    expect(dayLogsRepo(store).byDate(today)[0]).toMatchObject({ status: "done", dirty: true });
    expect(screen.getByText("Finished, waiting to sync")).toBeOnTheScreen();
    noButton("Done as planned");
    noButton("Start workout");
  });

  it("adds an ad hoc workout from the menu, even on a rest day", async () => {
    clearToday();
    store.delete("resolvedDays", todaysPlan().id);
    await renderToday();
    expect(screen.getByText("Rest day")).toBeOnTheScreen();
    await fireEvent.press(button("Workout options"));
    await fireEvent.press(await screen.findByRole("button", { name: "Ad hoc workout" }));
    const record = todaysRecord();
    expect(record).toMatchObject({ entries: [], dirty: true });
    expect(record.programId).toBeUndefined();
    expect(mockPush).toHaveBeenCalledWith(`/workout/${record.id}`);
    expect(screen.getByText("Ad hoc")).toBeOnTheScreen();
    expect(button("Continue workout")).toBeOnTheScreen();
    noButton("Done as planned");
  });

  it("starts a workout from a picked program day, the planned one preselected", async () => {
    clearToday();
    const { payload, programId } = todaysPlan();
    await renderToday();
    await fireEvent.press(button("Workout options"));
    await fireEvent.press(await screen.findByRole("button", { name: "From a program" }));
    expect(
      await screen.findByRole("button", { name: /Planned for this date/ }),
    ).toHaveAccessibleName(new RegExp(`^${payload.day}\\. ${payload.name}`));
    await fireEvent.press(button("Create"));
    expect(todaysRecord()).toMatchObject({ programId, week: payload.week, day: payload.day });
    expect(mockPush).toHaveBeenCalledWith(`/workout/${todaysRecord().id}`);
  });

  it("adds ad hoc meals from the menu", async () => {
    clearToday();
    await renderToday();
    await fireEvent.press(button("Meals options"));
    await fireEvent.press(await screen.findByRole("button", { name: "Ad hoc meals" }));
    expect(todaysMealLog()).toMatchObject({ meals: [], dirty: true });
    expect(todaysMealLog().target).toBeUndefined();
    expect(mockPush).toHaveBeenCalledWith(`/meals/${today}`);
  });

  it("creates the meal log from a picked meal day, the planned one preselected", async () => {
    clearToday();
    const plan = mealDaysRepo(store).byDate(today)[0];
    await renderToday();
    await fireEvent.press(button("Meals options"));
    await fireEvent.press(await screen.findByRole("button", { name: "From a program" }));
    expect(
      await screen.findByRole("button", { name: /Planned for this date/ }),
    ).toHaveAccessibleName(new RegExp(`^${plan.payload.name}`));
    await fireEvent.press(button("Create"));
    expect(todaysMealLog()).toMatchObject({ programId: plan.programId, dirty: true });
    expect(todaysMealLog().meals.map((m) => m.name)).toEqual(plan.payload.meals.map((m) => m.name));
    expect(
      todaysMealLog()
        .meals.flatMap((m) => m.items)
        .every((i) => i.qty === 0),
    ).toBe(true);
  });

  it("resets today's workout as a tombstone and shows the plan again", async () => {
    const record = todaysRecord();
    await renderToday();
    expect(screen.getByText(/In progress/)).toBeOnTheScreen();
    await fireEvent.press(button("Workout options"));
    await fireEvent.press(await screen.findByRole("button", { name: "Reset day" }));
    expect(screen.getByText(/Removes this day's workout/)).toBeOnTheScreen();
    await fireEvent.press(button("Reset"));
    expect(store.get("workoutRecords", record.id)).toMatchObject({
      deletedAt: expect.any(String),
      dirty: true,
    });
    expect(screen.queryByText(/In progress/)).toBeNull();
    expect(button("Start workout")).toBeOnTheScreen();
  });

  it("resets a finalized workout too and says the progression stays", async () => {
    const record = { ...todaysRecord(), finalized: true };
    workoutRecordsRepo(store).save(record);
    await renderToday();
    expect(screen.getByText("Finished")).toBeOnTheScreen();
    await fireEvent.press(button("Workout options"));
    await fireEvent.press(await screen.findByRole("button", { name: "Reset day" }));
    expect(screen.getByText(/progression it already applied stays/)).toBeOnTheScreen();
    await fireEvent.press(button("Reset"));
    expect(store.get("workoutRecords", record.id)).toMatchObject({
      deletedAt: expect.any(String),
      dirty: true,
    });
    expect(screen.queryByText("Finished")).toBeNull();
    expect(button("Start workout")).toBeOnTheScreen();
  });

  it("shows the ad hoc workout that replaced a reset finished one, not the plan", async () => {
    const { payload } = todaysPlan();
    workoutRecordsRepo(store).save({ ...todaysRecord(), finalized: true });
    await renderToday();
    await fireEvent.press(button("Workout options"));
    await fireEvent.press(await screen.findByRole("button", { name: "Reset day" }));
    await fireEvent.press(button("Reset"));
    await fireEvent.press(button("Workout options"));
    await fireEvent.press(await screen.findByRole("button", { name: "Ad hoc workout" }));
    const curl = { name: "Bicep Curl", slug: "", equipment: "", unilateral: false };
    workoutRecordsRepo(store).save(addExercise(todaysRecord(), curl));
    await renderToday();

    expect(screen.getByText("Ad hoc")).toBeOnTheScreen();
    expect(screen.getByText("Bicep Curl")).toBeOnTheScreen();
    expect(screen.getByText("1 set")).toBeOnTheScreen();
    expect(screen.queryByText(payload.name)).toBeNull();
    expect(screen.queryByText(payload.exercises[0].name)).toBeNull();
    expect(screen.getByText("In progress · 0 of 1 sets")).toBeOnTheScreen();
    expect(button("Continue workout")).toBeOnTheScreen();
    noButton("Done as planned");
  });

  it("shows today's finished floating workout, not the day the cursor moved on to", async () => {
    const record = todaysRecord();
    workoutRecordsRepo(store).save({ ...record, finalized: true });
    const program = activeWorkout();
    programsRepo(store).upsertFromServer([
      { ...program, mode: "floating", startDate: undefined, nextWeek: 4, nextDay: 1 },
    ]);
    const next = resolvedDaysRepo(store).at(program.id, 4, 1)!.payload;
    await renderToday();
    expect(screen.getByText(`Week ${record.week}, day ${record.day}`)).toBeOnTheScreen();
    expect(screen.getByText(record.dayName)).toBeOnTheScreen();
    expect(screen.queryByText(next.name)).toBeNull();
    expect(screen.getByText("Finished")).toBeOnTheScreen();
    noButton("Done as planned");
  });

  it("resets today's meal log as a tombstone", async () => {
    const log = todaysMealLog();
    await renderToday();
    noButton("Log as planned");
    await fireEvent.press(button("Meals options"));
    await fireEvent.press(await screen.findByRole("button", { name: "Reset day" }));
    await fireEvent.press(button("Reset"));
    expect(store.get("mealLogs", log.id)).toMatchObject({
      deletedAt: expect.any(String),
      dirty: true,
    });
    expect(button("Log as planned")).toBeOnTheScreen();
  });

  it("logs meals as planned from the meal day", async () => {
    clearToday();
    const plan = mealDaysRepo(store).byDate(today)[0];
    await renderToday();
    expect(screen.getByText(plan.payload.name)).toBeOnTheScreen();
    await fireEvent.press(button("Log as planned"));
    const log = todaysMealLog();
    expect(log).toMatchObject({
      programId: plan.programId,
      target: plan.payload.target,
      dirty: true,
    });
    expect(log.meals).toEqual(plan.payload.meals);
    expect(log.meals.flatMap((m) => m.items).every((i) => i.planned)).toBe(true);
    expect(log.totals.kcal).toBeGreaterThan(2000);
    noButton("Log as planned");
    await fireEvent.press(button("Open meals"));
    expect(mockPush).toHaveBeenCalledWith(`/meals/${today}`);
  });

  it("hides a meal deleted from today's log", async () => {
    const log = todaysMealLog();
    const [gone, ...kept] = log.meals;
    mealLogsRepo(store).save({ ...log, meals: kept, totals: dayTotals(kept) });
    await renderToday();
    expect(screen.queryByText(new RegExp(`^${gone.name}`))).toBeNull();
    for (const m of kept) expect(screen.getByText(new RegExp(`^${m.name}`))).toBeOnTheScreen();
  });

  it("shows an ad hoc log on a planned day as totals, without the plan (P2-R42)", async () => {
    clearToday();
    const meals = [
      {
        name: "meal",
        items: [
          {
            foodName: "Paneer",
            qty: 100,
            unit: "g",
            planned: false,
            macros: { kcal: 265, protein: 18, carbs: 1, fat: 21, fiber: 0, waterMl: 0 },
          },
        ],
      },
    ];
    mealLogsRepo(store).save({
      ...newMealLog({ id: "adhoc", userId: "user-1", date: today }),
      meals,
      totals: dayTotals(meals),
    });
    await renderToday();
    const plan = mealDaysRepo(store).byDate(today)[0].payload;
    expect(screen.queryByText(plan.name)).toBeNull();
    expect(screen.getByText("265 kcal · 18 g protein · 1 g carbs · 21 g fat")).toBeOnTheScreen();
    expect(screen.queryByText(/\/ \d+ kcal/)).toBeNull();
    noButton("Log as planned");
  });

  it("enables the meals a log still has disabled", async () => {
    const log = todaysMealLog();
    const plan = mealDaysRepo(store).byDate(today)[0].payload;
    mealLogsRepo(store).save({
      ...log,
      meals: plan.meals.map(disableMeal),
      totals: dayTotals(plan.meals.map(disableMeal)),
    });
    await renderToday();
    expect(screen.getByText(/^0 \/ \d+ kcal/)).toBeOnTheScreen();
    await fireEvent.press(button("Log as planned"));
    expect(todaysMealLog().id).toBe(log.id);
    expect(todaysMealLog().meals.every(mealEnabled)).toBe(true);
    expect(todaysMealLog().totals.kcal).toBeGreaterThan(2000);
    noButton("Log as planned");
  });

  it("saves a weight check-in for today once and shows the trend", async () => {
    const previous = checkinsRepo(store).range("2026-01-01", today).at(-1)!.weight!.value;
    await renderToday();
    expect(screen.getByText(`${previous} kg`)).toBeOnTheScreen();
    await fireEvent.changeText(screen.getByLabelText("Weight (kg)"), "82.4");
    await fireEvent.press(button("Save weight"));
    const saved = todaysCheckin();
    expect(saved).toMatchObject({ weight: { value: 82.4, unit: "kg" }, dirty: true });
    expect(screen.getByText("82.4 kg")).toBeOnTheScreen();
    const delta = Math.round((82.4 - previous) * 10) / 10;
    expect(
      screen.getByLabelText(
        `${delta > 0 ? "Up" : "Down"} ${Math.abs(delta)} kg since the last check-in`,
      ),
    ).toBeOnTheScreen();

    await fireEvent.changeText(screen.getByLabelText("Weight (kg)"), "82.0");
    await fireEvent.press(button("Save weight"));
    expect(checkinsRepo(store).byDate(today)).toHaveLength(1);
    expect(todaysCheckin()).toMatchObject({ id: saved.id, weight: { value: 82, unit: "kg" } });
  });

  it("shows the first check-in without a trend", async () => {
    for (const c of store.list("checkins")) store.remove("checkins", c.id);
    await renderToday();
    await fireEvent.changeText(screen.getByLabelText("Weight (kg)"), "80.5");
    await fireEvent.press(button("Save weight"));
    expect(screen.getByText("Weight today")).toBeOnTheScreen();
    expect(screen.getByText("80.5 kg")).toBeOnTheScreen();
    expect(screen.queryByLabelText(/since the last check-in/)).toBeNull();
  });

  it("stores a pound check-in in kilograms", async () => {
    await renderToday({ unit: "lb" });
    await fireEvent.changeText(screen.getByLabelText("Weight (lb)"), "180");
    await fireEvent.press(button("Save weight"));
    expect(todaysCheckin().weight).toEqual({ value: 81.65, unit: "kg" });
    expect(screen.getByText("180 lb")).toBeOnTheScreen();
  });

  it("rejects a weight that is not a number", async () => {
    await renderToday();
    await fireEvent.changeText(screen.getByLabelText("Weight (kg)"), "heavy");
    await fireEvent.press(button("Save weight"));
    expect(screen.getByText("Enter a weight")).toBeOnTheScreen();
    expect(todaysCheckin()).toBeUndefined();
  });

  it("shows the offline badge from the sync status", async () => {
    mockSync.status = "offline";
    await renderToday();
    expect(screen.getByText("Offline")).toBeOnTheScreen();
  });
});
