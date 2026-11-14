import { fireEvent, render, screen } from "@testing-library/react-native";

import type { ProgramSummary } from "../../domain/types";
import { ThemeProvider } from "../../ui";
import { ProgramDayPicker, type PickDay } from "./ProgramDayPicker";

const program: ProgramSummary = {
  id: "p1",
  kind: "workout",
  name: "Gaps",
  revision: 1,
  status: "active",
  weeks: 3,
  updatedAt: "2026-11-01T00:00:00Z",
};
const day = (week: number): PickDay => ({
  week,
  day: 1,
  name: "Day",
  kind: "training",
  summary: "",
  create: jest.fn(),
});

describe("ProgramDayPicker", () => {
  it("says when a week has no days", async () => {
    await render(
      <ThemeProvider>
        <ProgramDayPicker
          open
          onClose={jest.fn()}
          title="Pick"
          programs={[program]}
          loadDays={async () => [day(1), day(3)]}
          planned={{ programId: "p1", week: 1, day: 1, kind: "training" }}
        />
      </ThemeProvider>,
    );
    await fireEvent.press(await screen.findByRole("checkbox", { name: "Week 2" }));
    expect(screen.getByText("No days in week 2")).toBeOnTheScreen();
  });

  it("shows why a plan could not load, and says when only finished programs exist", async () => {
    await render(
      <ThemeProvider>
        <ProgramDayPicker
          open
          onClose={jest.fn()}
          title="Pick"
          programs={[program]}
          loadDays={async () => {
            throw new Error("line 3: unknown exercise");
          }}
          planned={{ programId: "p1", week: 1, day: 1, kind: "training" }}
        />
      </ThemeProvider>,
    );
    expect(await screen.findByText("Plan not available")).toBeOnTheScreen();
    expect(screen.getByText("line 3: unknown exercise")).toBeOnTheScreen();

    screen.unmount();
    await render(
      <ThemeProvider>
        <ProgramDayPicker
          open
          onClose={jest.fn()}
          title="Pick"
          programs={[]}
          finishedOnly
          loadDays={async () => []}
        />
      </ThemeProvider>,
    );
    expect(await screen.findByText("No active programs")).toBeOnTheScreen();
  });
});
