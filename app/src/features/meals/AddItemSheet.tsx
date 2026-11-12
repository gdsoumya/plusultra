import { useEffect, useState } from "react";
import { View } from "react-native";

import type { Api } from "../../api/clients";
import { errorMessage } from "../../api/errors";
import type { Store } from "../../data/store";
import type { Item } from "../../domain/types";
import type { Food } from "../../gen/plusultra/v1/food_pb";
import { Button, ErrorLine, Field, ListRow, Sheet, space, Text } from "../../ui";
import { BarcodeScanner } from "./BarcodeScanner";
import { CustomItemStep } from "./CustomItemStep";
import { mealFlags } from "./flags";
import { favoriteFoods, itemFromFood, recentFoods, rememberFood, unitsOf } from "./foods";
import { macroLine } from "./format";
import { QuantityStep } from "./QuantityStep";

type Props = {
  meal: string;
  store: Store;
  // Unset while offline: only recent and favorite foods and custom items are offered.
  api?: Api;
  // notice says why a custom item was not saved to the user's foods.
  onAdd(item: Item, notice?: string): void;
  onClose(): void;
};

const message = (what: string, e: unknown) => `${what} failed: ${errorMessage(e)}`;

// Find a food by search, recents, favorites or barcode, then choose its quantity; or type one in.
export function AddItemSheet({ meal, store, api, onAdd, onClose }: Props) {
  const [query, setQuery] = useState("");
  const [results, setResults] = useState<Food[]>([]);
  const [error, setError] = useState<string>();
  const [picked, setPicked] = useState<{ item: Item; food?: Food }>();
  const [scanning, setScanning] = useState(false);
  const [custom, setCustom] = useState(false);

  useEffect(() => {
    const q = query.trim();
    if (!api || !q) return;
    let stale = false;
    const timer = setTimeout(() => {
      api.foods.searchFoods({ query: q, limit: 20 }).then(
        ({ foods }) => !stale && setResults(foods),
        (e: unknown) => !stale && setError(message("Search", e)),
      );
    }, 250);
    return () => {
      stale = true;
      clearTimeout(timer);
    };
  }, [api, query]);

  // A food counted in units proposes one of its first unit; anything else proposes 100 g.
  const pickFood = (food: Food) => {
    const unit = unitsOf(food)[1] ?? "g";
    setPicked({ item: itemFromFood(food, unit === "g" ? 100 : 1, unit), food });
  };

  const lookup = (barcode: string) =>
    api?.foods.lookupBarcode({ barcode }).then(
      ({ food }) => {
        setScanning(false);
        if (food) pickFood(food);
      },
      (e: unknown) => {
        setScanning(false);
        setError(message("Lookup", e));
      },
    );

  const add = (item: Item, notice?: string) => {
    rememberFood(store, item);
    onAdd(item, notice);
  };
  const q = query.trim();

  const saved = (title: string, items: Item[]) =>
    items.length > 0 && (
      <View>
        <Text variant="label" color="textMuted">
          {title}
        </Text>
        {items.map((item) => (
          <ListRow
            key={`${title}-${item.foodRef ?? item.foodName}`}
            title={item.foodName}
            subtitle={macroLine(item.macros)}
            value={`${item.qty} ${item.unit}`}
            onPress={() => setPicked({ item })}
          />
        ))}
      </View>
    );

  return (
    <Sheet open title={`Add to ${meal}`} onClose={onClose} snapPoints={[0.9]} testID="add-sheet">
      {picked ? (
        <QuantityStep {...picked} store={store} onBack={() => setPicked(undefined)} onAdd={add} />
      ) : custom ? (
        <CustomItemStep api={api} onAdd={add} onBack={() => setCustom(false)} />
      ) : (
        <>
          <View style={{ flexDirection: "row", gap: space[2], alignItems: "flex-end" }}>
            <View style={{ flex: 1 }}>
              <Field
                label="Search foods"
                value={query}
                onChangeText={(text) => {
                  setError(undefined);
                  setQuery(text);
                }}
                placeholder={api ? "Name or alias" : "Offline"}
                editable={!!api}
                autoFocus={!!api}
              />
            </View>
            {mealFlags.scanBarcodes && api && (
              <Button
                title="Scan"
                icon="barcode-outline"
                variant="secondary"
                onPress={() => setScanning(true)}
              />
            )}
          </View>
          {!api && (
            <Text variant="caption">Offline: recent and favorite foods, or a custom item.</Text>
          )}
          <Button
            title="Custom item"
            icon="create-outline"
            variant="secondary"
            onPress={() => setCustom(true)}
          />
          <ErrorLine message={error} />
          {q &&
            results.map((food) => (
              <ListRow
                key={food.id}
                title={food.name}
                subtitle={food.per100g ? `${macroLine(food.per100g)} per 100 g` : undefined}
                onPress={() => pickFood(food)}
              />
            ))}
          {!q && saved("Favorites", favoriteFoods(store))}
          {!q && saved("Recent", recentFoods(store))}
        </>
      )}
      {scanning && <BarcodeScanner onScanned={lookup} onClose={() => setScanning(false)} />}
    </Sheet>
  );
}
