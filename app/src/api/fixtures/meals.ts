import type { Item, Meal, MealDay } from "../../domain/types";
import { foodNamed, macrosOf, refOf } from "./foods";
import { scale, sum } from "./macros";

const ref = (name: string) => refOf(foodNamed(name));

export const mealScript = `# Six weeks of recomp eating: protein steady, more carbs on training days.
# Calories step down by 100 every two weeks.
meal_program("Recomp meals", weeks=6, loop=False, notes="High protein; more rice and fruit on training days")

oats = food("Oats, rolled", ref="${ref("Oats, rolled")}")
milk = food("Milk, toned", ref="${ref("Milk, toned")}")
egg = food("Egg, whole, raw", ref="${ref("Egg, whole, raw")}", per_unit={"kcal": 71.5, "protein": 6.3, "carbs": 0.4, "fat": 4.8}, unit="egg")
banana = food("Banana, raw", ref="${ref("Banana, raw")}", per_unit={"kcal": 105, "protein": 1.3, "carbs": 26.9, "fat": 0.4, "fiber": 3.1}, unit="banana")
rice = food("Rice, white, raw", ref="${ref("Rice, white, raw")}")
chicken = food("Chicken breast, raw", ref="${ref("Chicken breast, raw")}")
toor = food("Toor dal, raw", ref="${ref("Toor dal, raw")}")
spinach = food("Spinach, raw", ref="${ref("Spinach, raw")}")
whey = food("Whey protein", ref="${ref("Whey protein")}", per_unit={"kcal": 120, "protein": 23.4, "carbs": 2.4, "fat": 1.8}, unit="scoop")
curd = food("Curd", ref="${ref("Curd")}")
almonds = food("Almonds", ref="${ref("Almonds")}")
roti = food("Roti", ref="${ref("Roti")}", per_unit={"kcal": 118.8, "protein": 3.9, "carbs": 18.6, "fat": 3, "fiber": 2}, unit="roti")
paneer = food("Paneer", ref="${ref("Paneer")}")
moong = food("Moong dal, raw", ref="${ref("Moong dal, raw")}")

for w in range(1, 7):
    step = 100 * ((w - 1) // 2)
    day(w, name="Training day", kind="training", desc="More rice and fruit around the session.",
        target=target(kcal=2600 - step, protein=170, carbs=300, fat=70, fiber=30, water_ml=3500))
    meal("Breakfast", oats(80), milk(250), egg(3), banana(1), time="07:30")
    meal("Lunch", rice(100), chicken(200), toor(40), spinach(100), time="13:00")
    meal("Snack", whey(1), curd(150), almonds(20), time="17:00")
    meal("Dinner", roti(3), paneer(100), moong(40), time="20:30")

    day(w, name="Rest day", kind="rest", target=target(kcal=2200 - step, protein=170, carbs=220, fat=70, fiber=30, water_ml=3000))
    meal("Breakfast", oats(60), milk(250), egg(3), time="07:30")
    meal("Lunch", rice(60), chicken(200), toor(40), spinach(100), time="13:00")
    meal("Snack", whey(1), curd(150), almonds(20), time="17:00")
    meal("Dinner", roti(2), paneer(100), moong(40), time="20:30")
`;

// A planned or logged item with its food's macros frozen, per unit too for foods counted in units.
export function item(name: string, qty: number, unit = "g", planned = true): Item {
  const food = foodNamed(name);
  const per100g = scale(food.per100g!, 1);
  const counted = unit !== "g" && food.units[unit] !== undefined;
  return {
    foodName: food.name,
    foodRef: refOf(food),
    per100g,
    perUnit: counted ? scale(per100g, food.units[unit] / 100) : undefined,
    foodUnit: counted ? unit : undefined,
    qty,
    unit,
    macros: macrosOf(food, qty, unit),
    planned,
  };
}

const meal = (name: string, time: string, ...items: Item[]): Meal => ({ name, time, items });

export const totals = (meals: Meal[]) => sum(meals.flatMap((m) => m.items.map((i) => i.macros)));

function mealDay(week: number, training: boolean): MealDay {
  const step = 100 * Math.floor((week - 1) / 2);
  const meals = [
    meal(
      "Breakfast",
      "07:30",
      item("Oats, rolled", training ? 80 : 60),
      item("Milk, toned", 250),
      item("Egg, whole, raw", 3, "egg"),
      ...(training ? [item("Banana, raw", 1, "banana")] : []),
    ),
    meal(
      "Lunch",
      "13:00",
      item("Rice, white, raw", training ? 100 : 60),
      item("Chicken breast, raw", 200),
      item("Toor dal, raw", 40),
      item("Spinach, raw", 100),
    ),
    meal(
      "Snack",
      "17:00",
      item("Whey protein", 1, "scoop"),
      item("Curd", 150),
      item("Almonds", 20),
    ),
    meal(
      "Dinner",
      "20:30",
      item("Roti", training ? 3 : 2, "roti"),
      item("Paneer", 100),
      item("Moong dal, raw", 40),
    ),
  ];
  return {
    week,
    day: 0,
    kind: training ? "training" : "rest",
    name: training ? "Training day" : "Rest day",
    desc: training ? "More rice and fruit around the session." : "",
    target: {
      kcal: (training ? 2600 : 2200) - step,
      protein: 170,
      carbs: training ? 300 : 220,
      fat: 70,
      fiber: 30,
      waterMl: training ? 3500 : 3000,
    },
    meals,
  };
}

export const mealDays = () =>
  [1, 2, 3, 4, 5, 6].flatMap((w) => [mealDay(w, true), mealDay(w, false)]);
