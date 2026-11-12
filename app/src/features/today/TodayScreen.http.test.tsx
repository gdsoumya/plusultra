import { createClients } from "../../api/clients";
import { createMockTransport } from "../../api/mockTransport";
import { fireEvent, render, screen } from "@testing-library/react-native";

import { createMemoryStore } from "../../data/memoryStore";
import { StoreProvider } from "../../data/StoreProvider";
import { insecureOrigin } from "../../data/test-utils";
import { ThemeProvider } from "../../ui";
import TodayScreen from "./TodayScreen";

// What expo-crypto's web module does: hand both calls to the page's crypto.
jest.mock("expo-crypto", () => ({
  randomUUID: () => globalThis.crypto.randomUUID(),
  getRandomValues: (a: Uint8Array<ArrayBuffer>) => globalThis.crypto.getRandomValues(a),
}));
jest.mock("expo-router", () => ({ useRouter: () => ({ push: jest.fn() }) }));
const mockApi = createClients(createMockTransport({ today: "2026-11-11" }).transport);
jest.mock("../../api/useApi", () => ({ useApi: () => mockApi }));

it("saves a check-in on a web page served over plain http", async () => {
  insecureOrigin();
  const store = createMemoryStore();
  const changed = jest.fn();
  store.onChange(changed);
  await render(
    <ThemeProvider>
      <StoreProvider store={store}>
        <TodayScreen today="2026-11-11" unit="kg" userId="user-1" />
      </StoreProvider>
    </ThemeProvider>,
  );
  await fireEvent.changeText(screen.getByLabelText("Weight (kg)"), "80.5");
  await fireEvent.press(screen.getByRole("button", { name: "Save weight" }));

  expect(store.listDirty("checkins")).toEqual([
    expect.objectContaining({ date: "2026-11-11", weight: { value: 80.5, unit: "kg" } }),
  ]);
  expect(changed).toHaveBeenCalled();
  expect(screen.getByText("80.5 kg")).toBeOnTheScreen();
});
