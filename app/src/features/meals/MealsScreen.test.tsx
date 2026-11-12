import { Code, ConnectError } from "@connectrpc/connect";
import { act, fireEvent, render, screen, within } from "@testing-library/react-native";
import { GestureHandlerRootView, State } from "react-native-gesture-handler";
import { fireGestureHandler, getByGestureTestId } from "react-native-gesture-handler/jest-utils";

import { createClients } from "../../api/clients";
import { fromProto } from "../../api/convert";
import { createMockTransport } from "../../api/mockTransport";
import { createMemoryStore } from "../../data/memoryStore";
import { mealLogsRepo } from "../../data/repositories/mealLogs";
import { programsRepo } from "../../data/repositories/programs";
import { mealDaysRepo } from "../../data/repositories/resolvedDays";
import type { Store } from "../../data/store";
import {
  dayTotals,
  disableMeal,
  fractionOf,
  itemMacros,
  mealEnabled,
  mealQuickFill,
  mealTotals,
  newMealLog,
  onTarget,
} from "../../domain/totals";
import type { Item } from "../../domain/types";
import { ThemeProvider } from "../../ui";
import { pullToRefresh } from "../../ui/test-utils";
import { mealFlags } from "./flags";
import { RECENT_FOODS } from "./foods";
import MealsScreen from "./MealsScreen";

jest.mock("../../data/uuid", () => ({ uuid: jest.fn(() => "meal-new") }));
const mockSync = { status: "idle", pendingCount: 0, reasons: {}, syncNow: jest.fn(async () => {}) };
jest.mock("../../data/useSync", () => ({ useSync: () => mockSync }));
jest.mock("expo-router", () => ({
  useRouter: () => ({ back: jest.fn(), canGoBack: () => true }),
}));
jest.mock("expo-camera", () => {
  const { View } = jest.requireActual<typeof import("react-native")>("react-native");
  return {
    CameraView: (props: object) => <View testID="camera" {...props} />,
    useCameraPermissions: () => [{ granted: true }, jest.fn()],
  };
});

const today = "2026-11-11";

// log: "planned" saves every planned meal as eaten, "fixture" the mock's log, "adhoc" an empty log
// made without the plan (Today's "Ad hoc meals"), "none" nothing.
// serverPlan: the plan's items unmarked, as the server resolves them (the fixtures mark them).
async function setup({ online = true, log = "planned", serverPlan = false } = {}) {
  const store: Store = createMemoryStore();
  const mock = createMockTransport({ today });
  const api = createClients(mock.transport);
  const day = (await api.calendar.getDay({ date: today })).day!;
  const plan = fromProto.mealDay(day.mealPlan!);
  if (serverPlan) {
    plan.payload.meals = plan.payload.meals.map((m) => ({
      ...m,
      items: m.items.map((i) => ({ ...i, planned: false })),
    }));
  }
  mealDaysRepo(store).upsertFromServer([plan]);
  const { programs } = await api.programs.listPrograms({});
  programsRepo(store).upsertFromServer(programs.map(fromProto.program));
  if (log === "fixture") {
    mealLogsRepo(store).upsertFromServer([fromProto.mealLog(day.meal!, "user-1")]);
  } else if (log === "adhoc") {
    mealLogsRepo(store).save(newMealLog({ id: "meal-adhoc", userId: "user-1", date: today }));
  } else if (log === "planned") {
    mealLogsRepo(store).save(
      mealQuickFill(plan.payload, {
        id: "meal-planned",
        userId: "user-1",
        date: today,
        programId: plan.programId,
      }),
    );
  }
  const tree = () => (
    <ThemeProvider>
      <GestureHandlerRootView>
        <MealsScreen date={today} store={store} api={online ? api : undefined} userId="user-1" />
      </GestureHandlerRootView>
    </ThemeProvider>
  );
  const view = await render(tree());
  // What a finished background sync does: the screen renders again over the store.
  const rerender = () => view.rerender(tree());
  const logged = () => mealLogsRepo(store).byDate(today)[0];
  const item = (meal: string, food: string) =>
    logged()
      .meals.find((m) => m.name === meal)!
      .items.find((i) => i.foodName === food);
  return { store, mock, api, plan: plan.payload, view, logged, item, rerender };
}

const press = (name: string, index = 0) =>
  fireEvent.press(screen.getAllByRole("button", { name })[index]);

// A row's accessible name starts with the food name; the merged ListRow appends subtitle and value.
const rowNamed = (food: string) => new RegExp(`^${food.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")}`);
const openItem = (food: string) =>
  fireEvent.press(screen.getByRole("button", { name: rowNamed(food) }));

// Queries scoped to the add-item sheet, since planned items share names with search results.
const sheet = () => within(screen.getByTestId("add-sheet"));
const pick = async (food: string) =>
  fireEvent.press(await sheet().findByRole("button", { name: rowNamed(food) }));

const menu = async (meal: number, action: string) => {
  await fireEvent.press(screen.getByTestId(`meal-${meal}-menu`));
  await press(action);
};
const names = (meals: { name: string }[]) => meals.map((m) => m.name);

// Rows need a height before a drag can pass them; the drop lands on the next microtask.
async function drag(from: number, translationY: number, count: number) {
  for (let i = 0; i < count; i++) {
    await fireEvent(screen.getByTestId(`meals-${i}`), "layout", {
      nativeEvent: { layout: { height: 200 } },
    });
  }
  fireGestureHandler(getByGestureTestId(`meals-${from}-pan`), [
    { state: State.BEGAN },
    { state: State.ACTIVE },
    { translationY },
    { state: State.END, translationY },
  ]);
  await act(async () => {});
}

describe("meals screen", () => {
  it("syncs on pull-to-refresh and shows the log the sync stored", async () => {
    const { store, logged } = await setup();
    mockSync.syncNow.mockImplementationOnce(async () => {
      const log = logged();
      store.put("mealLogs", {
        ...log,
        meals: log.meals.map((m, i) => (i === 0 ? disableMeal(m) : m)),
      });
    });
    expect(screen.getByTestId("meal-0-eaten")).toBeChecked();
    await pullToRefresh();
    expect(mockSync.syncNow).toHaveBeenCalledTimes(1);
    expect(screen.getByTestId("meal-0-eaten")).not.toBeChecked();
  });

  afterEach(() => {
    mealFlags.scanBarcodes = false;
  });

  it("shows every planned meal disabled, at the planned quantities, without logging anything", async () => {
    const { plan, logged } = await setup({ log: "none" });
    expect(logged()).toBeUndefined();
    for (const [i, meal] of plan.meals.entries()) {
      expect(screen.getByText(meal.name)).toBeOnTheScreen();
      expect(screen.getByTestId(`meal-${i}-eaten`)).not.toBeChecked();
    }
    expect(screen.getByText(/^80 g · 303 kcal/)).toBeOnTheScreen();
    expect(screen.queryByRole("button", { name: rowNamed("Oats, rolled") })).toBeNull();
    expect(screen.getByLabelText("Off target")).toBeOnTheScreen();
  });

  it("enabling a meal logs it as planned and disabling zeroes its items", async () => {
    const { plan, logged } = await setup({ log: "none" });
    await fireEvent.press(screen.getByTestId("meal-0-eaten"));
    const log = logged();
    expect(log).toMatchObject({
      id: "meal-new",
      userId: "user-1",
      programId: "program-meals",
      target: plan.target,
      dirty: true,
    });
    expect(names(log.meals)).toEqual(["Breakfast", "Lunch", "Snack", "Dinner"]);
    expect(log.meals[0].items.every((i) => i.planned && i.qty > 0)).toBe(true);
    expect(
      log.meals
        .slice(1)
        .flatMap((m) => m.items)
        .every((i) => i.qty === 0),
    ).toBe(true);
    expect(log.totals).toEqual(mealTotals(plan.meals[0]));
    expect(screen.getByTestId("meal-0-eaten")).toBeChecked();
    expect(screen.getByRole("button", { name: rowNamed("Oats, rolled") })).toBeOnTheScreen();

    await fireEvent.press(screen.getByTestId("meal-0-eaten"));
    const off = logged().meals[0];
    expect(off.items).toHaveLength(plan.meals[0].items.length);
    expect(off.items.every((i) => i.qty === 0)).toBe(true);
    expect(logged().totals.kcal).toBe(0);
  });

  it("enables every meal at once and reports the day on target", async () => {
    const { plan, logged } = await setup({ log: "none" });
    await press("Enable all");
    expect(logged().meals.every(mealEnabled)).toBe(true);
    expect(logged().totals).toEqual(dayTotals(plan.meals));
    const label = onTarget(dayTotals(plan.meals), plan.target) ? "On target" : "Off target";
    expect(screen.getByLabelText(label)).toBeOnTheScreen();
    expect(screen.queryByRole("button", { name: "Enable all" })).toBeNull();
  });

  it("deletes a meal after confirming", async () => {
    const { logged } = await setup({ log: "none", serverPlan: true });
    await menu(1, "Delete meal");
    await press("Delete");
    expect(names(logged().meals)).toEqual(["Breakfast", "Snack", "Dinner"]);
    expect(screen.queryByText("Lunch")).toBeNull();
    // The first change creates the log; the plan's items stay planned, not ad hoc.
    expect(
      logged()
        .meals.flatMap((m) => m.items)
        .every((i) => i.planned),
    ).toBe(true);
  });

  it("moves meals from the menu and by dragging", async () => {
    const { logged } = await setup();
    await menu(3, "Move up");
    expect(names(logged().meals)).toEqual(["Breakfast", "Lunch", "Dinner", "Snack"]);
    await menu(0, "Move down");
    expect(names(logged().meals)).toEqual(["Lunch", "Breakfast", "Dinner", "Snack"]);
    await drag(0, 450, 4);
    expect(names(logged().meals)).toEqual(["Breakfast", "Dinner", "Lunch", "Snack"]);
    expect(screen.getAllByTestId(/^meal-\d-eaten$/)).toHaveLength(4);
  });

  it("follows the log a sync stored, so the next edit goes onto it (P2-R41)", async () => {
    const { store, logged, rerender } = await setup();
    const log = logged();
    // The sync merged this device's log into the one another device made for the date.
    store.remove("mealLogs", log.id);
    store.put("mealLogs", {
      ...log,
      id: "meal-server",
      version: 3,
      dirty: false,
      meals: [...log.meals, { name: "meal", items: [] }],
    });
    await rerender();
    expect(screen.getByText("meal")).toBeOnTheScreen();
    await fireEvent.press(screen.getByTestId("meal-0-eaten"));
    expect(mealLogsRepo(store).byDate(today)).toEqual([
      expect.objectContaining({ id: "meal-server", version: 3, dirty: true }),
    ]);
    expect(names(logged().meals)).toEqual(["Breakfast", "Lunch", "Snack", "Dinner", "meal"]);
  });

  it("logs an ad hoc day without the plan: eaten as entered, totals without a target (P2-R42)", async () => {
    const { item, plan } = await setup({ log: "adhoc" });
    expect(screen.queryByText(new RegExp(plan.name))).toBeNull();
    expect(screen.getByText(/logged ad hoc/)).toBeOnTheScreen();
    // A plan meal's name gives an ad hoc meal no plan behaviour.
    await press("Add meal");
    await fireEvent.changeText(screen.getByLabelText("Meal name"), "Breakfast");
    await press("Add");
    await press("Add item");
    await fireEvent.changeText(sheet().getByLabelText("Search foods"), "paneer");
    await pick("Paneer");
    await press("Add");
    expect(item("Breakfast", "Paneer")!.qty).toBe(100);
    expect(screen.queryByTestId("meal-0-eaten")).toBeNull();
    expect(screen.queryByRole("button", { name: "Enable all" })).toBeNull();
    expect(screen.queryByLabelText(/(On|Off) target/)).toBeNull();
    expect(screen.queryAllByRole("progressbar")).toHaveLength(0);

    await openItem("Paneer");
    expect(screen.queryByRole("button", { name: "Not eaten" })).toBeNull();
    await fireEvent.changeText(screen.getByLabelText("Quantity (g)"), "120");
    await press("Save");
    expect(item("Breakfast", "Paneer")!.qty).toBe(120);
    expect(screen.getByRole("button", { name: "Add item" })).toBeOnTheScreen();

    await menu(0, "Delete meal");
    expect(screen.queryByText(/The plan keeps it/)).toBeNull();
  });

  it("applies the plan only to the plan's meals and items on a mixed day (P2-R42)", async () => {
    const { item, logged, plan } = await setup({ log: "none" });
    await press("Add meal");
    await fireEvent.changeText(screen.getByLabelText("Meal name"), "meal");
    await press("Add");
    expect(screen.queryByTestId("meal-4-eaten")).toBeNull();
    await press("Add item");
    await fireEvent.changeText(sheet().getByLabelText("Search foods"), "paneer");
    await pick("Paneer");
    await press("Add");

    // An item added to a plan meal keeps its quantity when the meal is marked not eaten.
    await fireEvent.press(screen.getByTestId("meal-0-eaten"));
    await press("Add item", 0);
    await fireEvent.changeText(sheet().getByLabelText("Search foods"), "apple");
    await pick("Apple, raw");
    await press("Add");
    await fireEvent.press(screen.getByTestId("meal-0-eaten"));
    expect(item("Breakfast", "Apple, raw")!.qty).toBeGreaterThan(0);
    expect(item("Breakfast", "Oats, rolled")!.qty).toBe(0);
    await openItem("Apple, raw");
    expect(screen.getByTestId("item-sheet")).toBeOnTheScreen();
    await press("Close");

    await press("Enable all");
    expect(item("Breakfast", "Oats, rolled")!.qty).toBe(plan.meals[0].items[0].qty);
    expect(item("meal", "Paneer")!.qty).toBe(100);
    expect(logged().meals.every(mealEnabled)).toBe(true);
    expect(screen.queryByRole("button", { name: "Enable all" })).toBeNull();
  });

  it("shows an existing log as it is", async () => {
    const { logged } = await setup({ log: "fixture" });
    expect(logged().dirty).toBe(false);
    expect(screen.getByTestId("meal-0-eaten")).toBeChecked();
    expect(screen.queryByText("Lunch")).toBeNull();
  });

  it("shows totals against the target", async () => {
    const { plan } = await setup();
    const bars = screen.getAllByRole("progressbar");
    const now = Math.min(
      100,
      Math.round(fractionOf(dayTotals(plan.meals), plan.target, "kcal") * 100),
    );
    expect(bars[0]).toHaveAccessibilityValue({ now });
  });

  it("rescales macros when a quantity is edited", async () => {
    const { item } = await setup();
    const before = item("Breakfast", "Oats, rolled")!;
    await openItem("Oats, rolled");
    await fireEvent.changeText(screen.getByLabelText("Quantity (g)"), "40");
    await press("Save");
    const after = item("Breakfast", "Oats, rolled")!;
    expect(after.qty).toBe(40);
    expect(after.macros).toEqual(itemMacros(before, 40));
    expect(after.macros.kcal).toBeCloseTo(before.macros.kcal / 2);
    expect(screen.queryByTestId("item-sheet")).toBeNull();
    expect(screen.getByText(/^40 g · 152 kcal/)).toBeOnTheScreen();
  });

  it("counts a planned item not eaten as zero and restores it as planned", async () => {
    const { item, logged } = await setup();
    const total = logged().totals.kcal;
    await openItem("Milk, toned");
    await press("Not eaten");
    const skipped = item("Breakfast", "Milk, toned")!;
    expect(skipped.qty).toBe(0);
    expect(skipped.macros.kcal).toBe(0);
    expect(logged().totals.kcal).toBeCloseTo(total - 145);
    expect(screen.getByText(/Not eaten/)).toBeOnTheScreen();

    await openItem("Milk, toned");
    await press("As planned");
    expect(item("Breakfast", "Milk, toned")!.qty).toBe(250);
    expect(logged().totals.kcal).toBeCloseTo(total);
  });

  it("removes an item", async () => {
    const { item } = await setup();
    await openItem("Almonds");
    await press("Remove");
    expect(item("Snack", "Almonds")).toBeUndefined();
    expect(screen.queryByRole("button", { name: rowNamed("Almonds") })).toBeNull();
  });

  it("adds an ad hoc item found by search", async () => {
    const { item, store } = await setup();
    await press("Add item", 1);
    await fireEvent.changeText(sheet().getByLabelText("Search foods"), "paneer");
    await pick("Paneer");
    expect(sheet().getByLabelText("Quantity (g)")).toHaveProp("value", "100");
    await fireEvent.changeText(sheet().getByLabelText("Quantity (g)"), "150");
    await press("Add");
    const added = item("Lunch", "Paneer")!;
    expect(added).toMatchObject({ qty: 150, unit: "g", planned: false, foodRef: "food-paneer" });
    expect(added.macros.kcal).toBeCloseTo(397.5);
    expect(store.getKv<Item[]>(RECENT_FOODS)?.[0]).toMatchObject({ foodName: "Paneer" });
  });

  it("offers recent foods and reports a failed search", async () => {
    const { mock, item } = await setup();
    mock.failNext("foods.searchFoods", new ConnectError("down", Code.Unavailable));
    await press("Add item", 0);
    await fireEvent.changeText(sheet().getByLabelText("Search foods"), "egg");
    expect(await sheet().findByText(/Search failed/)).toBeOnTheScreen();
    await fireEvent.changeText(sheet().getByLabelText("Search foods"), "banana");
    await pick("Banana, raw");
    expect(sheet().getByLabelText("Quantity (banana)")).toHaveProp("value", "1");
    await press("Add");
    expect(item("Breakfast", "Banana, raw")).toMatchObject({ qty: 1, unit: "banana" });
  });

  it("adds recent foods while offline", async () => {
    const { store, item } = await setup({ online: false });
    store.setKv(RECENT_FOODS, [paneer]);
    await press("Add item", 1);
    expect(sheet().getByText(/Offline/)).toBeOnTheScreen();
    await pick("Paneer");
    await press("Add");
    expect(item("Lunch", "Paneer")).toMatchObject({ qty: 100, planned: false });
  });

  it("adds a custom item per unit offline, its macros counted in the totals (P2-R43)", async () => {
    const { item, logged } = await setup({ online: false });
    const before = logged().totals.kcal;
    await press("Add item", 1);
    await press("Custom item");
    expect(sheet().queryByRole("checkbox", { name: "Save to my foods" })).toBeNull();
    await fireEvent.changeText(sheet().getByLabelText("Name"), "Thali");
    await fireEvent.press(sheet().getByRole("checkbox", { name: "Per unit" }));
    await fireEvent.changeText(sheet().getByLabelText("Unit"), "plate");
    await fireEvent.changeText(sheet().getByLabelText("kcal"), "650");
    await fireEvent.changeText(sheet().getByLabelText("Protein"), "20");
    await fireEvent.changeText(sheet().getByLabelText("Quantity (plate)"), "2");
    await press("Add");
    expect(item("Lunch", "Thali")).toMatchObject({
      qty: 2,
      unit: "plate",
      foodUnit: "plate",
      planned: false,
      perUnit: expect.objectContaining({ kcal: 650, protein: 20 }),
      macros: expect.objectContaining({ kcal: 1300, protein: 40 }),
    });
    expect(item("Lunch", "Thali")!.foodRef).toBeUndefined();
    expect(logged().totals.kcal).toBeCloseTo(before + 1300);
    expect(screen.getByText(/^2 plate · 1300 kcal/)).toBeOnTheScreen();
  });

  it("saves a custom item to my foods when asked, so search finds it (P2-R43)", async () => {
    const { api, item } = await setup();
    const created = jest.spyOn(api.foods, "createCustomFood");
    await press("Add item", 1);
    await press("Custom item");
    await fireEvent.changeText(sheet().getByLabelText("Name"), "Paneer tikka");
    await fireEvent.changeText(sheet().getByLabelText("kcal"), "280");
    await fireEvent.changeText(sheet().getByLabelText("Fat"), "18");
    expect(sheet().getByLabelText("Quantity (g)")).toHaveProp("value", "100");
    await fireEvent.changeText(sheet().getByLabelText("Quantity (g)"), "150");
    await fireEvent.press(sheet().getByRole("checkbox", { name: "Save to my foods" }));
    await press("Add");
    expect(created).toHaveBeenCalledWith({
      food: expect.objectContaining({
        name: "Paneer tikka",
        per100g: expect.objectContaining({ kcal: 280, fat: 18 }),
      }),
    });
    const { food } = await created.mock.results[0].value;
    expect(item("Lunch", "Paneer tikka")).toMatchObject({
      qty: 150,
      unit: "g",
      foodRef: food.id,
      macros: expect.objectContaining({ kcal: 420, fat: 27 }),
    });
    const { foods } = await api.foods.searchFoods({ query: "paneer tikka", limit: 5 });
    expect(foods.map((f) => f.name)).toContain("Paneer tikka");
  });

  it("keeps a custom item in the meal when saving it to my foods fails (P2-R43)", async () => {
    const { mock, item } = await setup();
    mock.failNext("foods.createCustomFood", new ConnectError("down", Code.Unavailable));
    await press("Add item", 1);
    await press("Custom item");
    await fireEvent.changeText(sheet().getByLabelText("Name"), "Thali");
    await fireEvent.changeText(sheet().getByLabelText("kcal"), "120");
    await fireEvent.press(sheet().getByRole("checkbox", { name: "Save to my foods" }));
    await press("Add");
    expect(item("Lunch", "Thali")).toMatchObject({ qty: 100 });
    expect(item("Lunch", "Thali")!.foodRef).toBeUndefined();
    expect(screen.getByText(/Thali is in the meal but not in your foods/)).toBeOnTheScreen();
  });

  it("overrides a planned item's macros per 100 g, marked edited, the plan untouched (P2-R44)", async () => {
    const { item, logged, plan } = await setup();
    const before = item("Breakfast", "Oats, rolled")!;
    const total = logged().totals.kcal;
    await openItem("Oats, rolled");
    await press("Edit macros");
    expect(screen.getByText("Per 100 g")).toBeOnTheScreen();
    expect(screen.getByLabelText("kcal")).toHaveProp("value", String(before.per100g!.kcal));
    await fireEvent.changeText(screen.getByLabelText("kcal"), "400");
    await fireEvent.changeText(screen.getByLabelText("Protein"), "15");
    await press("Save macros");
    const after = item("Breakfast", "Oats, rolled")!;
    expect(after).toMatchObject({ qty: before.qty, planned: true });
    expect(after.per100g).toMatchObject({ kcal: 400, protein: 15 });
    expect(after.macros.kcal).toBeCloseTo((400 * before.qty) / 100);
    expect(after).toMatchObject({ foodRef: before.foodRef, edited: true });
    expect(logged().totals.kcal).toBeCloseTo(total - before.macros.kcal + after.macros.kcal);
    expect(screen.getByRole("button", { name: /^Oats, rolled, .*edited$/ })).toBeOnTheScreen();
    expect(plan.meals[0].items.find((i) => i.foodName === "Oats, rolled")!.per100g).toEqual(
      before.per100g,
    );
  });

  it("overrides a counted item's macros per unit (P2-R44)", async () => {
    const { item } = await setup();
    await openItem("Egg, whole, raw");
    await press("Edit macros");
    expect(screen.getByText("Per egg")).toBeOnTheScreen();
    await fireEvent.changeText(screen.getByLabelText("kcal"), "80");
    await press("Save macros");
    const egg = item("Breakfast", "Egg, whole, raw")!;
    expect(egg.perUnit!.kcal).toBe(80);
    expect(egg.macros.kcal).toBeCloseTo(80 * egg.qty);
  });

  it("adds an ad hoc meal and moves it between the planned ones", async () => {
    const { logged } = await setup({ log: "none" });
    await press("Add meal");
    await fireEvent.changeText(screen.getByLabelText("Meal name"), "Second breakfast");
    await press("Add");
    expect(names(logged().meals)).toEqual([
      "Breakfast",
      "Lunch",
      "Snack",
      "Dinner",
      "Second breakfast",
    ]);
    expect(screen.queryByTestId("meal-4-eaten")).toBeNull();
    await drag(4, -650, 5);
    expect(names(logged().meals)).toEqual([
      "Breakfast",
      "Second breakfast",
      "Lunch",
      "Snack",
      "Dinner",
    ]);
  });

  it("hides the scanner without its flag and looks a barcode up with it", async () => {
    const { item } = await setup();
    await press("Add item", 3);
    expect(screen.queryByRole("button", { name: "Scan" })).toBeNull();
    await press("Close");

    mealFlags.scanBarcodes = true;
    await press("Add item", 3);
    await press("Scan");
    await act(() =>
      fireEvent(screen.getByTestId("camera"), "barcodeScanned", { data: "8901234567895" }),
    );
    expect(await sheet().findByLabelText("Quantity (scoop)")).toHaveProp("value", "1");
    await press("Add");
    expect(item("Dinner", "Whey protein")).toMatchObject({ qty: 1, unit: "scoop", planned: false });
  });
});

const paneer: Item = {
  foodName: "Paneer",
  foodRef: "food-paneer",
  per100g: { kcal: 265, protein: 18.3, carbs: 1.2, fat: 20.8, fiber: 0, waterMl: 0 },
  qty: 100,
  unit: "g",
  macros: { kcal: 265, protein: 18.3, carbs: 1.2, fat: 20.8, fiber: 0, waterMl: 0 },
  planned: false,
};
