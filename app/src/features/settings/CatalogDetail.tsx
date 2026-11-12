import type { Exercise } from "../../gen/plusultra/v1/exercise_pb";
import type { Food } from "../../gen/plusultra/v1/food_pb";
import { ExerciseDetail } from "../exercises/ExerciseDetail";
import { DetailLine } from "./parts";

export type Entry = { kind: "exercise"; item: Exercise } | { kind: "food"; item: Food };

const macrosLine = (f: Food) => {
  const m = f.per100g;
  return m ? `${m.kcal} kcal, ${m.protein} protein, ${m.carbs} carbs, ${m.fat} fat` : "";
};

export function CatalogDetail({ entry }: { entry: Entry }) {
  if (entry.kind === "exercise") return <ExerciseDetail exercise={entry.item} />;
  const f = entry.item;
  const units = Object.entries(f.units).map(([unit, grams]) => `${unit} = ${grams} g`);
  return (
    <>
      <DetailLine label="Per 100 g" value={macrosLine(f)} />
      <DetailLine label="Fiber" value={f.per100g?.fiber ? `${f.per100g.fiber} g` : undefined} />
      <DetailLine label="Units" value={units.join(", ")} />
      <DetailLine label="Source" value={f.source} />
      <DetailLine label="Barcode" value={f.barcode} />
      <DetailLine label="Also known as" value={f.aliases.join(", ")} />
    </>
  );
}
