import { useRouter } from "expo-router";
import { View } from "react-native";

import { useExerciseInfo } from "../../data/exerciseInfo";
import type { ExerciseEntry } from "../../domain/types";
import { ListCard, ListRow, MoveRows, Sheet, space, Text, Thumbnail } from "../../ui";
import { exerciseHref, ExerciseThumb, infoLine } from "../exercises/ExerciseRow";
import { UnilateralRow } from "./UnilateralRow";

type Props = {
  open: boolean;
  entry: ExerciseEntry;
  index: number;
  count: number;
  /** A finished workout's exercise: what it is and its details, nothing to change. */
  onSwitch: (name: string) => void;
  onMove: (to: number) => void;
  onRemove: () => void;
  onUnilateral: (unilateral: boolean) => void;
  onClose: () => void;
};

// The exercise menu: what it works, its details, switch between the alternates, reorder without
// dragging, drop the exercise from today, or make it unilateral.
export function ExerciseSheet({
  open,
  entry,
  index,
  count,
  onSwitch,
  onMove,
  onRemove,
  onUnilateral,
  onClose,
}: Props) {
  const router = useRouter();
  const { info } = useExerciseInfo({ slug: entry.slug, name: entry.name });
  return (
    <Sheet open={open} onClose={onClose} title={entry.name} snapPoints={[0.6]}>
      <View style={{ flexDirection: "row", alignItems: "center", gap: space[3] }}>
        <Thumbnail uri={info?.images[0]} />
        <Text color="textMuted" numberOfLines={2} style={{ flex: 1 }}>
          {infoLine(info) ?? entry.equipment}
        </Text>
      </View>
      {entry.alternates.length > 1 && (
        <>
          <Text variant="label" color="textMuted">
            Switch exercise
          </Text>
          <ListCard>
            {entry.alternates.map((name) => (
              <ListRow
                key={name}
                leading={<ExerciseThumb exercise={{ name }} />}
                title={name}
                value={name === entry.name ? "Current" : undefined}
                onPress={() => {
                  if (name !== entry.name) onSwitch(name);
                  onClose();
                }}
              />
            ))}
          </ListCard>
        </>
      )}
      <ListCard>
        {info && (
          <ListRow
            icon="information-circle-outline"
            title="Details"
            subtitle="Muscles, equipment and images"
            onPress={() => {
              onClose();
              router.push(exerciseHref(info.slug));
            }}
          />
        )}
        <MoveRows
          index={index}
          count={count}
          onMove={(to) => {
            onMove(to);
            onClose();
          }}
        />
        <ListRow icon="trash-outline" title="Remove from today" onPress={onRemove} />
      </ListCard>
      <UnilateralRow
        slug={entry.slug || info?.slug}
        unilateral={!!entry.unilateral}
        onChange={onUnilateral}
      />
    </Sheet>
  );
}
