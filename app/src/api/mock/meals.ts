import type { ServiceImpl } from "@connectrpc/connect";

import type { Meal } from "../../domain/types";
import { MealService } from "../../gen/plusultra/v1/meal_pb";
import { fromProto, toProto } from "../convert";
import { macrosOf, refOf } from "../fixtures/foods";
import { scale } from "../fixtures/macros";
import { totals } from "../fixtures/meals";
import { exists, find, inDates, onDate, page, remove, required, save, type MockDb } from "./db";
import { mealPlan } from "./plans";

export function mealLogs(db: MockDb): ServiceImpl<typeof MealService> {
  // Items of catalog foods freeze the catalog's macros and scale them to their quantity.
  const priced = (meals: Meal[]) =>
    meals.map((m) => ({
      ...m,
      items: m.items.map((i) => {
        const food = db.foods.find((f) => refOf(f) === i.foodRef);
        if (!food) return i;
        return {
          ...i,
          per100g: i.per100g ?? scale(food.per100g!, 1),
          macros: macrosOf(food, i.qty, i.unit),
        };
      }),
    }));

  function store(input: Parameters<typeof fromProto.mealLog>[0], date?: string) {
    const { id, programId, target, meals } = fromProto.mealLog(input, db.user.id);
    const all = priced(meals);
    const day = date ?? input.date;
    const saved = save(db, "mealLogs", {
      id: required(id, "meal log id"),
      date: day,
      programId,
      target: target ?? mealPlan(db, day)?.target,
      meals: all,
      totals: totals(all),
    });
    return { mealLog: toProto.mealLog(saved) };
  }

  return {
    listMealLogs({ dates, cursor, limit }) {
      const { items, ...more } = page(inDates(db, "mealLogs", dates), cursor, limit);
      return { mealLogs: items.map(toProto.mealLog), ...more };
    },
    getMealLog: ({ id }) => ({ mealLog: toProto.mealLog(find(db, "mealLogs", id)) }),
    createMealLog({ mealLog }) {
      const log = required(mealLog, "meal log");
      if (db.rows.mealLogs.has(log.id) || onDate(db, "mealLogs", log.date)) {
        throw exists(`a meal log for ${log.date}`);
      }
      return store(log);
    },
    updateMealLog({ mealLog }) {
      const log = required(mealLog, "meal log");
      return store(log, find(db, "mealLogs", log.id).date);
    },
    deleteMealLog: ({ id }) => remove(db, "mealLogs", id),
  };
}
