import { create } from "@bufbuild/protobuf";
import { fireEvent, render, screen } from "@testing-library/react-native";
import { Image } from "react-native";

import type { Api } from "../../api/clients";
import { OFFLINE } from "../../api/errors";
import { exerciseNamed } from "../../api/fixtures/exercises";
import { rememberExercise } from "../../data/exerciseInfo";
import { createMemoryStore } from "../../data/memoryStore";
import { StoreProvider } from "../../data/StoreProvider";
import { ExerciseSchema } from "../../gen/plusultra/v1/exercise_pb";
import { ThemeProvider } from "../../ui";
import ExerciseDetailScreen from "./ExerciseDetailScreen";

let mockApi: Api;
jest.mock("../../api/useApi", () => ({ useApi: () => mockApi }));
jest.mock("expo-router", () => ({
  useRouter: () => ({ back: jest.fn(), replace: jest.fn(), canGoBack: () => true }),
}));

const bench = {
  ...create(ExerciseSchema, {
    ...exerciseNamed("Bench Press"),
    images: ["https://lift.example.com/api/v1/exercises/images/bench-press-barbell/0.jpg"],
    custom: true,
  }),
  // The regenerated gen on main has this field; the base snapshot does not.
  instructions: ["Lie on the bench with your eyes under the bar.", "Press to lockout."],
};

const apiWith = (getExercise: () => Promise<{ exercise: typeof bench }>) =>
  ({ exercises: { getExercise } }) as unknown as Api;

const renderDetail = (store = createMemoryStore()) =>
  render(
    <ThemeProvider>
      <StoreProvider store={store}>
        <ExerciseDetailScreen slug="bench-press-barbell" fallback="/programs" />
      </StoreProvider>
    </ThemeProvider>,
  );

describe("ExerciseDetailScreen", () => {
  beforeEach(() => {
    jest.spyOn(Image, "getSize").mockImplementation((_uri, ok) => ok(600, 400));
  });

  it("renders the images, chips and details from the server", async () => {
    mockApi = apiWith(async () => ({ exercise: bench }));
    await renderDetail();
    expect(await screen.findByRole("header", { name: "Bench Press" })).toBeOnTheScreen();
    await fireEvent(screen.getByTestId("exercise-images"), "layout", {
      nativeEvent: { layout: { width: 320 } },
    });
    expect(screen.getAllByTestId("exercise-image")).toHaveLength(1);
    // Sized to the picture's own aspect once known, so no bands above or below it.
    expect(screen.getByTestId("exercise-image")).toHaveStyle({ width: 320, height: 213 });
    for (const muscle of ["chest", "triceps", "shoulders"]) {
      expect(screen.getByRole("checkbox", { name: muscle })).toBeOnTheScreen();
    }
    expect(screen.getByText("barbell")).toBeOnTheScreen();
    expect(screen.getByText("strength")).toBeOnTheScreen();
    expect(screen.getByText("Flat Bench Press")).toBeOnTheScreen();
    expect(screen.getByLabelText("Custom")).toBeOnTheScreen();
    expect(screen.getByText("Instructions")).toBeOnTheScreen();
    expect(screen.getByText("1. Lie on the bench with your eyes under the bar.")).toBeOnTheScreen();
    expect(screen.getByText("2. Press to lockout.")).toBeOnTheScreen();
  });

  it("renders from the cache when the server is unreachable", async () => {
    mockApi = apiWith(() => Promise.reject(new TypeError(OFFLINE)));
    const store = createMemoryStore();
    rememberExercise(store, bench);
    await renderDetail(store);
    expect(await screen.findByRole("header", { name: "Bench Press" })).toBeOnTheScreen();
    expect(screen.getByRole("checkbox", { name: "chest" })).toBeOnTheScreen();
  });

  it("says when the exercise cannot be loaded", async () => {
    mockApi = apiWith(() => Promise.reject(new TypeError(OFFLINE)));
    await renderDetail();
    expect(await screen.findByText("Exercise not available")).toBeOnTheScreen();
  });
});
