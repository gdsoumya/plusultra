import { Fragment, useState } from "react";
import { Pressable, View } from "react-native";
import Svg, { Circle, Line, Path, Text as SvgText } from "react-native-svg";

import { Button } from "../Button";
import { IconButton } from "../IconButton";
import { Text } from "../Text";
import { useTheme } from "../theme";
import { minTouch, space, typography } from "../tokens";
import { ChartFrame, type Table } from "./ChartFrame";
import { linear, linePath, niceTicks, type Point } from "./scale";

export type LineChartProps = {
  width: number;
  // Plot height without the axis labels.
  height?: number;
  // Measured values, drawn as dots; the line joins them unless a trend is given.
  points: Point[];
  // Smoothed series drawn as the line, e.g. a moving average.
  trend?: Point[];
  xDomain: [number, number];
  xLabels: [string, string];
  format: (y: number) => string;
  // Least range of the y axis, so a flat series is drawn flat rather than blown up to the plot.
  minSpan?: number;
  // Names of the points and the trend; shown as the legend when both are drawn.
  legend?: [string, string];
  // Names a tapped point's x in its caption, e.g. as a date.
  xLabel?: (x: number) => string;
  // Offers "Open workout" for the tapped point.
  onOpen?: (index: number) => void;
  summary: string;
  table: Table;
  testID?: string;
};

// Room for the y tick labels.
const gutter = 40;
// Weigh-ins under a trend line are muted; the key dot matches.
const dotColor = (colors: { accent: string; textMuted: string }, trend: boolean) =>
  trend ? colors.textMuted : colors.accent;

export function LineChart({
  width,
  height = 160,
  points,
  trend,
  xDomain,
  xLabels,
  format,
  minSpan = 0,
  legend,
  xLabel,
  onOpen,
  summary,
  table,
  testID,
}: LineChartProps) {
  const { colors, fontScale } = useTheme();
  const [selected, setSelected] = useState<number>();
  const fontSize = typography.caption.fontSize * fontScale;
  // The band above the plot holds the end value, clear of the last points.
  const top = fontSize + space[2];
  const ys = [...points, ...(trend ?? [])].map((p) => p.y);
  const ticks = niceTicks(Math.min(...ys), Math.max(...ys), 4, minSpan);
  const x = linear(xDomain, [gutter, width - space[2]]);
  const y = linear([ticks[0], ticks[ticks.length - 1]], [top + height, top]);
  const place = (p: Point) => ({ x: x(p.x), y: y(p.y) });
  const series = trend ?? points;
  const line = series.map(place);
  const end = line[line.length - 1];
  const dots = points.map(place);
  // The callers' arrays change under a kept index, e.g. when the lift changes.
  const picked = selected !== undefined && selected < points.length ? selected : undefined;
  const pick = (atX: number) => {
    let nearest = 0;
    dots.forEach((d, i) => {
      if (Math.abs(d.x - atX) < Math.abs(dots[nearest].x - atX)) nearest = i;
    });
    setSelected(nearest === picked ? undefined : nearest);
  };

  return (
    <ChartFrame summary={summary} table={table} testID={testID}>
      <Pressable testID={testID && `${testID}-plot`} onPress={(e) => pick(e.nativeEvent.locationX)}>
        <Svg width={width} height={top + height + space[2]}>
          {ticks.map((t) => (
            <Fragment key={t}>
              <Line
                x1={gutter}
                x2={width}
                y1={y(t)}
                y2={y(t)}
                stroke={colors.border}
                strokeWidth={1}
              />
              <SvgText
                x={gutter - space[2]}
                y={y(t) + fontSize / 3}
                fill={colors.textMuted}
                fontSize={fontSize}
                textAnchor="end"
              >
                {format(t)}
              </SvgText>
            </Fragment>
          ))}
          {line.length > 1 && (
            <Path
              d={linePath(line)}
              stroke={colors.accent}
              strokeWidth={2}
              strokeLinejoin="round"
              strokeLinecap="round"
              fill="none"
            />
          )}
          {picked !== undefined && (
            <Line
              testID="marker"
              x1={dots[picked].x}
              x2={dots[picked].x}
              y1={top}
              y2={top + height}
              stroke={colors.textMuted}
              strokeWidth={1}
            />
          )}
          {dots.map((p, i) => (
            <Circle
              key={i}
              testID="dot"
              cx={p.x}
              cy={p.y}
              r={i === picked ? 8 : 5}
              fill={dotColor(colors, !!trend)}
              stroke={colors.surface}
              strokeWidth={2}
            />
          ))}
          {end && (
            <SvgText
              x={end.x}
              y={fontSize}
              fill={colors.text}
              fontSize={fontSize}
              fontWeight="600"
              textAnchor="end"
            >
              {format(series[series.length - 1].y)}
            </SvgText>
          )}
        </Svg>
      </Pressable>
      <View style={{ flexDirection: "row", justifyContent: "space-between", marginLeft: gutter }}>
        <Text variant="caption">{xLabels[0]}</Text>
        <Text variant="caption">{xLabels[1]}</Text>
      </View>
      {picked !== undefined && (
        <View style={{ flexDirection: "row", alignItems: "center", gap: space[2] }}>
          <Text variant="caption" color="text" style={{ flex: 1 }}>
            {[xLabel?.(points[picked].x), format(points[picked].y)].filter(Boolean).join(" · ")}
          </Text>
          {onOpen && (
            <Button
              variant="ghost"
              title="Open workout"
              onPress={() => onOpen(picked)}
              style={{ paddingVertical: 0, minHeight: minTouch }}
            />
          )}
          <IconButton icon="close" label="Clear selection" onPress={() => setSelected(undefined)} />
        </View>
      )}
      {trend && legend && (
        <View style={{ flexDirection: "row", gap: space[4] }}>
          <Key label={legend[0]}>
            <View
              style={{ width: 8, height: 8, borderRadius: 4, backgroundColor: colors.textMuted }}
            />
          </Key>
          <Key label={legend[1]}>
            <View style={{ width: 16, height: 2, backgroundColor: colors.accent }} />
          </Key>
        </View>
      )}
    </ChartFrame>
  );
}

export const Key = ({ label, children }: { label: string; children: React.ReactNode }) => (
  <View style={{ flexDirection: "row", alignItems: "center", gap: space[1] }}>
    {children}
    <Text variant="caption">{label}</Text>
  </View>
);
