import { useRef, useState, type ReactNode } from "react";
import {
  Platform,
  ScrollView,
  View,
  type NativeScrollEvent,
  type ScrollViewInstance,
} from "react-native";

import { Dots } from "./Dots";
import { IconButton } from "./IconButton";
import { useTheme } from "./theme";
import { radius, space } from "./tokens";
import { useWide } from "./useWide";

type Props<T> = {
  items: T[];
  /** One page, as wide as the carousel. */
  renderItem: (item: T, width: number) => ReactNode;
  testID?: string;
};

// Pages sideways through items, one per swipe, with dots under them; a mouse user on a wide web
// window gets previous and next buttons instead of a scrollbar.
export function Carousel<T>({ items, renderItem, testID }: Props<T>) {
  const { colors } = useTheme();
  const ref = useRef<ScrollViewInstance>(null);
  const [width, setWidth] = useState(0);
  const [page, setPage] = useState(0);
  const wide = useWide();
  const buttons = Platform.OS === "web" && wide && items.length > 1;
  const onScroll = ({ contentOffset }: NativeScrollEvent) =>
    setPage(Math.min(items.length - 1, Math.max(0, Math.round(contentOffset.x / width))));
  const go = (to: number) => ref.current?.scrollTo({ x: to * width, animated: true });
  return (
    <View style={{ gap: space[2] }}>
      <View testID={testID} onLayout={(e) => setWidth(e.nativeEvent.layout.width)}>
        {width > 0 && (
          <ScrollView
            ref={ref}
            horizontal
            pagingEnabled
            showsHorizontalScrollIndicator={false}
            scrollEventThrottle={16}
            onScroll={(e) => onScroll(e.nativeEvent)}
            style={{ borderRadius: radius.control, backgroundColor: colors.surfaceRaised }}
          >
            {items.map((item, i) => (
              <View key={i} style={{ width }}>
                {renderItem(item, width)}
              </View>
            ))}
          </ScrollView>
        )}
      </View>
      {(buttons || items.length > 1) && (
        <View style={{ flexDirection: "row", alignItems: "center", justifyContent: "center" }}>
          {buttons && (
            <IconButton
              icon="chevron-back"
              label="Previous"
              disabled={page === 0}
              onPress={() => go(page - 1)}
            />
          )}
          <Dots count={items.length} current={page} />
          {buttons && (
            <IconButton
              icon="chevron-forward"
              label="Next"
              disabled={page === items.length - 1}
              onPress={() => go(page + 1)}
            />
          )}
        </View>
      )}
    </View>
  );
}
