import { ListCard, ListRow, Sheet } from "../../ui";

type Props = {
  open: boolean;
  onClose(): void;
  title: string;
  adHoc: string;
  onAdHoc(): void;
  onProgram(): void;
};

// The two ways to add a workout or meals to a date (P2-R35).
export function AddSheet({ open, onClose, title, adHoc, onAdHoc, onProgram }: Props) {
  return (
    <Sheet open={open} onClose={onClose} title={title} snapPoints={[0.3]}>
      <ListCard>
        <ListRow icon="add" title={adHoc} onPress={onAdHoc} />
        <ListRow icon="list-outline" title="From a program" onPress={onProgram} />
      </ListCard>
    </Sheet>
  );
}
