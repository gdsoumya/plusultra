import type { Api } from "../../api/clients";
import { fromProto } from "../../api/convert";
import type { Store } from "../../data/store";
import type { Unit } from "../../domain/types";
import { formatWeight, inUnit as valueIn } from "../../domain/units";
import type { Weight } from "../../gen/plusultra/v1/common_pb";

export type Range = { from: string; to: string; days: number };

export type SectionProps = {
  insights: Api["insights"];
  store: Store;
  range: Range;
  unit: Unit;
  // Chart width, measured by the screen.
  width: number;
};

export const inUnit = (w: Weight | undefined, unit: Unit) => valueIn(fromProto.weight(w), unit);

export const showWeight = (w: Weight | undefined, unit: Unit) =>
  formatWeight(fromProto.weight(w), unit);

export const signed = (v: number) => (v > 0 ? "+" : "") + v.toFixed(1);
