import { create } from "@bufbuild/protobuf";

import { CatalogProgramSchema, type CatalogProgram } from "../../gen/plusultra/v1/program_pb";
import { gzclpScript } from "./gzclp";
import { strength16Script } from "./strength16Script";

// Loads a percentage of a 1RM, so it needs maxes before it resolves.
export const fiveThreeOneScript = `program("5/3/1 for Beginners", weeks=3, units="kg")
day(1, 1, "Day 1")
ex("Squat", sets=[s(3, 5, pct(75))])
`;

// What the server's ListCatalog returns, beginners first.
export const catalog: CatalogProgram[] = [
  create(CatalogProgramSchema, {
    slug: "gzclp",
    name: "GZCLP 4-week",
    author: "Cody Lefever",
    level: "beginner",
    daysPerWeek: 3,
    weeks: 4,
    loop: true,
    summary:
      "Four workouts rotate over three days a week. T1 main lifts run 5x3+, T2 lifts 3x10 and T3 accessories 3x15+; T1 and T2 add weight every time they are completed.",
    script: gzclpScript,
  }),
  create(CatalogProgramSchema, {
    slug: "531-for-beginners",
    name: "5/3/1 for Beginners",
    author: "Jim Wendler",
    level: "beginner",
    daysPerWeek: 3,
    weeks: 3,
    loop: true,
    summary:
      "Three full-body days over a three-week wave. Each day trains two main lifts with the 5/3/1 sets, then 5x5 at the first set's weight. Maxes go up after week 3.",
    script: fiveThreeOneScript,
    maxes: ["Squat", "Bench Press", "Deadlift", "Overhead Press"],
  }),
  create(CatalogProgramSchema, {
    slug: "strength16-bulk",
    name: "16-Week Strength & Recomp — Bulk block",
    author: "plusultra",
    level: "intermediate",
    daysPerWeek: 4,
    weeks: 8,
    loop: false,
    summary:
      "Weeks 1-8 of a 16-week plan: two upper and two lower days with easy conditioning between. Volume in weeks 1-3, a light week 4, a strength block, then a taper with test singles in week 8.",
    script: strength16Script,
  }),
];
