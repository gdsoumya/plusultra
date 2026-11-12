import { fireEvent, render, screen } from "@testing-library/react-native";

import { ThemeProvider } from "../../ui";
import GalleryScreen from "./GalleryScreen";

const sections = [
  "Text",
  "Numeral",
  "Button",
  "Chip",
  "Badge",
  "Field",
  "Progress",
  "Charts",
  "ListRow",
  "Card",
  "Icon",
  "EmptyState",
  "Loading, ErrorLine and Divider",
  "Sheet",
  "Text scale",
];

async function renderGallery() {
  await render(
    <ThemeProvider>
      <GalleryScreen />
    </ThemeProvider>,
  );
}

it("lists every component", async () => {
  await renderGallery();
  for (const name of sections) {
    expect(screen.getByRole("header", { name })).toBeOnTheScreen();
  }
});

it("opens the sheet", async () => {
  await renderGallery();
  expect(screen.queryByTestId("gallery-sheet")).toBeNull();
  await fireEvent.press(screen.getByRole("button", { name: "Open sheet" }));
  expect(screen.getByTestId("gallery-sheet")).toBeOnTheScreen();
});
