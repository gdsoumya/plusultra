import { useState } from "react";
import { View } from "react-native";

import { countedPerUnit, itemMacros } from "../../domain/totals";
import type { Item, Macros } from "../../domain/types";
import { Button, ErrorLine, Field, Sheet, space, Text } from "../../ui";
import { macroLine, parseQty } from "./format";
import { MacroFields, macroText, readMacros, type MacroText } from "./MacroFields";

type Props = {
  item: Item;
  // The plan's quantity, when the item came from the plan.
  plannedQty?: number;
  onSave(qty: number): void;
  // The item's own macros, per unit or per 100 g as it is counted (P2-R44).
  onSaveMacros(per: Macros): void;
  onRemove(): void;
  onClose(): void;
};

// Edit one logged item: its quantity (macros follow), its macros, not eaten or back to the plan for
// a planned one, or remove.
export function ItemSheet({ item, plannedQty, onSave, onSaveMacros, onRemove, onClose }: Props) {
  const [text, setText] = useState(String(item.qty));
  const [macros, setMacros] = useState<MacroText>();
  const [problem, setProblem] = useState<string>();
  const qty = parseQty(text);
  const valid = qty !== undefined;
  const perUnit = countedPerUnit(item);
  if (macros) {
    const save = () => {
      try {
        onSaveMacros(readMacros(macros));
      } catch (e) {
        setProblem(e instanceof Error ? e.message : String(e));
      }
    };
    return (
      <Sheet open title={item.foodName} onClose={onClose} snapPoints={[0.9]} testID="item-sheet">
        <Text variant="caption">{perUnit ? `Per ${item.foodUnit}` : "Per 100 g"}</Text>
        <MacroFields value={macros} onChange={setMacros} />
        <ErrorLine message={problem} />
        <Button title="Save macros" onPress={save} />
        <Button title="Cancel" variant="ghost" onPress={() => setMacros(undefined)} />
      </Sheet>
    );
  }
  return (
    <Sheet open title={item.foodName} onClose={onClose} snapPoints={[0.6]} testID="item-sheet">
      <Field
        label={`Quantity (${item.unit})`}
        numeric
        value={text}
        onChangeText={setText}
        error={valid ? undefined : "Enter a number"}
        helper={valid ? macroLine(itemMacros(item, qty)) : undefined}
        autoFocus
        selectTextOnFocus
      />
      {item.note ? <Text variant="caption">{item.note}</Text> : null}
      <Button title="Save" disabled={!valid} onPress={() => valid && onSave(qty)} />
      <Button
        title="Edit macros"
        variant="secondary"
        icon="create-outline"
        onPress={() => setMacros(macroText(perUnit ? item.perUnit : item.per100g))}
      />
      <View style={{ flexDirection: "row", gap: space[2] }}>
        {plannedQty !== undefined && plannedQty !== item.qty && (
          <Button
            title="As planned"
            variant="secondary"
            style={{ flex: 1 }}
            onPress={() => onSave(plannedQty)}
          />
        )}
        {item.planned && item.qty !== 0 && (
          <Button
            title="Not eaten"
            variant="secondary"
            style={{ flex: 1 }}
            onPress={() => onSave(0)}
          />
        )}
        <Button title="Remove" variant="danger" style={{ flex: 1 }} onPress={onRemove} />
      </View>
    </Sheet>
  );
}
