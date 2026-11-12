import type { Meal } from "../../domain/types";
import { ListCard, ListRow, MoveRows, Sheet } from "../../ui";

type Props = {
  open: boolean;
  meal: Meal;
  index: number;
  count: number;
  onMove: (to: number) => void;
  onDelete: () => void;
  onClose: () => void;
};

// The meal menu: reorder without dragging, or drop the meal from the day (a skipped meal).
export function MealSheet({ open, meal, index, count, onMove, onDelete, onClose }: Props) {
  return (
    <Sheet open={open} onClose={onClose} title={meal.name} snapPoints={[0.4]}>
      <ListCard>
        <MoveRows
          index={index}
          count={count}
          onMove={(to) => {
            onMove(to);
            onClose();
          }}
        />
        <ListRow icon="trash-outline" title="Delete meal" onPress={onDelete} />
      </ListCard>
    </Sheet>
  );
}
