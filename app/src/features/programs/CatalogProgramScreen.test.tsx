import { create } from "@bufbuild/protobuf";
import { Code, ConnectError } from "@connectrpc/connect";
import { fireEvent, screen, waitFor } from "@testing-library/react-native";
import { router } from "expo-router";

import type { Api } from "../../api/clients";
import { SimulateProgramResponseSchema } from "../../gen/plusultra/v1/program_pb";
import CatalogProgramScreen from "./CatalogProgramScreen";
import { renderWith, setup, type Harness } from "./test-utils";

let mockApi: Api;
jest.mock("../../api/useApi", () => ({ useApi: () => mockApi }));
jest.mock("expo-router", () => {
  const router = { push: jest.fn(), replace: jest.fn(), back: jest.fn(), canGoBack: () => true };
  return { router, useRouter: () => router };
});

let h: Harness;
beforeEach(async () => {
  jest.clearAllMocks();
  h = await setup();
  mockApi = h.api;
});

describe("CatalogProgramScreen", () => {
  it("shows the program's details and a preview whose rows open nothing", async () => {
    await renderWith(h, <CatalogProgramScreen slug="gzclp" unit="kg" />);
    expect(await screen.findByRole("header", { name: "GZCLP 4-week" })).toBeOnTheScreen();
    expect(screen.getByText("Beginner, 3 days a week")).toBeOnTheScreen();
    expect(screen.getByText("4-week cycle, repeats")).toBeOnTheScreen();
    expect(screen.getByText("After Cody Lefever")).toBeOnTheScreen();
    expect(screen.getByText(/^Four workouts rotate/)).toBeOnTheScreen();
    expect(screen.queryByText(/your 1RM/)).toBeNull();
    expect(screen.queryByRole("button", { name: "Open 1RMs" })).toBeNull();
    expect(screen.getByText("Preview")).toBeOnTheScreen();
    expect(screen.getByText("1. A1")).toBeOnTheScreen();
    expect(screen.getAllByLabelText(/^Squat, /).length).toBeGreaterThan(0);
    expect(screen.queryByRole("button", { name: /^Squat, / })).toBeNull();
    await fireEvent.press(screen.getByRole("checkbox", { name: "Week 2" }));
    expect(screen.getByText("2. B1")).toBeOnTheScreen();
  });

  it("points at the 1RMs a program needs that the user has not set", async () => {
    for (const slug of ["squat-barbell", "deadlift-barbell"]) {
      await h.api.exercises.setExerciseData({ data: { slug } });
    }
    await renderWith(h, <CatalogProgramScreen slug="531-for-beginners" />);
    expect(
      await screen.findByText(
        "Needs your 1RM for Squat, Deadlift. Set them under Settings › 1RMs before activating.",
      ),
    ).toBeOnTheScreen();
    expect(screen.getByText("3-week cycle, repeats")).toBeOnTheScreen();
    await fireEvent.press(screen.getByRole("button", { name: "Open 1RMs" }));
    expect(router.push).toHaveBeenCalledWith("/(tabs)/(settings)/settings/maxes");
  });

  it("says nothing about 1RMs the user already has", async () => {
    await renderWith(h, <CatalogProgramScreen slug="531-for-beginners" />);
    await screen.findByText("3-week cycle, repeats");
    expect(screen.queryByText(/your 1RM/)).toBeNull();
    expect(screen.queryByRole("button", { name: "Open 1RMs" })).toBeNull();
  });

  it("shows the simulation's first error in place of the preview", async () => {
    jest.spyOn(h.api.programs, "simulateProgram").mockResolvedValue(
      create(SimulateProgramResponseSchema, {
        errors: [{ line: 3, col: 1, message: "week 1 day 1: no 1RM for Squat" }],
      }),
    );
    await renderWith(h, <CatalogProgramScreen slug="531-for-beginners" />);
    expect(
      await screen.findByText("week 1 day 1: no 1RM for Squat. Set it under Settings › 1RMs."),
    ).toBeOnTheScreen();
    expect(screen.getByText("Preview")).toBeOnTheScreen();
    expect(screen.queryByRole("checkbox", { name: "Week 1" })).toBeNull();
  });

  it("adds the program as a draft after confirmation", async () => {
    await renderWith(h, <CatalogProgramScreen slug="strength16-bulk" />);
    await screen.findByText("8 weeks");
    await fireEvent.press(screen.getByRole("button", { name: "Add to my programs" }));
    expect(screen.getByText("Add 16-Week Strength & Recomp — Bulk block?")).toBeOnTheScreen();
    expect(
      screen.getByText(
        "It's saved as a draft under Mine, with no dates. Review or edit it there, then Activate to pick a start date or run it floating (one day after another, no dates).",
      ),
    ).toBeOnTheScreen();
    await fireEvent.press(screen.getByRole("button", { name: "Add as draft" }));
    await waitFor(() => expect(router.replace).toHaveBeenCalledTimes(1));
    const id = (router.replace as jest.Mock).mock.calls[0][0].replace("/program/", "");
    expect(h.store.get("programs", id)).toMatchObject({
      name: "16-Week Strength & Recomp — Bulk block",
      status: "draft",
    });
    const { program } = await h.api.programs.getProgram({ id });
    expect(program?.script).toContain("program(");
  });

  it("shows a failed add in the sheet", async () => {
    h.mock.failNext("programs.createProgram", new ConnectError("down", Code.Unavailable));
    await renderWith(h, <CatalogProgramScreen slug="gzclp" />);
    await screen.findByText("4-week cycle, repeats");
    await fireEvent.press(screen.getByRole("button", { name: "Add to my programs" }));
    await fireEvent.press(screen.getByRole("button", { name: "Add as draft" }));
    expect(
      await screen.findByText("Could not reach the server. Check your connection and try again."),
    ).toBeOnTheScreen();
    expect(router.replace).not.toHaveBeenCalled();
  });

  it("shows a program the server does not have", async () => {
    await renderWith(h, <CatalogProgramScreen slug="nope" />);
    expect(await screen.findByText("catalog program nope not found")).toBeOnTheScreen();
  });
});
