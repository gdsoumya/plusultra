import type { ResolvedDay } from "../../domain/types";
import { ex, sets } from "./build";

export const gzclpScript = `# GZCLP over four weeks, after Cody Lefever's GZCL method, in kg.
#
# Four workouts (A1, B1, A2, B2) a week, done in order on whatever days suit.
# Each has a T1 main lift (5x3+), a T2 secondary lift (3x10) and a T3
# accessory (3x15+). T1 and T2 add weight every time they are completed.
program("GZCLP 4-week", weeks=4, units="kg", notes="Rotate A1, B1, A2, B2 three or four days a week")

def t1(name, start, increase):
    ex(name, label="t1", desc="T1. The last set is as many reps as you can with good form.",
       sets=[s(4, 3, kg(start), rest=180), s(1, 3, kg(start), rest=180, amrap=True)], progress=lp(increase))

def t2(name, start, increase):
    ex(name, label="t2", desc="T2.", sets=[s(3, 10, kg(start), rest=120)], progress=lp(increase))

def t3(name, weight):
    ex(name, label="t3", desc="T3. Add weight once the last set reaches 25 reps.",
       sets=[s(2, 15, kg(weight), rest=90), s(1, 15, kg(weight), rest=90, amrap=True)])

day(1, 1, "A1", weeks="1-4")
t1("Squat", 60, kg(5))
t2("Bench Press", 42.5, kg(2.5))
t3("Lat Pulldown", 40)

day(1, 2, "B1", weeks="1-4")
t1("Overhead Press", 35, kg(2.5))
t2("Deadlift", 70, kg(5))
t3("Bent Over Row", 40)

day(1, 3, "A2", weeks="1-4")
t1("Bench Press", 50, kg(2.5))
t2("Squat", 47.5, kg(5))
t3("Lat Pulldown", 40)

day(1, 4, "B2", weeks="1-4")
t1("Deadlift", 85, kg(5))
t2("Overhead Press", 27.5, kg(2.5))
t3("Bent Over Row", 40)
`;

const t1 = (name: string, weight: number) =>
  ex(
    name,
    [...sets(4, 3, weight, { rest: 180 }), ...sets(1, 3, weight, { rest: 180, amrap: true })],
    {
      label: "t1",
      desc: "T1. The last set is as many reps as you can with good form.",
    },
  );

const t2 = (name: string, weight: number) =>
  ex(name, sets(3, 10, weight, { rest: 120 }), { label: "t2", desc: "T2." });

const t3 = (name: string) =>
  ex(name, [...sets(2, 15, 40, { rest: 90 }), ...sets(1, 15, 40, { rest: 90, amrap: true })], {
    label: "t3",
    desc: "T3. Add weight once the last set reaches 25 reps.",
  });

const day = (
  week: number,
  n: number,
  name: string,
  exercises: ResolvedDay["exercises"],
): ResolvedDay => ({
  week,
  day: n,
  name,
  desc: "",
  kind: "training",
  exercises,
});

// Each lift appears once a week in each tier, so it progresses once a week.
export const gzclpDays = (): ResolvedDay[] =>
  [1, 2, 3, 4].flatMap((week) => {
    const s = week - 1;
    return [
      day(week, 1, "A1", [
        t1("Squat", 60 + 5 * s),
        t2("Bench Press", 42.5 + 2.5 * s),
        t3("Lat Pulldown"),
      ]),
      day(week, 2, "B1", [
        t1("Overhead Press", 35 + 2.5 * s),
        t2("Deadlift", 70 + 5 * s),
        t3("Bent Over Row"),
      ]),
      day(week, 3, "A2", [
        t1("Bench Press", 50 + 2.5 * s),
        t2("Squat", 47.5 + 5 * s),
        t3("Lat Pulldown"),
      ]),
      day(week, 4, "B2", [
        t1("Deadlift", 85 + 5 * s),
        t2("Overhead Press", 27.5 + 2.5 * s),
        t3("Bent Over Row"),
      ]),
    ];
  });
