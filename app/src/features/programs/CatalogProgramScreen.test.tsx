import { Code, ConnectError } from "@connectrpc/connect";
import { fireEvent, screen, waitFor } from "@testing-library/react-native";
import { router } from "expo-router";

import type { Api } from "../../api/clients";
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
    expect(screen.getByText("4 weeks, repeats")).toBeOnTheScreen();
    expect(screen.getByText("After Cody Lefever")).toBeOnTheScreen();
    expect(screen.getByText(/^Four workouts rotate/)).toBeOnTheScreen();
    expect(screen.queryByText(/Uses your 1RM/)).toBeNull();
    expect(screen.queryByRole("button", { name: "Open Maxes" })).toBeNull();
    expect(screen.getByText("Preview")).toBeOnTheScreen();
    expect(screen.getByText("1. A1")).toBeOnTheScreen();
    expect(screen.getAllByLabelText(/^Squat, /).length).toBeGreaterThan(0);
    expect(screen.queryByRole("button", { name: /^Squat, / })).toBeNull();
    await fireEvent.press(screen.getByRole("checkbox", { name: "Week 2" }));
    expect(screen.getByText("2. B1")).toBeOnTheScreen();
  });

  it("points at the maxes a program needs", async () => {
    await renderWith(h, <CatalogProgramScreen slug="531-for-beginners" />);
    expect(
      await screen.findByText(
        "Uses your 1RM for Squat, Bench Press, Deadlift, Overhead Press. Set them under Settings › Maxes before activating.",
      ),
    ).toBeOnTheScreen();
    expect(screen.getByText("3 weeks, repeats")).toBeOnTheScreen();
    await fireEvent.press(screen.getByRole("button", { name: "Open Maxes" }));
    expect(router.push).toHaveBeenCalledWith("/settings/maxes");
  });

  it("adds the program as a draft after confirmation", async () => {
    await renderWith(h, <CatalogProgramScreen slug="strength16-bulk" />);
    await screen.findByText("8 weeks");
    await fireEvent.press(screen.getByRole("button", { name: "Add to my programs" }));
    expect(screen.getByText("Add 16-Week Strength & Recomp — Bulk block?")).toBeOnTheScreen();
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
    await screen.findByText("4 weeks, repeats");
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
