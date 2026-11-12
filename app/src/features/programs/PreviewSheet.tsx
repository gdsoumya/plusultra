import { View } from "react-native";

import { setsSummary } from "../../domain/sets";
import type { ResolvedDay, Unit } from "../../domain/types";
import { Card, Sheet, space, Text } from "../../ui";

type Props = {
  week: number;
  unit: Unit;
  preview?: { days: ResolvedDay[]; output: string[] };
  onClose: () => void;
};

export function PreviewSheet({ week, unit, preview, onClose }: Props) {
  return (
    <Sheet open={!!preview} onClose={onClose} title={`Week ${week} preview`} snapPoints={[0.9]}>
      {preview?.days.length === 0 && (
        <Text color="textMuted">No days in this week. Does the program declare it?</Text>
      )}
      {preview?.days.map((d) => (
        <Card key={`${d.week}-${d.day}`} style={{ gap: space[2] }}>
          <Text variant="caption">{`Day ${d.day}, ${d.kind}`}</Text>
          <Text variant="heading">{d.name}</Text>
          {d.exercises.map((e, i) => (
            <View key={i}>
              <Text>{e.name}</Text>
              <Text variant="caption">{setsSummary(e.sets, unit)}</Text>
            </View>
          ))}
        </Card>
      ))}
      {!!preview?.output.length && (
        <Card>
          <Text variant="label">Output</Text>
          {preview.output.map((line, i) => (
            <Text key={i} variant="mono">
              {line}
            </Text>
          ))}
        </Card>
      )}
    </Sheet>
  );
}
