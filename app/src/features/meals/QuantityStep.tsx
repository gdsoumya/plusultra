import { useState } from "react";
import { View } from "react-native";

import type { Store } from "../../data/store";
import { itemMacros, withQty } from "../../domain/totals";
import type { Item } from "../../domain/types";
import type { Food } from "../../gen/plusultra/v1/food_pb";
import { Button, Chip, ChipStrip, Field, space, Text } from "../../ui";
import { isFavorite, itemFromFood, toggleFavorite, unitsOf } from "./foods";
import { macroLine, parseQty } from "./format";

type Props = {
  item: Item;
  // The catalog food the item came from; it offers the food's units. A saved item keeps its unit.
  food?: Food;
  store: Store;
  onAdd(item: Item): void;
  onBack(): void;
};

// Quantity and unit for a chosen food, with a favorite toggle.
export function QuantityStep({ item, food, store, onAdd, onBack }: Props) {
  const [chosen, setChosen] = useState(item);
  const [text, setText] = useState(String(item.qty));
  const [favorite, setFavorite] = useState(isFavorite(store, item));
  const qty = parseQty(text);
  const valid = qty !== undefined;
  const units = food ? unitsOf(food) : [];

  const pick = (unit: string) => {
    setChosen(itemFromFood(food!, unit === "g" ? 100 : 1, unit));
    setText(unit === "g" ? "100" : "1");
  };

  return (
    <View style={{ gap: space[3] }}>
      <Text variant="heading">{item.foodName}</Text>
      {units.length > 1 && (
        <ChipStrip selected={units.indexOf(chosen.unit)}>
          {units.map((u) => (
            <Chip key={u} label={u} selected={u === chosen.unit} onPress={() => pick(u)} />
          ))}
        </ChipStrip>
      )}
      <Field
        label={`Quantity (${chosen.unit})`}
        numeric
        value={text}
        onChangeText={setText}
        error={valid ? undefined : "Enter a number"}
        helper={valid ? macroLine(itemMacros(chosen, qty)) : undefined}
        selectTextOnFocus
      />
      <Chip
        label={favorite ? "Favorite" : "Add to favorites"}
        icon={favorite ? "star" : "star-outline"}
        selected={favorite}
        onPress={() => {
          toggleFavorite(store, item);
          setFavorite(!favorite);
        }}
      />
      <Button title="Add" disabled={!valid} onPress={() => valid && onAdd(withQty(chosen, qty))} />
      <Button title="Back" variant="ghost" onPress={onBack} />
    </View>
  );
}
