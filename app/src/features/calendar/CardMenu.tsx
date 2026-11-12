import { useState } from "react";

import { IconButton, ListCard, ListRow, Sheet, type ColorRole, type IconName } from "../../ui";

export type MenuItem = { title: string; icon: IconName; tone?: ColorRole; onPress(): void };

// A card's overflow menu: an ellipsis button opening a sheet of rows. Renders nothing without items.
export function CardMenu({ title, items }: { title: string; items: MenuItem[] }) {
  const [open, setOpen] = useState(false);
  if (items.length === 0) return null;
  return (
    <>
      <IconButton
        icon="ellipsis-horizontal"
        label={`${title} options`}
        onPress={() => setOpen(true)}
      />
      <Sheet open={open} onClose={() => setOpen(false)} title={title} snapPoints={[0.4]}>
        <ListCard>
          {items.map((item) => (
            <ListRow
              key={item.title}
              icon={item.icon}
              title={item.title}
              tone={item.tone}
              onPress={() => {
                setOpen(false);
                item.onPress();
              }}
            />
          ))}
        </ListCard>
      </Sheet>
    </>
  );
}

export const resetItem = (onPress: () => void): MenuItem => ({
  title: "Reset day",
  icon: "refresh-outline",
  tone: "danger",
  onPress,
});
