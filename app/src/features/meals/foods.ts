import type { Store } from "../../data/store";
import { scaleMacros, sumMacros, withQty } from "../../domain/totals";
import type { Item } from "../../domain/types";
import type { Food } from "../../gen/plusultra/v1/food_pb";

export const RECENT_FOODS = "meals.recentFoods";
const FAVORITE_FOODS = "meals.favoriteFoods";
const MAX_RECENT = 20;

const sameFood = (a: Item, b: Item) => (a.foodRef ?? a.foodName) === (b.foodRef ?? b.foodName);

export const recentFoods = (store: Store) => store.getKv<Item[]>(RECENT_FOODS) ?? [];
export const favoriteFoods = (store: Store) => store.getKv<Item[]>(FAVORITE_FOODS) ?? [];

export const isFavorite = (store: Store, item: Item) =>
  favoriteFoods(store).some((f) => sameFood(f, item));

// Recents keep the item as last logged, so adding it again proposes the same quantity.
export function rememberFood(store: Store, item: Item) {
  const rest = recentFoods(store).filter((i) => !sameFood(i, item));
  store.setKv(RECENT_FOODS, [item, ...rest].slice(0, MAX_RECENT));
}

export function toggleFavorite(store: Store, item: Item) {
  const all = favoriteFoods(store);
  const rest = all.filter((i) => !sameFood(i, item));
  store.setKv(FAVORITE_FOODS, rest.length < all.length ? rest : [item, ...rest]);
}

export const unitsOf = (food: Food) => ["g", ...Object.keys(food.units)];

// An ad hoc item of a catalog food with its macros frozen now, per unit too when counted in one.
export function itemFromFood(food: Food, qty: number, unit: string): Item {
  const per100g = food.per100g ? scaleMacros(food.per100g, 1) : sumMacros([]);
  const grams = food.units[unit];
  return withQty(
    {
      foodName: food.name,
      foodRef: food.id,
      per100g,
      perUnit: grams ? scaleMacros(per100g, grams / 100) : undefined,
      foodUnit: grams ? unit : undefined,
      qty,
      unit,
      macros: per100g,
      planned: false,
    },
    qty,
  );
}
