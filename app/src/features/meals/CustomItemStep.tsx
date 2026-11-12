import { create } from "@bufbuild/protobuf";
import { useState } from "react";
import { View } from "react-native";

import type { Api } from "../../api/clients";
import { errorMessage } from "../../api/errors";
import { scaleMacros, withQty } from "../../domain/totals";
import type { Item } from "../../domain/types";
import { FoodSchema } from "../../gen/plusultra/v1/food_pb";
import { Button, Chip, ChipStrip, ErrorLine, Field, space } from "../../ui";
import { parseQty } from "./format";
import { MacroFields, readMacros, type MacroText } from "./MacroFields";

type Props = {
  // Unset while offline, when the item cannot be saved to the user's foods.
  api?: Api;
  // notice says why a requested save to the user's foods did not happen.
  onAdd(item: Item, notice?: string): void;
  onBack(): void;
};

// A food typed in by hand, per 100 g or per one named unit, added to the meal as it is so it works
// offline; "Save to my foods" also makes it a custom food in the catalog (P2-R43).
export function CustomItemStep({ api, onAdd, onBack }: Props) {
  const [name, setName] = useState("");
  const [perUnit, setPerUnit] = useState(false);
  const [unit, setUnit] = useState("");
  const [grams, setGrams] = useState("");
  const [macros, setMacros] = useState<MacroText>({});
  const [qtyText, setQtyText] = useState("100");
  const [save, setSave] = useState(false);
  const [problem, setProblem] = useState<string>();
  const [busy, setBusy] = useState(false);
  const unitName = perUnit ? unit.trim() : "g";

  const basis = (unitBased: boolean) => {
    setPerUnit(unitBased);
    setQtyText(unitBased ? "1" : "100");
  };

  async function submit() {
    const foodName = name.trim();
    const qty = parseQty(qtyText);
    const weight = grams.trim() ? parseQty(grams) : 100;
    if (!foodName) return setProblem("Give it a name");
    if (perUnit && (!unitName || unitName === "g")) return setProblem("Name the unit, like plate");
    if (!weight) return setProblem("Grams per unit must be a number above 0");
    if (qty === undefined) return setProblem("Enter a quantity");
    let per: Item["macros"];
    try {
      per = readMacros(macros);
    } catch (e) {
      return setProblem(e instanceof Error ? e.message : String(e));
    }
    // A unit with no weight given counts as 100 g, so the catalog food has macros per 100 g.
    const per100g = perUnit ? scaleMacros(per, 100 / weight) : per;
    let foodRef: string | undefined;
    let notice: string | undefined;
    if (save && api) {
      setBusy(true);
      try {
        const units = perUnit ? { [unitName]: weight } : {};
        const { food } = await api.foods.createCustomFood({
          food: create(FoodSchema, { name: foodName, per100g, units }),
        });
        foodRef = food?.id;
      } catch (e) {
        notice = `${foodName} is in the meal but not in your foods: ${errorMessage(e)}`;
      }
      setBusy(false);
    }
    const item: Item = {
      foodName,
      foodRef,
      per100g,
      perUnit: perUnit ? per : undefined,
      foodUnit: perUnit ? unitName : undefined,
      qty,
      unit: unitName,
      macros: per,
      planned: false,
    };
    onAdd(withQty(item, qty), notice);
  }

  return (
    <View style={{ gap: space[3] }}>
      <Field label="Name" value={name} onChangeText={setName} />
      <ChipStrip selected={perUnit ? 1 : 0}>
        <Chip label="Per 100 g" selected={!perUnit} onPress={() => basis(false)} />
        <Chip label="Per unit" selected={perUnit} onPress={() => basis(true)} />
      </ChipStrip>
      {perUnit && (
        <>
          <Field
            label="Unit"
            helper="For example plate or serving"
            value={unit}
            onChangeText={setUnit}
          />
          <Field
            label="Grams per unit"
            helper="Optional; 100 g when empty"
            numeric
            value={grams}
            onChangeText={setGrams}
          />
        </>
      )}
      <MacroFields value={macros} onChange={setMacros} />
      <Field
        label={`Quantity (${unitName || "unit"})`}
        numeric
        value={qtyText}
        onChangeText={setQtyText}
        selectTextOnFocus
      />
      {api && (
        <Chip
          label="Save to my foods"
          icon={save ? "bookmark" : "bookmark-outline"}
          selected={save}
          onPress={() => setSave(!save)}
        />
      )}
      <ErrorLine message={problem} />
      <Button title="Add" loading={busy} onPress={() => void submit()} />
      <Button title="Back" variant="ghost" onPress={onBack} />
    </View>
  );
}
