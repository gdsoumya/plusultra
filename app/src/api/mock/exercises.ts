import { create } from "@bufbuild/protobuf";
import type { ServiceImpl } from "@connectrpc/connect";

import {
  ExerciseDataSchema,
  ExerciseSchema,
  ExerciseService,
  type Exercise,
} from "../../gen/plusultra/v1/exercise_pb";
import { slugify } from "../fixtures/exercises";
import { exists, notFound, nowStamp, required, type MockDb } from "./db";
import { applyExerciseData } from "./plans";
import { search } from "./search";

export function exercises(db: MockDb): ServiceImpl<typeof ExerciseService> {
  const bySlug = (slug: string, custom?: boolean) => {
    const found = db.exercises.find(
      (e) => e.slug === slug && (custom === undefined || e.custom === custom),
    );
    if (!found) throw notFound(`${custom ? "custom exercise" : "exercise"} ${slug}`);
    return found;
  };
  const custom = (input: Exercise, slug: string) =>
    create(ExerciseSchema, { ...input, id: `exercise-${slug}`, slug, images: [], custom: true });

  return {
    searchExercises: ({ query, equipment, muscle, limit }) => ({
      exercises: search(
        db.exercises.filter(
          (e) =>
            (!equipment || e.equipment === equipment) &&
            (!muscle || [...e.primaryMuscles, ...e.secondaryMuscles].includes(muscle)),
        ),
        query,
        limit,
      ),
    }),
    getExercise: ({ slug }) => ({ exercise: bySlug(slug) }),
    createCustomExercise({ exercise }) {
      const input = required(exercise, "exercise");
      const slug = slugify(`${input.name.split(",")[0]} ${input.equipment}`);
      if (db.exercises.some((e) => e.slug === slug))
        throw exists(`an exercise named ${input.name}`);
      const created = custom(input, slug);
      db.exercises.push(created);
      return { exercise: created };
    },
    updateCustomExercise({ exercise }) {
      const input = required(exercise, "exercise");
      const updated = custom(input, input.slug);
      db.exercises[db.exercises.indexOf(bySlug(input.slug, true))] = updated;
      return { exercise: updated };
    },
    deleteCustomExercise({ slug }) {
      const found = bySlug(slug, true);
      db.exercises = db.exercises.filter((e) => e !== found);
      return {};
    },
    getExerciseData: ({ slug }) => ({
      data: db.exerciseData.find((d) => d.slug === slug) ?? { slug },
    }),
    setExerciseData({ data }) {
      const stored = create(ExerciseDataSchema, {
        ...required(data, "data"),
        updatedAt: nowStamp(),
      });
      db.exerciseData = [...db.exerciseData.filter((d) => d.slug !== stored.slug), stored];
      applyExerciseData(db);
      return { data: stored };
    },
    listExerciseData: () => ({ items: db.exerciseData }),
    getEquipment: () => ({ equipment: db.equipment }),
    setEquipment({ equipment }) {
      db.equipment = equipment;
      return { equipment };
    },
  };
}
