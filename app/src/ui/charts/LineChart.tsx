import { Fragment } from "react";
import { View } from "react-native";
import Svg, { Circle, Line, Path, Text as SvgText } from "react-native-svg";

import { Text } from "../Text";
import { useTheme } from "../theme";
import { space, typography } from "../tokens";
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
  // Names of the points and the trend; shown as the legend when both are drawn.
  legend?: [string, string];
  summary: string;
  table: Table;
  testID?: string;
};

// Room for the y tick labels.
const gutter = 40;

export function LineChart({
  width,
  height = 160,
  points,
  trend,
  xDomain,
  xLabels,
  format,
  legend,
  summary,
  table,
  testID,
}: LineChartProps) {
  const { colors, fontScale } = useTheme();
  const fontSize = typography.caption.fontSize * fontScale;
  const top = fontSize + space[2];
  const ys = [...points, ...(trend ?? [])].map((p) => p.y);
  const ticks = niceTicks(Math.min(...ys), Math.max(...ys));
  const x = linear(xDomain, [gutter, width - space[2]]);
  const y = linear([ticks[0], ticks[ticks.length - 1]], [top + height, top]);
  const place = (p: Point) => ({ x: x(p.x), y: y(p.y) });
  const series = trend ?? points;
  const line = series.map(place);
  const end = line[line.length - 1];

  return (
    <ChartFrame summary={summary} table={table} testID={testID}>
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
        {points.map(place).map((p, i) => (
          <Circle
            key={i}
            testID="dot"
            cx={p.x}
            cy={p.y}
            r={5}
            fill={trend ? colors.border : colors.accent}
            stroke={colors.surface}
            strokeWidth={2}
          />
        ))}
        {end && (
          <SvgText
            x={end.x}
            y={end.y - space[3]}
            fill={colors.text}
            fontSize={fontSize}
            fontWeight="600"
            textAnchor="end"
          >
            {format(series[series.length - 1].y)}
          </SvgText>
        )}
      </Svg>
      <View style={{ flexDirection: "row", justifyContent: "space-between", marginLeft: gutter }}>
        <Text variant="caption">{xLabels[0]}</Text>
        <Text variant="caption">{xLabels[1]}</Text>
      </View>
      {trend && legend && (
        <View style={{ flexDirection: "row", gap: space[4] }}>
          <Key label={legend[0]}>
            <View
              style={{ width: 8, height: 8, borderRadius: 4, backgroundColor: colors.border }}
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

const Key = ({ label, children }: { label: string; children: React.ReactNode }) => (
  <View style={{ flexDirection: "row", alignItems: "center", gap: space[1] }}>
    {children}
    <Text variant="caption">{label}</Text>
  </View>
);
