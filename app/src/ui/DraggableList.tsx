import type { ReactNode } from "react";
import { View } from "react-native";
import { GestureDetector, usePanGesture } from "react-native-gesture-handler";
import Animated, {
  useAnimatedStyle,
  useSharedValue,
  type SharedValue,
} from "react-native-reanimated";
import { scheduleOnRN } from "react-native-worklets";

import { ListRow } from "./ListRow";
import { motion, space } from "./tokens";

export type DraggableListProps<T> = {
  items: T[];
  keyOf: (item: T, index: number) => string;
  renderItem: (item: T, index: number) => ReactNode;
  /** Called once a drag ends on another position; the parent reorders and persists. */
  onMove: (from: number, to: number) => void;
  /** Row `<testID>-<i>` and its pan gesture `<testID>-<i>-pan`. */
  testID?: string;
};

// The index a row dragged by dy lands on: it passes a neighbour once it covers more than half of it.
export function dropIndex(from: number, dy: number, heights: number[], gap: number) {
  "worklet";
  const step = dy > 0 ? 1 : -1;
  let to = from;
  let left = Math.abs(dy);
  while (to + step >= 0 && to + step < heights.length) {
    const next = heights[to + step] + gap;
    if (left <= next / 2) break;
    left -= next;
    to += step;
  }
  return to;
}

const gap = space[4];

type Shared = {
  active: SharedValue<number>;
  dy: SharedValue<number>;
  heights: SharedValue<number[]>;
};

type RowProps = Shared & {
  index: number;
  onMove: (from: number, to: number) => void;
  testID?: string;
  children: ReactNode;
};

function Row({ index, active, dy, heights, onMove, testID, children }: RowProps) {
  const pan = usePanGesture({
    testID: testID && `${testID}-pan`,
    activateAfterLongPress: motion.longPress,
    onActivate: () => {
      active.set(index);
      dy.set(0);
    },
    onUpdate: (e) => {
      dy.set(e.translationY);
    },
    onDeactivate: () => {
      const to = dropIndex(index, dy.get(), heights.get(), gap);
      active.set(-1);
      dy.set(0);
      if (to !== index) scheduleOnRN(onMove, index, to);
    },
  });
  const style = useAnimatedStyle(() => {
    const a = active.get();
    if (a === index) return { transform: [{ translateY: dy.get() }], zIndex: 1 };
    if (a < 0) return { transform: [{ translateY: 0 }], zIndex: 0 };
    const h = heights.get();
    const to = dropIndex(a, dy.get(), h, gap);
    const shift = h[a] + gap;
    const y = a < index && index <= to ? -shift : to <= index && index < a ? shift : 0;
    return { transform: [{ translateY: y }], zIndex: 0 };
  });
  return (
    <GestureDetector gesture={pan}>
      <Animated.View
        testID={testID}
        style={style}
        onLayout={(e) => {
          const { height } = e.nativeEvent.layout;
          // On the UI thread: rows laid out in one frame would each overwrite a stale JS-side copy.
          heights.modify((h) => {
            "worklet";
            const next = h.slice();
            next[index] = height;
            return next;
          });
        }}
      >
        {children}
      </Animated.View>
    </GestureDetector>
  );
}

// Rows reordered by long-press drag. Screen readers reorder through the row's own menu instead.
export function DraggableList<T>({
  items,
  keyOf,
  renderItem,
  onMove,
  testID,
}: DraggableListProps<T>) {
  const active = useSharedValue(-1);
  const dy = useSharedValue(0);
  const heights = useSharedValue<number[]>([]);
  return (
    <View style={{ gap }}>
      {items.map((item, i) => (
        <Row
          key={keyOf(item, i)}
          index={i}
          active={active}
          dy={dy}
          heights={heights}
          onMove={onMove}
          testID={testID && `${testID}-${i}`}
        >
          {renderItem(item, i)}
        </Row>
      ))}
    </View>
  );
}

// Keyboard and screen-reader reordering for a row's menu: Move up and Move down, when possible.
export function MoveRows({
  index,
  count,
  onMove,
}: {
  index: number;
  count: number;
  onMove: (to: number) => void;
}) {
  return (
    <>
      {index > 0 && <ListRow icon="arrow-up" title="Move up" onPress={() => onMove(index - 1)} />}
      {index < count - 1 && (
        <ListRow icon="arrow-down" title="Move down" onPress={() => onMove(index + 1)} />
      )}
    </>
  );
}
