import { act, screen } from "@testing-library/react-native";
import { GestureHandlerRootView, State } from "react-native-gesture-handler";
import { fireGestureHandler, getByGestureTestId } from "react-native-gesture-handler/jest-utils";

import { DraggableList } from "../DraggableList";
import { Text } from "../Text";
import { renderThemed } from "../test-utils";

// Row heights are written from onLayout on the JS thread, where a device defers the write to the UI
// thread and reads return the last value it handed back. The gesture's own values (numbers) live
// on the UI thread and stay synchronous.
jest.mock("react-native-reanimated", () => {
  const actual = jest.requireActual("react-native-reanimated");
  const { useRef } = jest.requireActual("react");
  const deferred = <T,>(initial: T) => {
    let committed = initial;
    const later = (apply: () => void) => void Promise.resolve().then(apply);
    return {
      get: () => committed,
      set: (v: T) => later(() => (committed = v)),
      modify: (f: (v: T) => T) => later(() => (committed = f(committed))),
      get value() {
        return committed;
      },
    };
  };
  return {
    __esModule: true,
    ...actual,
    default: actual.default,
    useSharedValue: <T,>(v: T) =>
      useRef(Array.isArray(v) ? deferred(v) : actual.makeMutable(v)).current,
  };
});

const items = ["Breakfast", "Lunch", "Snack", "Dinner"];

it("keeps every row height measured in the same frame", async () => {
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
  // All rows lay out before the UI thread applies any of the writes.
  await act(async () => {
    for (const i of items.keys()) {
      screen.getByTestId(`meals-${i}`).props.onLayout({ nativeEvent: { layout: { height: 100 } } });
    }
  });
  fireGestureHandler(getByGestureTestId("meals-3-pan"), [
    { state: State.BEGAN },
    { state: State.ACTIVE },
    { translationY: -120 },
    { state: State.END, translationY: -120 },
  ]);
  await act(async () => {});
  expect(onMove).toHaveBeenLastCalledWith(3, 2);
});
