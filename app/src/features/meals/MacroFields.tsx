import type { Macros } from "../../domain/types";
import { Field } from "../../ui";

const fields = [
  ["kcal", "kcal"],
  ["protein", "Protein"],
  ["carbs", "Carbs"],
  ["fat", "Fat"],
  ["fiber", "Fiber"],
] as const;

export type MacroText = Partial<Record<(typeof fields)[number][0], string>>;

export const macroText = (m?: Macros): MacroText =>
  Object.fromEntries(fields.map(([k]) => [k, m ? String(+m[k].toFixed(2)) : ""]));

// The typed macros; an empty field counts as 0. Throws naming a field that is not a number >= 0.
export function readMacros(text: MacroText): Macros {
  const out: Macros = { kcal: 0, protein: 0, carbs: 0, fat: 0, fiber: 0, waterMl: 0 };
  for (const [key, label] of fields) {
    const raw = text[key]?.trim().replace(",", ".") ?? "";
    const n = raw ? Number(raw) : 0;
    if (!Number.isFinite(n) || n < 0) throw new Error(`${label} must be a number`);
    out[key] = n;
  }
  return out;
}

// kcal, protein, carbs, fat and fiber of a food typed in by hand.
export function MacroFields({
  value,
  onChange,
}: {
  value: MacroText;
  onChange(v: MacroText): void;
}) {
  return fields.map(([key, label]) => (
    <Field
      key={key}
      label={label}
      numeric
      value={value[key] ?? ""}
      onChangeText={(text) => onChange({ ...value, [key]: text })}
    />
  ));
}
