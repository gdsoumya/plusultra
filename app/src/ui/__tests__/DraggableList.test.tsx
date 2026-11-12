import { act, fireEvent, screen } from "@testing-library/react-native";
import { GestureHandlerRootView, State } from "react-native-gesture-handler";
import { fireGestureHandler, getByGestureTestId } from "react-native-gesture-handler/jest-utils";

import { DraggableList, dropIndex } from "../DraggableList";
import { Text } from "../Text";
import { renderThemed } from "../test-utils";

const items = ["Breakfast", "Lunch", "Snack", "Dinner"];

async function setup() {
  const onMove = jest.fn();
  await renderThemed(
    <GestureHandlerRootView>
      <DraggableList
        items={items}
        keyOf={(m) => m}
        onMove={onMove}
        testID="meals"
        renderItem={(m) => <Text>{m}</Text>}
      />
    </GestureHandlerRootView>,
  );
  for (const i of items.keys()) {
    await fireEvent(screen.getByTestId(`meals-${i}`), "layout", {
      nativeEvent: { layout: { height: 100 } },
    });
  }
  return onMove;
}

// The drop is reported through scheduleOnRN, which lands on the next microtask.
async function drag(index: number, translationY: number) {
  fireGestureHandler(getByGestureTestId(`meals-${index}-pan`), [
    { state: State.BEGAN },
    { state: State.ACTIVE },
    { translationY },
    { state: State.END, translationY },
  ]);
  await act(async () => {});
}

describe("DraggableList", () => {
  it("renders every item in order", async () => {
    await setup();
    expect(screen.getAllByText(/./).map((t) => t.props.children)).toEqual(items);
  });

  it("reports where a long-press drag drops the row", async () => {
    const onMove = await setup();
    await drag(0, 230);
    expect(onMove).toHaveBeenLastCalledWith(0, 2);
    await drag(3, -120);
    expect(onMove).toHaveBeenLastCalledWith(3, 2);
  });

  it("ignores a drag that ends where it started", async () => {
    const onMove = await setup();
    await drag(1, 30);
    expect(onMove).not.toHaveBeenCalled();
  });
});

describe("dropIndex", () => {
  const heights = [100, 50, 200, 100];
  it("passes a row once the drag covers more than half of it", () => {
    expect(dropIndex(0, 24, heights, 0)).toBe(0);
    expect(dropIndex(0, 26, heights, 0)).toBe(1);
    expect(dropIndex(0, 150, heights, 0)).toBe(1);
    expect(dropIndex(0, 151, heights, 0)).toBe(2);
    expect(dropIndex(3, -101, heights, 0)).toBe(2);
    expect(dropIndex(3, -500, heights, 0)).toBe(0);
  });
  it("counts the gap between rows", () => {
    expect(dropIndex(0, 30, heights, 16)).toBe(0);
    expect(dropIndex(0, 34, heights, 16)).toBe(1);
  });
});
