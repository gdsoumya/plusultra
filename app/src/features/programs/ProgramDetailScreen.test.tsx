import { create } from "@bufbuild/protobuf";
import { Code, ConnectError } from "@connectrpc/connect";
import { act, fireEvent, screen, waitFor } from "@testing-library/react-native";
import { router } from "expo-router";
import { Share } from "react-native";

import { exerciseNamed } from "../../api/fixtures/exercises";
import { mealScript } from "../../api/fixtures/meals";
import { rememberExercise } from "../../data/exerciseInfo";
import { resolvedDaysRepo } from "../../data/repositories/resolvedDays";
import { fromProto } from "../../api/convert";
import { nextMonday, today as localToday } from "../../domain/dates";
import { setsSummary } from "../../domain/sets";
import { ProgramKind, SimulateProgramResponseSchema } from "../../gen/plusultra/v1/program_pb";
import ProgramDetailScreen from "./ProgramDetailScreen";
import type { Api } from "../../api/clients";
import { draftScript, renderWith, setup, type Harness } from "./test-utils";

let mockApi: Api;
jest.mock("../../api/useApi", () => ({ useApi: () => mockApi }));
const mockSyncNow = jest.fn(async () => {});
jest.mock("../../data/useSync", () => ({ useSync: () => ({ syncNow: mockSyncNow }) }));
jest.mock("expo-router", () => {
  const router = {
    push: jest.fn(),
    replace: jest.fn(),
    navigate: jest.fn(),
    back: jest.fn(),
    canGoBack: () => true,
  };
  return {
    router,
    useRouter: () => router,
    useFocusEffect: (effect: () => void) => jest.requireActual("react").useEffect(effect, [effect]),
  };
});
// The mock server's today; the strength fixture started two weeks earlier, so today is its week 3.
let mockToday = "2026-11-11";
jest.mock("../../domain/dates", () => ({
  ...jest.requireActual("../../domain/dates"),
  today: () => mockToday,
}));

const strength = "program-strength16";
const draftLine =
  "A draft has no dates yet. Edit it from the ⋯ menu if you like, then Activate to pick a start date, or run it floating: one day after another, no dates.";
// Chips are checkboxes in the merged UI kit, so fall back to the label.
const press = (name: string) =>
  fireEvent.press(screen.queryByRole("button", { name }) ?? screen.getByLabelText(name));

let h: Harness;
beforeEach(async () => {
  jest.clearAllMocks();
  mockToday = "2026-11-11";
  h = await setup();
  mockApi = h.api;
});

async function draft() {
  const { program } = await h.api.programs.createProgram({
    kind: ProgramKind.WORKOUT,
    script: draftScript,
  });
  await h.seed();
  return program!.id;
}

describe("ProgramDetailScreen", () => {
  it("renders an active anchored program with its structure and actions", async () => {
    await renderWith(h, <ProgramDetailScreen id={strength} />);
    expect(
      await screen.findByRole("header", { name: "16-Week Strength & Recomp — Bulk block" }),
    ).toBeOnTheScreen();
    expect(screen.getByText("Workout, Active")).toBeOnTheScreen();
    expect(screen.getByText("26\u00a0Oct\u00a02026 to 20\u00a0Dec\u00a02026")).toBeOnTheScreen();
    expect(screen.getByText("8 weeks, 51 planned days")).toBeOnTheScreen();
    expect(screen.queryByRole("button", { name: "Go to Today" })).toBeNull();
    await press("Week 8");
    expect(screen.getByText("2. Tue test singles")).toBeOnTheScreen();
    for (const name of ["Back", "Finish", "More"]) {
      expect(screen.getByRole("button", { name })).toBeOnTheScreen();
    }
    expect(screen.queryByRole("button", { name: "Activate" })).toBeNull();
  });

  it("keeps Edit, Export and Delete in the More sheet", async () => {
    await renderWith(h, <ProgramDetailScreen id={strength} />);
    await screen.findByText("Workout, Active");
    for (const name of ["Edit", "Export", "Delete"]) {
      expect(screen.queryByRole("button", { name })).toBeNull();
    }
    await press("More");
    for (const name of ["Edit", "Export", "Delete"]) {
      expect(screen.getByRole("button", { name })).toBeOnTheScreen();
    }
  });

  it("opens on the current week and marks the days already behind", async () => {
    const { days } = await h.api.programs.getResolvedDays({ programId: strength, week: 3 });
    const behind = days.filter((d) => d.date && d.date < "2026-11-11");
    expect(behind.length).toBeGreaterThan(0);
    await renderWith(h, <ProgramDetailScreen id={strength} />);
    expect(await screen.findByRole("checkbox", { name: "Week 3" })).toBeChecked();
    expect(screen.getAllByLabelText(/, past$/)).toHaveLength(behind.length);
    await press("Week 8");
    expect(screen.queryByLabelText(/, past$/)).toBeNull();
  });

  it("lists each day's exercises with their sets and a picture tile, opening the plan", async () => {
    const { days } = await h.api.programs.getResolvedDays({ programId: strength, week: 3 });
    const day = days.find((d) => d.kind === "training")!;
    const first = fromProto.resolvedDay(day).payload.exercises[0];
    await renderWith(h, <ProgramDetailScreen id={strength} unit="lb" />);
    const row = await screen.findByRole("button", {
      name: `${first.name}, ${setsSummary(first.sets, "lb")}`,
    });
    expect(screen.getAllByTestId("thumbnail-placeholder").length).toBeGreaterThan(0);
    await fireEvent.press(row);
    expect(router.push).toHaveBeenCalledWith(`/program/${strength}/plan/${day.week}/${day.day}/1`);
  });

  it("lists a meal program's days with their meals and opens a meal", async () => {
    await renderWith(h, <ProgramDetailScreen id="program-meals" />);
    expect(await screen.findByText("Meals, Active")).toBeOnTheScreen();
    // Each day leads with its name and its planned macros against the target, as labelled values.
    await screen.findByRole("header", { name: "Training day" });
    expect(screen.getByRole("header", { name: "Rest day" })).toBeOnTheScreen();
    for (const stat of [/^kcal \d+ of 2400$/, /^Protein \d+ of 170 g$/, /^Carbs \d+ of 300 g$/]) {
      expect(screen.getAllByLabelText(stat).length).toBeGreaterThan(0);
    }
    for (const stat of [/^Fat \d+ of 70 g$/, /^Fiber \d+ of 30 g$/, /^Water \d+ of 3500 ml$/]) {
      expect(screen.getAllByLabelText(stat).length).toBeGreaterThan(0);
    }
    expect(screen.queryByText(/kcal · P 170 · C 300/)).toBeNull();
    const breakfast = screen.getAllByRole("button", {
      name: /^Breakfast, 07:30 · Oats, rolled 80 g · /,
    });
    expect(breakfast[0]).toHaveAccessibleName(/, \d+ kcal · P \d+ g$/);
    await fireEvent.press(breakfast[0]);
    expect(router.push).toHaveBeenCalledWith("/program/program-meals/meal/5/training/1");
    await press("Week 3");
    expect(screen.getAllByLabelText(/^kcal \d+ of 2500$/).length).toBeGreaterThan(0);
  });

  it("simulates the days of a program that is not active", async () => {
    await renderWith(h, <ProgramDetailScreen id="program-gzclp" />);
    expect(await screen.findByText("1. A1")).toBeOnTheScreen();
    expect((await screen.findAllByRole("button", { name: /^Squat, / })).length).toBeGreaterThan(0);
  });

  it("simulates the meal days of a meal program that is not active", async () => {
    const { program } = await h.api.programs.createProgram({
      kind: ProgramKind.MEAL,
      script: mealScript,
    });
    await h.seed();
    await renderWith(h, <ProgramDetailScreen id={program!.id} />);
    expect(await screen.findByText("Training day")).toBeOnTheScreen();
    expect(screen.getAllByRole("button", { name: /^Breakfast, / }).length).toBeGreaterThan(0);
  });

  it("renders a floating program's cursor", async () => {
    await h.api.programs.finishProgram({ id: strength });
    await h.api.programs.activateProgram({ id: "program-gzclp" });
    await h.seed();
    await renderWith(h, <ProgramDetailScreen id="program-gzclp" />);
    expect(await screen.findByText("Next up: week 1, day 1")).toBeOnTheScreen();
  });

  it("renders a draft as not started, with what a draft is", async () => {
    const id = await draft();
    await renderWith(h, <ProgramDetailScreen id={id} />);
    expect(await screen.findByText("Workout, Draft")).toBeOnTheScreen();
    expect(screen.getByText("Not started")).toBeOnTheScreen();
    expect(screen.getByText(draftLine)).toBeOnTheScreen();
    expect(screen.getByRole("button", { name: "Activate" })).toBeOnTheScreen();
    expect(screen.queryByRole("button", { name: "Finish" })).toBeNull();
  });

  it("does not explain drafts on an active program", async () => {
    await renderWith(h, <ProgramDetailScreen id={strength} />);
    await screen.findByText("Workout, Active");
    expect(screen.queryByText(draftLine)).toBeNull();
  });

  it("shows a draft's script error, pointing a missing 1RM at the 1RMs", async () => {
    const id = await draft();
    jest.spyOn(h.api.programs, "simulateProgram").mockResolvedValue(
      create(SimulateProgramResponseSchema, {
        errors: [{ line: 3, col: 1, message: "week 1 day 1: no 1RM for Squat" }],
      }),
    );
    await renderWith(h, <ProgramDetailScreen id={id} />);
    expect(
      await screen.findByText("Week 1 day 1: no 1RM for Squat. Set it under Settings › 1RMs."),
    ).toBeOnTheScreen();
  });

  it("lists revisions newest first, saying where each came from, and pages them", async () => {
    const id = await draft();
    for (let i = 0; i < 21; i++) {
      await h.api.programs.updateProgram({ id, script: `${draftScript}# ${i}\n` });
    }
    await renderWith(h, <ProgramDetailScreen id={id} />);
    expect(await screen.findByText("Revision 22")).toBeOnTheScreen();
    expect(screen.getAllByText(/^saved in the app, /).length).toBeGreaterThan(0);
    expect(screen.getByText("Revision 3")).toBeOnTheScreen();
    expect(screen.queryByText("Revision 1")).toBeNull();
    await press("Load more");
    expect(await screen.findByText("Revision 1")).toBeOnTheScreen();
    expect(screen.queryByRole("button", { name: "Load more" })).toBeNull();
  });

  it("opens the editor and goes back", async () => {
    await renderWith(h, <ProgramDetailScreen id="program-gzclp" />);
    await screen.findByText("Workout, Finished");
    await press("More");
    await press("Edit");
    expect(router.push).toHaveBeenCalledWith({
      pathname: "/program/edit",
      params: { id: "program-gzclp" },
    });
    await press("Back");
    expect(router.back).toHaveBeenCalled();
  });

  it("exports the script through the share sheet", async () => {
    const share = jest
      .spyOn(Share, "share")
      .mockResolvedValue({ action: "sharedAction", activityType: undefined });
    await renderWith(h, <ProgramDetailScreen id="program-gzclp" />);
    await screen.findByText("Workout, Finished");
    await press("More");
    await press("Export");
    await waitFor(() =>
      expect(share).toHaveBeenCalledWith({
        title: "gzclp-4-week.star",
        message: expect.stringContaining("catalog_snapshot({})"),
      }),
    );
  });

  it("deletes after confirmation", async () => {
    await renderWith(h, <ProgramDetailScreen id="program-gzclp" />);
    await screen.findByText("Workout, Finished");
    await press("More");
    await press("Delete");
    await press("Delete program");
    await waitFor(() => expect(router.back).toHaveBeenCalled());
    expect(h.store.get("programs", "program-gzclp")).toBeUndefined();
    const { programs } = await h.api.programs.listPrograms({});
    expect(programs.map((p) => p.id)).not.toContain("program-gzclp");
  });

  it("finishes an active program after confirmation", async () => {
    await renderWith(h, <ProgramDetailScreen id={strength} />);
    await screen.findByText("Workout, Active");
    await press("Finish");
    await press("Finish program");
    expect(await screen.findByText("Workout, Finished")).toBeOnTheScreen();
    expect(h.store.get("programs", strength)?.status).toBe("finished");
    expect(screen.getByRole("button", { name: "Activate" })).toBeOnTheScreen();
    // Today and the calendar plan from the synced days, so the change is pulled at once.
    expect(mockSyncNow).toHaveBeenCalledTimes(1);
  });

  it("falls back to the stored summary when the server is unreachable", async () => {
    h.mock.failNext("programs.getProgram", new ConnectError("down", Code.Unavailable));
    await renderWith(h, <ProgramDetailScreen id={strength} />);
    expect(
      await screen.findByText("Could not reach the server. Check your connection and try again."),
    ).toBeOnTheScreen();
    expect(screen.getByRole("header", { name: "16-Week Strength & Recomp — Bulk block" }));
    expect(screen.getByText("26\u00a0Oct\u00a02026 to 20\u00a0Dec\u00a02026")).toBeOnTheScreen();
  });

  it("shows the cached days and exercise info when offline", async () => {
    const { days } = await h.api.programs.getResolvedDays({ programId: strength, week: 3 });
    resolvedDaysRepo(h.store).upsertFromServer(days.map(fromProto.resolvedDay));
    const first = days.find((d) => d.kind === "training")!.exercises[0];
    rememberExercise(h.store, exerciseNamed(first.name));
    const down = new ConnectError("down", Code.Unavailable);
    h.mock.failNext("programs.getProgram", down);
    h.mock.failNext("programs.getResolvedDays", down);
    h.mock.failNext("exercises.getExercise", down);
    await renderWith(h, <ProgramDetailScreen id={strength} />);
    expect(
      await screen.findByRole("button", { name: new RegExp(`^${first.name}, `) }),
    ).toBeOnTheScreen();
  });
});

describe("activation", () => {
  const monday = nextMonday(localToday());

  it("anchors to next Monday by default and then points at Today", async () => {
    await h.api.programs.finishProgram({ id: strength });
    const id = await draft();
    const activate = jest.spyOn(h.api.programs, "activateProgram");
    await renderWith(h, <ProgramDetailScreen id={id} />);
    await screen.findByText("Workout, Draft");
    await press("Activate");
    expect(screen.getByLabelText("Start date")).toHaveDisplayValue(monday);
    expect(screen.getByText("Starts on a Monday, at week 1 day 1")).toBeOnTheScreen();
    await press("Start program");
    expect(activate).toHaveBeenCalledWith({ id, startDate: monday });
    expect(await screen.findByText("Workout, Active")).toBeOnTheScreen();
    expect(screen.getByText("16\u00a0Nov\u00a02026 to 29\u00a0Nov\u00a02026")).toBeOnTheScreen();
    expect(h.store.get("programs", id)?.mode).toBe("anchored");
    expect(mockSyncNow).toHaveBeenCalledTimes(1);
    await press("Go to Today");
    expect(router.navigate).toHaveBeenCalledWith("/");
  });

  it("anchors to today when today is a Monday", async () => {
    mockToday = "2026-11-16";
    const id = await draft();
    await renderWith(h, <ProgramDetailScreen id={id} />);
    await screen.findByText("Workout, Draft");
    await press("Activate");
    expect(screen.getByLabelText("Start date")).toHaveDisplayValue("2026-11-16");
  });

  it("points a 1RM the activation needs at Settings", async () => {
    await h.api.programs.finishProgram({ id: strength });
    const id = await draft();
    h.mock.failNext(
      "programs.activateProgram",
      new ConnectError("week 1 day 1: no 1RM for Squat", Code.InvalidArgument),
    );
    await renderWith(h, <ProgramDetailScreen id={id} />);
    await screen.findByText("Workout, Draft");
    await press("Activate");
    await press("Start program");
    expect(
      await screen.findByText("Week 1 day 1: no 1RM for Squat. Set it under Settings › 1RMs."),
    ).toBeOnTheScreen();
  });

  it("explains a mid-week start and rejects a malformed date", async () => {
    const id = await draft();
    await renderWith(h, <ProgramDetailScreen id={id} />);
    await screen.findByText("Workout, Draft");
    await press("Activate");
    await fireEvent.changeText(screen.getByLabelText("Start date"), "2026-11-18");
    expect(screen.getByText("Starts on a Wednesday, at week 1 day 3")).toBeOnTheScreen();
    await fireEvent.changeText(screen.getByLabelText("Start date"), "2026-13-45");
    expect(screen.getByText("Enter a date as YYYY-MM-DD")).toBeOnTheScreen();
    expect(screen.getByRole("button", { name: "Start program" })).toBeDisabled();
  });

  it("runs floating without a date", async () => {
    await h.api.programs.finishProgram({ id: strength });
    const id = await draft();
    const activate = jest.spyOn(h.api.programs, "activateProgram");
    await renderWith(h, <ProgramDetailScreen id={id} />);
    await screen.findByText("Workout, Draft");
    await press("Activate");
    await press("Floating");
    expect(screen.queryByLabelText("Start date")).toBeNull();
    await press("Start program");
    expect(activate).toHaveBeenCalledWith({ id, startDate: "" });
    expect(await screen.findByText("Workout, Active")).toBeOnTheScreen();
    const stored = h.store.get("programs", id);
    expect(stored?.mode).toBe("floating");
    expect(stored?.startDate).toBeUndefined();
  });

  it("names the active program in the way and offers to finish it", async () => {
    const id = await draft();
    await renderWith(h, <ProgramDetailScreen id={id} />);
    await screen.findByText("Workout, Draft");
    await press("Activate");
    await press("Start program");
    expect(
      await screen.findByText(
        '"16-Week Strength & Recomp — Bulk block" is active. Finish it to start this one.',
      ),
    ).toBeOnTheScreen();
    await act(() => press('Finish "16-Week Strength & Recomp — Bulk block" and start'));
    expect(await screen.findByText("Workout, Active")).toBeOnTheScreen();
    expect(h.store.get("programs", strength)?.status).toBe("finished");
    expect(h.store.get("programs", id)?.status).toBe("active");
  });
});
