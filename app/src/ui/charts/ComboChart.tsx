import { Fragment } from "react";
import { View } from "react-native";
import Svg, { Circle, Line, Path, Rect, Text as SvgText } from "react-native-svg";

import { useTheme } from "../theme";
import { space, typography } from "../tokens";
import { ChartFrame, type Table } from "./ChartFrame";
import { Key } from "./LineChart";
import { linear, linePath, niceTicks } from "./scale";

export type ComboChartProps = {
  width: number;
  // Plot height without the axis labels.
  height?: number;
  // One slot per label, e.g. a week; a missing bar or line value leaves its slot empty.
  labels: string[];
  bars: (number | undefined)[];
  // Second series on a right-hand axis, with gaps where values are missing.
  line?: (number | undefined)[];
  format: (bar: number) => string;
  lineFormat?: (v: number) => string;
  // Least range of the right axis, so a flat line is drawn flat.
  lineMinSpan?: number;
  // Names of the bars and the line, shown as the legend when both are drawn.
  legend?: [string, string];
  summary: string;
  table: Table;
  testID?: string;
};

// Room for an axis' tick labels.
const gutter = 40;
const defined = (values: (number | undefined)[]) => values.filter((v) => v !== undefined);

export function ComboChart({
  width,
  height = 160,
  labels,
  bars,
  line,
  format,
  lineFormat = format,
  lineMinSpan = 0,
  legend,
  summary,
  table,
  testID,
}: ComboChartProps) {
  const { colors, fontScale } = useTheme();
  const fontSize = typography.caption.fontSize * fontScale;
  const top = space[2];
  const lineValues = defined(line ?? []);
  const right = lineValues.length ? width - gutter : width - space[2];
  const slot = (right - gutter) / Math.max(1, labels.length);
  const barWidth = slot * 0.6;
  const mid = (i: number) => gutter + slot * (i + 0.5);

  const barTicks = niceTicks(0, Math.max(1, ...defined(bars)), 4);
  const barY = linear([barTicks[0], barTicks[barTicks.length - 1]], [top + height, top]);
  const lineTicks = lineValues.length
    ? niceTicks(Math.min(...lineValues), Math.max(...lineValues), 4, lineMinSpan)
    : [];
  const lineY = linear([lineTicks[0], lineTicks[lineTicks.length - 1]], [top + height, top]);
  // Runs of consecutive values, each its own path, so a missing week breaks the line.
  const runs = (line ?? []).reduce<{ x: number; y: number }[][]>((all, v, i) => {
    if (v === undefined) return [...all, []];
    const last = all[all.length - 1] ?? [];
    return [...all.slice(0, -1), [...last, { x: mid(i), y: lineY(v) }]];
  }, []);
  // About six x labels, evenly spread; one that would run past an edge hugs it instead.
  const every = Math.ceil(labels.length / 6);
  const anchor = (x: number, label: string) => {
    const half = label.length * fontSize * 0.3;
    return x - half < 0 ? "start" : x + half > width ? "end" : "middle";
  };

  return (
    <ChartFrame summary={summary} table={table} testID={testID}>
      <Svg width={width} height={top + height + fontSize + space[3]}>
        {barTicks.map((t) => (
          <Fragment key={t}>
            <Line
              x1={gutter}
              x2={right}
              y1={barY(t)}
              y2={barY(t)}
              stroke={colors.border}
              strokeWidth={1}
            />
            <SvgText
              x={gutter - space[2]}
              y={barY(t) + fontSize / 3}
              fill={colors.textMuted}
              fontSize={fontSize}
              textAnchor="end"
            >
              {format(t)}
            </SvgText>
          </Fragment>
        ))}
        {lineTicks.map((t) => (
          <SvgText
            key={t}
            x={right + space[2]}
            y={lineY(t) + fontSize / 3}
            fill={colors.textMuted}
            fontSize={fontSize}
          >
            {lineFormat(t)}
          </SvgText>
        ))}
        {bars.map(
          (v, i) =>
            v !== undefined && (
              <Rect
                key={i}
                testID="bar"
                x={mid(i) - barWidth / 2}
                y={barY(v)}
                width={barWidth}
                height={barY(0) - barY(v)}
                fill={colors.accent}
                rx={2}
              />
            ),
        )}
        {runs.map(
          (run, i) =>
            run.length > 1 && (
              <Path
                key={i}
                d={linePath(run)}
                stroke={colors.info}
                strokeWidth={2}
                strokeLinejoin="round"
                strokeLinecap="round"
                fill="none"
              />
            ),
        )}
        {runs.flat().map((p, i) => (
          <Circle key={i} testID="dot" cx={p.x} cy={p.y} r={4} fill={colors.info} />
        ))}
        {labels.map(
          (label, i) =>
            i % every === 0 && (
              <SvgText
                key={label}
                testID="x-label"
                x={mid(i)}
                y={top + height + fontSize + space[1]}
                fill={colors.textMuted}
                fontSize={fontSize}
                textAnchor={anchor(mid(i), label)}
              >
                {label}
              </SvgText>
            ),
        )}
      </Svg>
      {line && legend && (
        <View style={{ flexDirection: "row", gap: space[4] }}>
          <Key label={legend[0]}>
            <View
              style={{ width: 8, height: 8, borderRadius: 2, backgroundColor: colors.accent }}
            />
          </Key>
          <Key label={legend[1]}>
            <View style={{ width: 16, height: 2, backgroundColor: colors.info }} />
          </Key>
        </View>
      )}
    </ChartFrame>
  );
}
