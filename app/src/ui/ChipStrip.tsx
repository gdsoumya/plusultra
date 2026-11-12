import { Children, useEffect, useRef, useState, type ReactNode } from "react";
import {
  ScrollView,
  View,
  type LayoutRectangle,
  type NativeScrollEvent,
  type ScrollViewInstance,
} from "react-native";

import { Dots } from "./Dots";
import { space } from "./tokens";

type Props = {
  children: ReactNode;
  /** Index of the child to keep in view, e.g. the selected chip. */
  selected?: number;
  testID?: string;
};

// Where to scroll so `box` sits inside the viewport with a gutter; undefined when it already does.
export function scrollTarget(box: LayoutRectangle, x: number, viewport: number) {
  const left = box.x - space[4];
  const right = box.x + box.width + space[4];
  if (left < x) return left;
  if (right > x + viewport) return right - viewport;
  return undefined;
}

// A single line of chips that scrolls sideways through the screen's gutters instead of wrapping;
// dots under it say how much more there is.
export function ChipStrip({ children, selected, testID }: Props) {
  const ref = useRef<ScrollViewInstance>(null);
  const boxes = useRef<LayoutRectangle[]>([]);
  const [viewport, setViewport] = useState(0);
  const [content, setContent] = useState(0);
  const [x, setX] = useState(0);
  const pages = viewport && content > viewport + 1 ? Math.ceil(content / viewport) : 1;

  useEffect(() => {
    // Children lay out before the strip, so the selected box is known by the first viewport.
    const box = selected !== undefined ? boxes.current[selected] : undefined;
    const to = box && viewport ? scrollTarget(box, x, viewport) : undefined;
    if (to !== undefined) ref.current?.scrollTo({ x: to, animated: true });
    // Only a new selection or size scrolls; the user's own scrolling must not snap back.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [selected, viewport, content]);

  const onScroll = ({ contentOffset }: NativeScrollEvent) => setX(contentOffset.x);
  return (
    <View style={{ gap: space[2] }}>
      <ScrollView
        ref={ref}
        horizontal
        showsHorizontalScrollIndicator={false}
        keyboardShouldPersistTaps="handled"
        scrollEventThrottle={16}
        onScroll={(e) => onScroll(e.nativeEvent)}
        onLayout={(e) => setViewport(e.nativeEvent.layout.width)}
        onContentSizeChange={setContent}
        style={{ marginHorizontal: -space[4] }}
        contentContainerStyle={{ paddingHorizontal: space[4], gap: space[2] }}
        testID={testID}
      >
        {/* One item per child; an empty one (null, false) gets none, so it takes no gap. */}
        {Children.map(children, (child, i) =>
          child == null ? null : (
            <View onLayout={(e) => (boxes.current[i] = e.nativeEvent.layout)}>{child}</View>
          ),
        )}
      </ScrollView>
      {/* By the scrolled fraction, since the last page is usually narrower than the viewport. */}
      <Dots
        count={pages}
        current={Math.round((x / Math.max(1, content - viewport)) * (pages - 1))}
      />
    </View>
  );
}
