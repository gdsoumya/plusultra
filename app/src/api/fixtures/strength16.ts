// Resolved days of strength16Script for weeks 1-8, as the server resolves them when every
// planned set is completed: main lifts follow the per-week tables, accessories double progression.
import type { DayKind, ResolvedDay, ResolvedExercise } from "../../domain/types";
import { dp, ex, sets, warmups, type ExerciseOptions, type Warmup } from "./build";

const BAR_WARMUP: Warmup = [
  [10, 20, "kg"],
  [8, 50, "%"],
  [5, 70, "%"],
  [3, 85, "%"],
  [1, 93, "%"],
];
const DEADLIFT_WARMUP: Warmup = [
  [8, 60, "kg"],
  [5, 60, "%"],
  [3, 75, "%"],
  [2, 88, "%"],
  [1, 95, "%"],
];

// Per week 1-7: [sets, reps, work kg, backoff kg or 0, rest seconds].
type Table = [number, number, number, number, number][];
const BENCH: Table = [
  [4, 5, 75, 62.5, 180],
  [4, 5, 77.5, 65, 180],
  [4, 5, 80, 67.5, 180],
  [3, 3, 72.5, 0, 180],
  [4, 3, 85, 72.5, 180],
  [4, 3, 87.5, 75, 180],
  [3, 2, 92.5, 0, 240],
];
const DEADLIFT: Table = [
  [3, 5, 110, 92.5, 180],
  [3, 5, 115, 97.5, 180],
  [3, 5, 120, 102.5, 180],
  [3, 3, 110, 0, 180],
  [3, 3, 130, 110, 180],
  [3, 3, 137.5, 117.5, 180],
  [2, 2, 145, 0, 240],
];
const SQUAT: Table = [
  [4, 5, 77.5, 65, 180],
  [4, 5, 82.5, 70, 180],
  [4, 5, 87.5, 75, 180],
  [3, 3, 82.5, 0, 180],
  [4, 3, 95, 80, 180],
  [4, 3, 100, 85, 180],
  [3, 2, 107.5, 0, 240],
];

function mainLift(name: string, table: Table, warmup: Warmup, week: number, desc: string) {
  const [n, reps, work, backoff, rest] = table[week - 1];
  return ex(
    name,
    [
      ...warmups(warmup, work),
      ...sets(n, reps, work, { rest, label: "Work" }),
      ...(backoff ? sets(2, 5, backoff, { rest: 120, label: "Backoff" }) : []),
    ],
    { desc },
  );
}

type Progressed = ExerciseOptions & { rest?: number; warmup?: Warmup };

// An accessory with dp(inc, min, max) after `step` sessions.
function acc(
  name: string,
  step: number,
  n: number,
  weight: number,
  progress: [number, number, number],
  { rest = 0, warmup, ...options }: Progressed = {},
) {
  const { reps, weight: w } = dp(step, weight, progress);
  return ex(name, [...(warmup ? warmups(warmup, w) : []), ...sets(n, reps, w, { rest })], options);
}

const day = (
  week: number,
  n: number,
  name: string,
  desc: string,
  exercises: ResolvedExercise[],
  kind: DayKind = "training",
): ResolvedDay => ({ week, day: n, name, desc, kind, exercises });

const circuit = { superset: "circuit" };

const conditioning = (week: number, n: number, name: string) =>
  day(
    week,
    n,
    name,
    "Easy pace, nasal breathing. Recovery, not a workout.",
    [
      ex("Jump Rope", sets(10, 1, 0, { work: 60, rest: 30, auto: true }), {
        desc: "Ten rounds: one minute on, thirty seconds off.",
      }),
      ex("Squat, Bodyweight", sets(3, 20, 0), {
        ...circuit,
        desc: "Circuit: move to the next exercise without resting.",
      }),
      ex("Reverse Lunge", sets(3, 10, 0), {
        ...circuit,
        alts: ["Split Squat"],
        desc: "Alternate legs; 10 is per side.",
      }),
      ex("Glute Bridge", sets(3, 15, 0), {
        ...circuit,
        alts: ["Hip Thrust, Bodyweight", "Single Leg Bridge"],
        desc: "Squeeze for a second at the top.",
      }),
      ex("Step up", sets(3, 10, 0), {
        ...circuit,
        alts: ["Squat, Bodyweight"],
        desc: "Knee-height box, drive through the heel.",
      }),
      ex("Dead Bug", sets(3, 10, 0), {
        ...circuit,
        alts: ["Plank"],
        desc: "Lower back stays flat on the floor.",
      }),
      ex("Plank", sets(3, 1, 0, { work: 45, rest: 60 }), {
        desc: "Hold the full 45 seconds, then rest.",
      }),
    ],
    "conditioning",
  );

const shoulderHealth = (s: number) => [
  ex("Reverse Fly, Band", sets(3, 20, 0, { rest: 30 }), {
    alts: ["Reverse Fly"],
    desc: "Light and strict; this is for the rotator cuff.",
  }),
  acc("Face Pull", s, 3, 20, [2.5, 15, 15], {
    rest: 45,
    desc: "Pull to the forehead, thumbs back.",
  }),
  ex("Side Lying External Rotation", sets(2, 15, 3, { rest: 30 })),
];

function regularWeek(w: number): ResolvedDay[] {
  const s = w - 1;
  return [
    conditioning(w, 1, "Mon conditioning"),
    day(w, 2, "Tue Upper A bench", "Bench day. Work sets stop one or two reps short of failure.", [
      mainLift(
        "Bench Press",
        BENCH,
        BAR_WARMUP,
        w,
        "Pause each rep on the chest. Backoffs keep the same form.",
      ),
      acc("Lat Pulldown", s, 4, 70, [2.5, 6, 8], {
        rest: 90,
        label: "heavy",
        alts: ["Pull Up"],
        desc: "Full stretch at the top, chest to the bar.",
        warmup: [[8, 40, "kg"]],
      }),
      acc("Overhead Press", s, 3, 42.5, [2.5, 6, 8], {
        alts: ["Shoulder Press Parallel Grip"],
        desc: "Glutes tight, no lean back.",
      }),
      acc("Incline Row", s, 3, 20, [2.5, 10, 12], { alts: ["T Bar Row"] }),
      // One template on both upper days, so it progresses twice a week.
      acc("Lateral Raise", 2 * s, 3, 12.5, [2.5, 12, 15], {
        alts: ["Lateral Raise, Cable"],
        desc: "Lead with the elbows.",
      }),
      acc("Bicep Curl, Cable", s, 3, 10, [2.5, 10, 12], { alts: ["Bicep Curl, EZ Bar"] }),
      acc("Triceps Extension", s, 3, 20, [2.5, 12, 12], {
        alts: ["Triceps Extension, Cable", "Skullcrusher"],
      }),
      acc("Reverse Curl", s, 2, 8, [2.5, 12, 12], {
        alts: ["Reverse Curl, Cable", "Wrist Roller"],
        desc: "Forearms and elbows.",
      }),
      ...shoulderHealth(s),
    ]),
    day(w, 3, "Wed Lower B deadlift", "Deadlift day. Reset each rep on the floor.", [
      mainLift(
        "Deadlift",
        DEADLIFT,
        DEADLIFT_WARMUP,
        w,
        "Wedge in, take the slack out, push the floor away.",
      ),
      acc("Hack Squat", s, 3, 40, [5, 8, 8], { alts: ["Front Squat"] }),
      acc("Bulgarian Split Squat", s, 3, 12, [2.5, 8, 8], {
        alts: ["Reverse Lunge, Bodyweight", "Lunge, Dumbbell"],
        desc: "8 per leg.",
      }),
      acc("Back Extension", s, 3, 0, [5, 12, 12], {
        alts: ["Good Morning", "Reverse Hyperextension"],
      }),
      acc("Seated Calf Raise", s, 3, 40, [5, 15, 15], {
        alts: ["Standing Calf Raise, Bodyweight"],
        desc: "Pause in the stretch.",
      }),
      ex("Ab Wheel", sets(3, 10, 0), { alts: ["Crunch, Cable", "Hollow Body Hold"] }),
      ex("Dead Hang", sets(3, 1, 0, { work: 30, countUp: true, rest: 60 }), {
        desc: "Hang past 30 seconds if you can; the timer counts up.",
      }),
    ]),
    conditioning(w, 4, "Thu conditioning"),
    day(w, 5, "Fri Upper B volume", "Volume day. Moderate loads, controlled tempo.", [
      acc("Chest Press, Leverage Machine", s, 4, 60, [5, 8, 12], {
        alts: ["Bench Press, Dumbbell"],
      }),
      acc("Bench Press Close Grip", s, 3, 47.5, [2.5, 6, 8], {
        alts: ["Bench Press Close Grip, Smith Machine"],
        desc: "Elbows tucked.",
      }),
      acc("Pendlay Row", s, 4, 60, [2.5, 6, 8], {
        alts: ["Bent Over Row, Dumbbell"],
        desc: "Bar back to the floor each rep.",
      }),
      acc("Lat Pulldown", s, 3, 50, [2.5, 10, 12], { alts: ["Seated Row"], label: "volume" }),
      acc("Lateral Raise", 2 * s + 1, 3, 12.5, [2.5, 12, 15]),
      acc("Preacher Curl", s, 3, 20, [2.5, 8, 10], {
        alts: ["Preacher Curl, Leverage Machine", "Concentration Curl"],
      }),
      acc("Hammer Curl", s, 3, 12, [2.5, 10, 12], { alts: ["Hammer Curl, Cable"] }),
      acc("Triceps Pushdown", s, 3, 25, [2.5, 12, 12], { alts: ["Triceps Extension, Band"] }),
      acc("Wrist Curl", s, 2, 15, [2.5, 15, 15], { alts: ["Wrist Curl, Dumbbell"] }),
      acc("Reverse Wrist Curl", s, 2, 10, [2.5, 15, 15], {
        alts: ["Reverse Wrist Curl, Dumbbell"],
      }),
      ...shoulderHealth(s),
    ]),
    day(w, 6, "Sat Lower A squat", "Squat day. Same depth on every rep.", [
      mainLift("Squat", SQUAT, BAR_WARMUP, w, "Hip crease below the knee; brace before each rep."),
      acc("Romanian Deadlift", s, 3, 90, [2.5, 8, 8], {
        alts: ["Stiff Leg Deadlift, Dumbbell", "Cable Pull Through"],
        desc: "Hinge until the hamstrings stop you.",
      }),
      acc("Leg Press", s, 3, 120, [5, 10, 10], { alts: ["Hack Squat", "Goblet Squat"] }),
      acc("Seated Leg Curl", s, 3, 60, [2.5, 12, 12], {
        alts: ["Lying Leg Curl", "Nordic Curl, Band"],
      }),
      acc("Calf Press on Leg Press", s, 3, 80, [5, 15, 15], {
        alts: ["Standing Calf Raise, Leverage Machine", "Standing Calf Raise, Bodyweight"],
      }),
      ex("Hanging Leg Raise", sets(3, 12, 0), {
        alts: ["Hanging Knee Raise", "Flat Leg Raise"],
        desc: "No swinging.",
      }),
    ]),
    conditioning(w, 7, "Sun conditioning"),
  ];
}

const single = (name: string, warmup: Warmup, weight: number) =>
  ex(name, [...warmups(warmup, weight), ...sets(2, 1, weight, { rest: 180, label: "Work" })]);

// Week 8 declares two days; the rest of the week is rest.
const taper: ResolvedDay[] = [
  day(8, 1, "Mon walk only", "A 30-45 minute easy walk. No lifting.", [], "note"),
  day(
    8,
    2,
    "Tue test singles",
    "Two crisp singles per lift, nothing grinding. Squat, bench, then deadlift.",
    [
      single("Squat", BAR_WARMUP, 100),
      single("Bench Press", BAR_WARMUP, 82.5),
      single("Deadlift", DEADLIFT_WARMUP, 130),
    ],
  ),
];

export const strength16Days = (): ResolvedDay[] => [
  ...[1, 2, 3, 4, 5, 6, 7].flatMap(regularWeek),
  ...taper,
];
