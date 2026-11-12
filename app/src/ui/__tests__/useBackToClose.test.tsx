import { act, screen } from "@testing-library/react-native";
import { BackHandler } from "react-native";

import { Sheet } from "../Sheet";
import { Text } from "../Text";
import { ThemeProvider } from "../theme";
import { renderThemed } from "../test-utils";

type Handler = Parameters<typeof BackHandler.addEventListener>[1];
const back = { type: "hardwareBackPress", timeStamp: 0 } as const;

// The Android hardware back button as the native layer delivers it to BackHandler.
function mockBack() {
  const handlers = new Set<Handler>();
  jest.spyOn(BackHandler, "addEventListener").mockImplementation((_, h) => {
    handlers.add(h);
    return { remove: () => handlers.delete(h) };
  });
  return {
    press: () => act(() => [...handlers].reverse().some((h) => h(back))),
    count: () => handlers.size,
  };
}

const sheet = (open: boolean, onClose: () => void) => (
  <Sheet open={open} onClose={onClose}>
    <Text>Body</Text>
  </Sheet>
);

afterEach(() => jest.restoreAllMocks());

it("closes an open sheet on hardware back and lets the button through once closed", async () => {
  const back = mockBack();
  const onClose = jest.fn();
  const view = await renderThemed(sheet(true, onClose));
  expect(screen.getByText("Body")).toBeOnTheScreen();
  expect(await back.press()).toBe(true);
  expect(onClose).toHaveBeenCalledTimes(1);

  await view.rerender(<ThemeProvider>{sheet(false, onClose)}</ThemeProvider>);
  expect(back.count()).toBe(0);
  expect(await back.press()).toBe(false);
  expect(onClose).toHaveBeenCalledTimes(1);
});
