import { create } from "@bufbuild/protobuf";
import { useState } from "react";

import { ExerciseSchema, type Exercise } from "../../gen/plusultra/v1/exercise_pb";
import { FoodSchema, type Food } from "../../gen/plusultra/v1/food_pb";
import { useApi } from "../../api/useApi";
import { muscleGroups, muscleRegions } from "../../domain/muscles";
import { View } from "react-native";

import { Button, Chip, ErrorLine, Field, Sheet, space, Text } from "../../ui";
import { BarcodeScanner } from "../meals/BarcodeScanner";
import { mealFlags } from "../meals/flags";
import { MacroFields, macroText, readMacros, type MacroText } from "../meals/MacroFields";

type Props<T> = {
  open: boolean;
  busy: boolean;
  error?: string;
  onCreate: (item: T) => void;
  onClose: () => void;
};

const capitalize = (s: string) => s.charAt(0).toUpperCase() + s.slice(1);

function MusclePicker({
  label,
  value,
  onChange,
  testID,
}: {
  label: string;
  value: string[];
  onChange: (next: string[]) => void;
  testID: string;
}) {
  const [query, setQuery] = useState("");
  const toggle = (m: string) =>
    onChange(value.includes(m) ? value.filter((x) => x !== m) : [...value, m]);
  const q = query.trim().toLowerCase();
  // Selected names stay visible whatever the search says, so a pick is never lost from view.
  const shown = (m: string) => value.includes(m) || !q || m.includes(q);
  return (
    <View style={{ gap: space[2] }} testID={testID}>
      <Field
        label={label}
        placeholder="Search muscles"
        autoCapitalize="none"
        value={query}
        onChangeText={setQuery}
      />
      {muscleGroups.map((g) => {
        const names = [g, ...muscleRegions[g]].filter(shown);
        if (!names.length) return null;
        return (
          <View key={g} style={{ gap: space[1] }}>
            <Text variant="caption">{capitalize(g)}</Text>
            <View style={{ flexDirection: "row", flexWrap: "wrap", gap: space[2] }}>
              {names.map((m) => (
                <Chip key={m} label={m} selected={value.includes(m)} onPress={() => toggle(m)} />
              ))}
            </View>
          </View>
        );
      })}
    </View>
  );
}

export function CustomExerciseSheet({ open, busy, error, onCreate, onClose }: Props<Exercise>) {
  const [name, setName] = useState("");
  const [equipment, setEquipment] = useState("");
  const [primary, setPrimary] = useState<string[]>([]);
  const [secondary, setSecondary] = useState<string[]>([]);
  const [instructions, setInstructions] = useState("");
  const [problem, setProblem] = useState<string>();
  const submit = () => {
    if (!name.trim()) return setProblem("Give it a name");
    if (primary.length === 0) return setProblem("Pick at least one primary muscle");
    onCreate(
      create(ExerciseSchema, {
        name: name.trim(),
        equipment: equipment.trim().toLowerCase(),
        primaryMuscles: primary,
        secondaryMuscles: secondary.filter((m) => !primary.includes(m)),
        instructions: instructions
          .split("\n")
          .map((line) => line.trim())
          .filter(Boolean),
      }),
    );
  };
  return (
    <Sheet
      open={open}
      onClose={onClose}
      title="New exercise"
      snapPoints={[0.75]}
      footer={<Button title="Create" onPress={submit} loading={busy} />}
    >
      <Field
        label="Name"
        helper="As scripts will name it, for example Zercher Squat"
        value={name}
        onChangeText={setName}
      />
      <Field
        label="Equipment"
        helper="barbell, dumbbell, cable, bodyweight..."
        autoCapitalize="none"
        value={equipment}
        onChangeText={setEquipment}
      />
      <MusclePicker
        label="Primary muscles"
        value={primary}
        onChange={setPrimary}
        testID="primary-muscles"
      />
      <MusclePicker
        label="Secondary muscles"
        value={secondary}
        onChange={setSecondary}
        testID="secondary-muscles"
      />
      <Field
        label="Instructions"
        helper="One step per line; optional"
        multiline
        numberOfLines={4}
        value={instructions}
        onChangeText={setInstructions}
      />
      <ErrorLine message={problem ?? error} />
    </Sheet>
  );
}

export function CustomFoodSheet({ open, busy, error, onCreate, onClose }: Props<Food>) {
  const api = useApi();
  const [name, setName] = useState("");
  const [macros, setMacros] = useState<MacroText>({});
  const [barcode, setBarcode] = useState("");
  const [scanning, setScanning] = useState(false);
  const [note, setNote] = useState<string>();
  const [problem, setProblem] = useState<string>();
  // A known barcode prefills the form from the catalog; an unknown one is kept on the new food so
  // the next scan finds it.
  const scanned = (code: string) => {
    setScanning(false);
    setBarcode(code);
    api.foods.lookupBarcode({ barcode: code }).then(
      ({ food }) => {
        if (!food) return;
        setName(food.name);
        setMacros(macroText(food.per100g));
        setNote(`Found ${food.name}; adjust it and create your copy.`);
      },
      () => setNote(`Barcode ${code} is not in the catalog; fill in the values.`),
    );
  };
  const submit = () => {
    if (!name.trim()) return setProblem("Give it a name");
    try {
      onCreate(create(FoodSchema, { name: name.trim(), per100g: readMacros(macros), barcode }));
    } catch (e) {
      setProblem(e instanceof Error ? e.message : String(e));
    }
  };
  return (
    <Sheet
      open={open}
      onClose={onClose}
      title="New food"
      snapPoints={[0.9]}
      footer={<Button title="Create" onPress={submit} loading={busy} />}
    >
      {mealFlags.scanBarcodes && (
        <Button
          title="Scan barcode"
          icon="barcode-outline"
          variant="secondary"
          onPress={() => setScanning(true)}
        />
      )}
      {note && <Text variant="caption">{note}</Text>}
      <Field label="Name" helper="Per 100 g below" value={name} onChangeText={setName} />
      <MacroFields value={macros} onChange={setMacros} />
      <ErrorLine message={problem ?? error} />
      {scanning && <BarcodeScanner onScanned={scanned} onClose={() => setScanning(false)} />}
    </Sheet>
  );
}
