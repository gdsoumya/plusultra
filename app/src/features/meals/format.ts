import type { Item, Macros } from "../../domain/types";

const round1 = (n: number) => Math.round(n * 10) / 10;

export const macroLine = (m: Macros) =>
  `${Math.round(m.kcal)} kcal · P ${round1(m.protein)} · C ${round1(m.carbs)} · F ${round1(m.fat)}`;

// Fiber and water join only when the target sets them.
export const kcalProtein = (m: Macros) => `${Math.round(m.kcal)} kcal · P ${Math.round(m.protein)}`;

// "Oats, rolled 80 g · Egg, whole, raw 3": a food counted in its own unit drops the unit.
export const itemsSummary = (items: Item[]) =>
  items
    .map((i) => `${i.foodName} ${round1(i.qty)}${i.unit === i.foodUnit ? "" : ` ${i.unit}`}`)
    .join(" · ");

export const itemLine = (item: Item) =>
  item.qty === 0
    ? "Not eaten"
    : `${round1(item.qty)} ${item.unit} · ${Math.round(item.macros.kcal)} kcal`;

// A non-negative quantity typed with a dot or comma; undefined when it is not a number.
export function parseQty(text: string): number | undefined {
  const n = Number(text.trim().replace(",", "."));
  return text.trim() && Number.isFinite(n) && n >= 0 ? n : undefined;
}
