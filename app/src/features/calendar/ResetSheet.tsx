import { ConfirmSheet } from "../../ui";

export type Content = "workout" | "meals";

const messages: Record<Content, string> = {
  workout: "Removes this day's workout. The day's plan stays and can be started again.",
  meals: "Removes this day's meal log. The day's planned meals stay.",
};
// A finalized record's progression ran on the server and is kept (R75).
const finalizedMessage =
  "Removes this day's finished workout. The program's progression it already applied stays.";

// Confirms removing a date's workout record or meal log (P2-R37).
export function ResetSheet({
  what,
  finalized,
  onConfirm,
  onClose,
}: {
  what?: Content;
  finalized?: boolean;
  onConfirm(): void;
  onClose(): void;
}) {
  return (
    <ConfirmSheet
      open={!!what}
      title="Reset day"
      message={what === "workout" && finalized ? finalizedMessage : messages[what ?? "workout"]}
      action="Reset"
      onConfirm={onConfirm}
      onClose={onClose}
    />
  );
}
