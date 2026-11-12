import type { Unit, Weight } from "./types";

const LB_PER_KG = 2.2046226218;
export const round1 = (v: number) => Math.round(v * 10) / 10;
const round2 = (v: number) => Math.round(v * 100) / 100;

// The store keeps kilograms, whatever unit the user types in.
export const toKg = (value: number, unit: Unit) =>
  unit === "kg" ? value : round2(value / LB_PER_KG);

export const fromKg = (kg: number, unit: Unit) => (unit === "kg" ? kg : round1(kg * LB_PER_KG));

// A stored weight's value in the user's unit. Server values can carry a raw conversion
// (110 kg as 242.508... lb), so same-unit values are rounded too; kg keeps 1.25 kg plates.
export const inUnit = (w: Weight, unit: Unit) =>
  w.unit !== unit
    ? round1(fromKg(toKg(w.value, w.unit), unit))
    : unit === "kg"
      ? round2(w.value)
      : round1(w.value);

export const formatWeight = (w: Weight, unit: Unit) => `${inUnit(w, unit)} ${unit}`;
