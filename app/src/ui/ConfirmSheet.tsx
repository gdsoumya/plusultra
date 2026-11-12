import { Button } from "./Button";
import { ErrorLine } from "./ErrorLine";
import { Sheet } from "./Sheet";
import { Text } from "./Text";

export type ConfirmSheetProps = {
  open: boolean;
  title: string;
  message: string;
  /** The destructive button's title, e.g. "Delete program". */
  action: string;
  /** The way out without acting; "Cancel" unless the context has a better word. */
  cancel?: string;
  busy?: boolean;
  error?: string;
  onConfirm: () => void;
  onClose: () => void;
};

export function ConfirmSheet({
  open,
  title,
  message,
  action,
  cancel = "Cancel",
  busy,
  error,
  onConfirm,
  onClose,
}: ConfirmSheetProps) {
  return (
    <Sheet open={open} onClose={onClose} title={title} snapPoints={[0.4]}>
      <Text color="textMuted">{message}</Text>
      <ErrorLine message={error} />
      <Button title={action} variant="danger" onPress={onConfirm} loading={busy} />
      <Button title={cancel} variant="secondary" onPress={onClose} disabled={busy} />
    </Sheet>
  );
}
