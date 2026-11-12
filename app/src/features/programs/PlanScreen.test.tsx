import { screen } from "@testing-library/react-native";

import { sets } from "../../api/fixtures/build";
import { exerciseNamed } from "../../api/fixtures/exercises";
import type { Api } from "../../api/clients";
import { fromProto } from "../../api/convert";
import { rememberExercise } from "../../data/exerciseInfo";
import { clock } from "../../domain/dates";
import { formatWeight } from "../../domain/units";
import PlanScreen from "./PlanScreen";
import { rememberDays } from "./programStore";
import { renderWith, setup, type Harness } from "./test-utils";

let mockApi: Api;
jest.mock("../../api/useApi", () => ({ useApi: () => mockApi }));
jest.mock("expo-router", () => ({
  router: { back: jest.fn(), canGoBack: () => true },
  useRouter: () => ({ push: jest.fn(), back: jest.fn() }),
}));

const strength = "program-strength16";

let h: Harness;
beforeEach(async () => {
  h = await setup();
  mockApi = h.api;
});

async function trainingDay() {
  const { days } = await h.api.programs.getResolvedDays({ programId: strength, week: 1 });
  const payloads = days.map((d) => fromProto.resolvedDay(d).payload);
  rememberDays(h.store, strength, payloads);
  return payloads.find((d) => d.kind === "training" && d.exercises[0].sets.some((s) => s.warmup))!;
}

describe("PlanScreen", () => {
  it("lists every planned set in the user's unit with the progression and catalog details", async () => {
    const day = await trainingDay();
    const e = { ...day.exercises[0], progression: "Add 2.5 kg after 3 sessions at the top" };
    rememberDays(h.store, strength, [{ ...day, exercises: [e, ...day.exercises.slice(1)] }]);
    rememberExercise(h.store, exerciseNamed(e.name));
    await renderWith(
      h,
      <PlanScreen programId={strength} week={day.week} day={day.day} n={1} unit="lb" />,
    );
    expect(await screen.findByRole("header", { name: e.name })).toBeOnTheScreen();
    const warmups = e.sets.filter((s) => s.warmup).length;
    expect(screen.getByText("Warmup 1")).toBeOnTheScreen();
    expect(screen.getByText(`Set ${e.sets.length - warmups}`)).toBeOnTheScreen();
    const top = e.sets[e.sets.length - 1];
    expect(
      screen.getAllByText(`${top.reps} × ${formatWeight(top.weight, "lb")}`).length,
    ).toBeGreaterThan(0);
    if (top.rest) {
      expect(screen.getAllByText(new RegExp(`rest ${clock(top.rest)}`)).length).toBeGreaterThan(0);
    }
    expect(screen.getByText("Progression")).toBeOnTheScreen();
    expect(screen.getByText(e.progression)).toBeOnTheScreen();
    expect(screen.getByText("Equipment")).toBeOnTheScreen();
  });

  it("shows a timed set by its time", async () => {
    const day = await trainingDay();
    const e = { ...day.exercises[0], sets: sets(2, 1, 0, { work: 45, rest: 30 }) };
    rememberDays(h.store, strength, [{ ...day, exercises: [e] }]);
    await renderWith(
      h,
      <PlanScreen programId={strength} week={day.week} day={day.day} n={1} unit="kg" />,
    );
    expect(await screen.findAllByText("0:45")).toHaveLength(2);
  });

  it("says when the day is not cached", async () => {
    await renderWith(h, <PlanScreen programId={strength} week={9} day={9} n={1} unit="kg" />);
    expect(await screen.findByText("Plan not available")).toBeOnTheScreen();
  });
});
