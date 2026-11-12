import type { ExerciseEntry } from "../../domain/types";
import { ChipStrip } from "../../ui";
import { ExerciseTile } from "./ExerciseTile";

type Props = {
  entries: ExerciseEntry[];
  shown: number;
  /** A finished workout is only looked at: a long press opens nothing. */
  onShow: (index: number) => void;
  /** A long press on a tile opens the Reorder sheet. */
  onList: () => void;
};

// The session at a glance: one tile per exercise, scrolling sideways through the gutters and
// keeping the shown exercise in view. Add, Reorder and Finish sit in the row under it.
export function ExerciseCarousel({ entries, shown, onShow, onList }: Props) {
  return (
    <ChipStrip selected={shown} testID="exercise-carousel">
      {entries.map((entry, i) => (
        <ExerciseTile
          key={i}
          entry={entry}
          selected={i === shown}
          onPress={() => onShow(i)}
          onLongPress={onList}
          testID={`tile-${i}`}
        />
      ))}
    </ChipStrip>
  );
}
