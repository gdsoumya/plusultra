import { create } from "@bufbuild/protobuf";
import { Code, ConnectError } from "@connectrpc/connect";
import { act, fireEvent, screen, waitFor } from "@testing-library/react-native";
import { router } from "expo-router";
import { BackHandler } from "react-native";

import { gzclpScript } from "../../api/fixtures/gzclp";
import { OFFLINE } from "../../api/errors";
import { recentPicks } from "../exercises/ExerciseSearchSheet";
import { ValidateProgramResponseSchema } from "../../gen/plusultra/v1/program_pb";
import ProgramEditorScreen from "./ProgramEditorScreen";
import type { Api } from "../../api/clients";
import { draftScript, renderWith, setup, type Harness } from "./test-utils";

let mockApi: Api;
jest.mock("../../api/useApi", () => ({ useApi: () => mockApi }));
jest.mock("expo-router", () => {
  const router = { push: jest.fn(), replace: jest.fn(), back: jest.fn(), canGoBack: () => true };
  return {
    router,
    useRouter: () => router,
    useFocusEffect: (effect: () => void) => jest.requireActual("react").useEffect(effect, [effect]),
  };
});

const broken = 'program("X", weeks=1)\nday(1, 1, "A")\nex("Squat", sets=error)\n';
const input = () => screen.getByLabelText("Script");
const press = (name: string | RegExp) => fireEvent.press(screen.getByRole("button", { name }));
const settle = () => act(() => jest.advanceTimersByTime(800));
const placeCursor = (start: number, end = start) =>
  fireEvent(input(), "selectionChange", { nativeEvent: { selection: { start, end } } });
const findAndPick = async (query: string, row: RegExp) => {
  await press("Find exercise");
  const field = screen.getByLabelText("Search");
  await fireEvent.changeText(field, query);
  await fireEvent(field, "submitEditing");
  await fireEvent.press(await screen.findByRole("button", { name: row }));
};

let h: Harness;
beforeEach(async () => {
  jest.clearAllMocks();
  h = await setup();
  mockApi = h.api;
});
afterEach(() => jest.useRealTimers());

describe("ProgramEditorScreen", () => {
  it("asks before discarding unsaved changes, on Back and on hardware back", async () => {
    const handlers: Parameters<typeof BackHandler.addEventListener>[1][] = [];
    jest.spyOn(BackHandler, "addEventListener").mockImplementation((_, h) => {
      handlers.push(h);
      return { remove: () => handlers.splice(handlers.indexOf(h), 1) };
    });
    await renderWith(h, <ProgramEditorScreen />);
    await press("Back");
    expect(router.back).toHaveBeenCalledTimes(1);
    await fireEvent.changeText(input(), draftScript);
    await press("Back");
    expect(screen.getByText("Discard changes?")).toBeOnTheScreen();
    await press("Keep editing");
    expect(router.back).toHaveBeenCalledTimes(1);
    await act(() => handlers.at(-1)!({ type: "hardwareBackPress", timeStamp: 0 }));
    await press("Discard");
    expect(router.back).toHaveBeenCalledTimes(2);
    jest.restoreAllMocks();
  });

  it("lists validation errors 800 ms after typing stops and moves the cursor on tap", async () => {
    jest.useFakeTimers();
    await renderWith(h, <ProgramEditorScreen />);
    await fireEvent.changeText(input(), broken);
    expect(screen.queryByText('name "error" is not defined')).toBeNull();
    await settle();
    expect(await screen.findByText('name "error" is not defined')).toBeOnTheScreen();
    await press(/^Line 3/);
    // Line 3 starts at offset 37; "error" is at column 18.
    expect(input()).toHaveProp("selection", { start: 54, end: 54 });
  });

  it("checks on demand and reports a clean script's structure", async () => {
    jest.useFakeTimers();
    await renderWith(h, <ProgramEditorScreen />);
    await fireEvent.changeText(input(), draftScript);
    await press("Check now");
    expect(await screen.findByText("No errors. 2 weeks, 1 day.")).toBeOnTheScreen();
  });

  it("numbers lines in one gutter node, even at 2000 lines", async () => {
    await renderWith(h, <ProgramEditorScreen />);
    const long = Array.from({ length: 2000 }, (_, i) => `# line ${i + 1}`).join("\n");
    await fireEvent.changeText(input(), long);
    const gutter = screen.getByTestId("script-gutter", { includeHiddenElements: true });
    expect(gutter).toHaveTextContent(/^1 2 3 .* 1999 2000$/);
    expect(gutter.children.length).toBeLessThan(4);
  });

  it("previews a simulated week", async () => {
    await renderWith(h, <ProgramEditorScreen />);
    await fireEvent.changeText(input(), gzclpScript);
    await press("Check now");
    await screen.findByText("No errors. 4 weeks, 4 days.");
    await press("Preview week 1");
    expect(await screen.findByText("Week 1 preview")).toBeOnTheScreen();
    for (const day of ["A1", "B1", "A2", "B2"]) expect(screen.getByText(day)).toBeOnTheScreen();
    expect(screen.getAllByText("Squat")).toHaveLength(2);
    expect(screen.getAllByText("4x3 60 kg · 1x3+ 60 kg · rest 3:00")).toHaveLength(1);
    expect(screen.getByText("3x10 47.5 kg · rest 2:00")).toBeOnTheScreen();
  });

  it("saves a new script as a draft and opens it", async () => {
    await renderWith(h, <ProgramEditorScreen />);
    await fireEvent.changeText(input(), draftScript);
    await press("Save");
    await waitFor(() => expect(router.replace).toHaveBeenCalled());
    const saved = h.store.list("programs").find((p) => p.name === "Draft plan");
    expect(saved).toMatchObject({ status: "draft", revision: 1, weeks: 2 });
    expect(router.replace).toHaveBeenCalledWith(`/program/${saved!.id}`);
  });

  it("saves a new revision of an existing program", async () => {
    await renderWith(h, <ProgramEditorScreen id="program-gzclp" />);
    expect(await screen.findByDisplayValue(gzclpScript)).toBeOnTheScreen();
    await fireEvent.changeText(input(), `${gzclpScript}# tweaked\n`);
    await press("Save");
    await waitFor(() => expect(router.back).toHaveBeenCalled());
    expect(h.store.get("programs", "program-gzclp")?.revision).toBe(2);
    const { program } = await h.api.programs.getProgram({ id: "program-gzclp" });
    expect(program?.script).toBe(`${gzclpScript}# tweaked\n`);
  });

  it("inserts a found exercise at the cursor as a quoted name", async () => {
    const partial = 'program("X", weeks=1)\nday(1, 1, "A")\nex(, sets=[s(3, 5, kg(60))])\n';
    const inParens = partial.indexOf("ex(") + 3;
    await renderWith(h, <ProgramEditorScreen />);
    await fireEvent.changeText(input(), partial);
    await placeCursor(inParens);
    await findAndPick("squat", /^Squat, quadriceps · barbell/);
    expect(input()).toHaveDisplayValue(
      'program("X", weeks=1)\nday(1, 1, "A")\nex("Squat", sets=[s(3, 5, kg(60))])\n',
    );
    expect(input()).toHaveProp("selection", { start: inParens + 7, end: inParens + 7 });
    expect(recentPicks(h.store)).toEqual(["squat-barbell"]);
    await press("Find exercise");
    expect(screen.getByText("Recent")).toBeOnTheScreen();
    expect(screen.getAllByRole("button", { name: /^Squat, / }).length).toBeGreaterThan(0);
  });

  it("inserts a whole ex line at a line start", async () => {
    await renderWith(h, <ProgramEditorScreen />);
    await fireEvent.changeText(input(), draftScript);
    await placeCursor(draftScript.length);
    await findAndPick("bench", /^Bench Press, chest · barbell/);
    expect(input()).toHaveDisplayValue(`${draftScript}ex("Bench Press", sets=[s(3, 5, kg(60))])`);
  });

  it("offers the server's suggestions and replaces the unknown name", async () => {
    const typo = 'program("X", weeks=1)\nday(1, 1, "A")\nex("Sqat", sets=[s(3, 5, kg(60))])\n';
    jest.spyOn(h.api.programs, "validateProgram").mockResolvedValue(
      create(ValidateProgramResponseSchema, {
        nameErrors: [{ line: 3, name: "Sqat", suggestions: ["Squat", "Squat, Bodyweight"] }],
      }),
    );
    await renderWith(h, <ProgramEditorScreen />);
    await fireEvent.changeText(input(), typo);
    await press("Check now");
    expect(await screen.findByText('unknown name "Sqat"')).toBeOnTheScreen();
    await press(/^Squat, Did you mean this\? Replaces "Sqat" on line 3/);
    expect(input()).toHaveDisplayValue(typo.replace("Sqat", "Squat"));
    expect(screen.queryByText('unknown name "Sqat"')).toBeNull();
  });

  it("lists an unknown name used on several lines once, at its first line", async () => {
    const typo =
      'program("X", weeks=1)\nday(1, 1, "A")\nex("Sqat", sets=[s(3, 5, kg(60))])\nday(1, 2, "B")\nex("Sqat", sets=[s(3, 5, kg(60))])\n';
    jest.spyOn(h.api.programs, "validateProgram").mockResolvedValue(
      create(ValidateProgramResponseSchema, {
        nameErrors: [
          { line: 5, name: "Sqat", suggestions: ["Squat"] },
          { line: 3, name: "Sqat", suggestions: ["Squat"] },
        ],
      }),
    );
    await renderWith(h, <ProgramEditorScreen />);
    await fireEvent.changeText(input(), typo);
    await press("Check now");
    expect(await screen.findAllByText('unknown name "Sqat"')).toHaveLength(1);
    expect(screen.getByText("Line 3")).toBeOnTheScreen();
    expect(screen.queryByText("Line 5")).toBeNull();
    expect(screen.getAllByText("Squat")).toHaveLength(1);
  });

  it("shows the server's line errors when a save is rejected", async () => {
    jest.useFakeTimers();
    await renderWith(h, <ProgramEditorScreen />);
    await fireEvent.changeText(input(), broken);
    await press("Save");
    expect(await screen.findByText('name "error" is not defined')).toBeOnTheScreen();
    expect(router.replace).not.toHaveBeenCalled();
  });

  it("keeps the text and says so when the server is unreachable", async () => {
    h.mock.failNext("programs.createProgram", new ConnectError("down", Code.Unavailable));
    await renderWith(h, <ProgramEditorScreen />);
    await fireEvent.changeText(input(), draftScript);
    await press("Save");
    expect(await screen.findByText(OFFLINE)).toBeOnTheScreen();
    expect(input()).toHaveDisplayValue(draftScript);
    expect(router.replace).not.toHaveBeenCalled();
  });
});
