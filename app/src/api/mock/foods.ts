import { create } from "@bufbuild/protobuf";
import type { ServiceImpl } from "@connectrpc/connect";

import { FoodSchema, FoodService, type Food } from "../../gen/plusultra/v1/food_pb";
import { exists, newId, notFound, required, type MockDb } from "./db";
import { search } from "./search";

export function foods(db: MockDb): ServiceImpl<typeof FoodService> {
  const custom = (input: Food, id: string) =>
    create(FoodSchema, {
      ...input,
      id,
      source: "custom",
      sourceId: id,
      custom: true,
      conceptId: "",
      conceptName: "",
    });
  const byId = (id: string, onlyCustom = false) => {
    const found = db.foods.find((f) => f.id === id && (!onlyCustom || f.custom));
    if (!found) throw notFound(`${onlyCustom ? "custom food" : "food"} ${id}`);
    return found;
  };

  return {
    // Raw foods rank above cooked ones.
    searchFoods: ({ query, source, limit }) => ({
      foods: search(
        db.foods.filter((f) => !source || f.source === source),
        query,
        limit,
        (f) => (/cooked/i.test(f.name) ? 1 : 0),
      ),
    }),
    getFood: ({ id }) => ({ food: byId(id) }),
    listConceptMembers({ conceptId }) {
      const members = db.foods.filter((f) => f.conceptId === conceptId);
      if (!members.length) throw notFound(`concept ${conceptId}`);
      return { foods: members, preferredFoodId: members[0].id };
    },
    lookupBarcode({ barcode }) {
      const food = db.foods.find((f) => f.barcode && f.barcode === barcode);
      if (!food) throw notFound(`a food with barcode ${barcode}`);
      return { food };
    },
    createCustomFood({ food }) {
      const input = required(food, "food");
      if (input.barcode && db.foods.some((f) => f.barcode === input.barcode)) {
        throw exists(`a food with barcode ${input.barcode}`);
      }
      const created = custom(input, newId(db, "food"));
      db.foods.push(created);
      return { food: created };
    },
    updateCustomFood({ food }) {
      const input = required(food, "food");
      const updated = custom(input, input.id);
      db.foods[db.foods.indexOf(byId(input.id, true))] = updated;
      return { food: updated };
    },
    deleteCustomFood({ id }) {
      const found = byId(id, true);
      db.foods = db.foods.filter((f) => f !== found);
      return {};
    },
  };
}
