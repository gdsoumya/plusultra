import { fireEvent, screen } from "@testing-library/react-native";

import { Chip } from "../Chip";
import { ChipStrip, scrollTarget } from "../ChipStrip";
import { renderThemed } from "../test-utils";

const weeks = [1, 2, 3, 4, 5, 6];
const render = (selected?: number) =>
  renderThemed(
    <ChipStrip selected={selected} testID="strip">
      {weeks.map((w) => (
        <Chip key={w} label={`Week ${w}`} selected={w - 1 === selected} />
      ))}
    </ChipStrip>,
  );
const strip = () => screen.getByTestId("strip");
const sized = async (viewport: number, content: number) => {
  await fireEvent(strip(), "layout", { nativeEvent: { layout: { width: viewport } } });
  await fireEvent(strip(), "contentSizeChange", content, 36);
};

describe("ChipStrip", () => {
  it("shows every chip in one line with dots only when they overflow", async () => {
    await render();
    expect(screen.getAllByRole("checkbox")).toHaveLength(6);
    await sized(400, 300);
    expect(screen.queryByLabelText(/^Page /)).toBeNull();
    await sized(400, 900);
    expect(screen.getByLabelText("Page 1 of 3")).toBeOnTheScreen();
    await fireEvent.scroll(strip(), { nativeEvent: { contentOffset: { x: 500 } } });
    expect(screen.getByLabelText("Page 2 of 3")).toBeOnTheScreen();
  });

  it("brings a chip into view with a gutter, only when it is outside", () => {
    const box = { x: 500, y: 0, width: 90, height: 36 };
    expect(scrollTarget(box, 0, 400)).toBe(590 + 16 - 400);
    expect(scrollTarget(box, 600, 400)).toBe(500 - 16);
    expect(scrollTarget(box, 300, 400)).toBeUndefined();
  });
});
