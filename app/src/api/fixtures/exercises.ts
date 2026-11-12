import { create } from "@bufbuild/protobuf";

import { ExerciseSchema, type Exercise } from "../../gen/plusultra/v1/exercise_pb";

export const slugify = (s: string) =>
  s
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-|-$/g, "");

// [name, equipment, primary muscles, secondary muscles, aliases, category]
type Row = [string, string, string[], string[]?, string[]?, string?];

const rows: Row[] = [
  ["Bench Press", "barbell", ["chest"], ["triceps", "shoulders"], ["Flat Bench Press"]],
  ["Bench Press, Dumbbell", "dumbbell", ["chest"], ["triceps", "shoulders"]],
  ["Bench Press Close Grip", "barbell", ["triceps"], ["chest"]],
  ["Incline Bench Press", "barbell", ["chest"], ["shoulders", "triceps"]],
  ["Chest Press, Leverage Machine", "leverage machine", ["chest"], ["triceps"]],
  ["Push Up", "bodyweight", ["chest"], ["triceps", "shoulders"]],
  ["Dip", "bodyweight", ["triceps"], ["chest"]],
  ["Overhead Press", "barbell", ["shoulders"], ["triceps"], ["OHP", "Military Press"]],
  ["Shoulder Press Parallel Grip", "dumbbell", ["shoulders"], ["triceps"]],
  ["Lateral Raise", "dumbbell", ["shoulders"]],
  ["Lateral Raise, Cable", "cable", ["shoulders"]],
  ["Face Pull", "cable", ["shoulders"], ["middle back"]],
  ["Reverse Fly", "dumbbell", ["shoulders"], ["middle back"]],
  ["Reverse Fly, Band", "band", ["shoulders"], ["middle back"]],
  ["Side Lying External Rotation", "dumbbell", ["shoulders"]],
  ["Lat Pulldown", "cable", ["lats"], ["biceps"], ["Pulldown"]],
  ["Pull Up", "bodyweight", ["lats"], ["biceps"]],
  ["Seated Row", "cable", ["middle back"], ["lats", "biceps"]],
  ["Bent Over Row", "barbell", ["middle back"], ["lats", "biceps"], ["Barbell Row"]],
  ["Bent Over Row, Dumbbell", "dumbbell", ["middle back"], ["lats"], ["Dumbbell Row"]],
  ["Pendlay Row", "barbell", ["middle back"], ["lats"]],
  ["Incline Row", "dumbbell", ["middle back"], ["lats", "biceps"], ["Chest Supported Row"]],
  ["T Bar Row", "barbell", ["middle back"], ["lats"]],
  ["Bicep Curl", "dumbbell", ["biceps"], [], ["Dumbbell Curl"]],
  ["Bicep Curl, Cable", "cable", ["biceps"]],
  ["Preacher Curl", "ez bar", ["biceps"]],
  ["Hammer Curl", "dumbbell", ["biceps"], ["forearms"]],
  ["Concentration Curl", "dumbbell", ["biceps"]],
  ["Reverse Curl", "ez bar", ["forearms"], ["biceps"]],
  ["Wrist Curl", "barbell", ["forearms"]],
  ["Reverse Wrist Curl", "barbell", ["forearms"]],
  ["Dead Hang", "bodyweight", ["forearms"], ["lats"]],
  ["Triceps Extension", "dumbbell", ["triceps"]],
  ["Triceps Pushdown", "cable", ["triceps"]],
  ["Skullcrusher", "ez bar", ["triceps"]],
  ["Squat", "barbell", ["quadriceps"], ["glutes", "hamstrings"], ["Back Squat"]],
  ["Squat, Bodyweight", "bodyweight", ["quadriceps"], ["glutes"], ["Air Squat"]],
  ["Front Squat", "barbell", ["quadriceps"], ["abdominals", "glutes"]],
  ["Goblet Squat", "kettlebell", ["quadriceps"], ["glutes"]],
  ["Hack Squat", "leverage machine", ["quadriceps"], ["glutes"]],
  ["Leg Press", "leverage machine", ["quadriceps"], ["glutes"]],
  ["Bulgarian Split Squat", "dumbbell", ["quadriceps"], ["glutes"]],
  ["Split Squat", "bodyweight", ["quadriceps"], ["glutes"]],
  ["Reverse Lunge", "bodyweight", ["quadriceps"], ["glutes", "hamstrings"]],
  ["Step up", "bodyweight", ["quadriceps"], ["glutes"]],
  [
    "Deadlift",
    "barbell",
    ["lower back", "hamstrings"],
    ["glutes", "traps"],
    ["Conventional Deadlift"],
  ],
  ["Romanian Deadlift", "barbell", ["hamstrings"], ["glutes", "lower back"], ["RDL"]],
  ["Good Morning", "barbell", ["hamstrings"], ["lower back"]],
  ["Back Extension", "bodyweight", ["lower back"], ["glutes", "hamstrings"], ["Hyperextension"]],
  ["Hip Thrust", "barbell", ["glutes"], ["hamstrings"]],
  ["Glute Bridge", "bodyweight", ["glute max"], ["hamstrings"]],
  ["Kettlebell Swing", "kettlebell", ["glutes"], ["hamstrings", "lower back"]],
  ["Seated Leg Curl", "leverage machine", ["hamstrings"]],
  ["Lying Leg Curl", "leverage machine", ["hamstrings"]],
  ["Seated Calf Raise", "leverage machine", ["calves"]],
  ["Standing Calf Raise", "leverage machine", ["calves"]],
  ["Calf Press on Leg Press", "leverage machine", ["calves"]],
  ["Plank", "bodyweight", ["abdominals"], ["shoulders"]],
  ["Dead Bug", "bodyweight", ["abdominals"]],
  ["Ab Wheel", "bodyweight", ["abdominals"], ["lats"], ["Ab Rollout"]],
  ["Hanging Leg Raise", "bodyweight", ["abdominals"]],
  ["Crunch, Cable", "cable", ["abdominals"]],
  ["Jump Rope", "bodyweight", ["calves"], ["shoulders"], ["Skipping"], "cardio"],
];

export const exercises: Exercise[] = rows.map(
  ([
    name,
    equipment,
    primaryMuscles,
    secondaryMuscles = [],
    aliases = [],
    category = "strength",
  ]) => {
    const slug = slugify(`${name.split(",")[0]} ${equipment}`);
    return create(ExerciseSchema, {
      id: `exercise-${slug}`,
      slug,
      name,
      equipment,
      aliases,
      primaryMuscles,
      secondaryMuscles,
      category,
    });
  },
);

export function exerciseNamed(name: string): Exercise {
  const exercise = exercises.find((e) => e.name === name);
  if (!exercise) throw new Error(`fixture exercise "${name}" is not in the catalog`);
  return exercise;
}
