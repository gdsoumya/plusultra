import { fireEvent, screen } from "@testing-library/react-native";

import { BackButton } from "../BackButton";
import { renderThemed } from "../test-utils";

const mockRouter = { back: jest.fn(), replace: jest.fn(), canGoBack: jest.fn(() => true) };
jest.mock("expo-router", () => ({ useRouter: () => mockRouter }));

beforeEach(() => jest.clearAllMocks());

it("pops when there is history", async () => {
  await renderThemed(<BackButton fallback="/settings" />);
  await fireEvent.press(screen.getByRole("button", { name: "Back" }));
  expect(mockRouter.back).toHaveBeenCalledTimes(1);
  expect(mockRouter.replace).not.toHaveBeenCalled();
});

it("lands on the fallback after a deep link", async () => {
  mockRouter.canGoBack.mockReturnValue(false);
  await renderThemed(<BackButton fallback="/settings" />);
  await fireEvent.press(screen.getByRole("button", { name: "Back" }));
  expect(mockRouter.replace).toHaveBeenCalledWith("/settings");
});
