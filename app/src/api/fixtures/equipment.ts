import { create } from "@bufbuild/protobuf";

import { Unit } from "../../gen/plusultra/v1/common_pb";
import {
  EquipmentSchema,
  ExerciseDataSchema,
  type Equipment,
  type ExerciseData,
} from "../../gen/plusultra/v1/exercise_pb";
import { addDays, stampAt } from "./dates";

const gear = (
  name: string,
  kind: string,
  over: { increment?: number; bar?: number; plates?: number[] } = {},
) => create(EquipmentSchema, { name, rounding: { kind, ...over } });

export const equipment = (): Equipment[] => [
  gear("barbell", "plates", { bar: 20, plates: [25, 20, 15, 10, 5, 2.5, 1.25] }),
  gear("ez bar", "plates", { bar: 10, plates: [10, 5, 2.5, 1.25] }),
  gear("dumbbell", "fixed", { plates: Array.from({ length: 20 }, (_, i) => (i + 1) * 2.5) }),
  gear("kettlebell", "fixed", { plates: [8, 12, 16, 20, 24, 28, 32] }),
  gear("cable", "stack", { increment: 2.5 }),
  gear("leverage machine", "stack", { increment: 5 }),
  gear("band", "bodyweight"),
  gear("bodyweight", "bodyweight"),
];

export const exerciseData = (today: string): ExerciseData[] => {
  const updatedAt = stampAt(addDays(today, -30), "06:00");
  return [
    ...(
      [
        ["squat-barbell", 102.5],
        ["bench-press-barbell", 92.5],
        ["deadlift-barbell", 140],
        ["overhead-press-barbell", 55],
      ] as const
    ).map(([slug, rm1]) =>
      create(ExerciseDataSchema, { slug, rm1: { value: rm1, unit: Unit.KG }, updatedAt }),
    ),
    create(ExerciseDataSchema, {
      slug: "bulgarian-split-squat-dumbbell",
      unilateral: true,
      updatedAt,
    }),
  ];
};
