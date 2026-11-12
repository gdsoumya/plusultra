import { create } from "@bufbuild/protobuf";

import type { Macros } from "../../domain/types";
import { FoodSchema, type Food } from "../../gen/plusultra/v1/food_pb";
import { slugify } from "./exercises";
import { macros, scale } from "./macros";

// [name, source, kcal, protein, carbs, fat, fiber per 100 g, grams per unit, aliases, barcode]
type Row = [
  string,
  "usda" | "indb",
  number,
  number,
  number,
  number,
  number,
  Record<string, number>?,
  string[]?,
  string?,
];

const rows: Row[] = [
  ["Chicken breast, raw", "usda", 120, 22.5, 0, 2.6, 0],
  ["Chicken thigh, raw", "usda", 144, 19.7, 0, 6.8, 0],
  ["Chicken breast, cooked", "usda", 165, 31, 0, 3.6, 0],
  ["Egg, whole, raw", "usda", 143, 12.6, 0.7, 9.5, 0, { egg: 50 }, ["Egg", "Anda"]],
  ["Egg white, raw", "usda", 52, 10.9, 0.7, 0.2, 0, { "egg white": 33 }],
  ["Rice, white, raw", "usda", 365, 7.1, 80, 0.7, 1.3, {}, ["Basmati rice", "Chawal"]],
  ["Rice, white, cooked", "usda", 130, 2.7, 28.2, 0.3, 0.4],
  ["Rice, brown, raw", "usda", 367, 7.5, 76.2, 3.2, 3.6],
  ["Oats, rolled", "usda", 379, 13.2, 67.7, 6.5, 10.1, {}, ["Oatmeal"]],
  ["Paneer", "indb", 265, 18.3, 1.2, 20.8, 0, {}, ["Cottage cheese, Indian"]],
  ["Toor dal, raw", "indb", 343, 22.3, 57.6, 1.7, 9.1, {}, ["Arhar dal", "Pigeon pea, split"]],
  ["Moong dal, raw", "indb", 348, 24.5, 59.9, 1.2, 8.2, {}, ["Mung bean, split"]],
  ["Masoor dal, raw", "indb", 352, 24.6, 63.4, 1.1, 10.7, {}, ["Red lentil"]],
  ["Chana dal, raw", "indb", 360, 20.8, 59.8, 5.6, 9.4, {}, ["Bengal gram, split"]],
  ["Chickpeas, raw", "indb", 364, 19.3, 60.7, 6, 17.4, {}, ["Kabuli chana", "Chole"]],
  ["Rajma, raw", "indb", 333, 23.6, 60, 0.8, 24.9, {}, ["Kidney beans"]],
  ["Besan", "indb", 387, 22.4, 57.8, 6.7, 10.8, {}, ["Gram flour"]],
  ["Whole wheat flour", "indb", 340, 13.2, 72, 2.5, 10.7, {}, ["Atta"]],
  ["Roti", "indb", 297, 9.8, 46.4, 7.5, 4.9, { roti: 40 }, ["Chapati", "Phulka"]],
  ["Poha, raw", "indb", 346, 6.6, 77.3, 1.2, 2.1, {}, ["Flattened rice"]],
  ["Idli", "indb", 132, 4.3, 27.5, 0.6, 1.2, { idli: 40 }],
  ["Curd", "indb", 61, 3.5, 4.7, 3.3, 0, { bowl: 150 }, ["Dahi", "Yogurt, whole milk"]],
  ["Greek yogurt, plain, nonfat", "usda", 59, 10.2, 3.6, 0.4, 0],
  ["Milk, toned", "indb", 58, 3.1, 4.7, 3, 0, { glass: 250 }],
  ["Milk, whole", "usda", 61, 3.2, 4.8, 3.3, 0, { glass: 250 }],
  ["Whey protein", "usda", 400, 78, 8, 6, 0, { scoop: 30 }, [], "8901234567895"],
  ["Soya chunks", "indb", 345, 52, 33, 0.5, 13, {}, ["Textured soy protein"], "8901725181222"],
  ["Tofu, firm", "usda", 144, 17.3, 2.8, 8.7, 2.3],
  ["Salmon, raw", "usda", 208, 20.4, 0, 13.4, 0],
  ["Tuna, canned in water", "usda", 116, 25.5, 0, 0.8, 0],
  ["Banana, raw", "usda", 89, 1.1, 22.8, 0.3, 2.6, { banana: 118 }, ["Kela"]],
  ["Apple, raw", "usda", 52, 0.3, 13.8, 0.2, 2.4, { apple: 182 }],
  ["Almonds", "usda", 579, 21.2, 21.6, 49.9, 12.5, {}, ["Badam"]],
  ["Peanuts, raw", "usda", 567, 25.8, 16.1, 49.2, 8.5, {}, ["Groundnuts", "Moongphali"]],
  ["Peanut butter", "usda", 588, 25, 20, 50, 6, {}, [], "8906001234561"],
  ["Ghee", "indb", 876, 0.3, 0, 99.5, 0],
  ["Olive oil", "usda", 884, 0, 0, 100, 0],
  ["Potato, raw", "usda", 77, 2, 17.5, 0.1, 2.2, {}, ["Aloo"]],
  ["Sweet potato, raw", "usda", 86, 1.6, 20.1, 0.1, 3],
  ["Spinach, raw", "usda", 23, 2.9, 3.6, 0.4, 2.2, {}, ["Palak"]],
  ["Onion, raw", "usda", 40, 1.1, 9.3, 0.1, 1.7, {}, ["Pyaz"]],
  ["Tomato, raw", "usda", 18, 0.9, 3.9, 0.2, 1.2, {}, ["Tamatar"]],
  ["Broccoli, raw", "usda", 34, 2.8, 6.6, 0.4, 2.6],
  ["Bread, whole wheat", "usda", 252, 12.4, 42.7, 3.5, 6, { slice: 32 }],
];

// Each fixture food is its own concept, named after it.
export const foods: Food[] = rows.map(
  ([name, source, kcal, protein, carbs, fat, fiber, units = {}, aliases = [], barcode = ""], i) =>
    create(FoodSchema, {
      id: `food-${slugify(name)}`,
      source,
      sourceId: String(170000 + i),
      name,
      aliases,
      barcode,
      per100g: macros(kcal, protein, carbs, fat, fiber),
      units,
      conceptId: slugify(name),
      conceptName: name,
    }),
);

// How a script or a frozen meal item refers to a catalog food.
export const refOf = (food: Food) => `${food.source}:${food.sourceId}`;

export function foodNamed(name: string): Food {
  const food = foods.find((f) => f.name === name);
  if (!food) throw new Error(`fixture food "${name}" is not in the catalog`);
  return food;
}

// Macros of qty in unit: "g" or one of the food's named units.
export function macrosOf(food: Food, qty: number, unit: string): Macros {
  const grams = unit === "g" ? qty : qty * (food.units[unit] ?? 0);
  return scale(food.per100g!, grams / 100);
}
