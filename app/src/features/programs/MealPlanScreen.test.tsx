import { screen } from "@testing-library/react-native";

import { fromProto } from "../../api/convert";
import { dayTotals, mealTotals } from "../../domain/totals";
import MealPlanScreen from "./MealPlanScreen";
import { rememberMealDays } from "./programStore";
import { renderWith, setup, type Harness } from "./test-utils";

jest.mock("expo-router", () => ({
  router: { back: jest.fn(), canGoBack: () => true },
  useRouter: () => ({ push: jest.fn(), back: jest.fn() }),
}));

const id = "program-meals";

let h: Harness;
beforeEach(async () => {
  h = await setup();
});

describe("MealPlanScreen", () => {
  it("lists the meal's items with macros, its totals and the day against its target", async () => {
    const { mealDays } = await h.api.programs.getResolvedDays({ programId: id, week: 2 });
    const days = mealDays.map((d) => fromProto.mealDay(d).payload);
    rememberMealDays(h.store, id, days);
    const plan = days.find((d) => d.kind === "rest")!;
    const meal = plan.meals[1];
    await renderWith(h, <MealPlanScreen programId={id} week={2} day="rest" n={2} />);
    expect(await screen.findByRole("header", { name: meal.name })).toBeOnTheScreen();
    expect(screen.getByText("Rest day · week 2 · 13:00")).toBeOnTheScreen();
    for (const item of meal.items) {
      expect(screen.getByText(item.foodName)).toBeOnTheScreen();
    }
    expect(screen.getByText("60 g")).toBeOnTheScreen();
    const totals = mealTotals(meal);
    expect(screen.getByLabelText(`kcal ${Math.round(totals.kcal)}`)).toBeOnTheScreen();
    expect(screen.getByLabelText(`Protein ${Math.round(totals.protein)} g`)).toBeOnTheScreen();
    expect(screen.getByText("Day vs target")).toBeOnTheScreen();
    const planned = dayTotals(plan.meals);
    expect(
      screen.getByLabelText(`kcal ${Math.round(planned.kcal)} of ${Math.round(plan.target!.kcal)}`),
    ).toBeOnTheScreen();
    expect(
      screen.getByLabelText(
        `Water ${Math.round(planned.waterMl)} of ${Math.round(plan.target!.waterMl)} ml`,
      ),
    ).toBeOnTheScreen();
  });

  it("says when the day is not cached", async () => {
    await renderWith(h, <MealPlanScreen programId={id} week={9} day="9" n={1} />);
    expect(await screen.findByText("Plan not available")).toBeOnTheScreen();
  });
});
