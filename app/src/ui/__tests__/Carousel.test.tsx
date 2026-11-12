import { fireEvent, screen } from "@testing-library/react-native";
import { Platform, Text } from "react-native";

import { Carousel } from "../Carousel";
import { renderThemed } from "../test-utils";

let mockWide = false;
jest.mock("../useWide", () => ({ useWide: () => mockWide }));

const pages = ["a", "b", "c"];
const render = () =>
  renderThemed(
    <Carousel
      items={pages}
      testID="carousel"
      renderItem={(item, width) => <Text testID="page">{`${item} ${width}`}</Text>}
    />,
  );
const layout = () =>
  fireEvent(screen.getByTestId("carousel"), "layout", { nativeEvent: { layout: { width: 320 } } });
const scrollTo = (x: number) =>
  fireEvent.scroll(screen.getAllByTestId("page")[0].parent!.parent!, {
    nativeEvent: { contentOffset: { x } },
  });

afterEach(() => {
  mockWide = false;
  jest.restoreAllMocks();
});

describe("Carousel", () => {
  it("pages as wide as itself and moves the dot with the scroll", async () => {
    await render();
    expect(screen.queryByTestId("page")).toBeNull();
    await layout();
    expect(screen.getAllByText(/ 320$/)).toHaveLength(3);
    expect(screen.getByLabelText("Page 1 of 3")).toBeOnTheScreen();
    expect(screen.getAllByTestId("dot")).toHaveLength(2);
    await scrollTo(640);
    expect(screen.getByLabelText("Page 3 of 3")).toBeOnTheScreen();
    expect(screen.queryByRole("button", { name: "Next" })).toBeNull();
  });

  it("adds previous and next buttons on a wide web window", async () => {
    jest.replaceProperty(Platform, "OS", "web");
    mockWide = true;
    await render();
    await layout();
    expect(screen.getByRole("button", { name: "Previous" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Next" })).toBeEnabled();
  });
});
