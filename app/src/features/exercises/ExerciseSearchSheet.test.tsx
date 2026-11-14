import { fireEvent, render, screen } from "@testing-library/react-native";

import { createClients } from "../../api/clients";
import { exercises } from "../../api/fixtures/exercises";
import { createMockTransport } from "../../api/mockTransport";
import { rememberExercise } from "../../data/exerciseInfo";
import { createMemoryStore } from "../../data/memoryStore";
import { StoreProvider } from "../../data/StoreProvider";
import { ThemeProvider } from "../../ui";
import { ExerciseSearchSheet, RECENTS_KEY } from "./ExerciseSearchSheet";

const mockApi = createClients(createMockTransport({ today: "2026-11-11" }).transport);
jest.mock("../../api/useApi", () => ({ useApi: () => mockApi }));
jest.mock("expo-router", () => ({ useRouter: () => ({ push: jest.fn() }) }));

const squat = exercises.find((e) => e.slug === "squat-barbell")!;

// Squat is cached; any other recent slug is not.
function renderSheet(recents: string[]) {
  const store = createMemoryStore();
  rememberExercise(store, squat);
  store.setKv(RECENTS_KEY, recents);
  return render(
    <ThemeProvider>
      <StoreProvider store={store}>
        <ExerciseSearchSheet open onPick={jest.fn()} onClose={jest.fn()} />
      </StoreProvider>
    </ThemeProvider>,
  );
}

describe("ExerciseSearchSheet", () => {
  it("says when nothing matches and keeps Recent below", async () => {
    await renderSheet([squat.slug]);
    const field = screen.getByLabelText("Search");
    await fireEvent.changeText(field, "zzzz");
    await fireEvent(field, "submitEditing");
    expect(await screen.findByText('No exercises match "zzzz"')).toBeOnTheScreen();
    expect(screen.getByText("Recent")).toBeOnTheScreen();
    expect(screen.getByRole("button", { name: /^Squat/ })).toBeOnTheScreen();
  });

  it("skips a recent pick that is no longer cached, leaving no bare divider", async () => {
    await renderSheet(["gone-for-good", squat.slug]);
    // The card holds that one row and no divider for the missing one.
    expect(screen.getByRole("button", { name: /^Squat/ }).parent!.children).toHaveLength(1);
  });
});
