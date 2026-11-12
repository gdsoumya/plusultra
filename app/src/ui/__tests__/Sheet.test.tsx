import { act, fireEvent, screen, within } from "@testing-library/react-native";
import { Dimensions, FlatList } from "react-native";
import { State } from "react-native-gesture-handler";
import { fireGestureHandler, getByGestureTestId } from "react-native-gesture-handler/jest-utils";

import { Sheet, type SheetProps } from "../Sheet";
import { Text } from "../Text";
import { ThemeProvider } from "../theme";
import { renderThemed, snapshots } from "../test-utils";

const { height } = Dimensions.get("window");
const body = <Text>Sheet body</Text>;
const settle = () => act(() => jest.advanceTimersByTime(1000));
const sheetHeight = () => screen.getByTestId("sheet").props.jestAnimatedStyle.value.height;
const scrollViews = () => JSON.stringify(screen.toJSON()).match(/"RCTScrollView"/g)?.length ?? 0;

async function renderSheet(props: Partial<SheetProps> = {}) {
  const onClose = jest.fn();
  await renderThemed(
    <Sheet open onClose={onClose} title="Log set" snapPoints={[0.5, 0.9]} testID="sheet" {...props}>
      {body}
    </Sheet>,
  );
  await settle();
  return onClose;
}

function drag(translationY: number, velocityY = 0) {
  fireGestureHandler(getByGestureTestId("sheet-pan"), [
    { state: State.BEGAN },
    { state: State.ACTIVE },
    { translationY },
    { state: State.END, translationY, velocityY },
  ]);
}

describe("Sheet", () => {
  snapshots(() => (
    <Sheet open title="Log set" onClose={() => {}}>
      {body}
    </Sheet>
  ));

  beforeEach(() => jest.useFakeTimers());
  afterEach(() => jest.useRealTimers());

  it("opens and closes via props", async () => {
    const view = await renderThemed(
      <Sheet open={false} onClose={() => {}}>
        {body}
      </Sheet>,
    );
    expect(screen.queryByText("Sheet body")).toBeNull();

    const rerender = (open: boolean) =>
      view.rerender(
        <ThemeProvider>
          <Sheet open={open} onClose={() => {}}>
            {body}
          </Sheet>
        </ThemeProvider>,
      );
    await rerender(true);
    expect(screen.getByText("Sheet body")).toBeOnTheScreen();
    await rerender(false);
    await settle();
    expect(screen.queryByText("Sheet body")).toBeNull();
  });

  it("asks to close when the backdrop is pressed", async () => {
    const onClose = await renderSheet();
    await fireEvent.press(screen.getByLabelText("Close"));
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it("is only as tall as its current snap point", async () => {
    await renderSheet();
    expect(sheetHeight()).toBeCloseTo(height * 0.5);
  });

  it("fits above the keyboard", async () => {
    await renderSheet({ snapPoints: [0.9] });
    // KeyboardAvoidingView shrinks the anchor when the keyboard opens.
    await fireEvent(screen.getByTestId("sheet-anchor"), "layout", {
      nativeEvent: { layout: { x: 0, y: 0, width: 400, height: 500 } },
    });
    await settle();
    expect(sheetHeight()).toBeCloseTo(500);
  });

  it("drags only from the handle, so the body can scroll", async () => {
    await renderSheet();
    const handle = screen.getByTestId("sheet-handle");
    expect(within(handle).getByText("Log set")).toBeOnTheScreen();
    expect(within(handle).queryByText("Sheet body")).toBeNull();
  });

  it("scrolls a long body so the last line is reachable", async () => {
    const lines = Array.from({ length: 60 }, (_, i) => `Line ${i + 1}`);
    await renderThemed(
      <Sheet open onClose={() => {}} testID="sheet">
        {lines.map((line) => (
          <Text key={line}>{line}</Text>
        ))}
      </Sheet>,
    );
    const sheet = within(screen.getByTestId("sheet"));
    expect(sheet.getByText("Line 60")).toBeOnTheScreen();
    expect(scrollViews()).toBe(1);
  });

  it("leaves scrolling to a body that is a list", async () => {
    const error = jest.spyOn(console, "error").mockImplementation(() => {});
    await renderThemed(
      <Sheet open onClose={() => {}} scroll={false} testID="sheet">
        <FlatList data={["a", "b"]} renderItem={({ item }) => <Text>{item}</Text>} />
      </Sheet>,
    );
    const sheet = within(screen.getByTestId("sheet"));
    expect(sheet.getByText("b")).toBeOnTheScreen();
    expect(scrollViews()).toBe(1);
    expect(error).not.toHaveBeenCalled();
    error.mockRestore();
  });

  it("snaps to the nearest point after a drag", async () => {
    const onClose = await renderSheet();
    await act(() => drag(-height * 0.3));
    await settle();
    expect(sheetHeight()).toBeCloseTo(height * 0.9);
    expect(onClose).not.toHaveBeenCalled();
  });

  it("closes after a downward swipe finishes", async () => {
    const onClose = await renderSheet();
    await act(() => drag(height * 0.2, 2000));
    expect(onClose).not.toHaveBeenCalled();
    await settle();
    expect(onClose).toHaveBeenCalledTimes(1);
  });
});
